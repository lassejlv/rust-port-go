// SPDX-License-Identifier: MIT OR Apache-2.0
extern crate tracing;
use tracing::{Subscriber,Event,Metadata,Level,span::{Attributes,Id,Record}};
use std::{sync::atomic::{AtomicU8,Ordering},io::Write};
static MODE:AtomicU8=AtomicU8::new(0);
fn log(v:&str){std::io::stdout().lock().write_all(format!("{v}\n").as_bytes()).unwrap()}
struct Probe;impl std::fmt::Debug for Probe{fn fmt(&self,f:&mut std::fmt::Formatter<'_>)->std::fmt::Result{f.write_str("Probe")}}
impl Drop for Probe{fn drop(&mut self){log("field-drop")}}
struct S;impl Drop for S{fn drop(&mut self){log("subscriber-drop")}}
impl Subscriber for S{
 fn on_register_dispatch(&self,_:&tracing::Dispatch){if MODE.load(Ordering::Relaxed)==3{log("on-register");panic!("inner")}}
 fn enabled(&self,_:&Metadata<'_>)->bool{true}
 fn new_span(&self,_:&Attributes<'_>)->Id{log("new-span");panic!("inner")}
 fn record(&self,_:&Id,_:&Record<'_>){} fn record_follows_from(&self,_:&Id,_:&Id){}
 fn event(&self,_:&Event<'_>){log("event");panic!("inner")}
 fn enter(&self,_:&Id){} fn exit(&self,_:&Id){}
}
fn main(){let mode=match std::env::args().nth(1).unwrap().as_str(){"event"=>1,"span"=>2,_=>3};MODE.store(mode,Ordering::Relaxed);std::panic::set_hook(Box::new(|p|{if p.payload().downcast_ref::<&str>()==Some(&"outer"){match MODE.load(Ordering::Relaxed){1=>tracing::event!(Level::INFO,value=?Probe),2=>{let _=tracing::span!(Level::INFO,"probe",value=?Probe);},_=>{let _=tracing::Dispatch::new(S);}}}}));if mode==3{let _=std::panic::catch_unwind(||panic!("outer"));}else{let d=tracing::Dispatch::new(S);tracing::dispatcher::with_default(&d,||panic!("outer"));}}
