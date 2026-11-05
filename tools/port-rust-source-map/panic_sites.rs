#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Calls actual full SourceMap; retained hook payloads expose internal raises.
use std::{io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe},sync::Mutex,path::PathBuf};
use rustc_span::{source_map::{SourceMap,FilePathMapping},BytePos,Span,FileName,RealFileName};
static EVENTS:Mutex<Vec<String>>=Mutex::new(Vec::new());
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn text(s:&str)->String{String::from_utf8((0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect()).unwrap()}
fn run(p:&[&str]){

 if p[4]=="getmap_empty"{if let Err(e)=std::thread::spawn(||{let _=rustc_span::source_map::get_source_map();}).join(){std::panic::resume_unwind(e)};return}
 if p[4]=="assert_id"{let a=rustc_span::SourceFile::new(FileName::Custom("assert-a".into()),"".into(),rustc_span::SourceFileHashAlgorithm::Md5,None).unwrap();let b=rustc_span::SourceFile::new(FileName::Custom("assert-b".into()),"".into(),rustc_span::SourceFileHashAlgorithm::Md5,None).unwrap();debug_assert_eq!(a.stable_id,b.stable_id);return}
 let sm=SourceMap::new(FilePathMapping::empty());let src=text(p[1]);let lo:usize=p[2].parse().unwrap();let hi:usize=p[3].parse().unwrap();
 if p[4]=="idx_empty"{let _=sm.lookup_source_file_idx(BytePos(lo as u32));return}
 if p[4]=="local_expect"{let name=RealFileName::from_virtual_path(&PathBuf::from("virtual"));let _=FilePathMapping::empty().to_real_filename(&name,PathBuf::from("local"));return}
 if p[4]=="doctest"{let offset:isize=p[5].parse().unwrap();let _=sm.doctest_offset_line(&FileName::DocTest(RealFileName::from_virtual_path(&PathBuf::from("doc")),offset),lo);return}
 if p[4].starts_with("max_"){let length:u32=src.parse().unwrap();import(&sm,"max-a",length);let span=Span::with_root_ctxt(BytePos(lo as u32),BytePos(hi as u32));match p[4]{"max_start"=>{let _=sm.start_point(span);},"max_end"=>{let _=sm.end_point(span);},"max_next"=>{let _=sm.next_point(span);},"max_import"=>{import(&sm,"max-b",0);},_=>unreachable!()};return}
 if p[4].starts_with("width_"){let f=rustc_span::SourceFile::new(FileName::Custom("width".into()),src.clone(),rustc_span::SourceFileHashAlgorithm::Md5,None).unwrap();let foreign=sm.new_imported_source_file(f.name,f.src_hash,f.checksum_hash,f.stable_id,p[5].parse().unwrap(),f.unnormalized_source_len,rustc_span::def_id::CrateNum::from_u32(1),f.lines,f.multibyte_chars,f.normalized_pos,0);assert!(foreign.add_external_src(||Some(src)));let span=Span::with_root_ctxt(BytePos(lo as u32),BytePos(hi as u32));match p[4]{"width_next"=>{let _=sm.next_point(span);},"width_end"=>{let _=sm.end_point(span);},_=>unreachable!()};return}
 let _file=sm.new_source_file(FileName::Custom("input.rs".into()),src);let span=Span::with_root_ctxt(BytePos(lo as u32),BytePos(hi as u32));
 match p[4]{"indent"=>{let _=sm.indentation_before(span);},"while"=>{let _=sm.span_extend_while(span,|_|true);},"prev_while"=>{let _=sm.span_extend_prev_while(span,|_|true);},"start"=>{let _=sm.start_point(span);},_=>panic!("bad operation")}
}
fn import(sm:&SourceMap,name:&str,length:u32){let f=rustc_span::SourceFile::new(FileName::Custom(name.into()),"".into(),rustc_span::SourceFileHashAlgorithm::Md5,None).unwrap();sm.new_imported_source_file(f.name,f.src_hash,f.checksum_hash,f.stable_id,length,f.unnormalized_source_len,rustc_span::def_id::CrateNum::from_u32(1),f.lines,f.multibyte_chars,f.normalized_pos,0);}
fn main(){std::panic::set_hook(Box::new(|p|{let l=p.location().unwrap();let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};EVENTS.lock().unwrap().push(format!("{kind}|{}|{}|{}|{}",l.file(),l.line(),l.column(),hex(p.payload_as_str().unwrap_or("opaque"))));}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);rustc_span::create_default_session_globals_then(||{for line in io::stdin().lock().lines(){let line=line.unwrap();let parts:Vec<_>=line.split('\t').collect();EVENTS.lock().unwrap().clear();let caught=catch_unwind(AssertUnwindSafe(||run(&parts))).is_err();println!("{}\t{}\t{}",parts[0],caught,hex(EVENTS.lock().unwrap().join("\n")));}});}
