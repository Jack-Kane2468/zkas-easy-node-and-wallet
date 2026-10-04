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
 const bytes=new TextEncoder().encode(line),ptr=w.alloc_input(bytes.length);
 new Uint8Array(w.memory.buffer,ptr,bytes.length).set(bytes);
 const result=w.recover_json(ptr,bytes.length),outPtr=Number(result>>32n),outLen=Number(result&0xffffffffn);
 const text=new TextDecoder().decode(new Uint8Array(w.memory.buffer,outPtr,outLen));
 w.free_result(outPtr,outLen);console.log(text);
}
