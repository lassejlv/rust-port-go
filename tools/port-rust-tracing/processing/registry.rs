// SPDX-License-Identifier: MIT OR Apache-2.0
extern crate tracing;
use tracing::{Subscriber,Event,Metadata,span::{Attributes,Id,Record}};
use std::sync::atomic::{AtomicBool,Ordering};
static FAIL_READ:AtomicBool=AtomicBool::new(false);
struct S(&'static str,bool);impl Drop for S{fn drop(&mut self){println!("drop:{}:{}",self.0,std::thread::panicking())}}
fn fail_register(){panic!("register")}
fn fail_callsite(){panic!("callsite")}
impl Subscriber for S{
 fn on_register_dispatch(&self,_:&tracing::Dispatch){println!("on-register:{}",self.0);if self.1{fail_register()}}
 fn register_callsite(&self,_:&'static Metadata<'static>)->tracing::subscriber::Interest{println!("callsite:{}",self.0);if FAIL_READ.load(Ordering::Relaxed){fail_callsite()}tracing::subscriber::Interest::always()}
 fn enabled(&self,_:&Metadata<'_>)->bool{true}fn new_span(&self,_:&Attributes<'_>)->Id{Id::from_u64(1)}fn record(&self,_:&Id,_:&Record<'_>){}fn record_follows_from(&self,_:&Id,_:&Id){}fn event(&self,_:&Event<'_>){}fn enter(&self,_:&Id){}fn exit(&self,_:&Id){}
}
fn caught<T>(label:&str,call:impl FnOnce()->T)->Option<T>{match std::panic::catch_unwind(std::panic::AssertUnwindSafe(call)){Ok(value)=>{println!("caught:{label}:false");Some(value)},Err(_)=>{println!("caught:{label}:true");None}}}
fn script(case:&str,processing:bool){
 let mut bases=Vec::new();if case!="write0"{bases.push(tracing::Dispatch::new(S("base0",false)));bases.push(tracing::Dispatch::new(S("base1",false)));}
 if case=="read2"{FAIL_READ.store(true,Ordering::Relaxed);caught("first",||tracing::event!(tracing::Level::INFO,value=1));FAIL_READ.store(false,Ordering::Relaxed);}else{caught("first",||tracing::Dispatch::new(S("failed",true)));}
 if processing{outer_panic()}
 let after=caught("after",||tracing::Dispatch::new(S("after",false)));caught("rebuild",tracing::callsite::rebuild_interest_cache);drop(after);drop(bases);
}
fn hex(value:&str)->String{value.bytes().map(|b|format!("{b:02x}")).collect()}
fn outer_panic(){panic!("outer")}
struct Outer(String);impl Drop for Outer{fn drop(&mut self){println!("outer-drop:{}",std::thread::panicking());script(&self.0,false);println!("outer-end:{}",std::thread::panicking())}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();std::panic::set_hook(Box::new(move|info|{let loc=info.location().unwrap();let payload=info.payload().downcast_ref::<String>().map(|s|s.as_str()).or_else(||info.payload().downcast_ref::<&str>().copied()).unwrap_or("opaque");println!("hook:{}:{}:{}:{}:{}:{}",if info.payload().is::<String>(){"owned_string"}else{"static_str"},hex(payload),hex(loc.file()),loc.line(),loc.column(),std::thread::panicking());if mode.starts_with("processing")&&payload=="outer"{if mode=="processing_write"{let _=tracing::Dispatch::new(S("after",false));}else{tracing::callsite::rebuild_interest_cache()}}}));match args[1].as_str(){"normal"=>script(&args[2],false),"inherited"=>{let result=std::panic::catch_unwind(||{let _outer=Outer(args[2].clone());outer_panic()});println!("outer-caught:{}",result.is_err())},_=>script("write2",true)}}
