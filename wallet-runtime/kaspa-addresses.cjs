'use strict';
// Indexes come from the official wallet descriptor, never from a local counter.
function addressText(a) {
 if(typeof a==='string')return a;
 if(a?.prefix&&a?.payload)return a.prefix+':'+a.payload;
 return a?.toString?.()||'';
}
function addressPage(sdk,descriptor,offset=0,limit=50){
 if(!Number.isSafeInteger(offset)||offset<0)throw Error('Invalid address page.');
 const kind=String(descriptor.kind);
 if(kind.includes('keypair'))return {total:1,offset:0,rows:[{branch:'Receive',index:0,address:addressText(descriptor.receiveAddress)}]};
 if(!kind.includes('bip32')||descriptor.ecdsa||descriptor.xpubKeys?.length!==1)throw Error('Address indexing is unavailable for this account type.');
 const receive=descriptor.derivationIndexes?.receive,change=descriptor.derivationIndexes?.change;
 if(!Number.isSafeInteger(receive)||!Number.isSafeInteger(change)||receive<0||change<0||receive>2147483647||change>2147483647)throw Error('Wallet did not report valid derivation indexes.');
 const total=receive+change+2;
 offset=Math.min(offset,Math.floor((total-1)/limit)*limit);
 const gen=sdk.PublicKeyGenerator.fromXPub(descriptor.xpubKeys[0]);
 try {
  // Check the generator against the SDK's current addresses before displaying any.
  if(gen.receiveAddressAsString('mainnet',receive)!==addressText(descriptor.receiveAddress)||gen.changeAddressAsString('mainnet',change)!==addressText(descriptor.changeAddress))throw Error('Address derivation does not match this account.');
  const rows=[];
  for(let n=offset;n<Math.min(total,offset+limit);n++){
   const isReceive=n<=receive,index=isReceive?n:n-receive-1;
   rows.push({branch:isReceive?'Receive':'Change',index,address:isReceive?gen.receiveAddressAsString('mainnet',index):gen.changeAddressAsString('mainnet',index)});
  }
  return {total,offset,rows};
 } finally{gen.free();}
}
function groupUtxos(rows,entries){
 const grouped=new Map(rows.map(r=>[r.address,{sum:0n,count:0,utxos:[]}]));
 for(const u of entries){
  const a=u.address;const address=addressText(a);if(a?.free)a.free();
  const g=grouped.get(address);if(!g)throw Error('Node returned a UTXO for an unrequested address.');
  const amount=BigInt(u.amount);if(amount<0n)throw Error('Invalid UTXO amount.');g.sum+=amount;g.count++;
  if(g.utxos.length<100){const out=u.outpoint;g.utxos.push({txid:String(out.transactionId),index:Number(out.index),amount:amount.toString(),daa:String(u.blockDaaScore),coinbase:!!u.isCoinbase});if(out?.free)out.free();}
 }
 return rows.map(r=>{const g=grouped.get(r.address);return {...r,known:true,amount:g.sum.toString(),count:g.count,utxos:g.utxos};});
}
module.exports={addressText,addressPage,groupUtxos};
