// Unchanged official rustc-stable-hash0.1.2 functional observer.
#![feature(hasher_prefixfree_extras)]
use rustc_stable_hash::{ExtendedHasher,StableHasher,FromStableHash,SipHasher128Hash};
use rustc_stable_hash::hashers::SipHasher128;
use serde_json::{Value,json};
use std::{fmt::Debug,hash::Hasher,io::{self,BufRead},panic::{self,AssertUnwindSafe}};
fn n(q:&Value,k:&str)->u64{q[k].as_u64().unwrap_or(0)}fn text<'a>(q:&'a Value,k:&str)->&'a str{q[k].as_str().unwrap_or("")}
fn bytes(s:&str)->Vec<u8>{s.as_bytes().chunks(2).map(|s|u8::from_str_radix(std::str::from_utf8(s).unwrap(),16).unwrap()).collect()}
fn apply<H:Hasher>(h:&mut H,a:&Value){let v=n(a,"value");let bits=((n(a,"hi")as u128)<<64)|v as u128;match text(a,"op"){
 "bytes"=>h.write(&bytes(text(a,"bytes"))),"str"=>h.write_str(text(a,"text")),"len"=>h.write_length_prefix(v as usize),
 "u8"=>h.write_u8(v as u8),"u16"=>h.write_u16(v as u16),"u32"=>h.write_u32(v as u32),"u64"=>h.write_u64(v),"u128"=>h.write_u128(bits),"usize"=>h.write_usize(v as usize),
 "i8"=>h.write_i8(v as i8),"i16"=>h.write_i16(v as i16),"i32"=>h.write_i32(v as i32),"i64"=>h.write_i64(v as i64),"i128"=>h.write_i128(bits as i128),"isize"=>h.write_isize(v as isize),_=>panic!("unknown action")}}
fn short(h:&mut SipHasher128,a:&Value){let bytes=bytes(text(a,"bytes"));macro_rules! call{($($n:literal),*)=>{match n(a,"len"){$($n=>h.short_write::<$n>(bytes.as_slice().try_into().unwrap()),)*_=>panic!("unsupported oracle array size")}}}call!(0,1,2,3,4,5,6,7,8,9,10,16,32);}
struct Pair([u64;2]);impl FromStableHash for Pair{type Hash=SipHasher128Hash;fn from(SipHasher128Hash(v):SipHasher128Hash)->Self{Pair(v)}}
fn snapshot<H:Hasher+Debug>(h:&H,hash:[u64;2])->Value{json!({"hash":hash,"finish64":h.finish(),"debug":format!("{h:?}"),"pretty":format!("{h:#?}"),"hex":format!("{h:+#08X?}")})}
fn panic_message(p:Box<dyn std::any::Any+Send>)->String{if let Some(p)=p.downcast_ref::<&str>(){(*p).into()}else if let Some(p)=p.downcast_ref::<String>(){p.clone()}else{"opaque panic".into()}}
fn run(q:&Value)->Value{let k0=n(q,"key0");let k1=n(q,"key1");let mut steps=vec![];if text(q,"kind")=="raw"{let mut h=SipHasher128::new_with_keys(k0,k1);let mut clones=vec![];steps.push(json!({"state":snapshot(&h,ExtendedHasher::finish(h.clone()).0)}));for a in q["actions"].as_array().unwrap(){let result=panic::catch_unwind(AssertUnwindSafe(||match text(a,"op"){"short"=>short(&mut h,a),"clone"=>clones.push(h.clone()),"restore"=>h.clone_from(&clones[n(a,"value")as usize]),_=>apply(&mut h,a)}));let mut row=json!({"state":snapshot(&h,ExtendedHasher::finish(h.clone()).0)});if let Err(p)=result{row["panic"]=json!(panic_message(p))};steps.push(row)}}else{let mut h=StableHasher::with_hasher(SipHasher128::new_with_keys(k0,k1));let mut clones=vec![];let Pair(hash)=h.clone().finish();steps.push(json!({"state":snapshot(&h,hash)}));for a in q["actions"].as_array().unwrap(){let result=panic::catch_unwind(AssertUnwindSafe(||match text(a,"op"){"clone"=>clones.push(h.clone()),"restore"=>h.clone_from(&clones[n(a,"value")as usize]),_=>apply(&mut h,a)}));let Pair(hash)=h.clone().finish();let mut row=json!({"state":snapshot(&h,hash)});if let Err(p)=result{row["panic"]=json!(panic_message(p))};steps.push(row)}};json!({"request":q,"result":steps})}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",run(&q))}}
