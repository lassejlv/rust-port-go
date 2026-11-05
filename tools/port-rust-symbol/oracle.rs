#![feature(rustc_private)]
//! Client of the complete, unchanged pinned rustc_span crate.
use std::cell::Cell;
use std::sync::{Arc,Mutex};
use std::fmt;
use std::hash::{Hash,Hasher};
use std::io::{self,BufRead};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::stable_hash::{DummyStableHashCtxt,RawDefId,RawSpan,StableCompare,StableHash,StableHashCtxt,StableHashControls,StableHasher};
use rustc_serialize::{ContextOrd,Decodable,Encodable,Decoder};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES};
use rustc_serialize::opaque::mem_encoder::MemEncoder;
use rustc_span::{BytePos,ByteSymbol,Ident,MacroRulesNormalizedIdent,Span,Symbol,DUMMY_SP};
use rustc_span::edition::Edition;
use rustc_span::symbol::{self,IdentPrintMode,IdentPrinter};

fn unhex(value:&str)->Vec<u8>{assert!(value.len()%2==0);(0..value.len()).step_by(2).map(|i|u8::from_str_radix(&value[i..i+2],16).unwrap()).collect()}
fn text(value:&str)->String{String::from_utf8(unhex(value)).unwrap()}
fn hex(value:&[u8])->String{value.iter().map(|b|format!("{b:02x}")).collect()}
fn edition(value:&str)->Edition{match value{"0"=>Edition::Edition2015,"1"=>Edition::Edition2018,"2"=>Edition::Edition2021,"3"=>Edition::Edition2024,"4"=>Edition::EditionFuture,_=>panic!("bad edition")}}
fn num<T:std::str::FromStr>(value:&str)->T where T::Err:fmt::Debug{value.parse().unwrap()}
fn symbols(value:&str)->Vec<Symbol>{if value=="-"{vec![]}else{value.split(',').map(|v|Symbol::intern(&text(v))).collect()}}
fn ids(values:&[Symbol])->String{values.iter().map(|v|v.as_u32().to_string()).collect::<Vec<_>>().join(",")}
fn option(value:Option<Symbol>)->String{value.map_or("none".into(),|v|hex(v.as_str().as_bytes()))}
fn distance(value:&str)->Option<usize>{if value=="-"{None}else{Some(num(value))}}
fn mode(value:&str)->IdentPrintMode{match value{"0"=>IdentPrintMode::Normal,"1"=>IdentPrintMode::RawIdent,"2"=>IdentPrintMode::ForcedKeywordIdent,"3"=>IdentPrintMode::RawLifetime,_=>panic!("bad mode")}}

#[derive(Default)]struct TraceHasher(Vec<String>);
impl Hasher for TraceHasher{
 fn finish(&self)->u64{0}
 fn write(&mut self,bytes:&[u8]){self.0.push(format!("bytes:{}",hex(bytes)))}
 fn write_u32(&mut self,value:u32){self.0.push(format!("u32:{value}"))}
}
#[derive(Default)]struct Sink{calls:usize,fail:usize,events:Vec<String>,rendered:String}
#[derive(Default)]struct RawTraceHasher(Vec<String>);
impl Hasher for RawTraceHasher{fn finish(&self)->u64{0}fn write(&mut self,bytes:&[u8]){self.0.push(hex(bytes))}}
impl fmt::Write for Sink{
 fn write_str(&mut self,value:&str)->fmt::Result{self.calls+=1;self.events.push(format!("S:{}",hex(value.as_bytes())));if self.calls==self.fail{return Err(fmt::Error)}self.rendered.push_str(value);Ok(())}
 fn write_char(&mut self,value:char)->fmt::Result{self.calls+=1;self.events.push(format!("C:{:x}",value as u32));if self.calls==self.fail{return Err(fmt::Error)}self.rendered.push(value);Ok(())}
}
fn display<T:fmt::Display>(sink:&mut Sink,index:usize,value:&T)->fmt::Result{match index{
 // DISPLAY_ARMS
 _=>panic!("not a Display pattern"),
}}
fn debug<T:fmt::Debug>(sink:&mut Sink,index:usize,value:&T)->fmt::Result{match index{
 // DEBUG_ARMS
 _=>panic!("not a Debug pattern"),
}}
fn render<T:fmt::Display+fmt::Debug>(sink:&mut Sink,index:usize,is_debug:bool,value:&T)->fmt::Result{if is_debug{debug(sink,index,value)}else{display(sink,index,value)}}

