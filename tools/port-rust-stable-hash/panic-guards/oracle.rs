// Caller of the unchanged official rustc-stable-hash0.1.2 crate. The hook
// records the selected compiler's actual payload type, flags and locations.
#![feature(panic_can_unwind,panic_internals)]
use rustc_stable_hash::{StableHasher,ExtendedHasher,FromStableHash};
use serde_json::json;
use std::{hash::Hasher,panic::{self,AssertUnwindSafe}};
fn event(text:impl AsRef<str>){println!("E:{}",text.as_ref())}
struct Guard(&'static str);impl Drop for Guard{fn drop(&mut self){event(format!("local:{}:{}",self.0,std::thread::panicking()))}}
struct H{drop_fail:bool,non_copy:Vec<u8>}
impl Drop for H{fn drop(&mut self){event(format!("drop-h:{}",std::thread::panicking()));if self.drop_fail{panic!("drop")}}}
impl Clone for H{fn clone(&self)->Self{event(format!("clone:{}",std::thread::panicking()));let _g=Guard("clone");panic!("clone")}}
impl Hasher for H{fn write(&mut self,b:&[u8]){self.non_copy.extend_from_slice(b);event(format!("write:{}",std::thread::panicking()));let _g=Guard("write");panic!("write")}fn finish(&self)->u64{self.non_copy.len()as u64}}
impl ExtendedHasher for H{type Hash=u64;fn finish(self)->u64{event(format!("finish:{}",std::thread::panicking()));let _g=Guard("finish");panic!("finish")}}
struct W(u64);impl FromStableHash for W{type Hash=u64;fn from(v:u64)->Self{Self(v)}}
fn borrowed(op:&str){event(format!("enter:{}",std::thread::panicking()));let mut h=StableHasher::with_hasher(H{drop_fail:false,non_copy:vec![1,2,3]});let result=panic::catch_unwind(AssertUnwindSafe(||if op=="write"{h.write(b"x")}else{let c=h.clone();drop(c)}));event(format!("caught:{}",result.is_err()));drop(h)}
fn consumed(drop_fail:bool){event(format!("enter:{}",std::thread::panicking()));let h=StableHasher::with_hasher(H{drop_fail,non_copy:vec![1,2,3]});let result=panic::catch_unwind(AssertUnwindSafe(||{let _:W=h.finish();}));event(format!("caught:{}",result.is_err()))}
fn invoke(op:&str,drop_fail:bool){if op=="finish"{consumed(drop_fail)}else{borrowed(op)}}
struct Outer{op:String,drop_fail:bool}impl Drop for Outer{fn drop(&mut self){invoke(&self.op,self.drop_fail)}}
fn hook(info:&panic::PanicHookInfo<'_>){let(payload,kind)=if let Some(v)=info.payload().downcast_ref::<&str>(){((*v).to_owned(),"static_str")}else if let Some(v)=info.payload().downcast_ref::<String>(){(v.clone(),"owned_string")}else{("opaque".into(),"opaque")};let location=info.location().map(|l|json!({"File":l.file(),"Line":l.line(),"Column":l.column()}));println!("H:{}",json!({"Payload":payload,"Kind":kind,"Location":location,"CanUnwind":info.can_unwind(),"ForceNoBacktrace":info.force_no_backtrace()}))}
fn main(){let args:Vec<_>=std::env::args().collect();let op=args[1].clone();let context=&args[2];let drop_fail=args[3]=="true";panic::set_hook(Box::new(hook));
 if context=="inherited"{let _=panic::catch_unwind(AssertUnwindSafe(||{let _g=Outer{op,drop_fail};panic!("outer")}));event("outer-caught:true")}
 else if context=="processing"{panic::set_hook(Box::new(move|info|{hook(info);invoke(&op,drop_fail)}));let _=panic::catch_unwind(||panic!("outer"));event("outer-caught:true")}
 else{invoke(&op,drop_fail)}
}
