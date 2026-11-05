#![feature(rustc_private,fn_traits,unboxed_closures,panic_can_unwind,panic_internals,step_trait)]
// Authored non-Copy owned StableHashCtxt/FnMut/context clients of the complete
// unchanged original hygiene module. No source algorithms are replaced.
use rustc_span::{Span,BytePos,DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_span::def_id::CrateNum;
use rustc_data_structures::{fingerprint::Fingerprint,stable_hash::{StableHashCtxt,StableHasher,RawSpan,RawDefId,StableHashControls}};
use rustc_serialize::{ContextOrd,Encodable,Decodable,opaque::{mem_encoder::MemEncoder,MemDecoder,MAGIC_END_BYTES}};
use std::{any::Any,cell::{Cell,RefCell},hash::Hasher,panic::{self,catch_unwind,AssertUnwindSafe},rc::Rc,sync::Arc};
#[path="failures.rs"]mod failures;
thread_local!{
 static RUNTIME:Cell<u8>=const{Cell::new(1)};
 static EXTERNAL:RefCell<Option<Arc<[Symbol]>>>=const{RefCell::new(None)};
 static ACTION:RefCell<Option<Box<dyn FnOnce()>>>=const{RefCell::new(None)};
}
fn runtime()->u8{RUNTIME.with(Cell::get)}
fn refs()->usize{EXTERNAL.with(|s|Arc::strong_count(s.borrow().as_ref().unwrap()))}
fn features()->Arc<[Symbol]>{EXTERNAL.with(|s|s.borrow().as_ref().unwrap().clone())}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Default)]struct Config{fail:&'static str,fail_at:usize,drop:bool,field:bool,reenter:bool}
fn config(case:&str)->Config{if let Some(number)=case.strip_prefix("call"){let (number,suffix)=number.split_once('_').unwrap_or((number,""));if let Ok(fail_at)=number.parse(){return Config{fail_at,drop:suffix=="drop",field:suffix=="field",..Config::default()}}}match case{
 "ok"=>Config::default(),"assert"=>Config{fail:"assert",..Config::default()},"span"=>Config{fail:"span",..Config::default()},"path"=>Config{fail:"path",..Config::default()},"resolve"=>Config{fail:"resolve",..Config::default()},
 "drop"=>Config{drop:true,..Config::default()},"call_drop"=>Config{fail:"assert",drop:true,..Config::default()},"resolve_drop"=>Config{fail:"resolve",drop:true,..Config::default()},"field"=>Config{field:true,..Config::default()},"drop_field"=>Config{drop:true,field:true,..Config::default()},"reenter"=>Config{reenter:true,..Config::default()},"reenter_drop"=>Config{reenter:true,drop:true,..Config::default()},_=>panic!("case")}}
struct Core{cfg:Config,calls:Cell<usize>}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){println!("local:{}:{}:{}:{}",self.0,std::thread::panicking(),runtime(),refs())}}
impl Core{
 fn call(&self,stage:&str,detail:&str){let _local=Local("call");self.calls.set(self.calls.get()+1);println!("call:{stage}:{}:{}:{}:{}:{}",self.calls.get(),hex(detail),std::thread::panicking(),runtime(),refs());if self.cfg.fail==stage||self.cfg.fail_at==self.calls.get(){panic!("callback")}}
 fn drop_value(&self){let _local=Local("drop");println!("drop:{}:{}:{}:{}",self.calls.get(),std::thread::panicking(),runtime(),refs());if self.cfg.reenter{let id=LocalExpnId::fresh_empty();println!("reentered:{}:{}",id.as_u32(),std::thread::panicking())}if self.cfg.drop{panic!("drop")}}
}
struct Field{core:Rc<Core>}
impl Drop for Field{fn drop(&mut self){println!("field:{}:{}:{}",std::thread::panicking(),runtime(),refs());if self.core.cfg.field{panic!("field")}}}
struct Env{core:Rc<Core>,features:Arc<[Symbol]>,metadata:Vec<u8>,_field:Field}
impl Env{fn new(core:Rc<Core>)->Self{Self{_field:Field{core:core.clone()},core,features:features(),metadata:vec![1,2,3]}}fn call(&self,stage:&str,detail:&str){assert_eq!(self.metadata.len(),3);self.core.call(stage,detail)}}
impl Drop for Env{fn drop(&mut self){self.core.drop_value()}}
struct Returned{_features:Arc<[Symbol]>}
impl Returned{fn new()->Self{let value=Self{_features:features()};println!("prepared:{}:{}:{}",std::thread::panicking(),runtime(),refs());value}}
impl Drop for Returned{fn drop(&mut self){println!("result_drop:{}:{}:{}",std::thread::panicking(),runtime(),refs());let id=LocalExpnId::fresh_empty();println!("result_reentered:{}:{}",id.as_u32(),std::thread::panicking())}}
struct Quiet;
impl StableHashCtxt for Quiet{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Fingerprint::new(id.0 as u64+100,id.1 as u64+1000)}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
impl StableHashCtxt for Env{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){self.call("span",&s.0.to_string());h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{self.call("path",&format!("{}/{}",id.0,id.1));Fingerprint::new(id.0 as u64+100,id.1 as u64+1000)}
 fn stable_hash_controls(&self)->StableHashControls{self.call("controls","");StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,message:&str){self.call("assert",message)}
}
impl FnOnce<(SyntaxContext,)>for Env{type Output=Symbol;extern "rust-call" fn call_once(mut self,args:(SyntaxContext,))->Symbol{self.call_mut(args)}}
impl FnMut<(SyntaxContext,)>for Env{extern "rust-call" fn call_mut(&mut self,(ctxt,):(SyntaxContext,))->Symbol{self.call("resolve",&ctxt.as_u32().to_string());let _=ctxt.edition();Symbol::intern(&format!("name{}",ctxt.as_u32()))}}
fn data()->ExpnData{ExpnData::allow_unstable(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("owned")),Span::new(BytePos(11),BytePos(19),SyntaxContext::root(),None),Edition::Edition2024,features(),None,None)}
fn hash(a:u64,b:u64)->ExpnHash{let mut e=MemEncoder::new();Fingerprint::new(a,b).encode(&mut e);let mut bytes=e.finish();bytes.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&bytes,0).unwrap())}
fn build(operation:&str,core:Rc<Core>)->Box<dyn FnOnce()>{match operation{
 "fresh"=>{let data=data();let env=Env::new(core);Box::new(move||{let id=LocalExpnId::fresh(data,env);println!("returned:fresh:{}",id.as_u32())})},
 "collision"=>{let data=data();let _=LocalExpnId::fresh(data.clone(),Quiet);let env=Env::new(core);Box::new(move||{let id=LocalExpnId::fresh(data,env);println!("returned:collision:{}",id.as_u32())})},
 "set"=>{let id=LocalExpnId::fresh_empty();let data=data();let env=Env::new(core);Box::new(move||{id.set_expn_data(data,env);println!("returned:set:{}",id.as_u32())})},
 "mark"=>{let env=Env::new(core);let unstable=Some(features());let span=Span::new(BytePos(11),BytePos(19),SyntaxContext::root(),None);Box::new(move||{let marked=span.mark_with_reason(unstable,DesugaringKind::Async,Edition::Edition2024,env);println!("returned:mark:{}:{}:{}",marked.lo().0,marked.hi().0,marked.ctxt().as_u32())})},
 "reset"=>{let id=LocalExpnId::fresh(data(),Quiet);let data=data();let env=Env::new(core);Box::new(move||{id.set_expn_data(data,env);println!("returned:reset")})},
 "bad_parent"|"bad_parent_set"=>{let krate=CrateNum::from_u32(3);let index=ExpnIndex::from_u32(17);register_expn_id(krate,index,ExpnData::default(ExpnKind::Root,DUMMY_SP,Edition::Edition2024,None,None),hash(9,9));let mut data=data();data.parent=ExpnId{krate,local_id:index};let env=Env::new(core);let set=operation=="bad_parent_set";let id=if set{Some(LocalExpnId::fresh_empty())}else{None};Box::new(move||{if let Some(id)=id{id.set_expn_data(data,env);println!("returned:bad_parent_set")}else{let _=LocalExpnId::fresh(data,env);println!("returned:bad_parent")}})},
 "already_set"=>{let first=data();let _=LocalExpnId::fresh(first.clone(),Quiet);let id=LocalExpnId::fresh(first,Quiet);let data=id.expn_data();let env=Env::new(core);Box::new(move||{let _=LocalExpnId::fresh(data,env);println!("returned:already_set")})},
 "update"|"borrowed_update"=>{let mut ctxt=SyntaxContext::root();for _ in 0..3{let id=LocalExpnId::fresh(ExpnData::default(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("resolve")),DUMMY_SP,Edition::Edition2024,None,None),Quiet);ctxt=ctxt.apply_mark(id.to_expn_id(),Transparency::Opaque);}let mut env=Env::new(core);let borrowed=operation=="borrowed_update";Box::new(move||{if borrowed{update_dollar_crate_names(&mut env);println!("returned:borrowed_update")}else{update_dollar_crate_names(env);println!("returned:update")}})},
 "comparator"=>{let env=Env::new(core);let(a,b)=(hash(1,9),hash(2,0));Box::new(move||{let mut cmp=<ExpnHash as ContextOrd<Env>>::stable_cmp_with_context(env);println!("returned:comparator:{:?}",cmp(&a,&b))})},
 "remap_syntax"|"remap_syntax_before"|"remap_syntax_repeat"|"remap_local"|"remap_local_before"|"remap_local_repeat"=>{let env=Env::new(core);let syntax=operation.starts_with("remap_syntax");let before=operation.ends_with("_before");let repeat=operation.ends_with("_repeat");Box::new(move||{if syntax{SyntaxContextRemapper::with(move|mut r|{env.call("remap","syntax");if !before{r.commit_end_of_determinism();}if repeat{r.commit_end_of_determinism();}println!("boundary:{}",r.first_non_det_index());let _=r.remapping_results::<u32>(&mut Quiet);});println!("returned:remap_syntax")}else{LocalExpnIdRemapper::with(move|mut r|{env.call("remap","local");if !before{r.commit_end_of_determinism();}if repeat{r.commit_end_of_determinism();}println!("boundary:{}",r.first_non_det_index());let _=r.remapping_results::<u32>(&mut Quiet);});println!("returned:remap_local")}})},
 "remap_syntax_value"|"remap_local_value"=>{let env=Env::new(core);let syntax=operation=="remap_syntax_value";Box::new(move||{if syntax{let value=SyntaxContextRemapper::with(move|_|{env.call("remap","syntax");Returned::new()});println!("returned:remap_syntax_value");drop(value)}else{let value=LocalExpnIdRemapper::with(move|_|{env.call("remap","local");Returned::new()});println!("returned:remap_local_value");drop(value)}})},
 _=>failures::build(operation,core)}}
