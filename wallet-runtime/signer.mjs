// Pinned official ZKas signer. Input/output use private process pipes, never argv.
import fs from 'node:fs';
import * as z from './official-signer.mjs';
let request;
try {
 request=JSON.parse(fs.readFileSync(0,'utf8'));
 z.initSync({module:fs.readFileSync(new URL('./official-signer.wasm',import.meta.url))});
 let result;
 if(request.op==='create') {
  const w=z.new_wallet_mnemonic('mainnet');
  const secret=w.mnemonic;w.free();
  const seed=z.account_seed_hex(secret,0);
  result={secret,seed,address:z.address_from_seed(seed,'mainnet'),fvk:z.fvk_hex(seed)};
 } else if(request.op==='import') {
  const secret=request.secret.trim().toLowerCase().replace(/\s+/g,' ');
  const account=request.account??0;
  if(!Number.isInteger(account)||account<0||account>=2147483648)throw Error('Invalid account index');
  let seed;
  if(/^[0-9a-f]{64}$/.test(secret)){if(account!==0)throw Error('Account index applies to recovery phrases only');seed=secret;}
  else {if(!z.is_valid_mnemonic(secret))throw Error('Recovery phrase is invalid: check words and checksum');seed=z.account_seed_hex(secret,account);}
  result={secret,seed,address:z.address_from_seed(seed,'mainnet'),fvk:z.fvk_hex(seed)};
 } else if(request.op==='address') {
  const a=new z.Address(request.address);
  if(a.prefix!=='zkas'||a.version!=='ShieldedOrchard')throw Error('Use a mainnet shielded ZKas address');a.free();result={valid:true};
 } else if(request.op==='sign_payment') {
  const sigs=z.verify_and_sign_payment(request.seed,'mainnet',request.to,BigInt(request.amount),BigInt(request.maxFee),request.bundle,request.disclosureJson,request.spendAuthJson);
  result={sigs:JSON.parse(sigs)};
 } else {throw Error('Unknown signer operation');}
 process.stdout.write(JSON.stringify({ok:true,result}));
} catch(e) {
 let message=String(e?.message??e).slice(0,400);
 for(const v of [request?.secret,request?.seed])if(v)message=message.replaceAll(v,'[private key]');
 process.stdout.write(JSON.stringify({ok:false,error:message}));process.exitCode=1;
}
