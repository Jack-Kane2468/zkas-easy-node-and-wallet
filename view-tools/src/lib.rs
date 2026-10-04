use orchard::{Action,keys::OutgoingViewingKey,note::{Nullifier,ExtractedNoteCommitment,TransmittedNoteCiphertext},note_encryption::OrchardDomain,primitives::redpallas::{Signature,SpendAuth,VerificationKey},value::ValueCommitment};
use zcash_note_encryption::try_output_recovery_with_ovk;
use serde_json::{Value,json};

fn arr<const N:usize>(b:&[u8],at:usize)->Option<[u8;N]>{b.get(at..at+N)?.try_into().ok()}
fn recover(ovk:&[u8;32],payload:&[u8])->Option<Value>{
 // ZKas canonical bundle: flags, i64 value balance, anchor, var binding sig, u32 action count; all lengths big-endian.
 if payload.len()<49{return None}
 let siglen=u32::from_be_bytes(arr(payload,41)?) as usize;if siglen!=64{return None}
 let start=45+siglen;let count=u32::from_be_bytes(arr(payload,start)?) as usize;if count==0||count>512{return None}
 let end=start+4+count*884;let proof=u32::from_be_bytes(arr(payload,end)?) as usize;
 let extra=if payload[0]&4!=0{40}else{0};if payload.len()!=end+4+proof+extra{return None}
 let mut found=vec![];
 for i in 0..count{
 let b=&payload[start+4+i*884..start+4+(i+1)*884];
 let nf:Nullifier=match Option::from(Nullifier::from_bytes(&arr::<32>(b,0)?)){Some(v)=>v,None=>continue};
 let rk=match VerificationKey::<SpendAuth>::try_from(arr::<32>(b,32)?){Ok(v)=>v,Err(_)=>continue};
 let cmx:ExtractedNoteCommitment=match Option::from(ExtractedNoteCommitment::from_bytes(&arr::<32>(b,64)?)){Some(v)=>v,None=>continue};
 let cv:ValueCommitment=match Option::from(ValueCommitment::from_bytes(&arr::<32>(b,96)?)){Some(v)=>v,None=>continue};
 let ct=TransmittedNoteCiphertext{epk_bytes:arr(b,128)?,enc_ciphertext:arr(b,160)?,out_ciphertext:arr(b,740)?};
 let action=match Action::from_parts(nf,rk,cmx,ct,cv,Signature::<SpendAuth>::from(arr::<64>(b,820)?)){Ok(a)=>a,Err(_)=>continue};
 let domain=OrchardDomain::for_action(&action);
 if let Some((note,addr,memo))=try_output_recovery_with_ovk(&domain,&OutgoingViewingKey::from(*ovk),&action,action.cv_net(),&arr::<80>(b,740)?){
 let n=memo.iter().rposition(|&b|b!=0).map(|n|n+1).unwrap_or(0);let m=if memo[..n]==[0xf6]{String::new()}else{String::from_utf8_lossy(&memo[..n]).to_string()};
 found.push(json!({"action":i,"recipient_raw":hex::encode(addr.to_raw_address_bytes()),"amount_sompi":note.value().inner().to_string(),"memo":m}));
 }
 }
 Some(json!(found))
}

fn process(line:&str)->String {
 let result=(||{if line.len()>8*1024*1024{return None};let v:Value=serde_json::from_str(line).ok()?;let ovk:[u8;32]=hex::decode(v["ovk"].as_str()?).ok()?.try_into().ok()?;let bytes=hex::decode(v["payload"].as_str()?).ok()?;recover(&ovk,&bytes)})();
 match result {Some(rows)=>json!({"ok":true,"rows":rows}).to_string(),None=>json!({"ok":false,"error":"Malformed key or bundle"}).to_string()}
}
#[no_mangle]
pub extern "C" fn alloc_input(len:u32)->u32 {let b=vec![0u8;len as usize].into_boxed_slice();Box::into_raw(b) as *mut u8 as u32}
#[no_mangle]
pub unsafe extern "C" fn recover_json(ptr:u32,len:u32)->u64 {
 let mut input=Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr as *mut u8,len as usize));
 let response=match std::str::from_utf8(&input){Ok(s)=>process(s),Err(_)=>"{\"ok\":false}".into()};input.fill(0);drop(input);
 let b=response.into_bytes().into_boxed_slice();let n=b.len() as u64;let p=Box::into_raw(b) as *mut u8 as u64;(p<<32)|n
}
#[no_mangle]
pub unsafe extern "C" fn free_result(ptr:u32,len:u32){let mut b=Box::from_raw(std::ptr::slice_from_raw_parts_mut(ptr as *mut u8,len as usize));b.fill(0);drop(b);}