#[derive(Clone)]struct Events(Arc<Mutex<Vec<String>>>);
impl Events{fn push(&self,value:impl Into<String>){let value=value.into();if std::env::var_os("FERRUGO_SYMBOL_ABORT_TRACE").is_some(){println!("{value}")}self.0.lock().unwrap().push(value)}}
struct DroppedContext{value:u32,fail:bool,events:Events}
impl Drop for DroppedContext{
 fn drop(&mut self){self.events.push(format!("drop:{}:{}",self.value,std::thread::panicking()));self.value+=1;self.events.push(format!("value:{}",self.value));if self.fail{panic!("context-drop")}}
}
fn context_factory(fail:bool,events:Events){
 let mut compare=<Symbol as ContextOrd<DroppedContext>>::stable_cmp_with_context(DroppedContext{value:7,fail,events:events.clone()});
 events.push("returned");
 rustc_span::create_default_session_globals_then(||events.push(format!("compare:{:?}",compare(&Symbol::intern("zzzz_context"),&Symbol::intern("aaaa_context")))));
 rustc_span::create_default_session_globals_then(||events.push(format!("compare:{:?}",compare(&Symbol::intern("aaaa_context"),&Symbol::intern("zzzz_context")))));
}
struct Outer{fail:bool,catch:bool,events:Events}
impl Drop for Outer{
 fn drop(&mut self){
  self.events.push("outer-drop");
  if self.catch{let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||context_factory(self.fail,self.events.clone())));self.events.push(format!("inner:{}",if result.is_ok(){"ok"}else{"panic"}));}
  else{context_factory(self.fail,self.events.clone())}
 }
}
fn owned_context(mode:&str,fail:bool)->String{
 let events=Events(Arc::new(Mutex::new(Vec::new())));
 let result=if mode=="0"{std::panic::catch_unwind(std::panic::AssertUnwindSafe(||context_factory(fail,events.clone())))}
 else if mode=="1"||mode=="2"{std::panic::catch_unwind(std::panic::AssertUnwindSafe(||{let _outer=Outer{fail,catch:mode=="1",events:events.clone()};panic!("outer")}))}
 else if mode=="3"{
  let values=events.0.clone();
  std::panic::set_hook(Box::new(move |_|{let events=Events(values.clone());events.push("processing");context_factory(fail,events)}));
  let result=std::panic::catch_unwind(||panic!("outer"));std::panic::set_hook(Box::new(|_|{}));result
 }else{panic!("bad context mode")};
 let status=if result.is_ok(){"ok"}else{"panic"};
 format!("{}\t{}",status,events.0.lock().unwrap().join(","))
}

