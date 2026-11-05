// Actual public source enums and standard Box operations; addresses are compared
// only within one process. No allocator address/ABI is part of the protocol.
extern crate rustc_span;
use rustc_span::*;
use std::{fmt,fmt::Write,io::{self,Write as IOWrite}};
fn event(text:&str){eprintln!("EVENT:{text}");io::stderr().flush().unwrap();}
fn value(seed:u32)->DistinctSources{DistinctSources{begin:(FileName::Custom(format!("begin-{seed}")),BytePos(seed)),end:(FileName::Custom("end-é".to_owned()),BytePos(seed))}}
fn line_box(v:&SpanLinesError)->&DistinctSources{match v{SpanLinesError::DistinctSources(v)=>v}}
fn snippet_box(v:&SpanSnippetError)->&DistinctSources{match v{SpanSnippetError::DistinctSources(v)=>v,_=>unreachable!()}}
fn custom(v:&FileName)->&str{match v{FileName::Custom(s)=>s,_=>unreachable!()}}
fn identities(seed:u32){
 let boxed=Box::new(value(seed));let original=(&*boxed)as *const DistinctSources;let original_text=custom(&boxed.begin.0).as_ptr();let line=SpanLinesError::DistinctSources(boxed);
 event(&format!("line:{}",std::ptr::eq(line_box(&line),original)));
 let cloned=line.clone();event(&format!("clone_line:{}:{}",!std::ptr::eq(line_box(&cloned),original),custom(&line_box(&cloned).begin.0).as_ptr()!=original_text));event(&format!("line_eq:{}",line==cloned));
 let boxed=match line{SpanLinesError::DistinctSources(v)=>v};event(&format!("take:{}",std::ptr::eq(&*boxed,original)));let mut snippet=SpanSnippetError::DistinctSources(boxed);event(&format!("snippet:{}",std::ptr::eq(snippet_box(&snippet),original)));
 let cloned=snippet.clone();event(&format!("clone_snippet:{}",!std::ptr::eq(snippet_box(&cloned),original)));
 if let SpanSnippetError::DistinctSources(v)=&mut snippet{v.begin.1=BytePos(seed^1)};event(&format!("mut:{}:{}",snippet_box(&snippet).begin.1.0==seed^1,snippet==cloned));
 let mut target=SpanSnippetError::DistinctSources(Box::new(value(9)));let old=snippet_box(&target)as *const DistinctSources;target.clone_from(&snippet);event(&format!("clone_from:{}:{}",!std::ptr::eq(snippet_box(&target),old),!std::ptr::eq(snippet_box(&target),original)));
 let boxed=match snippet{SpanSnippetError::DistinctSources(v)=>v,_=>unreachable!()};let line=SpanLinesError::DistinctSources(boxed);event(&format!("take_back:{}",std::ptr::eq(line_box(&line),original)));
}
struct Owner{value:SpanSnippetError,expected:*const DistinctSources}
impl Drop for Owner{fn drop(&mut self){event(&format!("drop:{}:{}",std::ptr::eq(snippet_box(&self.value),self.expected),std::thread::panicking()));}}
struct Sink{calls:isize,fail:isize,panic:bool}
impl fmt::Write for Sink{fn write_str(&mut self,_:&str)->fmt::Result{let n=self.calls;self.calls+=1;event(&format!("write:{n}:{}",std::thread::panicking()));if n==self.fail{if self.panic{panic!("sink")};Err(fmt::Error)}else{Ok(())}}}
fn format_owned(fail:isize,panic:bool){let value=SpanSnippetError::DistinctSources(Box::new(value(31)));let pointer=snippet_box(&value)as *const DistinctSources;let owner=Owner{value,expected:pointer};let result=write!(&mut Sink{calls:0,fail,panic},"{:?}",owner.value);event(&format!("return:{}:{}",result.is_ok(),std::ptr::eq(snippet_box(&owner.value),pointer)));}
fn attempt(fail:isize,panic:bool){let caught=std::panic::catch_unwind(||format_owned(fail,panic)).is_err();event(&format!("caught:{caught}"));}
struct Ambient{fail:isize,panic:bool}impl Drop for Ambient{fn drop(&mut self){event("ambient_drop");attempt(self.fail,self.panic);}}
fn main(){let args:Vec<_>=std::env::args().collect();if args[1]=="identity"{identities(args[2].parse().unwrap());return}let mode=&args[1];let fail=args[2].parse().unwrap();let panic=args[3].parse().unwrap();
 if mode=="processing"{std::panic::set_hook(Box::new(move|_|{event("ambient_hook");attempt(fail,panic);}));let caught=std::panic::catch_unwind(||panic!("ambient")).is_err();event(&format!("ambient_caught:{caught}"));}
 else{std::panic::set_hook(Box::new(|_|event("hook")));if mode=="normal"{attempt(fail,panic)}else{let caught=std::panic::catch_unwind(||{let _guard=Ambient{fail,panic};panic!("ambient")}).is_err();event(&format!("ambient_caught:{caught}"));}}
}
