// SPDX-License-Identifier: MIT OR Apache-2.0
#![feature(internal_output_capture, panic_can_unwind, panic_internals)]
#![allow(internal_features)]
use std::{fmt,io::{self,Write},sync::{Arc,Mutex,OnceLock},thread};
use std::fmt::Write as FmtWrite;

fn hex(bytes:&[u8])->String {bytes.iter().map(|b|format!("{b:02x}")).collect()}
fn log(line:&str){io::stdout().lock().write_all(format!("{line}\n").as_bytes()).unwrap();}
type Capture=Arc<Mutex<Vec<u8>>>;
static ORIGINAL:OnceLock<Capture>=OnceLock::new();
static ALTERNATE:OnceLock<Capture>=OnceLock::new();
static AMBIENT:OnceLock<String>=OnceLock::new();
static MODE:OnceLock<String>=OnceLock::new();
const MAX_FILE_SIZE:u32=4294967294;
fn body_panic()->! {panic!("body-panic")}
fn outer_panic()->! {panic!("outer")}
fn counts(stage:&str){log(&format!("counts:{stage}:{}:{}",Arc::strong_count(ORIGINAL.get().unwrap()),Arc::strong_count(ALTERNATE.get().unwrap())));}
fn snapshots(stage:&str){for(name,c)in[("capture",ORIGINAL.get().unwrap()),("alternate",ALTERNATE.get().unwrap())]{let state=match c.try_lock(){Ok(g)=>format!("{}:{}",g.len(),g.capacity()),Err(std::sync::TryLockError::Poisoned(e))=>{let g=e.into_inner();format!("{}:{}",g.len(),g.capacity())},Err(std::sync::TryLockError::WouldBlock)=>"locked".to_string()};log(&format!("snapshot:{stage}:{name}:{state}"));}}
struct Body;
impl fmt::Display for Body {
 fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {
  log(&format!("body:{}",thread::panicking()));
  counts("body");snapshots("body");
  f.write_str("left")?;
  let mode=MODE.get().unwrap().as_str();
  if mode.contains("nested"){eprint!("[N]");}
  if mode.contains("replace"){io::set_output_capture(Some(ALTERNATE.get().unwrap().clone()));}
  if mode.contains("panic")||mode=="capture_poison_reuse"{body_panic();}
  if mode.contains("fmt_error"){return Err(fmt::Error);}
  f.write_str("")?;
  f.write_char('β')?;
  if mode=="ignore_error" {let _=f.write_str("ignored");return Ok(());}
  f.write_str("right")
 }
}
struct Scope;
impl Drop for Scope {fn drop(&mut self){log(&format!("scope_drop:{}",thread::panicking()));counts("scope_drop");}}
fn statement(){let _scope=Scope; if MODE.get().unwrap()=="capture_fatal"{eprintln!("fatal error: rustc does not support text files larger than {MAX_FILE_SIZE} bytes");}else if MODE.get().unwrap().contains("preheld"){let _guard=io::stderr().lock();eprint!("A{Body}Z");}else{eprint!("A{Body}Z");}}
fn run(){
 let mode=MODE.get().unwrap();
 counts("before");snapshots("before");
 if mode.contains("capture"){io::set_output_capture(Some(ORIGINAL.get().unwrap().clone()));}
 counts("set");
 let caught=std::panic::catch_unwind(statement).is_err();
 log(&format!("caught:{caught}"));
 counts("caught");snapshots("caught");
 let slot=io::set_output_capture(None);
 let name=match slot {None=>"none",Some(ref c)if Arc::ptr_eq(c,ORIGINAL.get().unwrap())=>"original",Some(ref c)if Arc::ptr_eq(c,ALTERNATE.get().unwrap())=>"alternate",_=>"other"};
 log(&format!("slot:{name}"));
 for (name,c)in [("capture",ORIGINAL.get().unwrap()),("alternate",ALTERNATE.get().unwrap())]{
  log(&format!("{name}_poison:{}",c.is_poisoned()));
  log(&format!("{name}:{}",hex(&c.lock().unwrap_or_else(|e|e.into_inner()))));
 }
 if mode=="capture_poison_reuse" {
  io::set_output_capture(Some(ORIGINAL.get().unwrap().clone()));
  eprint!("reuse");io::set_output_capture(None);
  log(&format!("reuse:{}",hex(&ORIGINAL.get().unwrap().lock().unwrap_or_else(|e|e.into_inner()))));
 }
 drop(slot);counts("after_slot_drop");snapshots("after_slot_drop");
}
struct Outer;
impl Drop for Outer{fn drop(&mut self){run();}}
struct Chunks(usize);
impl fmt::Display for Chunks{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{for _ in 0..512{write!(f,"{}",self.0)?;thread::yield_now();}Ok(())}}
fn main(){
 let mode=std::env::args().nth(1).unwrap();let ambient=std::env::args().nth(2).unwrap();
 MODE.set(mode.clone()).unwrap();AMBIENT.set(ambient.clone()).unwrap();
 let capacity=std::env::args().nth(3).map(|s|s.parse::<usize>().unwrap()).unwrap_or(0);
 ORIGINAL.set(Arc::new(Mutex::new(Vec::with_capacity(capacity)))).unwrap();ALTERNATE.set(Arc::new(Mutex::new(vec![]))).unwrap();
 if mode=="concurrent"||mode=="capture_concurrent"{
  if mode=="capture_concurrent"{io::set_output_capture(Some(ORIGINAL.get().unwrap().clone()));}
  let mut handles=vec![];
  for i in 0..8{let capture=if mode=="capture_concurrent"{Some(ORIGINAL.get().unwrap().clone())}else{None};handles.push(thread::spawn(move||{io::set_output_capture(capture);for _ in 0..8{eprintln!("BEGIN{i}:{}END{i}",Chunks(i))}}));}
  for h in handles{h.join().unwrap();}io::set_output_capture(None);log("complete");
  if mode=="capture_concurrent"{log(&format!("capture:{}",hex(&ORIGINAL.get().unwrap().lock().unwrap())));}
  return;
 }
 std::panic::set_hook(Box::new(|p|{
  let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};
  let l=p.location().unwrap();log(&format!("hook:{kind}:{}:{}:{}:{}:{}:{}",hex(p.payload_as_str().unwrap_or("opaque").as_bytes()),hex(l.file().as_bytes()),l.line(),l.column(),p.can_unwind(),p.force_no_backtrace()));
  log(&format!("hook_capture_poison:{}",ORIGINAL.get().unwrap().is_poisoned()));
  counts("hook");snapshots("hook");
  if MODE.get().unwrap()=="release"&&p.payload_as_str().unwrap_or("").starts_with("failed printing"){
   thread::spawn(||{let mut guard=io::stderr().lock();guard.write_all(b"THREAD\n").unwrap();}).join().unwrap();log("release_thread_complete");
  }
  if p.payload_as_str()==Some("outer")&&AMBIENT.get().unwrap()=="processing" {run();}
 }));
 match ambient.as_str(){
  "normal"=>run(),
  "unwinding"=>{let caught=std::panic::catch_unwind(||{let _outer=Outer;outer_panic();}).is_err();log(&format!("outer_caught:{caught}"));},
  "processing"=>{let caught=std::panic::catch_unwind(outer_panic).is_err();log(&format!("outer_caught:{caught}"));},
  _=>panic!("invalid ambient")
 }
}