// A caller-defined, fully implemented context policy. The full original
// hygiene/derive code invokes it; no compiler crate interface is replaced.
struct ProbeContext{hash_spans:bool,spans:Vec<u64>}
impl StableHashCtxt for ProbeContext{
 fn stable_hash_span(&mut self,span:RawSpan,hasher:&mut StableHasher){self.spans.push(span.0);if self.hash_spans{hasher.write_u64(span.0^0x12345678)}}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Fingerprint::new(0x1234u64+u64::from(id.0),0x5678u64+u64::from(id.1))}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:self.hash_spans}}
 fn assert_default_stable_hash_controls(&self,msg:&str){assert!(!self.hash_spans,"{msg}")}
}
static BUG_EVENTS:Mutex<Vec<String>>=Mutex::new(Vec::new());
static BUG_EMIT:std::sync::atomic::AtomicBool=std::sync::atomic::AtomicBool::new(false);
fn span_raw_via_public_context(span:Span)->u64{let mut context=ProbeContext{hash_spans:false,spans:vec![]};span.stable_hash(&mut context,&mut StableHasher::new());context.spans[0]}
fn observe_bug(span:Option<Span>,args:fmt::Arguments<'_>,location:&'static std::panic::Location<'static>)->bool{
 let file=location.file();let file=if let Some(i)=file.find("compiler/rustc_span/"){&file[i..]}else{"symbol-oracle.rs"};
 BUG_EVENTS.lock().unwrap().push(format!("{}\t{}\t{}\t{}\t{}\t{}",span_raw_via_public_context(span.unwrap()),hex(args.to_string().as_bytes()),args.as_str().is_some(),hex(file.as_bytes()),location.line(),location.column()));
 BUG_EMIT.load(std::sync::atomic::Ordering::SeqCst)
}
static OBSERVE_BUG:fn(Option<Span>,fmt::Arguments<'_>,&'static std::panic::Location<'static>)->bool=observe_bug;

fn process(parts:&[&str])->String{
 match parts[0]{
 "K"=>{
  let value=Symbol::new(num(parts[2]));let ident=Ident{name:value,span:DUMMY_SP};let calls=Cell::new(0);
  let e=edition(parts[1]);let reserved=value.is_reserved(||{calls.set(calls.get()+1);e});
  let flags=[value.is_empty(),reserved,value.is_weak(),value.is_path_segment_keyword(),value.is_bool_lit(),value.can_be_raw(),Symbol::is_predefined(value.as_u32()),ident.is_special(),ident.is_used_keyword(),ident.is_unused_keyword(),ident.is_reserved(),ident.is_path_segment_keyword(),ident.is_raw_guess(),ident.is_reserved_lifetime(),ident.is_raw_lifetime_guess(),ident.is_numeric()];
  let mut hash=TraceHasher::default();value.hash(&mut hash);let mut ihash=TraceHasher::default();ident.hash(&mut ihash);
  let print_mode=match ident.guess_print_mode(){IdentPrintMode::Normal=>0,IdentPrintMode::RawIdent=>1,IdentPrintMode::ForcedKeywordIdent=>2,IdentPrintMode::RawLifetime=>3};
  format!("{}\t{}\t{}\t{}\t{}\t{}",flags.map(|v|if v{'1'}else{'0'}).iter().collect::<String>(),calls.get(),print_mode,hex(value.as_str().as_bytes()),hash.0.join(","),ihash.0.join(","))
 },
 "U"=>{
  let sequence=parts[2].split(',').map(edition).collect::<Vec<_>>();let calls=Cell::new(0usize);
  let values=symbol::used_keywords(||{let i=calls.get();calls.set(i+1);sequence[i%sequence.len()]});
  format!("{}\t{}",ids(&values),calls.get())
 },
 "I"=>{
  let mut result=Vec::new();
  for entry in parts[2].split(','){
   let mut bytes=unhex(&entry[1..]);
   if entry.starts_with('s'){let value=Symbol::intern(std::str::from_utf8(&bytes).unwrap());result.push(format!("{}:{}",value.as_u32(),hex(value.as_str().as_bytes())))}
   else{let value=ByteSymbol::intern(&bytes);if entry.starts_with('m')&&!bytes.is_empty(){bytes[0]^=255}result.push(format!("{}:{}",value.as_u32(),hex(value.as_byte_str())))}
  }
  result.join(",")
 },
 "M"=>{
  let candidates=symbols(parts[2]);let lookup=Symbol::intern(&text(parts[3]));let dist=distance(parts[4]);
  let value=if parts[5]=="1"{rustc_span::edit_distance::find_best_match_for_name_with_substrings(&candidates,lookup,dist)}else{rustc_span::edit_distance::find_best_match_for_name(&candidates,lookup,dist)};
  option(value)
 },
 "N"=>{let candidates=symbols(parts[2]);let lookups=symbols(parts[3]);option(rustc_span::edit_distance::find_best_match_for_names(&candidates,&lookups,distance(parts[4])))},
 "R"=>{let candidates=symbols(parts[2]);let lookup=Symbol::intern(&text(parts[3]));lookup.find_similar(&candidates).map_or("none".into(),|(s,incorrect)|format!("{}:{}",hex(s.as_str().as_bytes()),incorrect))},
 "V"=>{
  let a=Symbol::intern(&text(parts[2]));let b=Symbol::intern(&text(parts[3]));
  let mut std=TraceHasher::default();a.hash(&mut std);
  let mut stable=StableHasher::new();stable.write(&unhex(parts[4]));a.stable_hash(&mut DummyStableHashCtxt,&mut stable);let (lo,hi)=stable.finish::<Fingerprint>().split();
  format!("{}\t{}\t{:?}\t{:?}\t{}\t{}\t{}",a.as_u32(),b.as_u32(),a.cmp(&b),a.stable_cmp(&b),std.0.join(","),lo.as_u64(),hi.as_u64())
 },
 "B"=>{
  let value=ByteSymbol::intern(&unhex(parts[2]));let mut std=TraceHasher::default();value.hash(&mut std);
  let mut stable=StableHasher::new();stable.write(&unhex(parts[3]));value.stable_hash(&mut DummyStableHashCtxt,&mut stable);let (lo,hi)=stable.finish::<Fingerprint>().split();
  format!("{}\t{}\t{}\t{}",value.as_u32(),std.0.join(","),lo.as_u64(),hi.as_u64())
 },
 "F"=>{
  let mut sink=Sink{fail:num(parts[6]),..Default::default()};let index=num(parts[4]);let is_debug=parts[5]=="1";
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||{
   let bytes=unhex(parts[3]);
   match parts[2]{
    "B"=>debug(&mut sink,index,&ByteSymbol::intern(&bytes)),
    "S"=>render(&mut sink,index,is_debug,&Symbol::intern(std::str::from_utf8(&bytes).unwrap())),
    "I"=>render(&mut sink,index,is_debug,&Ident{name:Symbol::intern(std::str::from_utf8(&bytes).unwrap()),span:DUMMY_SP}),
    "N"=>render(&mut sink,index,is_debug,&MacroRulesNormalizedIdent::new(Ident{name:Symbol::intern(std::str::from_utf8(&bytes).unwrap()),span:DUMMY_SP})),
    "P"=>display(&mut sink,index,&IdentPrinter::new(Symbol::intern(std::str::from_utf8(&bytes).unwrap()),mode(parts[7]),None)),
    "A"=>{
     let dollar=Symbol::intern(&text(parts[8]));rustc_span::hygiene::update_dollar_crate_names(|_|dollar);
     display(&mut sink,index,&IdentPrinter::for_ast_ident(Ident{name:Symbol::intern(std::str::from_utf8(&bytes).unwrap()),span:DUMMY_SP},mode(parts[7])))
    },_=>panic!("unknown format kind"),
   }
  }));
  let status=match result{Ok(Ok(()))=>"ok",Ok(Err(_))=>"error",Err(_)=>"panic"};
  format!("{}\t{}\t{}",status,hex(sink.rendered.as_bytes()),sink.events.join(","))
 },
 "C"=>{
  let bytes=unhex(parts[3]);let mut encoder=MemEncoder::new();
  match parts[2]{"S"=>Symbol::intern(std::str::from_utf8(&bytes).unwrap()).encode(&mut encoder),"B"=>ByteSymbol::intern(&bytes).encode(&mut encoder),"I"=>Ident{name:Symbol::intern(std::str::from_utf8(&bytes).unwrap()),span:Span::new(BytePos(num(parts[4])),BytePos(num(parts[5])),rustc_span::hygiene::SyntaxContext::root(),None)}.encode(&mut encoder),_=>panic!("bad codec kind")}
  let raw=encoder.finish();let mut framed=raw.clone();framed.extend_from_slice(MAGIC_END_BYTES);let mut decoder=MemDecoder::new(&framed,0).unwrap();
  let decoded=match parts[2]{"S"=>{let value:Symbol=Decodable::decode(&mut decoder);hex(value.as_str().as_bytes())},"B"=>{let value:ByteSymbol=Decodable::decode(&mut decoder);hex(value.as_byte_str())},"I"=>{let value:Ident=Decodable::decode(&mut decoder);format!("{}:{}:{}:{}",hex(value.name.as_str().as_bytes()),value.span.lo().0,value.span.hi().0,value.span.ctxt().as_u32())},_=>unreachable!()};
  format!("{}\t{}\t{}",hex(&raw),decoded,decoder.position())
 },
 "J"=>{
  let value=num(parts[2]);let result=std::panic::catch_unwind(||Symbol::new(value));result.map_or("panic".into(),|v|v.as_u32().to_string())
 },
 "E"=>{
  let text=text(parts[2]);let value=Ident{name:Symbol::intern(&text),span:DUMMY_SP};
  format!("{}\t{}\t{}",hex(value.without_first_quote().as_str().as_bytes()),hex(value.name.to_ident_string().as_bytes()),hex(Ident::dummy().as_str().as_bytes()))
 },
 "L"=>{
  let c=char::from_u32(num(parts[2])).unwrap();let value=symbol::sym::character(c);
  format!("{}\t{}\t{}",value.as_u32(),hex(value.as_str().as_bytes()),symbol::sym::ascii_letter_digit(c).map_or("none".into(),|s|s.as_u32().to_string()))
 },
 "G"=>{
  let value=match parts[2]{"i128"=>symbol::sym::integer(num::<i128>(parts[3])),"u128"=>symbol::sym::integer(num::<u128>(parts[3])),"isize"=>symbol::sym::integer(num::<isize>(parts[3])),"usize"=>symbol::sym::integer(num::<usize>(parts[3])),_=>panic!("bad integer kind")};
  format!("{}\t{}",value.as_u32(),hex(value.as_str().as_bytes()))
 },
 "H"=>{
  use rustc_span::hygiene::{ExpnData,ExpnKind,LocalExpnId,MacroKind,SyntaxContext,Transparency};
  let mut ctxt=SyntaxContext::root();
  for(i,mark)in parts[2].chars().enumerate(){
   let ed=[Edition::Edition2015,Edition::Edition2018,Edition::Edition2021,Edition::Edition2024,Edition::EditionFuture][(num::<usize>(parts[1])+i+1)%5];
   let data=ExpnData::default(ExpnKind::Macro(MacroKind::Bang,Symbol::intern(&format!("symbol_macro_{i}"))),DUMMY_SP,ed,None,None);
   let id=LocalExpnId::fresh(data,ProbeContext{hash_spans:false,spans:vec![]});
   let transparency=match mark{'0'=>Transparency::Transparent,'1'=>Transparency::SemiOpaque,'2'=>Transparency::Opaque,_=>panic!("bad mark")};ctxt=ctxt.apply_mark(id.to_expn_id(),transparency);
  }
  let ident=Ident::new(Symbol::intern(&text(parts[3])),Span::new(BytePos(1),BytePos(2),ctxt,None));
  let pos=ident.with_span_pos(Span::new(BytePos(90),BytePos(99),SyntaxContext::root(),None));
  let normalized=ident.normalize_to_macros_2_0();let rules=ident.normalize_to_macro_rules();let wrapped=MacroRulesNormalizedIdent::new(ident);
  let mut hasher=TraceHasher::default();ident.hash(&mut hasher);
  format!("{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",ctxt.as_u32(),normalized.span.ctxt().as_u32(),rules.span.ctxt().as_u32(),wrapped.ident()==rules,ident==pos,ident==Ident::with_dummy_span(ident.name),pos.span.lo().0,pos.span.hi().0,pos.span.ctxt().as_u32(),hex(ident.to_string().as_bytes()),hex(format!("{ident:?}").as_bytes()),hex(normalized.to_string().as_bytes()),hasher.0.join(","))
 },
 "W"=>{
  let name=Symbol::intern(&text(parts[2]));let ident=Ident{name,span:Span::new(BytePos(num(parts[3])),BytePos(num(parts[4])),rustc_span::hygiene::SyntaxContext::root(),None)};
  let mut hcx=ProbeContext{hash_spans:true,spans:vec![]};let mut hasher=StableHasher::new();hasher.write(&unhex(parts[5]));ident.stable_hash(&mut hcx,&mut hasher);let (lo,hi)=hasher.finish::<Fingerprint>().split();
  format!("{}\t{}\t{}",hcx.spans.iter().map(|v|v.to_string()).collect::<Vec<_>>().join(","),lo.as_u64(),hi.as_u64())
 },
 "X"=>{
  BUG_EVENTS.lock().unwrap().clear();BUG_EMIT.store(parts[3]=="1",std::sync::atomic::Ordering::SeqCst);let old=rustc_span::macros::EMIT_BUG_DIAGNOSTIC.swap(&OBSERVE_BUG);
  let span=Span::new(BytePos(1),BytePos(2),rustc_span::hygiene::SyntaxContext::root(),None);let empty=Ident{name:symbol::sym::empty,span};
  let result=std::panic::catch_unwind(||match parts[2]{
   "new0"=>{Ident::new(symbol::sym::empty,span);},
   "new1"=>{Ident::new(symbol::sym::empty,span);},
   "dummy"=>{Ident::with_dummy_span(symbol::sym::empty);},
   "from"=>{Ident::from_str("");},
   "from_span"=>{Ident::from_str_and_span("",span);},
   "spanpos"=>{empty.with_span_pos(Span::new(BytePos(90),BytePos(99),rustc_span::hygiene::SyntaxContext::root(),None));},
   "quote"=>{Ident{name:Symbol::intern("'"),span}.without_first_quote();},
   "macro20"=>{empty.normalize_to_macros_2_0();},
   "rules"=>{empty.normalize_to_macro_rules();},
   "normalizer"=>{MacroRulesNormalizedIdent::new(empty);},_=>panic!("unknown constructor")
  });
  rustc_span::macros::EMIT_BUG_DIAGNOSTIC.swap(old);
  let status=match result{Ok(())=>"ok",Err(value)=>if value.is::<rustc_span::macros::ExplicitBug>(){"panic:explicit"}else if value.is::<String>(){"panic:string"}else{"panic:other"}};
  format!("{}\t{}",status,BUG_EVENTS.lock().unwrap().join(";"))
 },
 "P"=>{
  let a=Symbol::new(num(parts[2]));let b=Symbol::new(num(parts[3]));let c=Symbol::new(num(parts[4]));
  let clamp=std::panic::catch_unwind(||a.clamp(b,c)).map_or("panic".into(),|v|v.as_u32().to_string());
  let mut raw=RawTraceHasher::default();Symbol::hash_slice(&[a,b,c],&mut raw);
  let ba=ByteSymbol::new(a.as_u32());let bb=ByteSymbol::new(b.as_u32());let bc=ByteSymbol::new(c.as_u32());let mut bytes=RawTraceHasher::default();ByteSymbol::hash_slice(&[ba,bb,bc],&mut bytes);
  format!("{:?}\t{}\t{}\t{}\t{}\t{}",a.partial_cmp(&b),a.min(b).as_u32(),a.max(b).as_u32(),clamp,raw.0.join(","),bytes.0.join(","))
 },
 _=>panic!("unknown request"),
 }
}
fn main(){
 assert_eq!(std::char::UNICODE_VERSION,(18,0,0));std::panic::set_hook(Box::new(|_|{}));
 for line in io::stdin().lock().lines(){
  let line=line.unwrap();assert!(line.len()<1<<20);let parts=line.split('\t').collect::<Vec<_>>();
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||if parts[0]=="O"{owned_context(parts[2],parts[3]=="1")}else if parts[0]=="Z"{
   let extras:Vec<&'static str>=if parts[2]=="-"{vec![]}else{parts[2].split(',').map(|value|&*Box::leak(text(value).into_boxed_str())).collect()};
   rustc_span::create_session_globals_then(edition(parts[1]),&extras,None,||process(&["I",parts[1],parts[3]]))
  }else{rustc_span::create_session_globals_then(edition(parts[1]),&[],None,||process(&parts))}));
  match result{Ok(value)=>println!("ok\t{value}"),Err(value)=>if parts[0]=="Z"{if let Some(message)=value.downcast_ref::<String>(){println!("panic\t{}",hex(message.as_bytes()))}else{println!("panic")}}else{println!("panic")}}
 }
}
