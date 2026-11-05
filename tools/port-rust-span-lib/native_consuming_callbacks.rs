#![feature(fn_traits, unboxed_closures, panic_can_unwind, panic_internals)]
//! Actual public FnOnce ownership through unchanged SourceFile/Span methods.
use std::{cell::{Cell,RefCell},panic::{self,catch_unwind,AssertUnwindSafe},sync::Arc};
use rustc_data_structures::sync::FreezeLock;
use rustc_span::{BytePos,ExternalSource,ExternalSourceKind,FileName,SourceFile,SourceFileHashAlgorithm,Span,SPAN_TRACK};
use rustc_span::{def_id::{DefIndex,LocalDefId},hygiene::SyntaxContext};
thread_local! {
 static TOKEN:RefCell<Option<Arc<[u8]>>>=const{RefCell::new(None)};
 static ARG:RefCell<Option<F>>=const{RefCell::new(None)};
 static STORED:RefCell<Option<F>>=const{RefCell::new(None)};
 static FILE:RefCell<Option<SourceFile>>=const{RefCell::new(None)};
 static SPAN:Cell<Span>=const{Cell::new(rustc_span::DUMMY_SP)};
 static ROUTE:RefCell<String>=const{RefCell::new(String::new())};
 static ACTION:RefCell<String>=const{RefCell::new(String::new())};
 static RUNTIME:Cell<u8>=const{Cell::new(1)};
 static PROCESSING:Cell<bool>=const{Cell::new(false)};
}
fn refs()->usize{TOKEN.with(|v|Arc::strong_count(v.borrow().as_ref().unwrap()))}
fn event(text:&str){let phase=FILE.with(|f|f.borrow().as_ref().map(|v|v.external_src.is_frozen().to_string()).unwrap_or("_".into()));println!("{text}:{}:{}:{}:{phase}",std::thread::panicking(),RUNTIME.with(Cell::get),refs())}
fn action()->String{ACTION.with(|v|v.borrow().clone())}
struct Local(&'static str);impl Drop for Local{fn drop(&mut self){event(self.0)}}
struct Field;impl Drop for Field{fn drop(&mut self){event("field");if matches!(action().as_str(),"field"|"drop_field"){panic!("field")}}}
struct F{token:Arc<[u8]>,data:Vec<u8>,_field:Field,state:u32}
impl Drop for F{fn drop(&mut self){let _local=Local("drop_local");assert_eq!(self.data,[1,2,3]);assert_eq!(self.token.as_ref(),[7]);event(&format!("self_drop:{}",self.state));if matches!(action().as_str(),"drop"|"drop_field"){panic!("self_drop")}}}
macro_rules! consume {
 ($value:expr,$result:expr)=>{{
  let mut this=$value;let _local=Local("call_local");this.state+=1;event(&format!("call:{}",this.state));
  match action().as_str(){
   "take_drop"=>{drop(this);event("after_explicit_drop")},
   "forget"=>{std::mem::forget(this);event("forgot")},
   "forget_panic"=>{std::mem::forget(this);event("forgot");panic!("call")},
   "move"=>{STORED.with(|s|*s.borrow_mut()=Some(this));event("moved")},
   "move_panic"=>{STORED.with(|s|*s.borrow_mut()=Some(this));event("moved");panic!("call")},
   "panic"=>panic!("call"),
   _=>{},
  }
  $result
 }}
}
impl FnOnce<()> for F{type Output=Option<String>;extern "rust-call" fn call_once(self,_:())->Self::Output{consume!(self,Some("abc".into()))}}
impl FnOnce<(SyntaxContext,)> for F{type Output=SyntaxContext;extern "rust-call" fn call_once(self,(ctxt,):(SyntaxContext,))->Self::Output{consume!(self,ctxt)}}
fn track(id:LocalDefId){let _local=Local("track_local");event(&format!("track:{}",id.local_def_index.as_u32()))}
static TRACK:fn(LocalDefId)=track;
fn payload(p:&(dyn std::any::Any+Send))->(&'static str,String){if let Some(v)=p.downcast_ref::<&str>(){("static_str",v.to_string())}else if let Some(v)=p.downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())}}
fn hex(text:&str)->String{text.bytes().map(|b|format!("{b:02x}")).collect()}
fn report(label:&str,call:impl FnOnce()){
 match catch_unwind(AssertUnwindSafe(call)){Ok(())=>event(&format!("caught:{label}:false::")),Err(p)=>{let(kind,text)=payload(p.as_ref());event(&format!("caught:{label}:true:{kind}:{}",hex(&text)))}}
}
fn invoke(){
 let f=ARG.with(|a|a.borrow_mut().take().unwrap());
 if ROUTE.with(|r|r.borrow().starts_with("external")){let result=FILE.with(|file|file.borrow().as_ref().unwrap().add_external_src(f));event(&format!("returned:external:{result}"))}
 else{let result=SPAN.with(Cell::get).map_ctxt(f);let data=result.data_untracked();event(&format!("returned:map:{},{},{}",data.lo.0,data.hi.0,data.ctxt.as_u32()))}
}
fn outer(){panic!("outer")}
struct Ambient{caught:bool}impl Drop for Ambient{fn drop(&mut self){event("ambient_drop");if self.caught{report("target",invoke)}else{invoke()}}}
fn setup(route:&str){
 if route.starts_with("external"){
  let mut file=SourceFile::new(FileName::Custom("consuming".into()),"abc".into(),SourceFileHashAlgorithm::Md5,None).unwrap();file.src=None;
  let value=if route=="external_bad_state"{ExternalSource::Unneeded}else{ExternalSource::Foreign{kind:if route=="external_frozen"{ExternalSourceKind::AbsentErr}else{ExternalSourceKind::AbsentOk},metadata_index:0}};
  file.external_src=if route=="external_frozen"{FreezeLock::frozen(value)}else{FreezeLock::new(value)};FILE.with(|f|*f.borrow_mut()=Some(file));
 }else{
  let(lo,hi,ctxt,parent)=match route{
   "map_inline"=>(1,2,0,None),
   "map_parent"=>(1,2,0,Some(22)),
   "map_pair"=>(1,2,2,Some(22)),
   _=>(1,70000,0,Some(22)),
  };
  SPAN.with(|s|s.set(Span::new(BytePos(lo),BytePos(hi),SyntaxContext::from_u32(ctxt),parent.map(|v|LocalDefId{local_def_index:DefIndex::from_u32(v)}))));
 }
}
fn execute(route:&str,selected:&str,mode:&str){
 ROUTE.with(|r|*r.borrow_mut()=route.into());ACTION.with(|a|*a.borrow_mut()=selected.into());TOKEN.with(|t|*t.borrow_mut()=Some(Arc::from([7u8])));if route!="map_absent"{setup(route)};event("construct");
 ARG.with(|a|*a.borrow_mut()=Some(F{token:TOKEN.with(|t|t.borrow().as_ref().unwrap().clone()),data:vec![1,2,3],_field:Field,state:0}));event("prepared");
 RUNTIME.with(|v|v.set(if mode=="normal"{1}else if mode=="nil"{0}else{2}));PROCESSING.with(|v|v.set(mode=="processing"));
 panic::set_hook(Box::new(|info|{let(kind,text)=payload(info.payload());let location=info.location().map(|v|format!("{}:{}:{}",v.file(),v.line(),v.column())).unwrap_or_default();event(&format!("hook:{kind}:{}:{}:{}:{location}",hex(&text),info.can_unwind(),info.force_no_backtrace()));if PROCESSING.with(Cell::get){report("target",invoke);event("hook_return")}}));
 if mode=="nil"{panic::set_hook(Box::new(|_|{}));}
 event("invoke");
 match mode{"normal"|"currentB"|"nil"=>report("target",invoke),"processing"=>report("ambient",outer),_=>report("ambient",||{let _guard=Ambient{caught:mode=="inherited_caught"};outer()})}
 PROCESSING.with(|v|v.set(false));event("after_target");RUNTIME.with(|v|v.set(3));
 let stored=STORED.with(|s|s.borrow_mut().take());if let Some(value)=stored{event("stored_before_drop");report("stored",||drop(value));event("stored_after_drop")}
 event("complete");FILE.with(|f|f.borrow_mut().take());event("file_dropped");
}
fn main(){let args:Vec<_>=std::env::args().collect();panic::set_hook(Box::new(|_|println!("stale_hook")));let old=SPAN_TRACK.swap(&TRACK);if args[1]=="map_absent"{rustc_span::create_default_session_globals_then(||setup("map_interned"));execute(&args[1],&args[2],&args[3])}else{rustc_span::create_default_session_globals_then(||execute(&args[1],&args[2],&args[3]))};event("finish");SPAN_TRACK.swap(old);TOKEN.with(|t|t.borrow_mut().take());}
