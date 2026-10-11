// Public upstream v1.0.47 derivation vectors; never fund these test keys.
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {spawnSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import * as z from '../wallet-runtime/official-signer.mjs';
z.initSync({module:readFileSync(new URL('../wallet-runtime/official-signer.wasm',import.meta.url))});
const phrase='legal winner thank year wave sausage worth useful legal winner thank yellow';
assert.equal(z.address_from_seed('00'.repeat(32),'mainnet'),'zkas:p8xrvcqetysnk6cvmwt2whqhcwnx32tlp44gch8pvjj3365m4xjsaf63j87cvxc07y8x9vqla2vf9df');
assert.equal(z.fvk_hex('00'.repeat(32)),'8777e215101df45aa468bb10b2f93ffe08a2f79ebff095aaeb7473c771349621bbca152cfbf9811819cc624434d1237577c13805cc3ded444e755a6b78facd168ca7fbba26470fea0b10d30db27366ec65040c72a09ad8425888ef26f1c0793f');
assert.equal(z.account_seed_hex(phrase,0),'f289db28b1611dc2fa1597667e3d572b3de2332451c8955b907847e16464c267');
assert.equal(z.account_seed_hex(phrase,7),'7fff0f7f74275df7c395a8ef90a8ee013f7f095fe3d4c67f6081823769a7034d');
function call(q){const p=spawnSync(process.execPath,[fileURLToPath(new URL('../wallet-runtime/signer.mjs',import.meta.url))],{input:JSON.stringify(q),encoding:'utf8'});return JSON.parse(p.stdout)}
const imported=call({op:'import',secret:phrase,account:7});assert(imported.ok);assert.equal(imported.result.seed,z.account_seed_hex(phrase,7));
for(const bundle of ['','00','0b'+'00'.repeat(200)]){
const r=call({op:'sign_payment',seed:'00'.repeat(32),to:z.address_from_seed('00'.repeat(32),'mainnet'),amount:'1',maxFee:'30000000',bundle,disclosureJson:'[]',spendAuthJson:'[]'});assert.equal(r.ok,false,'must reject malformed or undisclosed payment');
}
console.log('PASS: upstream address/FVK/account vectors, manager import adapter and malformed-payment rejection. No live payment was signed.');
