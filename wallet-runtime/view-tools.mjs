import {WASI} from 'node:wasi';
import {readFileSync} from 'node:fs';
import {createInterface} from 'node:readline';
const wasi=new WASI({version:'preview1',args:[],env:{},preopens:{},returnOnExit:true});
const module=await WebAssembly.compile(readFileSync(new URL('./view-tools.wasm',import.meta.url)));
const instance=await WebAssembly.instantiate(module,wasi.getImportObject());
wasi.initialize(instance);
const w=instance.exports;
for await(const line of createInterface({input:process.stdin,crlfDelay:Infinity})){
 if(line.length>8*1024*1024){console.log('{"ok":false}');continue;}
 let input=line;
 try {
  const q=JSON.parse(line);
  if(typeof q.payload!=='string'||!/^([0-9a-fA-F]{2})+$/.test(q.payload))throw Error('Malformed bundle');
  const payload=Buffer.from(q.payload,'hex');
  if(payload[0]&8){
   // v1.0.11 appends a 32-byte named anchor AFTER the optional burn field.
   // Adapt only for output decryption by the pinned legacy viewing reactor.
   // The original bundle is never modified for signing, hashing or consensus.
   if(payload.length<113||payload[0]&~15||payload.readUInt32BE(41)!==64)throw Error('Malformed bundle');
   const count=payload.readUInt32BE(109),end=113+count*884;
   if(count<1||count>512||end+4>payload.length)throw Error('Malformed bundle');
   const expected=end+4+payload.readUInt32BE(end)+((payload[0]&4)?40:0)+32;
   if(payload.length!==expected)throw Error('Malformed anchor field');
   const legacy=Buffer.from(payload.subarray(0,-32));legacy[0]&=~8;q.payload=legacy.toString('hex');input=JSON.stringify(q);
  }
 } catch {console.log('{"ok":false,"error":"Malformed key or bundle"}');continue;}
 const bytes=new TextEncoder().encode(input),ptr=w.alloc_input(bytes.length);
 new Uint8Array(w.memory.buffer,ptr,bytes.length).set(bytes);
 const result=w.recover_json(ptr,bytes.length),outPtr=Number(result>>32n),outLen=Number(result&0xffffffffn);
 const text=new TextDecoder().decode(new Uint8Array(w.memory.buffer,outPtr,outLen));
 w.free_result(outPtr,outLen);console.log(text);
}
