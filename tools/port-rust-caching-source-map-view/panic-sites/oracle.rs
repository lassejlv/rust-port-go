#![feature(rustc_private, panic_can_unwind, panic_internals)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Public safe operations on the entire unchanged pinned compiler crate.
use std::{cell::{Cell,RefCell},panic::{catch_unwind,AssertUnwindSafe},sync::Arc};
use rustc_span::{BytePos,CachingSourceMapView,FileName,RelativeBytePos,SourceFile,SourceFileHashAlgorithm,SourceFileLines,SpanData,SyntaxContext};
use rustc_span::source_map::{FilePathMapping,SourceMap};
use rustc_span::def_id::CrateNum;
use rustc_data_structures::sync::FreezeLock;
thread_local!{
 static MAP:Cell<Option<&'static SourceMap>>=const{Cell::new(None)};
 static VIEW:RefCell<Option<CachingSourceMapView<'static>>>=const{RefCell::new(None)};
 static CALLER:Cell<u8>=const{Cell::new(0)};
}
fn label()->&'static str{CALLER.with(|c|match c.get(){0=>"A",1=>"B",_=>"nil"})}
fn counts()->String{MAP.with(|m|m.get().unwrap().files().iter().map(|f|Arc::strong_count(f).to_string()).collect::<Vec<_>>().join("_"))}
fn event(tag:&str){println!("E:{tag}:{}:{}:{}",label(),std::thread::panicking(),counts())}
fn hex(value:&str)->String{value.bytes().map(|b|format!("{b:02x}")).collect()}
fn import(map:&SourceMap,name:&str,len:u32,lines:&[u32]){
 let f=SourceFile::new(FileName::Custom(name.into()),"seed".into(),SourceFileHashAlgorithm::Md5,None).unwrap();
 drop(map.new_imported_source_file(f.name,f.src_hash,f.checksum_hash,f.stable_id,len,f.unnormalized_source_len,CrateNum::ZERO,FreezeLock::new(SourceFileLines::Lines(lines.iter().copied().map(RelativeBytePos).collect())),f.multibyte_chars,f.normalized_pos,0));
}
fn point(tag:&str,pos:u32){VIEW.with(|v|{let mut slot=v.borrow_mut();match slot.as_mut().unwrap().byte_pos_to_line_and_col(BytePos(pos)){Some((file,line,col))=>event(&format!("{tag}:b:{}:{line}:{}",file.start_pos.0,col.0)),None=>event(&format!("{tag}:none"))}})}
fn span(lo:u32,hi:u32){VIEW.with(|v|{let mut slot=v.borrow_mut();let data=SpanData{lo:BytePos(lo),hi:BytePos(hi),ctxt:SyntaxContext::root(),parent:None};match slot.as_mut().unwrap().span_data_to_lines_and_cols(&data){Some((file,ll,lc,hl,hc))=>event(&format!("span:{}:{ll}:{}:{hl}:{}",file.start_pos.0,lc.0,hc.0)),None=>event("span:none")}})}
fn attempt(tag:&str,body:impl FnOnce()){let caught=catch_unwind(AssertUnwindSafe(body)).is_err();println!("C:{tag}:{caught}:{}:{}",label(),counts())}
fn prepare(case:&str){
 let map=Box::leak(Box::new(SourceMap::new(FilePathMapping::empty())));MAP.with(|m|m.set(Some(map)));
 match case{
  "empty"=>return,
  "unwrap_byte"=>import(map,"A",10,&[]),
  "unwrap_span"=>import(map,"A",10,&[2]),
  "lo_upper"=>{import(map,"A",0,&[]);import(map,"B",10,&[0,u32::MAX]);},
  "hi_upper"=>{import(map,"A",0,&[]);import(map,"B",10,&[0,5,u32::MAX]);},
  "stale_hi_lower"|"stale_contains_lo"|"stale_hit"=>{import(map,"A",10,&[0]);import(map,"B",10,if case=="stale_contains_lo"{&[u32::MAX]}else{&[7]});},
  "ok"=>import(map,"A",10,&[0,5]),_=>unreachable!(),
 }
 VIEW.with(|v|*v.borrow_mut()=Some(CachingSourceMapView::new(map)));
 if case.starts_with("stale_"){attempt("prime0",||point("prime0",0));attempt("prime1",||point("prime1",13));}
 event("prepared");
}
struct Local;impl Drop for Local{fn drop(&mut self){event("local")}}
fn operate(case:&str){let _local=Local;event("before");match case{
 "empty"=>{MAP.with(|m|{let _view=CachingSourceMapView::new(m.get().unwrap());event("new")});},
 "unwrap_byte"=>point("point",0),"unwrap_span"=>span(0,1),"lo_upper"=>span(1,1),"hi_upper"=>span(1,6),
 "stale_hi_lower"|"stale_contains_lo"=>span(0,10),"stale_hit"=>point("point",0),"ok"=>point("point",1),_=>unreachable!(),
}}
fn outer_panic(){panic!("outer")}
struct Outer(String);impl Drop for Outer{fn drop(&mut self){event("outer-drop");attempt("operation",||operate(&self.0));event("outer-end")}}
fn main(){
 let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let case=args[2].clone();
 std::panic::set_hook(Box::new(move|info|{let l=info.location().unwrap();let text=info.payload().downcast_ref::<String>().map(|s|s.as_str()).or_else(||info.payload().downcast_ref::<&str>().copied()).unwrap_or("opaque");if label()!="nil"{println!("H:{}:{}:{}:{}:{}:{}:{}:{}:{}:{}",label(),if info.payload().is::<String>(){"owned_string"}else if info.payload().is::<&str>(){"static_str"}else{"opaque"},hex(text),hex(l.file()),l.line(),l.column(),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),counts());}if mode=="processing"&&text=="outer"{operate(&case)}}));
 rustc_span::create_default_session_globals_then(||{
  prepare(&args[2]);CALLER.with(|c|c.set(match args[3].as_str(){"A"=>0,"B"=>1,"nil"=>2,_=>unreachable!()}));
  match args[1].as_str(){"normal"=>attempt("operation",||operate(&args[2])),"inherited"=>{let caught=catch_unwind(AssertUnwindSafe(||{let _outer=Outer(args[2].clone());outer_panic()})).is_err();println!("C:outer:{caught}:{}:{}",label(),counts());},"processing"=>{let caught=catch_unwind(outer_panic).is_err();println!("C:outer:{caught}:{}:{}",label(),counts());},_=>unreachable!()}
  event("final");VIEW.with(|v|drop(v.borrow_mut().take()));event("closed");
 });
}
