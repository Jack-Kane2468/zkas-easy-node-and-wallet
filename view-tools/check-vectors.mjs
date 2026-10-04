import {spawnSync} from 'node:child_process';
import {readFileSync,existsSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
const fixtures=JSON.parse(readFileSync(new URL('./test-vectors.json',import.meta.url),'utf8'));
const queries=fixtures.flatMap(v=>[{ovk:v.ovk,payload:v.payload},{ovk:'00'.repeat(32),payload:v.payload}]);
queries.push({ovk:fixtures[0].ovk,payload:'abcd'});
let runtime=new URL('../wallet-runtime/view-tools.mjs',import.meta.url);if(!existsSync(runtime))runtime=new URL('../../wallet-runtime/view-tools.mjs',import.meta.url);
const p=spawnSync(process.execPath,['--no-warnings',fileURLToPath(runtime)],{input:queries.map(q=>JSON.stringify(q)).join('\n')+'\n',encoding:'utf8',maxBuffer:10*1024*1024});
if(p.status!==0)throw Error(p.stderr);
const out=p.stdout.trim().split('\n').map(JSON.parse);
for(let i=0;i<fixtures.length;i++){
 const v=fixtures[i],r=out[i*2];if(!r.ok||r.rows.length!==1||r.rows[0].recipient_raw!==v.recipient||r.rows[0].amount_sompi!==v.amount)throw Error('Positive vector failed: '+i+' '+JSON.stringify(r));
 if(!out[i*2+1].ok||out[i*2+1].rows.length)throw Error('Wrong key yielded output');
}
if(out.at(-1).ok)throw Error('Malformed bundle accepted');
console.log(`${fixtures.length} upstream Orchard recovery vectors passed; wrong keys and malformed input rejected.`);
