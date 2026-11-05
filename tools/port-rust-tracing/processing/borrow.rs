// SPDX-License-Identifier: MIT OR Apache-2.0
extern crate tracing;
use tracing::{Subscriber,Event,Metadata,span::{Attributes,Id,Record}};
struct S;
impl Subscriber for S {
 fn enabled(&self,_:&Metadata<'_>)->bool{true}
 fn new_span(&self,_:&Attributes<'_>)->Id{Id::from_u64(1)}
 fn record(&self,_:&Id,_:&Record<'_>){} fn record_follows_from(&self,_:&Id,_:&Id){}
 fn event(&self,_:&Event<'_>){} fn enter(&self,_:&Id){} fn exit(&self,_:&Id){}
}
fn hex(value:&str)->String{value.bytes().map(|b|format!("{b:02x}")).collect()}
fn operation(case:&str){
 if case=="id"{let _=Id::from_u64(0);return}
 let d=tracing::Dispatch::new(S);let mut guard=Some(tracing::dispatcher::set_default(&d));
 tracing::dispatcher::get_default(|_|match case{"set"=>{let _=tracing::dispatcher::set_default(&d);},"close"=>drop(guard.take()),_=>unreachable!()});
}
fn attempt(case:&str){let caught=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||operation(case))).is_err();println!("caught:{caught}")}
fn outer_panic(){panic!("outer")}
struct Outer(String);impl Drop for Outer{fn drop(&mut self){println!("outer-drop:{}",std::thread::panicking());attempt(&self.0);println!("outer-end:{}",std::thread::panicking())}}
fn main(){
 let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let case=args[2].clone();
 std::panic::set_hook(Box::new(move|info|{let location=info.location().unwrap();let payload=info.payload().downcast_ref::<String>().map(|s|s.as_str()).or_else(||info.payload().downcast_ref::<&str>().copied()).unwrap_or("opaque");println!("hook:{}:{}:{}:{}:{}:{}",if info.payload().is::<String>(){"owned_string"}else if info.payload().is::<&str>(){"static_str"}else{"opaque"},hex(payload),hex(location.file()),location.line(),location.column(),std::thread::panicking());if mode=="processing"&&payload=="outer"{operation(&case)}}));
 match args[1].as_str(){"normal"=>attempt(&args[2]),"inherited"=>{let caught=std::panic::catch_unwind(||{let _outer=Outer(args[2].clone());outer_panic()}).is_err();println!("outer-caught:{caught}")},"processing"=>outer_panic(),_=>unreachable!()}
}
