#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Client of the complete unmodified pinned rustc_span crate, not a replacement.
use std::{fmt,io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{source_map::{SourceMap,FilePathMapping},BytePos,Span,FileName,SpanSnippetError};
fn bytes(s:&str)->Vec<u8>{(0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect()}
fn text(s:&str)->String{String::from_utf8(bytes(s)).unwrap()}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn span(s:Span)->String{format!("{}:{}",s.lo().0,s.hi().0)}
fn error(e:SpanSnippetError)->String{match e{SpanSnippetError::IllFormedSpan(_)=>"ill".into(),SpanSnippetError::DistinctSources(_)=>"distinct".into(),SpanSnippetError::MalformedForSourcemap(v)=>format!("malformed:{}:{}:{}",v.source_len,v.begin_pos.0,v.end_pos.0),SpanSnippetError::SourceNotAvailable{..}=>"unavailable".into()}}
fn string_result(v:Result<String,SpanSnippetError>)->String{match v{Ok(s)=>format!("ok:{}",hex(s)),Err(e)=>format!("err:{}",error(e))}}
fn span_result(v:Result<Span,SpanSnippetError>)->String{match v{Ok(s)=>format!("ok:{}",span(s)),Err(e)=>format!("err:{}",error(e))}}
fn span_option(v:Option<Span>)->String{v.map_or("none".into(),|s|span(s))}
fn run(p:&[&str])->String{
 let src=text(p[1]);let lo:u32=p[2].parse().unwrap();let hi:u32=p[3].parse().unwrap();let op=p[4];let arg=text(p[5]);
 let sm=SourceMap::new(FilePathMapping::empty());let file=sm.new_source_file(FileName::Custom("input.rs".into()),src);let s=Span::with_root_ctxt(BytePos(lo),BytePos(hi));
 let c=arg.chars().next().unwrap_or(' ');
 match op{
 "snippet"=>string_result(sm.span_to_snippet(s)),"prev"=>string_result(sm.span_to_prev_source(s)),"next"=>string_result(sm.span_to_next_source(s)),
 "accessible"=>sm.is_span_accessible(s).to_string(),"multiline"=>sm.is_multiline(s).to_string(),"imported"=>sm.is_imported(s).to_string(),
 "diag"=>hex(sm.span_to_diagnostic_string(s)),"short"=>hex(sm.span_to_short_string(s,rustc_span::RemapPathScopeComponents::DIAGNOSTICS)),
 "indent"=>sm.indentation_before(s).map_or("none".into(),|s|format!("some:{}",hex(s))),"margin"=>sm.span_to_margin(s).map_or("none".into(),|n|n.to_string()),"line_empty"=>sm.is_line_before_span_empty(s).to_string(),
 "lines"=>match sm.span_to_lines(s){Err(_)=>"err:distinct".into(),Ok(v)=>format!("ok:{}:{}",v.file.start_pos.0,v.lines.iter().map(|l|format!("{}/{}/{}",l.line_index,l.start_col.0,l.end_col.0)).collect::<Vec<_>>().join(","))},
 "start"=>span(sm.start_point(s)),"end"=>span(sm.end_point(s)),"point"=>span(sm.next_point(s)),"follow"=>span_option(sm.span_followed_by(s,&arg)),
 "extend_prev"=>span(sm.span_extend_to_prev_char(s,c,false)),"extend_prev_nl"=>span(sm.span_extend_to_prev_char(s,c,true)),"extend_before"=>span(sm.span_extend_to_prev_char_before(s,c,false)),"extend_before_nl"=>span(sm.span_extend_to_prev_char_before(s,c,true)),
 "extend_next"=>span(sm.span_extend_to_next_char(s,c,false)),"extend_next_nl"=>span(sm.span_extend_to_next_char(s,c,true)),"extend_line"=>span(sm.span_extend_to_line(s)),"until"=>span(sm.span_until_char(s,c)),"through"=>span(sm.span_through_char(s,c)),"head"=>span(sm.guess_head_span(s)),
 "while"=>span_result(sm.span_extend_while(s,|c|c.is_whitespace())),"prev_while"=>span_result(sm.span_extend_prev_while(s,|c|c.is_whitespace())),"take"=>span(sm.span_take_while(s,|c|c.is_whitespace())),"until_white"=>span(sm.span_until_whitespace(s)),"until_nonwhite"=>span(sm.span_until_non_whitespace(s)),"wrapped"=>sm.span_wrapped_by_angle_or_parentheses(s).to_string(),
 "prev_str"=>span_option(sm.span_extend_to_prev_str(s,&arg,false,false)),"prev_str_ws"=>span_option(sm.span_extend_to_prev_str(s,&arg,false,true)),"prev_str_nl"=>span_option(sm.span_extend_to_prev_str(s,&arg,true,false)),"prev_str_nl_ws"=>span_option(sm.span_extend_to_prev_str(s,&arg,true,true)),
 "metadata"=>format!("{}:{}:{}:{}:{}:{}",file.start_pos.0,file.normalized_source_len.0,file.unnormalized_source_len,sm.count_lines(),file.lines().iter().map(|v|v.0.to_string()).collect::<Vec<_>>().join(","),file.multibyte_chars.iter().map(|v|format!("{}/{}",v.pos.0,v.bytes)).collect::<Vec<_>>().join(",")),
 _=>panic!("unknown operation")
 }
}
fn main(){std::panic::set_hook(Box::new(|_|{}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);rustc_span::create_default_session_globals_then(||{for line in io::stdin().lock().lines(){let line=line.unwrap();let parts:Vec<_>=line.split('\t').collect();let id=parts[0];let output=catch_unwind(AssertUnwindSafe(||run(&parts)));match output{Ok(v)=>println!("{id}\tok\t{}",hex(v)),Err(_)=>println!("{id}\tpanic\t")}}});}
