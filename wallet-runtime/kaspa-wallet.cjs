'use strict';
// Official Kaspa Wallet API, private stdin/stdout IPC. Never opens an HTTP server.
const fs = require('fs'), path = require('path'), crypto = require('crypto');
const readline = require('readline');
const write = process.stdout.write.bind(process.stdout);
console.log = console.info = console.warn = console.error = () => {};
const sdk = require('./kaspa.cjs');
const {addressText,addressPage,groupUtxos}=require('./kaspa-addresses.cjs');
const folder = process.argv[2];
const port=Number(process.argv[3]||17110);if(!Number.isInteger(port)||port<1024||port>65535)process.exit(2);
const localURL='ws://127.0.0.1:'+port;
if (!folder || !path.isAbsolute(folder)) process.exit(2);
fs.mkdirSync(folder, { recursive:true, mode:0o700 });
sdk.setDefaultStorageFolder(folder);
const wallet = new sdk.Wallet({resident:false, networkId:'mainnet', url:localURL, encoding:'borsh'});
const balances = new Map();
let selected = '', filename = '', pending = null, lastError = '';
wallet.addEventListener(({type,data}) => {
 if(type==='balance') balances.set(String(data.id), data.balance);
 if(type==='wallet-error') lastError = 'Wallet synchronization reported an error. Reconnect and check the node.';
 if(type==='disconnect') { balances.clear(); pending=null; }
});
const money = v => sdk.sompiToKaspaString(v ?? 0n);
function accounts(list) { return list.map(a=>({id:String(a.accountId),name:a.accountName||'Account',address:(a.receiveAddress?.prefix ? a.receiveAddress.prefix+':'+a.receiveAddress.payload : String(a.receiveAddress||'')),kind:String(a.kind),index:a.derivationIndexes?.receive??0})); }
async function state() {
 const list = wallet.isOpen ? accounts((await wallet.accountsEnumerate({})).accountDescriptors) : [];
 const b = balances.get(selected);
 return {open:wallet.isOpen,filename,selected,accounts:list,connected:wallet.rpc.isConnected,synced:wallet.isSynced,ready:wallet.rpc.isConnected&&wallet.isSynced&&!!b,
  balance:b?`Available: ${money(b.mature)} KAS | Pending: ${money(b.pending)} KAS | Outgoing: ${money(b.outgoing)} KAS`:'Balance not synchronized yet.',error:lastError};
}
async function close() {
 pending=null; selected=''; filename=''; balances.clear(); lastError='';
 await wallet.disconnect(); if(wallet.isOpen) await wallet.walletClose({});
}
function requireOpen() { if(!wallet.isOpen || !selected) throw Error('Unlock a wallet and select an account first.'); }
function requireReady() { requireOpen(); if(!wallet.rpc.isConnected || !wallet.isSynced || !balances.has(selected)) throw Error('Connect to your synced local Kaspa node and wait for the wallet balance.'); }
async function connect() {
 requireOpen(); await wallet.start();
 if(!wallet.rpc.isConnected) await wallet.connect({url:localURL,strategy:'fallback',timeoutDuration:5000,blockAsyncConnect:true});
 await wallet.accountsActivate({accountIds:[selected]});
}
async function run(q) {
 switch(q.op) {
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
  const all=(await wallet.walletEnumerate({})).walletDescriptors;
  if(!all.some(x=>x.filename===q.filename)) throw Error('Choose a saved wallet.');
  await close();
  const r=await wallet.walletOpen({walletSecret:q.password,filename:q.filename,accountDescriptors:true});
  filename=q.filename; selected=String(r.accountDescriptors[0]?.accountId||''); return state();
 }
 case 'select': {
  requireOpen(); const all=(await wallet.accountsEnumerate({})).accountDescriptors;
  if(!all.some(x=>String(x.accountId)===q.account)) throw Error('Unknown account.');
  await wallet.accountsDeactivate({accountIds:[selected]}); selected=q.account; pending=null;
  if(wallet.rpc.isConnected) await wallet.accountsActivate({accountIds:[selected]}); return state();
 }
 case 'connect': await connect(); return state();
 case 'status': return state();
 case 'lock': await close(); return state();
 case 'address': requireOpen(); await wallet.accountsCreateNewAddress({accountId:selected,addressKind:'receive'}); return state();
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
  return {token,text:`Send ${q.amount} KAS\nTo: ${q.address}\nEstimated network fee: ${money(pending.fees)} KAS\nTransactions: ${result.generatorSummary.transactions}`};
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
const lines=readline.createInterface({input:process.stdin,crlfDelay:Infinity});
let queue=Promise.resolve();
lines.on('line', line=>{
 if(line.length>131072) return process.exit(3);
 queue=queue.then(async()=>{
  let q; try { q=JSON.parse(line); const result=await run(q); write('KASPA_REPLY '+JSON.stringify({ok:true,result},(_,v)=>typeof v==='bigint'?v.toString():v)+'\n'); }
  catch(e) { const message=String(e?.message||e); write('KASPA_REPLY '+JSON.stringify({ok:false,error:message.slice(0,600)})+'\n'); }
  finally { if(q) {q.password='';q.secret='';q.passphrase='';} }
 });
});
lines.on('close',()=>{queue.finally(()=>process.exit(0));});
