#![allow(internal_features)]
#![feature(rustc_attrs,negative_impls,step_trait,pattern_types,pattern_type_macro,fn_traits,unboxed_closures,auto_traits)]
include!("reference_runtime.rs");
use serde_json::{json,Value};
use std::{cell::RefCell,io::{self,BufRead,Write as IoWrite},panic::{catch_unwind,AssertUnwindSafe},rc::Rc,sync::atomic::{AtomicPtr,Ordering}};
thread_local!{static EVENTS:RefCell<Vec<String>>=RefCell::new(vec![]);static LIVE:RefCell<bool>=RefCell::new(false);static WORK:RefCell<Option<Box<dyn FnOnce()>>>=RefCell::new(None);}
fn event(text:String){EVENTS.with(|e|e.borrow_mut().push(text.clone()));if LIVE.with(|v|*v.borrow()){eprintln!("PROBE_EVENT\t{}",json!(text));}}
fn track(parent:LocalDefId){event(format!("track:{}:{}",parent.local_def_index.as_u32(),std::thread::panicking()));}
static GLOBALS:AtomicPtr<SessionGlobals>=AtomicPtr::new(std::ptr::null_mut());
unsafe extern "C"{fn signal(number:i32,handler:extern "C" fn(i32))->usize;fn write(fd:i32,data:*const u8,len:usize)->isize;}
extern "C" fn on_abort(_:i32){
 // Linux-only development audit. No allocation, TLS, formatting, or stdio in
 // this handler: actual try_lock performs its nonblocking primitive fast path.
 let globals=GLOBALS.load(Ordering::Relaxed);
 if !globals.is_null(){let lock=unsafe{&(*globals).span_interner};let guard=lock.try_lock();let text:&[u8]=if guard.is_some(){b"PROBE_LOCK\tfree\n"}else{b"PROBE_LOCK\theld\n"};unsafe{write(2,text.as_ptr(),text.len());}}
}
#[derive(Clone,Copy)]enum Target{Span(Span),Ord(OrdSpan),Data(SpanData)}
impl fmt::Debug for Target{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{match self{Self::Span(v)=>fmt::Debug::fmt(v,f),Self::Ord(v)=>fmt::Debug::fmt(v,f),Self::Data(v)=>fmt::Debug::fmt(v,f)}}}
fn target(request:&Value)->Target{
 let name=request["value"].as_str().unwrap();let parent=if name=="parent"{Some(LocalDefId{local_def_index:DefIndex::from_u32(7)})}else{None};
 if request["kind"]=="data"{return Target::Data(if name=="needs_interner"{SpanData{lo:BytePos(0),hi:BytePos(40000),ctxt:SyntaxContext::root(),parent:None}}else{SpanData{lo:BytePos(1),hi:BytePos(6),ctxt:SyntaxContext::root(),parent}})}
 let span=match name{"inline"|"parent"=>Span::new(BytePos(1),BytePos(6),SyntaxContext::root(),parent),"bad_index"=>Span::from_raw_span(stable_hash::RawSpan(7u64<<61)),"overflow"=>Span::from_raw_span(stable_hash::RawSpan((1u64<<48)|u32::MAX as u64)),_=>panic!("unknown target")};
 if request["kind"]=="ord"{Target::Ord(OrdSpan(span))}else{Target::Span(span)}
}
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{writes:Vec<Value>,accepted:String,chars:bool,fail:i64}
impl Sink{fn record(&mut self,kind:&str,text:&str)->fmt::Result{event(format!("write:{kind}:{}",hex(text.as_bytes())));let index=self.writes.len()as i64;self.writes.push(json!({"kind":kind,"text":text}));if index==self.fail{Err(fmt::Error)}else{self.accepted.push_str(text);Ok(())}}}
impl fmt::Write for Sink{fn write_str(&mut self,text:&str)->fmt::Result{self.record("S",text)}fn write_char(&mut self,c:char)->fmt::Result{if self.chars{self.record("C",c.encode_utf8(&mut[0;4]))}else{self.write_str(c.encode_utf8(&mut[0;4]))}}}
#[derive(Default)]struct State{before:Value,after:Value,ok:Option<bool>}
fn state(f:&fmt::Formatter<'_>)->Value{json!([f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3}])}
struct Observed{value:Target,state:Rc<RefCell<State>>}
impl fmt::Debug for Observed{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.state.borrow_mut().before=state(f);let result=fmt::Debug::fmt(&self.value,f);let mut s=self.state.borrow_mut();s.after=state(f);s.ok=Some(result.is_ok());result}}
struct Scope;impl Drop for Scope{fn drop(&mut self){event(format!("scope_drop:{}",std::thread::panicking()));}}
fn class(payload:&(dyn std::any::Any+Send))->String{let s=payload.downcast_ref::<&str>().copied().or_else(||payload.downcast_ref::<String>().map(String::as_str)).unwrap_or("");if s.contains("cannot access a scoped thread local"){"missing_session".into()}else if s.contains("index out of bounds"){"index".into()}else if s.contains("attempt to add with overflow"){"add".into()}else{format!("unclassified:{s}")}}
type Payload=Box<dyn std::any::Any+Send>;
struct OnDrop<'a,F:FnOnce()>{call:Option<F>,outcome:&'a RefCell<Option<Result<(),Payload>>>}
impl<F:FnOnce()>Drop for OnDrop<'_,F>{fn drop(&mut self){let call=self.call.take().unwrap();*self.outcome.borrow_mut()=Some(catch_unwind(AssertUnwindSafe(call)));}}
fn ambient(name:&str,work:Box<dyn FnOnce()>)->Result<(),Payload>{match name{
 "normal"=>catch_unwind(AssertUnwindSafe(work)),
 "unwinding"=>{let outcome=RefCell::new(None);let _=catch_unwind(AssertUnwindSafe(||{let _drop=OnDrop{call:Some(work),outcome:&outcome};panic!("outer unwind")}));outcome.into_inner().unwrap()},
 "processing_panic"=>{WORK.with(|w|*w.borrow_mut()=Some(work));let previous=std::panic::take_hook();std::panic::set_hook(Box::new(|_|{let work=WORK.with(|w|w.borrow_mut().take().unwrap());work();}));let _=catch_unwind(||panic!("outer processing"));std::panic::set_hook(previous);Ok(())},_=>panic!("unknown ambient")}}
