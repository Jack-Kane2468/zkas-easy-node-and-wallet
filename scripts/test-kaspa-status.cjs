'use strict';
// Exercise the adapter's event/status contract without real keys, a node or funds.
const vm=require('vm'), fs=require('fs'),path=require('path'),assert=require('assert/strict');
let line, event, output;
const account={accountId:'test-account',accountName:'Test',receiveAddress:'test-address',kind:'kaspa-keypair-standard'};
class Wallet {
 constructor(){this.isOpen=false;this.isSynced=true;this.rpc={isConnected:false,getUtxosByAddresses:async()=>({entries:[{address:'test-address',amount:123n,outpoint:{transactionId:'test-tx',index:0},blockDaaScore:1n,isCoinbase:false}]})};global.wallet=this;}
 addEventListener(cb){event=cb}
 async disconnect(){this.rpc.isConnected=false;event({type:'disconnect'})}
 async walletClose(){this.isOpen=false}
 async walletEnumerate(){return {walletDescriptors:[{filename:'test-wallet'}]}}
 async walletOpen(){this.isOpen=true;return {accountDescriptors:[account]}}
 async accountsEnumerate(){return {accountDescriptors:[account]}}
 async start(){}
 async connect(){this.rpc.isConnected=true}
 async accountsActivate(){}
}
const sdk={PublicKeyGenerator:{fromXPub:()=>({receiveAddressAsString:(_,i)=>'receive-'+i,changeAddressAsString:(_,i)=>'change-'+i,free(){}})},Wallet,setDefaultStorageFolder(){},sompiToKaspaString:v=>String(v)};
const proc={argv:['node','script','/test'],stdout:{write:s=>output(JSON.parse(s.slice(12)))},stderr:{write(){}},stdin:{},exit(){throw Error('unexpected exit')}};
vm.runInNewContext(fs.readFileSync(path.join(__dirname,'../wallet-runtime/kaspa-wallet.cjs'),'utf8'),{require:n=>n==='./kaspa.cjs'?sdk:n==='readline'?{createInterface:()=>({on:(name,cb)=>{if(name==='line')line=cb}})}:n==='fs'?{mkdirSync(){}}:require(n.startsWith('./')?path.join(__dirname,'../wallet-runtime',n):n),process:proc,console:{},Buffer});
async function q(op){const r=await new Promise(resolve=>{output=resolve;line(JSON.stringify(op))});assert(r.ok,r.error);return r.result}
(async()=>{
 await q({op:'open',filename:'test-wallet',password:'public-test'});
 wallet.rpc.isConnected=true;
 let s=await q({op:'status'});assert.equal(s.synced,true);assert.equal(s.ready,false);assert.match(s.balance,/On-chain balance: 123/);
 event({type:'balance',data:{id:'test-account',balance:{mature:0n,pending:0n,outgoing:0n}}});
 s=await q({op:'status'});assert.equal(s.ready,true);assert.match(s.balance,/Available: 0/);
 event({type:'balance',data:{id:'test-account',balance:null}});s=await q({op:'status'});assert.equal(s.ready,false);
 event({type:'utxo-index-not-enabled'});s=await q({op:'status'});assert.match(s.error,/no UTXO index/);
 event({type:'error'});s=await q({op:'status'});assert.match(s.error,/SDK reported/);
 await q({op:'lock'});s=await q({op:'status'});assert.equal(s.ready,false);assert.equal(s.open,false);
 wallet.rpc.getUtxosByAddresses=async()=>{throw Error('offline')};
 await q({op:'open',filename:'test-wallet',password:'public-test'});wallet.rpc.isConnected=true;
 s=await q({op:'status'});assert.match(s.balance,/Balance unavailable/);assert.equal(s.ready,false);
 await q({op:'lock'});
 Object.assign(account,{kind:'kaspa-bip32-standard',xpubKeys:['public'],derivationIndexes:{receive:50,change:0},receiveAddress:'receive-50',changeAddress:'change-0'});
 let batches=0;
 wallet.rpc.getUtxosByAddresses=async({addresses})=>{batches++;return {entries:addresses.map(address=>({address,amount:9007199254740993n,outpoint:{transactionId:'public',index:0},blockDaaScore:1n,isCoinbase:false}))}};
 await q({op:'open',filename:'test-wallet',password:'public-test'});wallet.rpc.isConnected=true;
 s=await q({op:'status'});assert.match(s.balance,new RegExp(String(52n*9007199254740993n)));assert.equal(batches,2);assert.equal(s.ready,false);
 await q({op:'status'});assert.equal(batches,2); // exact cached total, no extra RPC
 await q({op:'lock'});s=await q({op:'status'});assert(!s.balance.includes(String(52n*9007199254740993n)));
 // Each logical session owns its own review token, even for the same account.
 let sends=0;
 Wallet.prototype.accountsActivate=async()=>event({type:'balance',data:{id:'test-account',balance:{mature:100n,pending:0n,outgoing:0n}}});
 Wallet.prototype.accountsEstimate=async()=>({generatorSummary:{fees:1n,transactions:1}});
 Wallet.prototype.accountsSend=async()=>{sends++;return {transactionIds:['public-receipt']}};
 sdk.kaspaToSompi=v=>BigInt(v);sdk.Address=class{constructor(){this.prefix='kaspa'}free(){}};
 await q({session:'desktop',op:'open',filename:'test-wallet',password:'public-test'});
 await q({session:'desktop',op:'connect'});
 const desktopReview=await q({session:'desktop',op:'estimate',address:'kaspa:test',amount:'1'});
 await q({session:'telegram',op:'open',filename:'test-wallet',password:'public-test'});
 await q({session:'telegram',op:'connect'});
 const telegramReview=await q({session:'telegram',op:'estimate',address:'kaspa:test',amount:'2'});
 await assert.rejects(q({session:'desktop',op:'send',token:telegramReview.token,password:'public-test'}));assert.equal(sends,0);
 const fresh=await q({session:'desktop',op:'estimate',address:'kaspa:test',amount:'1'});
 await q({session:'telegram',op:'send',token:telegramReview.token,password:'public-test'});assert.equal(sends,1);
 await q({session:'desktop',op:'send',token:fresh.token,password:'public-test'});assert.equal(sends,2);
 await assert.rejects(q({session:'desktop',op:'send',token:fresh.token,password:'public-test'}));assert.equal(sends,2);
 await q({session:'desktop',op:'session_drop'});
 assert.equal((await q({session:'telegram',op:'status'})).open,true);
 assert.equal((await q({session:'desktop',op:'status'})).open,false);
 console.log('PASS: payment tokens are session-bound, survive view switches, cannot cross sessions or replay, and locks are independent.');
 console.log('PASS: missing balance is not zero, real zero is ready, index/SDK errors are visible, lock clears state.');
})().catch(e=>{console.error(e);process.exitCode=1});
