// Safe public calls into the unchanged complete rustc_span crate. The boxed
// callback state is a non-Copy source value; no library body is reproduced.
#![feature(fn_traits, unboxed_closures, panic_can_unwind, panic_internals)]
use std::{cell::{Cell,RefCell}, panic::{self, AssertUnwindSafe, catch_unwind}, sync::{Arc, OnceLock}};
use rustc_span::*;
use rustc_span::def_id::{DefIndex, LocalDefId};
use rustc_span::edition::Edition;
use rustc_span::hygiene::*;
use rustc_data_structures::{fingerprint::Fingerprint, stable_hash::{RawSpan, RawDefId, StableHashCtxt, StableHashControls, StableHasher}};
#[path="native_source_helpers.rs"] mod source_helpers;
static OP:OnceLock<String> = OnceLock::new();
static MODE:OnceLock<String> = OnceLock::new();
static CALL:OnceLock<bool> = OnceLock::new();
static DROP:OnceLock<bool> = OnceLock::new();
static STRATEGY:OnceLock<String> = OnceLock::new();
thread_local!{static RUNTIME:Cell<u8>=const{Cell::new(1)}; static PROCESSING:Cell<bool>=const{Cell::new(false)}; static INPUT:RefCell<Option<Input>>=const{RefCell::new(None)}; static CALLBACK:RefCell<Option<Callback>>=const{RefCell::new(None)}; static WITNESS:RefCell<Option<Arc<()>>>=const{RefCell::new(None)};}
fn event(s:impl AsRef<str>){eprintln!("EVENT:{}:{}:{}",s.as_ref(),std::thread::panicking(),RUNTIME.with(Cell::get));}
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
fn payload(p:&(dyn std::any::Any+Send))->(&'static str,String){if let Some(v)=p.downcast_ref::<&str>(){("static_str",v.to_string())}else if let Some(v)=p.downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())}}
fn outer()->!{panic!("outer")}
fn callback_panic()->!{panic!("callback")}
fn drop_panic()->!{panic!("drop")}
struct Hcx;
impl StableHashCtxt for Hcx{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){std::hash::Hasher::write_u64(h,s.0)}
 fn def_path_hash(&self,d:RawDefId)->Fingerprint{Fingerprint::new(u64::from(d.0)+100,u64::from(d.1)+1000)}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
fn shape(s:Span)->String{let d=s.data_untracked();format!("{},{},{},{}",d.lo.0,d.hi.0,d.ctxt.as_u32(),d.parent.map(|p|p.local_def_index.as_u32().to_string()).unwrap_or("_".into()))}
struct Input{span:Span,id:ExpnId,source:SourceFile}
fn prepare()->Input{
 let id=LocalExpnId::fresh(ExpnData::new(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("remaining_macro")),ExpnId::root(),DUMMY_SP,DUMMY_SP,None,Edition::Edition2021,None,None,false,false,false,false),Hcx).to_expn_id();
 let ctxt=SyntaxContext::root().apply_mark(id,Transparency::Opaque);
 let span=Span::new(BytePos(7),BytePos(40007),ctxt,Some(LocalDefId{local_def_index:DefIndex::from_u32(9)}));
 let source=if OP.get().unwrap()=="lines_bad0"{source_helpers::source_with_encoded_line_diffs(1,0,&[])}else if OP.get().unwrap()=="lines_bad3"{source_helpers::source_with_encoded_line_diffs(1,3,&[])}else{SourceFile::new(FileName::Custom("remaining".into()),"a\nb\n".into(),SourceFileHashAlgorithm::Md5,None).unwrap()};
 Input{span,id,source}
}
struct Field;
impl Drop for Field{fn drop(&mut self){event("env_field_drop")}}
struct Return{value:Arc<()>}
impl Drop for Return{fn drop(&mut self){event(format!("result_drop:{}",Arc::strong_count(&self.value)))}}
struct CallbackState{calls:u32,_field:Field,result:Arc<()>,initial:usize}
struct Callback{state:Box<CallbackState>}
impl Callback{
 fn invoke(&mut self)->Return{
  let identity=(&*self.state as *const CallbackState as usize)==self.state.initial;
  event(format!("call:{}:{identity}:{}",self.state.calls,Arc::strong_count(&self.state.result)));self.state.calls+=1;
  event(format!("edition:{}",SyntaxContext::root().edition()));
  if *CALL.get().unwrap(){callback_panic()}
  let out=Return{value:self.state.result.clone()};event(format!("prepared:{}",Arc::strong_count(&out.value)));out
 }
}
impl Callback{fn finish(self)->Return{if STRATEGY.get().unwrap()=="early"{event("early_forget_self");let mut owned=std::mem::ManuallyDrop::new(self);return owned.invoke()};let mut owned=self;let result=owned.invoke();if STRATEGY.get().unwrap()=="late"{event("forget_self");std::mem::forget(owned)};result}}
impl<'a>FnOnce<(&'a SessionGlobals,)> for Callback{type Output=Return;extern "rust-call" fn call_once(self,_:(&SessionGlobals,))->Return{self.finish()}}
impl FnOnce<()> for Callback{type Output=Return;extern "rust-call" fn call_once(self,_:())->Return{self.finish()}}
impl<'a>FnOnce<(&'a MetavarSpansMap,)>for Callback{type Output=Return;extern "rust-call" fn call_once(self,(map,):(&MetavarSpansMap,))->Return{
 let span=Span::with_root_ctxt(BytePos(1),BytePos(3));let inserted=map.insert(DUMMY_SP,span);let same=map.get(DUMMY_SP)==Some(span);event(format!("metavar:{inserted}:{same}"));self.finish()
}}
impl Drop for Callback{fn drop(&mut self){let identity=(&*self.state as *const CallbackState as usize)==self.state.initial;event(format!("env_drop:{}:{identity}:{}",self.state.calls,Arc::strong_count(&self.state.result)));if !["meta_absent","with_absent","create_ctorfail"].contains(&OP.get().unwrap().as_str()){event(format!("env_drop_edition:{}",SyntaxContext::root().edition()))};if *DROP.get().unwrap(){drop_panic()}}}
fn is_owned()->bool{!["def","call","mixed","ed2015","ed2018","ed2021","ed2024","subst","lines"].contains(&OP.get().unwrap().split('_').next().unwrap())}
fn prepare_callback(){
 if !is_owned(){return}
 let witness=Arc::new(());let mut state=Box::new(CallbackState{calls:0,_field:Field,result:witness.clone(),initial:0});state.initial=&*state as *const CallbackState as usize;let cb=Callback{state};
 event(format!("construct:{}",Arc::strong_count(&witness)));CALLBACK.with(|v|*v.borrow_mut()=Some(cb));WITNESS.with(|v|*v.borrow_mut()=Some(witness));
}
fn owned(){
 let witness=WITNESS.with(|v|v.borrow_mut().take().unwrap());let cb=CALLBACK.with(|v|v.borrow_mut().take().unwrap());
 let outcome=catch_unwind(AssertUnwindSafe(||{let result=match OP.get().unwrap().as_str(){
 "default"|"default_present"=>create_default_session_globals_then(cb),
 "meta_present"|"meta_absent"=>with_metavar_spans(cb),
 "set"|"set_present"=>{let session=SessionGlobals::new(Edition::Edition2018,&[],None);set_session_globals_then(&session,cb)},
 "with_present"|"with_absent"=>with_session_globals(cb),
 "create"|"create_present"=>create_session_globals_then(Edition::Edition2018,&[],None,cb),
 "create_ctorfail"=>create_session_globals_then(Edition::Edition2018,&["remaining_duplicate_symbol","remaining_duplicate_symbol"],None,cb),
 "if_present"|"if_absent"=>create_session_if_not_set_then(Edition::Edition2018,cb),
 _=>unreachable!()
 };event(format!("returned:{}",Arc::strong_count(&result.value)));drop(result);}));
 match outcome{Ok(())=>event("caught:none"),Err(p)=>{let(k,t)=payload(p.as_ref());event(format!("caught:{}",hex(&t)))}}
 event(format!("witness:{}",Arc::strong_count(&witness)));drop(witness);
 if !OP.get().unwrap().ends_with("present"){create_default_session_globals_then(||event("scope_restored"))}else{with_session_globals(|_|event("scope_present"))}
}
fn surface(input:&Input){let op=OP.get().unwrap().split('_').next().unwrap();
 match op{
  "def"=>event(format!("result:{}",shape(input.span.with_def_site_ctxt(input.id)))),
  "call"=>event(format!("result:{}",shape(input.span.with_call_site_ctxt(input.id)))),
  "mixed"=>event(format!("result:{}",shape(input.span.with_mixed_site_ctxt(input.id)))),
  "ed2015"=>event(format!("result:{}",input.span.is_rust_2015())),
  "ed2018"=>event(format!("result:{}",input.span.at_least_rust_2018())),
  "ed2021"=>event(format!("result:{}",input.span.at_least_rust_2021())),
  "ed2024"=>event(format!("result:{}",input.span.at_least_rust_2024())),
  "subst"=>{let span=if OP.get().unwrap()=="subst_dummy"{DUMMY_SP}else{input.span};event(format!("result:{}",shape(span.substitute_dummy(Span::with_root_ctxt(BytePos(10),BytePos(20))))))},
  "lines"=>event(format!("result:{}",input.source.count_lines())),
  _=>unreachable!()
 }
}
fn target_input(input:&Input){
 if is_owned(){owned()}else{let result=catch_unwind(AssertUnwindSafe(||surface(input)));match result{Ok(())=>event("caught:none"),Err(p)=>{let(k,t)=payload(p.as_ref());event(format!("caught:{}",hex(&t)))}}}
}
fn target(){INPUT.with(|i|target_input(i.borrow().as_ref().unwrap()))}
struct Ambient;
impl Drop for Ambient{fn drop(&mut self){event("ambient_drop");target()}}
fn execute(){
 RUNTIME.with(|v|v.set(if MODE.get().unwrap()=="nil"{0}else if MODE.get().unwrap()=="normal"{1}else{2}));
 PROCESSING.with(|v|v.set(MODE.get().unwrap()=="processing"));
 // The hook accesses the real input through source-thread TLS, without
 // requiring a Send/Sync fiction for SourceFile or any unsafe pointer.
 panic::set_hook(Box::new(move|p|{
  let(k,t)=payload(p.payload());if RUNTIME.with(Cell::get)!=0{let l=p.location().unwrap();event(format!("hook:{k}:{}:{}:{}:{}:{}:{}",hex(&t),hex(l.file()),l.line(),l.column(),p.can_unwind(),p.force_no_backtrace()))}
  if t=="outer"&&PROCESSING.with(Cell::get){target()}
 }));
 event("invoke");
 match MODE.get().unwrap().as_str(){
  "normal"|"currentB"|"nil"=>target(),
  "unwinding"=>{let result=catch_unwind(AssertUnwindSafe(||{let _ambient=Ambient;outer()}));event(format!("outer_caught:{}",result.is_err()))},
  "processing"=>{let result=catch_unwind(outer);event(format!("outer_caught:{}",result.is_err()))},
  _=>unreachable!()
 }
 PROCESSING.with(|v|v.set(false));
 panic::set_hook(Box::new(|_|{}));
 event("after");
}
fn main(){
 let args:Vec<_>=std::env::args().collect();OP.set(args[1].clone()).unwrap();MODE.set(args[2].clone()).unwrap();CALL.set(args[3]=="true").unwrap();DROP.set(args[4]=="true").unwrap();STRATEGY.set(args[5].clone()).unwrap();
 rustc_data_structures::sync::set_dyn_thread_safe_mode(true);panic::set_hook(Box::new(|_|{}));
 let present=OP.get().unwrap().ends_with("present");
 if present{create_session_globals_then(Edition::Edition2021,&[],None,||{let input=prepare();event("prepared_input");INPUT.with(|i|*i.borrow_mut()=Some(input));prepare_callback();execute();INPUT.with(|i|i.borrow_mut().take());})}
 else{let input=create_session_globals_then(Edition::Edition2021,&[],None,prepare);event("prepared_input");INPUT.with(|i|*i.borrow_mut()=Some(input));prepare_callback();execute();INPUT.with(|i|i.borrow_mut().take());}
 println!("DONE");
}
