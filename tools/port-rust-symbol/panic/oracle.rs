#![feature(rustc_private,panic_can_unwind,panic_internals)]
//! External client of the complete unchanged original rustc_span crate.
use std::{cell::{Cell,RefCell},fmt,io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{Symbol,ByteSymbol,Ident,SessionGlobals,DUMMY_SP,edition::Edition};
use rustc_span::symbol::{self,IdentPrinter,IdentPrintMode};
use rustc_serialize::ContextOrd;
#[derive(Clone)]struct Request{op:String,mode:u8,value:u64,fail:u8,aux:u8}
thread_local!{static TRACE:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};static SCOPE:Cell<&'static str>=const{Cell::new("call")};static ACTIVE:RefCell<Option<Request>>=const{RefCell::new(None)};static RESULT:RefCell<String>=const{RefCell::new(String::new())};}
fn hex(v:&str)->String{v.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn event(v:impl AsRef<str>){let v=format!("{}:{}:{}",v.as_ref(),std::thread::panicking(),SCOPE.with(Cell::get));if std::env::var_os("FERRUGO_SYMBOL_PANIC_TRACE").is_some(){println!("{v}")};TRACE.with(|t|t.borrow_mut().push(v))}
fn scope(v:&'static str){SCOPE.with(|s|s.set(v))}
fn hook(info:&std::panic::PanicHookInfo<'_>){
 let(kind,text)=if let Some(v)=info.payload().downcast_ref::<&str>(){("static_str",*v)}else if let Some(v)=info.payload().downcast_ref::<String>(){("owned_string",v.as_str())}else if info.payload().is::<rustc_span::macros::ExplicitBug>(){("opaque","ExplicitBug")}else{("opaque","")};
 let(file,line,column)=info.location().map_or(("",0,0),|l|(l.file(),l.line(),l.column()));
 event(format!("hook:{kind}:{}:{}:{}:{}:{line}:{column}",hex(text),info.can_unwind(),info.force_no_backtrace(),hex(file)));
 if text=="called `Ident::new` with empty symbol"||text=="ExplicitBug"{event(format!("location-same:{}",info.location().map_or(0,|l|l as*const _ as usize)==BUG_LOCATION.load(std::sync::atomic::Ordering::SeqCst)));}
 if ACTIVE.with(|a|a.borrow().as_ref().is_some_and(|r|r.op=="reenter-index-hook")){event("reenter");let _=Symbol::new(1).as_str();}
 if let Some(req)=ACTIVE.with(|a|a.borrow().clone()).filter(|r|r.mode==3){event("processing");RESULT.with(|r|*r.borrow_mut()=operation(&req));}
}
struct Guard(&'static str);impl Drop for Guard{fn drop(&mut self){event(format!("guard:{}",self.0))}}
struct Field{seen:u32,fail:bool}impl Drop for Field{fn drop(&mut self){let _guard=Guard("field");event(format!("field:{}",self.seen));if self.fail{panic!("field-drop")}}}
struct Context{value:u32,fail:bool,field:Field,constructed:&'static str}impl Drop for Context{fn drop(&mut self){let _guard=Guard("context");event(format!("drop:{}",self.value));event(format!("saved-scope:{}",self.constructed));event(format!("runtime-scope:{}",SCOPE.with(Cell::get)));self.value+=1;self.field.seen=self.value;event(format!("value:{}",self.value));let s=Symbol::intern("symbol_drop_current_context");event(format!("drop-symbol:{}:{}",s.as_u32(),hex(s.as_str())));if self.fail{panic!("context-drop")}}}
fn context_operation(req:&Request)->String{
 scope("construction");let context=Context{value:7,fail:req.fail&1!=0,field:Field{seen:0,fail:req.fail&2!=0},constructed:SCOPE.with(Cell::get)};scope("factory");
 let mut compare=rustc_span::create_default_session_globals_then(||<Symbol as ContextOrd<Context>>::stable_cmp_with_context(context));event("returned");
 scope("invoke1");let first=rustc_span::create_default_session_globals_then(||{let a=Symbol::intern("zzzz_context");let b=Symbol::intern("aaaa_context");compare(&a,&b)});event(format!("compare:{first:?}"));
 scope("invoke2");let second=rustc_span::create_default_session_globals_then(||{let a=Symbol::intern("aaaa_context");let b=Symbol::intern("zzzz_context");compare(&a,&b)});event(format!("compare:{second:?}"));scope("call");"compared".into()
}
struct Sink{calls:u64,fail:u64}impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.calls+=1;event(format!("write:{}",hex(s)));if self.calls==self.fail{Err(fmt::Error)}else{Ok(())}}}
static BUG_LOCATION:std::sync::atomic::AtomicUsize=std::sync::atomic::AtomicUsize::new(0);
static EMIT:std::sync::atomic::AtomicBool=std::sync::atomic::AtomicBool::new(false);
fn bug(_span:Option<rustc_span::Span>,args:fmt::Arguments<'_>,location:&'static std::panic::Location<'static>)->bool{BUG_LOCATION.store(location as*const _ as usize,std::sync::atomic::Ordering::SeqCst);event(format!("diagnostic:{}:{}:{}:{}:{}",hex(&args.to_string()),args.as_str().is_some(),hex(location.file()),location.line(),location.column()));EMIT.load(std::sync::atomic::Ordering::SeqCst)}
static BUG:fn(Option<rustc_span::Span>,fmt::Arguments<'_>,&'static std::panic::Location<'static>)->bool=bug;
struct FormatError;impl fmt::Display for FormatError{fn fmt(&self,_:&mut fmt::Formatter<'_>)->fmt::Result{event("format-error");Err(fmt::Error)}}
fn ordinary(req:&Request)->String{
 let _guard=Guard("operation");
 match req.op.as_str(){
 "new-symbol"=>Symbol::new(req.value as u32).as_u32().to_string(),
 "new-byte"=>ByteSymbol::new(req.value as u32).as_u32().to_string(),
 "get-symbol"|"reenter-index-hook"=>hex(Symbol::new(req.value as u32).as_str()),
 "get-byte"=>{let b=ByteSymbol::new(req.value as u32);b.as_byte_str().iter().map(|x|format!("{x:02x}")).collect()},
 "clamp-symbol"=>Symbol::new(0).clamp(Symbol::new(2),Symbol::new(1)).as_u32().to_string(),
 "clamp-byte"=>ByteSymbol::new(0).clamp(ByteSymbol::new(2),ByteSymbol::new(1)).as_u32().to_string(),
 "prefill"=>{let extras:&[&str]=if req.value==0{&["symbol_duplicate_a","symbol_duplicate_a"]}else{&["async"]};let s=SessionGlobals::new(Edition::Edition2024,extras,None);drop(s);"constructed".into()},
 "lifetime"=>{let mut sink=Sink{calls:0,fail:req.aux as u64};let p=IdentPrinter::new(Symbol::intern(if req.value==0{"bare"}else{"'static"}),IdentPrintMode::RawLifetime,None);if fmt::write(&mut sink,format_args!("{p}")).is_ok(){"ok"}else{"error"}.into()},
 "debug-get"=>{let mut sink=Sink{calls:0,fail:req.aux as u64};let s=Symbol::new(req.value as u32);if fmt::write(&mut sink,format_args!("{s:?}")).is_ok(){"ok"}else{"error"}.into()},
 "format-error"=>FormatError.to_string(),
 "substring"=>rustc_span::edit_distance::edit_distance_with_substrings("a","aa",req.value as usize).map_or("none".into(),|v|v.to_string()),
 "ident"=>{EMIT.store(req.fail!=0,std::sync::atomic::Ordering::SeqCst);let old=rustc_span::macros::EMIT_BUG_DIAGNOSTIC.swap(&BUG);let value=Ident::new(symbol::sym::empty,DUMMY_SP);rustc_span::macros::EMIT_BUG_DIAGNOSTIC.swap(old);value.name.as_u32().to_string()},
 "no-session"=>hex(Symbol::new(1).as_str()),
 _=>panic!("unknown operation")
 }
}
fn operation(req:&Request)->String{if req.op=="context"{context_operation(req)}else{ordinary(req)}}
struct Outer(Request);impl Drop for Outer{fn drop(&mut self){event("outer-drop");if self.0.mode==1{let value=catch_unwind(AssertUnwindSafe(||operation(&self.0)));RESULT.with(|r|*r.borrow_mut()=value.unwrap_or_else(|_|"panic".into()));event("inner-caught");}else{RESULT.with(|r|*r.borrow_mut()=operation(&self.0));}}}
fn execute(req:&Request)->String{
 let result=match req.mode{0=>catch_unwind(AssertUnwindSafe(||operation(req))),1|2=>catch_unwind(AssertUnwindSafe(||{let _outer=Outer(req.clone());panic!("outer")})),3=>catch_unwind(||panic!("outer")),_=>panic!("bad mode")};
 scope("after");let result=if req.mode==0{result.unwrap_or_else(|_|"panic".into())}else{RESULT.with(|r|std::mem::take(&mut*r.borrow_mut()))};
 if ["get-symbol","get-byte","debug-get"].contains(&req.op.as_str()){let good=Symbol::intern("symbol_lock_released");event(format!("after-symbol:{}:{}",good.as_u32(),hex(good.as_str())))}
 result
}
fn main(){
 rustc_data_structures::sync::set_dyn_thread_safe_mode(std::env::var_os("FERRUGO_SYMBOL_NO_SYNC").is_none());std::panic::set_hook(Box::new(hook));
 for line in io::stdin().lock().lines(){let line=line.unwrap();assert!(line.len()<4096);let p=line.split('\t').collect::<Vec<_>>();assert_eq!(p.len(),5);let req=Request{op:p[0].into(),mode:p[1].parse().unwrap(),value:p[2].parse().unwrap(),fail:p[3].parse().unwrap(),aux:p[4].parse().unwrap()};TRACE.with(|t|t.borrow_mut().clear());scope("call");ACTIVE.with(|a|*a.borrow_mut()=Some(req.clone()));
  let result=if ["prefill","context","no-session"].contains(&req.op.as_str()){execute(&req)}else{rustc_span::create_default_session_globals_then(||execute(&req))};
  ACTIVE.with(|a|*a.borrow_mut()=None);let trace=TRACE.with(|t|t.borrow().join("|"));println!("{}\t{}",hex(&result),hex(&trace));
 }
}
