#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Full original hygiene source + real tracing subscriber; no source bodies replaced.
use std::{fmt,fmt::Write,sync::{Arc,Mutex},panic::{catch_unwind,AssertUnwindSafe}};
use tracing::{Subscriber,Metadata,Event,span::{Attributes,Id,Record},subscriber::Interest,metadata::LevelFilter,field::{Field,Visit}};
use rustc_span::{BytePos,Span,DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_span::def_id::CrateNum;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher,RawSpan,RawDefId,StableHashControls};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use rustc_serialize::{Encoder,Decoder};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{cell::RefCell,rc::Rc,hash::Hasher};
thread_local!{static ENCODE:RefCell<Option<Rc<RefCell<HygieneEncodeContext>>>>=const{RefCell::new(None)};}

fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Clone)]struct Log(Arc<Mutex<Vec<String>>>);
impl Log{fn push(&self,s:String){self.0.lock().unwrap_or_else(|p|p.into_inner()).push(s)}}
fn meta(m:&Metadata<'_>)->String{format!("{}|{}|{}|{}|{}|{}|{}|{}",m.name(),m.level(),m.target(),m.module_path().unwrap_or(""),m.file().unwrap_or(""),m.line().unwrap_or(0),if m.is_span(){"span"}else{"event"},m.fields().iter().map(|f|f.name()).collect::<Vec<_>>().join(","))}
struct Sink{out:String,calls:Vec<String>,n:usize,fail:usize}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.n+=1;self.calls.push(format!("S:{}",hex(s)));if self.n==self.fail{return Err(fmt::Error)}self.out.push_str(s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{self.n+=1;self.calls.push(format!("C:{:x}",c as u32));if self.n==self.fail{return Err(fmt::Error)}self.out.push(c);Ok(())}
}
fn debug(sink:&mut Sink,style:u32,value:&dyn fmt::Debug)->fmt::Result{match style{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:*^12.3?}"),3=>write!(sink,"{value:#x?}"),_=>unreachable!()}}
struct Visitor{log:Log,style:u32,fail:usize}
impl Visit for Visitor{
 fn record_debug(&mut self,f:&Field,v:&dyn fmt::Debug){let mut sink=Sink{out:String::new(),calls:vec![],n:0,fail:self.fail};let result=debug(&mut sink,self.style,v);self.log.push(format!("debug|{}|{}|{}|{}|{}",f.name(),std::thread::panicking(),result.is_ok(),hex(sink.out),sink.calls.join(",")))}
 fn record_bool(&mut self,f:&Field,v:bool){self.log.push(format!("bool|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_i64(&mut self,f:&Field,v:i64){self.log.push(format!("i64|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_u64(&mut self,f:&Field,v:u64){self.log.push(format!("u64|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_str(&mut self,f:&Field,v:&str){self.log.push(format!("str|{}|{}|{}",f.name(),hex(v),std::thread::panicking()))}
}
struct State{log:Log,interest:String,enabled:bool,max:LevelFilter,next:Mutex<u64>,style:u32,fail:usize}
impl State{fn visitor(&self)->Visitor{Visitor{log:self.log.clone(),style:self.style,fail:self.fail}}}
impl Drop for State{fn drop(&mut self){self.log.push(format!("subscriber_drop|{}",std::thread::panicking()))}}
impl Subscriber for State{
 fn register_callsite(&self,m:&'static Metadata<'static>)->Interest{self.log.push(format!("register|{}|{}",meta(m),std::thread::panicking()));match self.interest.as_str(){"always"=>Interest::always(),"never"=>Interest::never(),_=>Interest::sometimes()}}
 fn enabled(&self,m:&Metadata<'_>)->bool{self.log.push(format!("enabled|{}|{}",meta(m),std::thread::panicking()));self.enabled}
 fn max_level_hint(&self)->Option<LevelFilter>{self.log.push(format!("max|{}|{}",self.max,std::thread::panicking()));Some(self.max)}
 fn new_span(&self,a:&Attributes<'_>)->Id{let mut next=self.next.lock().unwrap();let id=*next;*next+=1;drop(next);self.log.push(format!("new|{}|{}|{}",id,meta(a.metadata()),std::thread::panicking()));a.record(&mut self.visitor());Id::from_u64(id)}
 fn record(&self,id:&Id,values:&Record<'_>){self.log.push(format!("record|{}|{}",id.into_u64(),std::thread::panicking()));values.record(&mut self.visitor())}
 fn record_follows_from(&self,id:&Id,f:&Id){self.log.push(format!("follows|{}|{}|{}",id.into_u64(),f.into_u64(),std::thread::panicking()))}
 fn event_enabled(&self,e:&Event<'_>)->bool{self.log.push(format!("event_enabled|{}|{}",meta(e.metadata()),std::thread::panicking()));true}
 fn event(&self,e:&Event<'_>){self.log.push(format!("event|{}|{}",meta(e.metadata()),std::thread::panicking()));ENCODE.with(|cell|{if let Some(cell)=cell.borrow().as_ref(){self.log.push(format!("borrow|{}",catch_unwind(AssertUnwindSafe(||drop(cell.borrow_mut()))).is_err()));}});e.record(&mut self.visitor())}
 fn enter(&self,id:&Id){self.log.push(format!("enter|{}|{}",id.into_u64(),std::thread::panicking()))}
 fn exit(&self,id:&Id){self.log.push(format!("exit|{}|{}",id.into_u64(),std::thread::panicking()))}
 fn clone_span(&self,id:&Id)->Id{self.log.push(format!("clone|{}|{}",id.into_u64(),std::thread::panicking()));id.clone()}
 fn try_close(&self,id:Id)->bool{self.log.push(format!("close|{}|{}",id.into_u64(),std::thread::panicking()));true}
}
struct Hcx;
impl StableHashCtxt for Hcx {
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Fingerprint::new(Hash64::new(id.0 as u64+100),Hash64::new(id.1 as u64+1000))}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
fn run(op:usize,log:Log){
 let base=ExpnData::default(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("traced🦀")),DUMMY_SP,Edition::Edition2024,None,None);
 let a=LocalExpnId::fresh(base.clone(),Hcx).to_expn_id();
 let _collision=LocalExpnId::fresh(base,Hcx);
 let ca=SyntaxContext::root().apply_mark(a,Transparency::Opaque);
 let call=Span::new(BytePos(11),BytePos(19),ca,None);
 let data=ExpnData::new(ExpnKind::Macro(MacroKind::Attr,Symbol::intern("nested")),a,call,DUMMY_SP,None,Edition::Edition2021,None,None,false,false,true,false);
 let b=LocalExpnId::fresh(data,Hcx).to_expn_id();
 let cb=ca.apply_mark(b,Transparency::Transparent);
 if op==0||op==3{
  let _=cb.marks();let _=walk_chain(Span::new(BytePos(40),BytePos(50),cb,None),SyntaxContext::root());
  let _=walk_chain_collapsed(Span::new(BytePos(40),BytePos(50),cb,None),DUMMY_SP);
 }
 if op==1||op==3{
  let cell=Rc::new(RefCell::new(HygieneEncodeContext::default()));ENCODE.with(|slot|*slot.borrow_mut()=Some(cell.clone()));
  {let mut h=cell.borrow_mut();h.enqueue_syntax_context_for_encoding(cb);h.schedule_expn_data_for_encoding(b);}
  HygieneEncodeContext::encode(&cell,&mut (),|_,_,key|{let mut h=cell.borrow_mut();h.enqueue_syntax_context_for_encoding(key.0);h.schedule_expn_data_for_encoding(key.1);},|_,_,data,_|{if let Some(data)=data{let mut h=cell.borrow_mut();h.schedule_expn_data_for_encoding(data.parent);h.enqueue_syntax_context_for_encoding(data.call_site.ctxt());}});
  ENCODE.with(|slot|*slot.borrow_mut()=None);
  cell.borrow_mut().enqueue_syntax_context_for_encoding(cb);
  HygieneEncodeContext::encode(&cell,&mut (),|_,_,_|{},|_,_,_,_|{});
 }
 if op==2||op==3{
  let _=decode_expn_id(CrateNum::from_u32(1),0,|_|panic!("root callback"));
  let mut e=MemEncoder::new();e.emit_u32(0);let mut bytes=e.finish();bytes.extend_from_slice(MAGIC_END_BYTES);let mut d=MemDecoder::new(&bytes,0).unwrap();
  let _=decode_syntax_context(&mut d,&HygieneDecodeContext::default(),|_,_|panic!("root callback"));log.push(format!("read|{}",d.position()));
 }
 log.push(format!("done|{op}"));
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let a:Vec<usize>=std::env::args().skip(1).map(|s|s.parse().unwrap()).collect();let(op,mode)=(a[0],a[1]);let config=mode%8;let(style,fail)=(mode/8%4,[0,1,3][mode/32]);let(interest,max,enabled)=match config{0=>("always",LevelFilter::TRACE,true),1=>("sometimes",LevelFilter::TRACE,true),2=>("sometimes",LevelFilter::TRACE,false),3=>("never",LevelFilter::TRACE,true),4=>("always",LevelFilter::INFO,true),5=>("always",LevelFilter::DEBUG,true),6=>("always",LevelFilter::OFF,true),_=>("always",LevelFilter::TRACE,false)};
 let log=Log(Arc::new(Mutex::new(vec![])));let state=State{log:log.clone(),interest:interest.into(),enabled,max,next:Mutex::new(1),style:style as u32,fail};
 rustc_data_structures::sync::set_dyn_thread_safe_mode(false);rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||tracing::subscriber::with_default(state,||run(op,log.clone())));
 let events=log.0.lock().unwrap();for event in &*events{println!("{event}")}
}
