#![allow(internal_features)]
#![feature(rustc_attrs,negative_impls,step_trait,pattern_types,pattern_type_macro,if_let_guard,fn_traits,unboxed_closures,auto_traits)]
include!("reference_runtime.rs");
use serde_json::{json,Value};
use std::{cell::RefCell,hash::{Hash,Hasher},io::{self,BufRead,Write as IoWrite},panic::{catch_unwind,AssertUnwindSafe},sync::Mutex};
thread_local! {static PROBE:RefCell<Probe>=RefCell::new(Probe::default());}
#[derive(Default)]struct Probe {events:Vec<String>,track_count:i64,track_panic_at:i64,track_reenter:Option<Value>,source_text:String,source_panic:bool,live:bool}
fn event(text:String){let live=PROBE.with(|p|{let mut p=p.borrow_mut();p.events.push(text.clone());p.live});if live {eprintln!("PROBE_EVENT\t{}",json!(text));}}
fn take_events()->Vec<String>{PROBE.with(|p|std::mem::take(&mut p.borrow_mut().events))}
fn u32v(value:&Value)->u32 {u32::try_from(value.as_u64().expect("expected u32")).unwrap()}
fn index(value:&Value)->usize{usize::try_from(value.as_u64().expect("expected index")).unwrap()}
fn parent(value:&Value)->Option<LocalDefId>{if value.is_null(){None}else{Some(LocalDefId{local_def_index:DefIndex::from_u32(u32v(value))})}}
fn raw(span:Span)->String{format!("{:016x}",span.to_raw_span().0)}
fn span_fields(value:&Value,ordered:bool)->Span {let(lo,hi,ctxt,parent)=(BytePos(u32v(&value["lo"])),BytePos(u32v(&value["hi"])),SyntaxContext::from_u32(u32v(&value["ctxt"])),parent(&value["parent"]));if ordered{Span::new_ordered(lo,hi,ctxt,parent)}else{Span::new(lo,hi,ctxt,parent)}}
fn snapshot(globals:&SessionGlobals)->Value{json!(span_encoding::snapshot(&globals.span_interner.lock()))}
fn data(value:SpanData)->Value{json!([value.lo.0,value.hi.0,value.ctxt.as_u32(),value.parent.map(|p|p.local_def_index.as_u32())])}
fn track(parent:LocalDefId){
 let number=parent.local_def_index.as_u32();event(format!("track:{number}"));
 let(panic,reenter)=PROBE.with(|p|{let mut p=p.borrow_mut();let panic=p.track_count==p.track_panic_at;p.track_count+=1;(panic,p.track_reenter.clone())});
 if panic{panic!("probe track panic")}
 if let Some(value)=reenter {let span=span_fields(&value,false);event(format!("track_reenter:{}",raw(span)));}
}
struct RecordingSourceMap;
impl DiagnosticSourceMap for RecordingSourceMap {
 fn span_to_diagnostic_string(&self,span:Span)->String{event(format!("source_map:{}",raw(span)));PROBE.with(|p|{let p=p.borrow();if p.source_panic{panic!("probe source_map panic")};p.source_text.clone()})}
}
fn panic_class(payload:&(dyn std::any::Any+Send))->String {
 let text=payload.downcast_ref::<&str>().copied().or_else(||payload.downcast_ref::<String>().map(String::as_str)).unwrap_or("");
 for(kind,needle)in[("track","probe track panic"),("map","probe map panic"),("drop","probe drop panic"),("drop","probe token panic"),("source_map","probe source_map panic"),("hash","probe hash panic"),("subtract","attempt to subtract with overflow"),("add","attempt to add with overflow"),("missing_session","cannot access a scoped thread local variable without calling `set` first"),("index","index out of bounds"),("identity_limit","assertion failed: value <="),("context_limit","Interned::CTXT"),("debug_assert","lo <= hi"),("debug_assert","self.mask()")]{if text.contains(needle){return kind.to_string()}}
 format!("unclassified:{text}")
}
struct HashSink{count:i64,panic_at:i64}
impl Hasher for HashSink{
 fn finish(&self)->u64{0}
 fn write(&mut self,_:&[u8]){panic!("unclassified byte hasher dispatch")}
 fn write_u64(&mut self,value:u64){event(format!("hash:u64:{value:016x}"));if self.count==self.panic_at{panic!("probe hash panic")};self.count+=1;}
}
fn hex(bytes:&[u8])->String{bytes.iter().map(|v|format!("{v:02x}")).collect()}
struct Sink {writes:Vec<Value>,accepted:String,fail:i64,chars:bool}
impl Sink{fn record(&mut self,kind:&str,text:&str)->fmt::Result{let n=self.writes.len()as i64;event(format!("write:{kind}:{}",hex(text.as_bytes())));self.writes.push(json!({"kind":kind,"text":text}));if n==self.fail{Err(fmt::Error)}else{self.accepted.push_str(text);Ok(())}}}
impl fmt::Write for Sink {
 fn write_str(&mut self,text:&str)->fmt::Result{self.record("S",text)}
 fn write_char(&mut self,c:char)->fmt::Result{if self.chars{self.record("C",c.encode_utf8(&mut[0;4]))}else{self.write_str(c.encode_utf8(&mut[0;4]))}}
}
#[derive(Default)]struct FormatState{first:bool,reused:bool,before:Value,after:Value,final_state:Value}
static FORMAT_STATE:Mutex<Option<FormatState>>=Mutex::new(None);
fn fmt_state(f:&fmt::Formatter<'_>)->Value{json!([f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3}])}
struct Observed<'a>{value:&'a dyn fmt::Debug,reuse:bool}
impl fmt::Debug for Observed<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let before=fmt_state(f);let first=fmt::Debug::fmt(self.value,f);let after=fmt_state(f);let second=if self.reuse{fmt::Debug::fmt(&26usize,f)}else{Ok(())};let final_state=fmt_state(f);*FORMAT_STATE.lock().unwrap()=Some(FormatState{first:first.is_ok(),reused:second.is_ok(),before,after,final_state});first.and(second)}}
fn render(sink:&mut Sink,value:&dyn fmt::Debug,pattern:usize,reuse:bool)->fmt::Result{
 use fmt::Write;let value=Observed{value,reuse};match pattern{
 0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:x?}"),3=>write!(sink,"{value:X?}"),4=>write!(sink,"{value:#x?}"),
 5=>write!(sink,"{value:+08?}"),6=>write!(sink,"{value:08x?}"),7=>write!(sink,"{value:*>12?}"),8=>write!(sink,"{value:*<12?}"),9=>write!(sink,"{value:*^13?}"),
 10=>write!(sink,"{value:.2?}"),11=>write!(sink,"{value:+#08x?}"),12=>write!(sink,"{value:-08?}"),13=>write!(sink,"{value:🦀^12?}"),14=>write!(sink,"{value:0>12?}"),
 15=>write!(sink,"{value:\n>8?}"),16=>write!(sink,"{value:\0>8?}"),17=>write!(sink,"{value:#08?}"),18=>write!(sink,"{value:#.2?}"),_=>panic!("invalid formatter pattern")}}
