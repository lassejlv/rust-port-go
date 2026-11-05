#![feature(panic_can_unwind, panic_internals)]
// Primitive runtime observations from the exact official compiler's std.
// This is an authored client, not a claim to port a standard-library file.
use std::{any::Any,fmt,panic::{self,catch_unwind,AssertUnwindSafe},sync::atomic::{AtomicUsize,Ordering}};
static EXPECTED:AtomicUsize=AtomicUsize::new(0);
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
struct Payload(Vec<u8>);
impl Drop for Payload{fn drop(&mut self){println!("payload_drop:{}:{}",self.0.len(),std::thread::panicking())}}
fn tag(value:&(dyn Any+Send))->String{
 if let Some(v)=value.downcast_ref::<&str>(){format!("static_str:{}",hex(v))}
 else if let Some(v)=value.downcast_ref::<String>(){format!("owned_string:{}",hex(v))}
 else if let Some(v)=value.downcast_ref::<Box<Payload>>(){format!("opaque:{}:{}",v.0.len(),(&**v as*const Payload as usize)==EXPECTED.load(Ordering::SeqCst))}
 else{"other".into()}
}
struct Token(&'static str,bool);
impl Drop for Token{fn drop(&mut self){println!("drop:{}:{}",self.0,std::thread::panicking());if self.1{panic!("drop panic")}}}
struct Error{panic:bool}
impl fmt::Debug for Error{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 println!("error_debug:{}",std::thread::panicking());let _local=Token("debug_local",false);
 if self.panic{panic!("debug panic")};f.write_str("Error")
}}
impl Drop for Error{fn drop(&mut self){println!("drop:error:{}",std::thread::panicking())}}
fn action(which:&str){
 let _local=Token("body",which=="double");
 println!("call:{}:{}",which,std::thread::panicking());
 match which{
 "return"=>(),
 "static"=>panic!("literal panic"),
 "owned"=>panic!("owned {}",std::hint::black_box("value")),
 "opaque"|"resume_opaque"=>{let v=Box::new(Payload(vec![1,2,3]));EXPECTED.store(&*v as*const Payload as usize,Ordering::SeqCst);if which=="opaque"{panic::panic_any(v)}else{panic::resume_unwind(Box::new(v))}},
 "expect"|"debug_panic"=>{let result:Result<(),Error>=Err(Error{panic:which=="debug_panic"});result.expect("expect failed")},
 "resume_static"=>panic::resume_unwind(Box::new("resume payload")),
 "double"=>panic!("body panic"),
 _=>panic!("invalid action")
 }
}
fn report(f:impl FnOnce()){
 match catch_unwind(AssertUnwindSafe(f)){
 Ok(())=>println!("caught:none:{}",std::thread::panicking()),
 Err(value)=>{println!("caught:{}:{}",tag(value.as_ref()),std::thread::panicking());drop(value)}
 }
}
struct Outer{action:String,escape:bool}
impl Drop for Outer{fn drop(&mut self){println!("drop:outer:{}",std::thread::panicking());if self.escape{action(&self.action)}else{report(||action(&self.action))}}}
fn main(){
 let a:Vec<_>=std::env::args().collect();let mode=a[1].clone();let which=a[2].clone();let hook_action=a[3].clone();
 panic::set_hook(Box::new(move|info|{
  let location=info.location().map(|l|format!("{}:{}:{}",l.file(),l.line(),l.column())).unwrap_or_default();
  println!("hook:{}:{}:{}:{}:{}",tag(info.payload()),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),location);
  let _local=Token("hook_local",hook_action=="drop_panic");
  match hook_action.as_str(){"panic"=>panic!("hook panic"),"resume"=>panic::resume_unwind(Box::new("hook resume")),_=>()}
 }));
 match mode.as_str(){
 "normal"=>report(||action(&which)),
 "inherited_caught"|"inherited_escape"=>report(||{let _outer=Outer{action:which,escape:mode=="inherited_escape"};panic!("outer panic")}),
 _=>panic!("invalid mode")
 }
}