fn invoke(){let action=ACTION.with(|s|s.borrow_mut().take().unwrap());action()}
fn payload(value:&(dyn Any+Send))->(&'static str,String){if let Some(v)=value.downcast_ref::<&str>(){("static_str",v.to_string())}else if let Some(v)=value.downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())}}
fn report(call:impl FnOnce()){match catch_unwind(AssertUnwindSafe(call)){Ok(())=>println!("caught:false::{}:{}:{}",std::thread::panicking(),runtime(),refs()),Err(p)=>println!("caught:true:{}:{}:{}:{}",hex(payload(p.as_ref()).1),std::thread::panicking(),runtime(),refs())}}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer:{}:{}:{}",std::thread::panicking(),runtime(),refs());if self.caught{report(invoke)}else{invoke()}}}
fn outer_panic(){panic!("outer")}
fn construct(operation:&str,case:&str){
 EXTERNAL.with(|s|*s.borrow_mut()=Some(Arc::from([Symbol::intern("feature")])));RUNTIME.with(|r|r.set(1));panic::set_hook(Box::new(|_|println!("stale_hook")));println!("construct:{}:{}",runtime(),refs());
 let core=Rc::new(Core{cfg:config(case),calls:Cell::new(0)});let action=build(operation,core);ACTION.with(|s|*s.borrow_mut()=Some(action));
}
fn invoke_ready(mode:&str){
 RUNTIME.with(|r|r.set(if mode=="unobserved"{0}else{2}));println!("invoke:{}:{}",runtime(),refs());
 let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let(kind,text)=payload(info.payload());let location=info.location().map(|l|format!("{}:{}:{}",l.file(),l.line(),l.column())).unwrap_or_default();println!("hook:{kind}:{}:{}:{}:{}:{}:{}:{}",hex(text),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),runtime(),refs(),location);if processing{invoke();println!("hook_return:{}:{}:{}",std::thread::panicking(),runtime(),refs())}}));if mode=="unobserved"{panic::set_hook(Box::new(|_|{}));}
 match mode{"normal"|"unobserved"=>report(invoke),"processing"=>report(outer_panic),_=>report(||{let _outer=Outer{caught:mode=="inherited_caught"};outer_panic()})};println!("complete:{}:{}",runtime(),refs());
}
fn main(){let a:Vec<_>=std::env::args().collect();let(operation,case,mode)=(&a[1],&a[2],&a[3]);rustc_data_structures::sync::set_dyn_thread_safe_mode(false);
 if operation=="scope_missing"{rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||construct(operation,case));invoke_ready(mode);rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||println!("--final--\n{}",debug_hygiene_data(true)));}
 else{rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{construct(operation,case);invoke_ready(mode);println!("--final--\n{}",debug_hygiene_data(true));});}
 EXTERNAL.with(|s|s.borrow_mut().take());
}
