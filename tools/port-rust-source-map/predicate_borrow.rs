#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual HRTB FnMut(&char) temporary/environment borrows through full SourceMap.
use rustc_span::{BytePos,Span,FileName};use rustc_span::source_map::{SourceMap,FilePathMapping};
use std::{cell::{Cell,RefCell},io::{self,BufRead},sync::atomic::{AtomicUsize,Ordering}};
static ENV:AtomicUsize=AtomicUsize::new(0);static CHARACTER:AtomicUsize=AtomicUsize::new(0);
thread_local!{static LOG:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};}
fn log(s:String){LOG.with(|v|v.borrow_mut().push(s));}
struct Env{count:Cell<usize>,limit:usize,tag:Vec<u8>}
impl Env{fn predicate(&self,value:&char)->bool{assert_eq!(self.tag,[1,2]);let address=self as*const Self as usize;let old=ENV.swap(address,Ordering::SeqCst);let character=value as*const char as usize;let previous=CHARACTER.swap(character,Ordering::SeqCst);let n=self.count.get();self.count.set(n+1);let result=n<self.limit;log(format!("call:{:x}:{result}:{}:{}:{}",*value as u32,old==0||old==address,previous==0||previous==character,std::thread::panicking()));result}}
impl Drop for Env{fn drop(&mut self){let old=ENV.load(Ordering::SeqCst);log(format!("drop:{}:{}:{}",self.count.get(),old==0||old==self as*const Self as usize,std::thread::panicking()));}}
fn unhex(s:&str)->String{String::from_utf8((0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect()).unwrap()}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn run(p:&[&str])->String{ENV.store(0,Ordering::SeqCst);CHARACTER.store(0,Ordering::SeqCst);LOG.with(|v|v.borrow_mut().clear());let sm=SourceMap::new(FilePathMapping::empty());sm.new_source_file(FileName::Custom("borrow.rs".into()),unhex(p[1]));let env=Env{count:Cell::new(0),limit:p[4].parse().unwrap(),tag:vec![1,2]};let out=sm.span_take_while(Span::with_root_ctxt(BytePos(p[2].parse().unwrap()),BytePos(p[3].parse().unwrap())),move|value|env.predicate(value));log(format!("out:{}:{}",out.lo().0,out.hi().0));LOG.with(|v|v.borrow().join("|"))}
fn main(){rustc_data_structures::sync::set_dyn_thread_safe_mode(true);rustc_span::create_default_session_globals_then(||{for line in io::stdin().lock().lines(){let line=line.unwrap();let p:Vec<_>=line.split('\t').collect();println!("{}\t{}",p[0],hex(&run(&p)));}});}