fn debug(span:Span,op:&Value)->Value{
 let mut sink=Sink{writes:vec![],accepted:String::new(),fail:op["fail"].as_i64().unwrap_or(-1),chars:op["chars"].as_bool().unwrap_or(false)};
 let pattern=index(&op["pattern"]);let reuse=op["reuse"].as_bool().unwrap_or(false);
 let ok=match op["kind"].as_str().unwrap_or("span"){"span"=>render(&mut sink,&span,pattern,reuse),"ord"=>render(&mut sink,&OrdSpan(span),pattern,reuse),"data"=>render(&mut sink,&span.data_untracked(),pattern,reuse),_=>panic!("invalid debug target")}.is_ok();
 let state=FORMAT_STATE.lock().unwrap().take().unwrap();json!({"ok":ok,"first_ok":state.first,"reuse_ok":state.reused,"writes":sink.writes,"accepted":sink.accepted,"state":{"before":state.before,"after":state.after,"final":state.final_state}})
}

// An actual user-defined FnOnce, with a boxed mutable environment cell. Its
// destructor/field glue are Rust's own; no callback-drop timing is synthesized.
struct Token;
impl Drop for Token {fn drop(&mut self){event(format!("token_drop:{}",std::thread::panicking()));}}
struct MapperCell {count:u32,token:Option<Token>}
struct Mapper {
 cell:Box<MapperCell>,moved:Arc<Mutex<Option<Token>>>,to:u32,
 call_panic:bool,drop_panic:bool,move_token:bool,
}
impl Drop for Mapper {fn drop(&mut self){
 event(format!("map_drop:{}:{}:{}",self.cell.count,self.cell.token.is_some(),std::thread::panicking()));
 if self.drop_panic{panic!("probe drop panic")}
}}
impl FnOnce<(SyntaxContext,)> for Mapper {
 type Output=SyntaxContext;
 extern "rust-call" fn call_once(mut self,(ctxt,):(SyntaxContext,))->SyntaxContext {
  event(format!("map_call:{}:{}:{}",ctxt.as_u32(),self.cell.count,std::thread::panicking()));
  self.cell.count+=1;
  if self.move_token { *self.moved.lock().unwrap()=self.cell.token.take();event("move_token".to_owned()); }
  if self.call_panic{panic!("probe map panic")}
  SyntaxContext::from_u32(self.to)
 }
}
fn owned_map(span:Span,op:&Value)->Span {
 let moved=Arc::new(Mutex::new(None));
 let mapper=Mapper{cell:Box::new(MapperCell{count:0,token:Some(Token)}),moved:moved.clone(),to:u32v(&op["to"]),call_panic:op["call_panic"].as_bool().unwrap_or(false),drop_panic:op["drop_panic"].as_bool().unwrap_or(false),move_token:op["move_token"].as_bool().unwrap_or(false)};
 let outcome=catch_unwind(AssertUnwindSafe(||span.map_ctxt(mapper)));
 // A moved field is owned by the surrounding caller and is destroyed after
 // map_ctxt's result has been caught, rather than by Mapper's drop glue.
 drop(moved.lock().unwrap().take());
 match outcome {Ok(span)=>span,Err(p)=>std::panic::resume_unwind(p)}
}
type PanicPayload=Box<dyn std::any::Any+Send>;
struct OuterDrop<'a,F:FnOnce()->Span>{call:Option<F>,result:&'a RefCell<Option<Result<Span,PanicPayload>>>}
impl<F:FnOnce()->Span> Drop for OuterDrop<'_,F>{fn drop(&mut self){let call=self.call.take().unwrap();*self.result.borrow_mut()=Some(catch_unwind(AssertUnwindSafe(call)));}}
thread_local! {
 static HOOK_WORK:RefCell<Option<Box<dyn FnOnce()->Span>>>=RefCell::new(None);
 static HOOK_RESULT:RefCell<Option<Span>>=RefCell::new(None);
}
fn map_owned(span:Span,op:&Value)->Span {
 match op["ambient"].as_str().unwrap_or("normal") {
 "normal"=>owned_map(span,op),
 "unwinding"=>{
  let result=RefCell::new(None);
  let _outer=catch_unwind(AssertUnwindSafe(||{let _guard=OuterDrop{call:Some(||owned_map(span,op)),result:&result};panic!("probe outer unwind")}));
  match result.into_inner().unwrap(){Ok(value)=>value,Err(p)=>std::panic::resume_unwind(p)}
 },
 "processing_panic"=>{
  let value=op.clone();HOOK_WORK.with(|work|*work.borrow_mut()=Some(Box::new(move||owned_map(span,&value))));
  let previous=std::panic::take_hook();std::panic::set_hook(Box::new(|_|{let work=HOOK_WORK.with(|work|work.borrow_mut().take().unwrap());let value=work();HOOK_RESULT.with(|result|*result.borrow_mut()=Some(value));}));
  let _outer=catch_unwind(||panic!("probe processing trigger"));std::panic::set_hook(previous);
  HOOK_RESULT.with(|result|result.borrow_mut().take().unwrap())
 },
 _=>panic!("invalid owned ambient context")
 }
}
fn operation(op:&Value,slots:&mut Vec<Span>)->Value {
 let name=op["op"].as_str().expect("op name");
 let at=|slots:&Vec<Span>|slots[index(&op["span"])];
 let produced=match name{
 "new"=>Some(span_fields(op,false)),"new_ordered"=>Some(span_fields(op,true)),
 "raw"=>Some(Span::from_raw_span(stable_hash::RawSpan(u64::from_str_radix(op["bits"].as_str().unwrap(),16).unwrap()))),"dummy"=>Some(DUMMY_SP),
 "clone"=>{let original=at(slots);let mut copied=DUMMY_SP;copied.clone_from(&original);assert!(copied==original.clone());Some(copied)},
 "ord_clone"=>{let original=OrdSpan(at(slots));let mut copied=OrdSpan(DUMMY_SP);copied.clone_from(&original);assert!(copied.0==original.clone().0);Some(copied.0)},
 "inline_try"=>span_encoding::InlineCtxt::try_new_span(u32v(&op["lo"]),u32v(&op["len"]),u32v(&op["ctxt"])),
 "map_ctxt"=>Some(at(slots).map_ctxt(|ctxt|{event(format!("map:{}",ctxt.as_u32()));if op["panic"].as_bool().unwrap_or(false){panic!("probe map panic")};SyntaxContext::from_u32(u32v(&op["to"]))})),
 "map_owned"=>Some(map_owned(at(slots),op)),
 "with_parent"=>Some(at(slots).with_parent(parent(&op["parent"]))),
 _=>None};
 if let Some(span)=produced{let bits=raw(span);slots.push(span);return json!(bits)}
 match name {
 "inline_try"=>Value::Null,
 "inline_from"=>json!(span_encoding::InlineCtxt::try_from_span(at(slots)).map(|i|[i.lo,i.len,i.ctxt])),
 "data"=>data(at(slots).data()),"data_untracked"=>data(at(slots).data_untracked()),"ctxt"=>json!(at(slots).ctxt().as_u32()),"parent"=>json!(at(slots).parent().map(|p|p.local_def_index.as_u32())),
 "is_dummy"=>json!(at(slots).is_dummy()),"from_expansion"=>json!(at(slots).from_expansion()),
 "ord_eq"=>json!(OrdSpan(at(slots))==OrdSpan(slots[index(&op["other"])])),"ord_ne"=>json!(OrdSpan(at(slots))!=OrdSpan(slots[index(&op["other"])])),
 "ord_cmp"=>json!(OrdSpan(at(slots)).cmp(&OrdSpan(slots[index(&op["other"])]))as i8),
 "ord_partial_cmp"=>json!({"some":OrdSpan(at(slots)).partial_cmp(&OrdSpan(slots[index(&op["other"])] )).unwrap()as i8}),
 "hash"=>{at(slots).hash(&mut HashSink{count:0,panic_at:op["panic_at"].as_i64().unwrap_or(-1)});Value::Null},
 "debug"=>debug(at(slots),op),
 _=>panic!("unimplemented reference op:{name}")}
}
fn execute(request:&Value)->Value{
 assert_eq!(request["debug_assertions"].as_bool().unwrap(),cfg!(debug_assertions));
 assert_eq!(request["overflow_checks"].as_bool().unwrap(),cfg!(probe_overflow_checks));
 let source=request.get("source_map");PROBE.with(|p|*p.borrow_mut()=Probe{events:vec![],track_count:0,track_panic_at:request["track_panic_at"].as_i64().unwrap_or(-1),track_reenter:request.get("track_reenter").cloned(),source_text:source.and_then(|v|v["text"].as_str()).unwrap_or("").to_owned(),source_panic:source.and_then(|v|v["panic"].as_bool()).unwrap_or(false),live:request["abort"].as_bool().unwrap_or(false)});
 let globals=SessionGlobals{span_interner:sync::Lock::new(Default::default()),source_map:source.map(|_|Arc::new(RecordingSourceMap)as Arc<SourceMap>)};
 let previous=SPAN_TRACK.swap(&(track as fn(_)));let mut slots=vec![];let mut steps=vec![];
 for(op_index,op)in request["ops"].as_array().unwrap().iter().enumerate(){
  PROBE.with(|p|p.borrow_mut().track_count=0);take_events();
  if request["abort"].as_bool().unwrap_or(false){eprintln!("PROBE_START\t{}",json!({"op":op_index,"interner":snapshot(&globals),"slots":slots.iter().map(|s|raw(*s)).collect::<Vec<_>>()}));}
  let outcome=catch_unwind(AssertUnwindSafe(||{if request["session"].as_bool().unwrap(){SESSION_GLOBALS.set(&globals,||operation(op,&mut slots))}else{operation(op,&mut slots)}}));
  let(result,panic)=match outcome{Ok(v)=>(v,Value::Null),Err(p)=>(Value::Null,json!(panic_class(p.as_ref())))};
  let step=json!({"result":result,"panic":panic,"events":take_events(),"interner":snapshot(&globals),"slots":slots.iter().map(|s|raw(*s)).collect::<Vec<_>>()});
  if request["abort"].as_bool().unwrap_or(false){eprintln!("PROBE_STEP\t{step}");}steps.push(step);
 }
 SPAN_TRACK.swap(previous);json!({"steps":steps})
}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));let mut out=io::BufWriter::new(io::stdout().lock());for line in io::stdin().lock().lines(){let line=line?;let request:Value=serde_json::from_str(&line).unwrap();let response=execute(&request);writeln!(out,"{response}")?;out.flush()?;}Ok(())}
