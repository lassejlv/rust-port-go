// Observer of unchanged rustc-stable-hash0.1.2 generic/owned operations.
use rustc_stable_hash::{ExtendedHasher,FromStableHash,StableHasher};
use rustc_stable_hash::hashers::SipHasher128;
use std::{cell::{Cell,RefCell},hash::Hasher,fmt,panic::{self,AssertUnwindSafe}};
thread_local!{
 static FAIL_OLD_DROP:Cell<bool>=const{Cell::new(false)};
 static FAIL_CLONE:Cell<bool>=const{Cell::new(false)};
 static FAIL_FINISH:Cell<bool>=const{Cell::new(false)};
 static FAIL_HASH_DROP:Cell<bool>=const{Cell::new(false)};
 static FAIL_FROM:Cell<bool>=const{Cell::new(false)};
 static FAIL_H_RETURN_DROP:Cell<bool>=const{Cell::new(false)};
 static TRACE:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};
}
fn event(s:String){println!("EVENT\t{s}");TRACE.with(|v|v.borrow_mut().push(s));}
struct H{tag:&'static str,value:u64,queries:Cell<u64>,non_copy:Vec<u8>}
impl H{fn new(tag:&'static str,value:u64)->Self{Self{tag,value,queries:Cell::new(0),non_copy:vec![1,2,3]}}}
impl Clone for H{fn clone(&self)->Self{event(format!("clone:{}:{}:panic={}",self.tag,self.value,std::thread::panicking()));if FAIL_CLONE.with(|v|v.replace(false)){panic!("clone")};Self{tag:"clone",value:self.value,queries:Cell::new(self.queries.get()),non_copy:self.non_copy.clone()}}}
impl Drop for H{fn drop(&mut self){event(format!("drop-h:{}:{}:panic={}",self.tag,self.value,std::thread::panicking()));if(self.tag=="old"&&FAIL_OLD_DROP.with(|v|v.replace(false)))||FAIL_H_RETURN_DROP.with(|v|v.replace(false)){panic!("drop-h")}}}
impl Default for H{fn default()->Self{event("default".into());H::new("default",5)}}
impl fmt::Debug for H{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{event(format!("debug:{}:pretty={}:width={:?}:fill={}:plus={}",self.tag,f.alternate(),f.width(),f.fill(),f.sign_plus()));write!(f,"H({})",self.value)}}
impl Hasher for H{fn write(&mut self,b:&[u8]){event(format!("write:{}:{b:?}",self.tag));self.non_copy.extend_from_slice(b);self.value+=b.len()as u64}fn finish(&self)->u64{self.queries.set(self.queries.get()+1);event(format!("finish64:{}:{}:query={}",self.tag,self.value,self.queries.get()));self.value}}
struct R{value:u64}
impl Drop for R{fn drop(&mut self){event(format!("drop-r:{}:panic={}",self.value,std::thread::panicking()));if FAIL_HASH_DROP.with(|v|v.replace(false)){panic!("drop-r")}}}
impl ExtendedHasher for H{type Hash=R;fn finish(self)->R{event(format!("finish-owned:{}:{}",self.tag,self.value));if FAIL_FINISH.with(|v|v.replace(false)){panic!("finish")};R{value:self.value}}}
struct W{value:u64}
impl Drop for W{fn drop(&mut self){event(format!("drop-w:{}:panic={}",self.value,std::thread::panicking()))}}
impl FromStableHash for W{type Hash=R;fn from(r:R)->Self{event(format!("from:{}",r.value));if FAIL_FROM.with(|v|v.replace(false)){panic!("from")};W{value:r.value}}}
fn clone_from(mode:&str){
 FAIL_OLD_DROP.with(|v|v.set(mode=="drop"));FAIL_CLONE.with(|v|v.set(mode=="clone"));
 let source=StableHasher::with_hasher(H::new("source",9));let mut target=StableHasher::with_hasher(H::new("old",2));
 let failed=panic::catch_unwind(AssertUnwindSafe(||target.clone_from(&source))).is_err();event(format!("caught:{failed}"));
 event(format!("after:{}",Hasher::finish(&target)));drop(target);drop(source);
}
fn finish(mode:&str){
 FAIL_FINISH.with(|v|v.set(mode=="finish"||mode=="finish-h-drop"));FAIL_FROM.with(|v|v.set(mode=="from"||mode=="from-r-drop"));FAIL_H_RETURN_DROP.with(|v|v.set(mode=="h-drop"||mode=="finish-h-drop"));FAIL_HASH_DROP.with(|v|v.set(mode=="r-drop"||mode=="from-r-drop"));
 let h=StableHasher::with_hasher(H::new("finish",7));let result=panic::catch_unwind(AssertUnwindSafe(||{let w:W=h.finish();event(format!("returned:{}",w.value));drop(w)}));event(format!("caught:{}",result.is_err()));
}
fn raw(cloned:bool){let mut target=SipHasher128::default();if cloned{let source=SipHasher128::default();event("raw-clone".into());target.clone_from(&source)}else{event("raw-new".into())};event("raw-short".into());let failed=panic::catch_unwind(AssertUnwindSafe(||target.short_write([0;9]))).is_err();event(format!("caught:{failed}"))}
fn operation(op:&str,mode:&str){match op{"clone"=>clone_from(mode),"finish"=>finish(mode),"raw-clone"=>raw(true),"raw-new"=>raw(false),_=>panic!("unknown operation")}}
struct DuringUnwind{operation:String,mode:String}
impl Drop for DuringUnwind{fn drop(&mut self){event(format!("inherited:panic={}",std::thread::panicking()));operation(&self.operation,&self.mode)}}
fn main(){
 let args=std::env::args().collect::<Vec<_>>();let operation=args.get(1).map(String::as_str).unwrap_or("clone");let mode=args.get(2).map(String::as_str).unwrap_or("normal");let context=args.get(3).map(String::as_str).unwrap_or("normal");
 panic::set_hook(Box::new(|_|{}));
 if context=="inherited"{let op=operation.to_owned();let m=mode.to_owned();let result=panic::catch_unwind(AssertUnwindSafe(||{let _guard=DuringUnwind{operation:op,mode:m};panic!("outer")}));event(format!("outer-caught:{}",result.is_err()));}
 else if context=="processing"{let op=operation.to_owned();let m=mode.to_owned();panic::set_hook(Box::new(move |_|{event(format!("processing:panic={}",std::thread::panicking()));crate::operation(&op,&m)}));let result=panic::catch_unwind(||panic!("outer"));event(format!("outer-caught:{}",result.is_err()));}
 else if operation=="borrow"{let mut h=StableHasher::<H>::new();h.write_u16(7);event(format!("first:{}",Hasher::finish(&h)));event(format!("second:{}",Hasher::finish(&h)));event(format!("debug-output:{h:+#032?}"));drop(h)}else{crate::operation(operation,mode)}
}
