'use strict';
// Official Kaspa Wallet API, private stdin/stdout IPC. Never opens an HTTP server.
const fs = require('fs'), path = require('path'), crypto = require('crypto');
const readline = require('readline');
const write = process.stdout.write.bind(process.stdout);
console.log = console.info = console.warn = () => {};
// Retain safe panic locations/categories without logging wallet input or secrets.
console.error = (...args) => {
 for(const arg of args){
  const text=typeof arg==='string'?arg:(arg?.stack||'');
  const locations=text.match(/[A-Za-z0-9_.-]+\.rs:\d+(?::\d+)?/g)||[];
  const categories=['BorrowMutError','already borrowed','assertion failed','called `Option::unwrap()`','called `Result::unwrap()`'].filter(x=>text.includes(x));
  if(locations.length||categories.length)process.stderr.write('SDK_PANIC '+[...categories,...locations].slice(0,6).join(' ')+'\n');
 }
};
const phase=name=>process.stderr.write('WALLET_PHASE '+name+'\n');
const sdk = require('./kaspa.cjs');
const {addressText,addressPage,groupUtxos}=require('./kaspa-addresses.cjs');
const {incomingReceipts}=require('./kaspa-notifications.cjs');
const folder = process.argv[2];
const port=Number(process.argv[3]||17110);if(!Number.isInteger(port)||port<1024||port>65535)process.exit(2);
const localURL='ws://127.0.0.1:'+port;
if (!folder || !path.isAbsolute(folder)) process.exit(2);
fs.mkdirSync(folder, { recursive:true, mode:0o700 });
sdk.setDefaultStorageFolder(folder);
const wallet = new sdk.Wallet({resident:false, networkId:'mainnet', url:localURL, encoding:'borsh'});
const balances = new Map();
let selected = '', filename = '', pending = null, lastError = '';
let addressBalance=null;
wallet.addEventListener(({type,data}) => {
 if(type==='balance') { if(data.balance == null) balances.delete(String(data.id)); else balances.set(String(data.id), data.balance); }
 if(type==='wallet-error' || type==='error') lastError = 'The wallet SDK reported a synchronization error. Reconnect the wallet.';
 if(type==='utxo-index-not-enabled') lastError = 'The connected Kaspa node has no UTXO index. Enable Wallet / application backend in Settings > Kaspa and restart the node.';
 if(type==='disconnect') { balances.clear(); pending=null; addressBalance=null; }
});
const money = v => sdk.sompiToKaspaString(v ?? 0n);
function accounts(list) { return list.map(a=>({id:String(a.accountId),name:a.accountName||'Account',address:(a.receiveAddress?.prefix ? a.receiveAddress.prefix+':'+a.receiveAddress.payload : String(a.receiveAddress||'')),kind:String(a.kind),index:a.derivationIndexes?.receive??0,accountIndex:Number(a.accountIndex??0)})); }
// Read the same UTXOs as the address table. This is an on-chain total;
// pending, maturity, and signing readiness remain owned by the SDK.
async function readAddressBalance(descriptor, force=false) {
 if(!wallet.rpc.isConnected || !wallet.isSynced || !descriptor) return null;
 if(!force && addressBalance?.id===selected && Date.now()-addressBalance.at<5000) return addressBalance.amount;
 try {
  let sum=0n,offset=0,total=0;
  do {
   const page=addressPage(sdk,descriptor,offset);total=page.total;
   if(total>10000) throw Error('Account has too many addresses for a quick total.');
   const result=await wallet.rpc.getUtxosByAddresses({addresses:page.rows.map(r=>r.address)});
   try {for(const row of groupUtxos(page.rows,result.entries))sum+=BigInt(row.amount);}
   finally {for(const entry of result.entries)if(entry.free)entry.free();}
   offset+=page.rows.length;
  } while(offset<total);
  addressBalance={id:selected,at:Date.now(),amount:sum};return sum;
 } catch(e) {addressBalance=null;return null;}
}
async function state() {
 const descriptors=wallet.isOpen?(await wallet.accountsEnumerate({})).accountDescriptors:[];
 const list=accounts(descriptors);
 const b=balances.get(selected);
 let total=null;
 try {if(!b)total=await readAddressBalance(descriptors.find(a=>String(a.accountId)===selected));}
 finally {for(const d of descriptors)if(d.kind?.free)d.kind.free();}
 return {open:wallet.isOpen,filename,selected,accounts:list,connected:wallet.rpc.isConnected,synced:wallet.isSynced,ready:wallet.rpc.isConnected&&wallet.isSynced&&!!b,
  balance:total!==null?`On-chain balance: ${money(total)} KAS (all generated receive and change addresses)`:b?`Available: ${money(b.mature)} KAS | Pending: ${money(b.pending)} KAS | Outgoing: ${money(b.outgoing)} KAS`:(!wallet.rpc.isConnected ? 'Wallet disconnected from local node.' : !wallet.isSynced ? 'Waiting for the connected node to report synced.' : 'Balance unavailable from the local node. Refresh or check its UTXO index.'),error:lastError};
}
async function close() {
 pending=null; selected=''; filename=''; balances.clear(); addressBalance=null; lastError='';
 await wallet.disconnect(); if(wallet.isOpen) await wallet.walletClose({});
}
function requireOpen() { if(!wallet.isOpen || !selected) throw Error('Unlock a wallet and select an account first.'); }
function requireReady() { requireOpen(); if(!wallet.rpc.isConnected || !wallet.isSynced || !balances.has(selected)) throw Error('Connect to your synced local Kaspa node and wait for the wallet balance.'); }
// Processing lives for this subprocess, including across wallet close/open and reconnect.
// The SDK traps if start() is called twice; retain its promise even if startup fails.
let startPromise;
async function connect() {
 requireOpen(); lastError=''; phase('start');
 if (!startPromise) startPromise = wallet.start();
 await startPromise;
 phase('connect');
 if(!wallet.rpc.isConnected) await wallet.connect({url:localURL,strategy:'fallback',timeoutDuration:5000,blockAsyncConnect:true});
 phase('activate'); await wallet.accountsActivate({accountIds:[selected]});
}
async function run(q) {
 switch(q.op) {
 case 'ping': return {ready:true};
 case 'list': return {wallets:(await wallet.walletEnumerate({})).walletDescriptors.map(x=>({filename:x.filename,title:x.title||x.filename}))};
 case 'generate': return {phrase:sdk.Mnemonic.random(24).phrase};
 case 'create': {
  if(typeof q.password!=='string'||q.password.length<10) throw Error('Use a wallet password of at least 10 characters.');
  const secret=String(q.secret||'').trim().replace(/\s+/g,' ');
  const key=/^[0-9a-fA-F]{64}$/.test(secret);
  if(key) { const k=new sdk.PrivateKey(secret); k.free(); } else { const n=new sdk.Mnemonic(secret); n.free(); }
  if(!Number.isInteger(q.index)||q.index<0||q.index>2147483647) throw Error('Invalid account number.');
  if(key && q.index!==0) throw Error('A private key has no account number. Use 0.');
  await close();
  const name=q.filename || ('easy-'+crypto.randomBytes(16).toString('hex'));
  if(!/^easy-[a-f0-9]{32}$/.test(name)) throw Error('Invalid wallet filename.');
  await wallet.walletCreate({walletSecret:q.password,filename:name,title:String(q.title||'Kaspa wallet'),overwriteWalletStorage:false});
  const k=await wallet.prvKeyDataCreate({walletSecret:q.password,kind:key?'secretKey':'mnemonic',...(key?{secretKey:secret}:{mnemonic:secret}),...(q.passphrase?{paymentSecret:q.passphrase}:{})});
  const r=await wallet.accountsCreate({walletSecret:q.password,type:key?'kaspa-keypair-standard':'bip32',accountName:String(q.title||'Account'),accountIndex:q.index,prvKeyDataId:k.prvKeyDataId,...(q.passphrase?{paymentSecret:q.passphrase}:{})});
  filename=name; selected=String(r.accountDescriptor.accountId);
  return state();
 }
 case 'open': {
  phase('open');
  const all=(await wallet.walletEnumerate({})).walletDescriptors;
  if(!all.some(x=>x.filename===q.filename)) throw Error('Choose a saved wallet.');
  await close();
  const r=await wallet.walletOpen({walletSecret:q.password,filename:q.filename,accountDescriptors:true});
  filename=q.filename; selected=String(r.accountDescriptors[0]?.accountId||''); if(q.connect) await connect(); return state();
 }
 case 'select': {
  requireOpen(); const all=(await wallet.accountsEnumerate({})).accountDescriptors;
  if(!all.some(x=>String(x.accountId)===q.account)) throw Error('Unknown account.');
  await wallet.accountsDeactivate({accountIds:[selected]}); selected=q.account; pending=null; addressBalance=null;
  if(wallet.rpc.isConnected) await wallet.accountsActivate({accountIds:[selected]}); return state();
 }
 case 'connect': await connect(); return state();
 case 'status': return state();
 case 'lock': await close(); return state();
 case 'address': requireOpen(); addressBalance=null; await wallet.accountsCreateNewAddress({accountId:selected,addressKind:'receive'}); return state();
 case 'addresses': {
  requireOpen();const descriptors=(await wallet.accountsEnumerate({})).accountDescriptors;
  const descriptor=descriptors.find(a=>String(a.accountId)===selected);if(!descriptor)throw Error('Selected account is unavailable.');
  let page;try{page=addressPage(sdk,descriptor,q.offset??0);}finally{for(const d of descriptors)if(d.kind?.free)d.kind.free();}
  if(wallet.rpc.isConnected&&wallet.isSynced){
   const result=await wallet.rpc.getUtxosByAddresses({addresses:page.rows.map(r=>r.address)});
   try{page.rows=groupUtxos(page.rows,result.entries);}finally{for(const e of result.entries)if(e.free)e.free();}
   page.online=true;page.checkedAt=new Date().toISOString();
  }else{page.online=false;page.rows=page.rows.map(r=>({...r,known:false,utxos:[]}));}
  return page;
 }
 case 'verify_recovery':
 case 'export_key': {
  requireOpen();
  const verifying=q.op==='verify_recovery';if(verifying)q.index=0;
  if(q.account!==selected||!Number.isInteger(q.index)||q.index<0||q.index>2147483647)throw Error('Choose a receiving address from the selected account.');
  const descriptors=(await wallet.accountsEnumerate({})).accountDescriptors;
  let mnemonic,xprv,generator,key,derived;
  try{
   const d=descriptors.find(a=>String(a.accountId)===selected);if(!d)throw Error('Selected account is unavailable.');
   if(verifying){const page=addressPage(sdk,d,0);q.address=page.rows.find(r=>r.branch==='Receive'&&r.index===0)?.address;}
   const kind=String(d.kind),raw=String(q.secret||'').trim().replace(/\s+/g,' ');
   if(kind.includes('keypair')){
    if(q.index!==0||!/^[a-fA-F0-9]{64}$/.test(raw))throw Error('Use this address’s original 64-character private key.');
    if(addressText(d.receiveAddress)!==q.address)throw Error('Selected address does not match the wallet.');
    key=new sdk.PrivateKey(raw);
   }else if(kind==='kaspa-bip32-standard'&&!d.ecdsa){
    if(q.index>d.derivationIndexes.receive)throw Error('This receiving index has not been generated by the wallet.');
    const page=addressPage(sdk,d,Math.floor(q.index/50)*50);
    if(!page.rows.some(r=>r.branch==='Receive'&&r.index===q.index&&r.address===q.address))throw Error('Selected address does not match its receiving index.');
    mnemonic=new sdk.Mnemonic(raw);xprv=new sdk.XPrv(mnemonic.toSeed(q.passphrase||''));
    generator=new sdk.PrivateKeyGenerator(xprv,false,BigInt(d.accountIndex));key=generator.receiveKey(q.index);
   }else{throw Error('Private-key export is unavailable for this account type.');}
   derived=key.toAddress('mainnet');
   if(addressText(derived)!==q.address)throw Error('The recovery secret does not match this receiving address.');
   if(verifying)return {verified:true};
   return {privateKey:key.toString(),address:q.address,index:q.index};
  }catch(e){throw Error('Key export failed: check the recovery phrase/key, account and address. No key was exported.');}
  finally{for(const x of [derived,key,generator,xprv,mnemonic])if(x?.free)x.free();for(const d of descriptors)if(d.kind?.free)d.kind.free();}
 }
 case 'alert_history': {
  requireReady(); const r=await wallet.transactionsDataGet({accountId:selected,networkId:'mainnet',start:0,end:500});
  const rows=incomingReceipts(r.transactions,money);
  return {filename,account:selected,rows};
 }
 case 'history': {
  requireOpen(); const r=await wallet.transactionsDataGet({accountId:selected,networkId:'mainnet',start:0,end:100});
  return {text:r.transactions.map(tx=>{const d=tx.data.data;return `${tx.data.type} | ${money(d.paymentValue??d.value??d.changeValue)} KAS\n${tx.id}`;}).join('\n\n')||'No transactions recorded yet. The node supplies current unspent funds; it cannot reconstruct every old spent transaction.'};
 }
 case 'estimate': {
  requireReady(); pending=null;
  if(!/^\d+(\.\d{1,8})?$/.test(q.amount)) throw Error('Enter a positive KAS amount, with up to 8 decimal places.');
  const amount=sdk.kaspaToSompi(q.amount); if(!amount||amount<=0n) throw Error('Amount must be positive.');
  const address=new sdk.Address(q.address); if(address.prefix!=='kaspa') throw Error('Use a mainnet kaspa: address.'); address.free();
  const args={accountId:selected,destination:[{address:q.address,amount}],priorityFeeSompi:0n};
  const result=await wallet.accountsEstimate(args);
  const token=crypto.randomBytes(24).toString('hex');
  pending={token,args,fees:result.generatorSummary.fees,expires:Date.now()+120000};
  return {token,feeSompi:String(pending.fees),text:`Send ${q.amount} KAS\nTo: ${q.address}\nEstimated network fee: ${money(pending.fees)} KAS\nTransactions: ${result.generatorSummary.transactions}`};
 }
 case 'send': {
  requireReady(); const p=pending; pending=null;
  if(!p||q.token!==p.token||Date.now()>p.expires||p.args.accountId!==selected) throw Error('Review this payment again; its confirmation expired.');
  const check=await wallet.accountsEstimate(p.args);
  if(check.generatorSummary.fees>p.fees) throw Error('Network fee increased. Review a new estimate before sending.');
  const r=await wallet.accountsSend({...p.args,walletSecret:q.password,...(q.passphrase?{paymentSecret:q.passphrase}:{})});
  return {text:'Submitted transaction IDs:\n'+r.transactionIds.join('\n')};
 }
 case 'export': requireOpen(); return {data:(await wallet.walletExport({walletSecret:q.password,includeTransactions:true})).walletData};
 default: throw Error('Unknown wallet operation.');
 }
}
// One SDK storage owner, two independent authenticated views. Requests are
// serialized by the IPC queue. Switching views never changes the other's lock
// state, selected account, password, or pending payment review.
const sessions=new Map();
let activeSession=null;
const closedState=()=>({open:false,filename:'',selected:'',accounts:[],connected:false,synced:false,ready:false,balance:'Wallet locked.',error:''});
function saveSession(s){
 s.filename=filename;s.selected=selected;s.connected=wallet.rpc.isConnected;s.pending=pending;
 if(!filename)s.password='';
}
async function runSession(q){
 if(q.session==null)return run(q); // standalone adapter/tests
 const id=q.session;
 if(id!=='desktop'&&id!=='telegram')throw Error('Invalid wallet session.');
 if(q.op==='session_drop'||q.op==='lock'){
  if(activeSession===id){await close();activeSession=null;}
  const old=sessions.get(id);if(old){old.password='';old.pending=null;}sessions.delete(id);
  return closedState();
 }
 let s=sessions.get(id);
 if(!s){s={filename:'',selected:'',password:'',connected:false,pending:null};sessions.set(id,s);}
 if(q.op==='status'&&!s.filename&&activeSession!==id)return closedState();
 if(activeSession!==id){
  const previous=sessions.get(activeSession);if(previous)saveSession(previous);
  activeSession=null;
  // A new explicit open/create supplies fresh credentials; no need to restore
  // a previous selection first (and no way to inherit the other view's wallet).
  if(q.op==='open'||q.op==='create'){
   // Clear the other view before validating a fresh open/create. A rejected
   // filename or invalid creation input must never inherit its open wallet.
   await close();
  }else{
   if(s.filename){
    if(!wallet.isOpen||filename!==s.filename)await run({op:'open',filename:s.filename,password:s.password});
    if(selected!==s.selected)await run({op:'select',account:s.selected});
    if(s.connected&&!wallet.rpc.isConnected)await connect();
   }else await close();
  }
  activeSession=id;pending=s.pending;
 }
 try{
  const result=await run(q);
  if((q.op==='open'||q.op==='create')&&wallet.isOpen)s.password=q.password;
  return result;
 }finally{if(activeSession===id)saveSession(s);}
}

const lines=readline.createInterface({input:process.stdin,crlfDelay:Infinity});
let queue=Promise.resolve();
lines.on('line', line=>{
 if(line.length>131072) return process.exit(3);
 queue=queue.then(async()=>{
  let q; try { q=JSON.parse(line); const result=await runSession(q); write('KASPA_REPLY '+JSON.stringify({ok:true,result},(_,v)=>typeof v==='bigint'?v.toString():v)+'\n'); }
  catch(e) { const message=String(e?.message||e); write('KASPA_REPLY '+JSON.stringify({ok:false,error:message.slice(0,600)})+'\n'); }
  finally { if(q) {q.password='';q.secret='';q.passphrase='';} }
 });
});
lines.on('close',()=>{queue.finally(()=>process.exit(0));});
