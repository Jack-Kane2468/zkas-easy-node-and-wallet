import {spawnSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const fixtures=JSON.parse(readFileSync(new URL('../view-tools/test-vectors.json',import.meta.url)));
const queries=[];
for(const v of fixtures){const b=Buffer.from(v.payload,'hex');b[0]|=8;const payload=b.toString('hex')+'ab'.repeat(32);queries.push({ovk:v.ovk,payload},{ovk:v.ovk,payload:payload.slice(0,-2)},{ovk:v.ovk,payload:payload+'00'});}
const p=spawnSync(process.execPath,['--no-warnings',fileURLToPath(new URL('../wallet-runtime/view-tools.mjs',import.meta.url))],{input:queries.map(q=>JSON.stringify(q)).join('\n')+'\n',encoding:'utf8',maxBuffer:10<<20});
assert.equal(p.status,0,p.stderr);const out=p.stdout.trim().split('\n').map(JSON.parse);
fixtures.forEach((v,i)=>{assert(out[i*3].ok);assert.equal(out[i*3].rows[0].recipient_raw,v.recipient);assert.equal(out[i*3].rows[0].amount_sompi,v.amount);assert.equal(out[i*3+1].ok,false);assert.equal(out[i*3+2].ok,false)});
console.log('PASS: named-anchor viewing vectors; truncated and extra anchor bytes rejected. Decryption only, not consensus validation.');
