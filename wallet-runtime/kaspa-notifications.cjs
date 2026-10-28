'use strict';
const {addressText}=require('./kaspa-addresses.cjs');
// Incoming records only: outgoing payments, change, reorgs and internal account
// transfers are not new external receipts. Never infer amounts from balance deltas.
function incomingReceipts(transactions,money){
 const rows=[];
 for(const tx of transactions){
  if(tx.data?.type!=='incoming')continue;
  const value=tx.data?.data?.value, timestamp=Number(tx.unixtimeMsec||0), id=String(tx.id||'');
  if(!/^[0-9a-f]{64}$/i.test(id)||!Number.isSafeInteger(timestamp)||timestamp<=0)continue;
  if(typeof value!=='bigint' || value<=0n)continue;
  rows.push({id,amount:money(value),timestampMS:timestamp,
   status:'Recorded incoming transaction; maturity and confirmation count are not reported.',
   address:[...new Set((tx.data.data.utxoEntries||[]).map(u=>addressText(u.address)).filter(Boolean))].join(', ')});
 }
 return rows;
}
module.exports={incomingReceipts};