fn execute(request:&Value)->Value{
 assert_eq!(request["debug_assertions"].as_bool().unwrap(),cfg!(debug_assertions));assert_eq!(request["overflow_checks"].as_bool().unwrap(),cfg!(probe_overflow_checks));
 EVENTS.with(|e|e.borrow_mut().clear());LIVE.with(|v|*v.borrow_mut()=request["ambient"]=="processing_panic");
 let globals=SessionGlobals{span_interner:sync::Lock::new(Default::default()),source_map:None};GLOBALS.store((&globals as *const SessionGlobals).cast_mut(),Ordering::Relaxed);
 let previous=SPAN_TRACK.swap(&(track as fn(_)));let value=target(request);let sink=Rc::new(RefCell::new(Sink{writes:vec![],accepted:String::new(),chars:request["chars"].as_bool().unwrap(),fail:request["fail"].as_i64().unwrap()}));let flags=Rc::new(RefCell::new(State::default()));
 let work={let sink=sink.clone();let flags=flags.clone();let pretty=request["pretty"].as_bool().unwrap();Box::new(move||{use fmt::Write;let _scope=Scope;let value=Observed{value,state:flags};let _result=if pretty{write!(&mut *sink.borrow_mut(),"{value:#?}")}else{write!(&mut *sink.borrow_mut(),"{value:?}")};})as Box<dyn FnOnce()>};
 let outcome=if request["session"].as_bool().unwrap(){SESSION_GLOBALS.set(&globals,||ambient(request["ambient"].as_str().unwrap(),work))}else{ambient(request["ambient"].as_str().unwrap(),work)};
 SPAN_TRACK.swap(previous);let panic=outcome.err().map(|p|class(p.as_ref()));let sink=sink.borrow();let flags=flags.borrow();let events=EVENTS.with(|e|e.borrow().clone());let interner=span_encoding::snapshot(&globals.span_interner.lock());GLOBALS.store(std::ptr::null_mut(),Ordering::Relaxed);
 json!({"ok":flags.ok,"panic":panic,"events":events,"writes":sink.writes,"accepted":sink.accepted,"state":{"before":flags.before,"after":flags.after},"interner":interner})
}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));unsafe{signal(6,on_abort);}let mut out=io::BufWriter::new(io::stdout().lock());for line in io::stdin().lock().lines(){let request:Value=serde_json::from_str(&line?).unwrap();let response=execute(&request);writeln!(out,"{response}")?;out.flush()?;}Ok(())}
