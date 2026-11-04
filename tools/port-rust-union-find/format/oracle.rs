// SPDX-License-Identifier: MIT OR Apache-2.0
// Only a trait client/protocol wrapper: the entire original union_find.rs,
// idx.rs, slice.rs, vec.rs, and original union_find/tests.rs are copied unchanged.
#![feature(formatting_options)]
#![allow(dead_code)]
extern crate original_index as rustc_index;
mod upstream { pub mod union_find; }

use std::{cell::RefCell,fmt,fmt::Write,hash::{Hash,Hasher},io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe},rc::Rc};
use rustc_index::Idx;
use upstream::union_find::UnionFind;

#[derive(Default)]struct Hooks {warm:bool,cells:Vec<usize>,slot:usize,events:Vec<String>,counts:[usize;5],panic_kind:usize,panic_at:usize,catch:bool,variant:usize}
thread_local!{static HOOKS:RefCell<Hooks>=RefCell::new(Hooks::default());}
fn hex(s:&str)->String {s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn event(kind:usize,text:String){
 let panic=HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.push(text);h.counts[kind]+=1;h.panic_kind==kind&&h.counts[kind]==h.panic_at});
 if panic{panic!("formatter probe")}
}
fn flags(f:&fmt::Formatter<'_>)->String{
 let width=f.width().map(|n|n.to_string()).unwrap_or("-".to_owned());let precision=f.precision().map(|n|n.to_string()).unwrap_or("-".to_owned());
 let align=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
 format!("{}/{}/{}/{}/{}/{}/{}/{}",f.alternate()as u8,width,precision,f.fill()as u32,align,f.sign_plus()as u8,f.sign_minus()as u8,f.sign_aware_zero_pad()as u8)
}
#[derive(Clone,Copy)]struct Key(usize,&'static[u8]);
impl PartialEq for Key{fn eq(&self,b:&Self)->bool{self.0==b.0}}impl Eq for Key{}
impl Hash for Key{fn hash<H:Hasher>(&self,h:&mut H){self.0.hash(h)}}
impl Idx for Key{fn new(n:usize)->Self{Self(n,&[1,2])}fn index(self)->usize{self.0}}
struct Local(usize);
impl Drop for Local{fn drop(&mut self){event(4,format!("local:{}:{}",self.0,std::thread::panicking()as u8))}}
impl fmt::Debug for Key{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 if HOOKS.with(|h|h.borrow().warm){HOOKS.with(|h|h.borrow_mut().cells.push(self as *const Key as usize));return Ok(())}
 let _local=Local(self.0);
 let (same,slot,catch,variant)=HOOKS.with(|h|{let mut h=h.borrow_mut();let slot=h.slot;let same=h.cells.get(slot).copied()==Some(self as *const Key as usize);h.slot+=1;(same,h.slot,h.catch,h.variant)});
 event(1,format!("key:{}:{}:{}",self.0,flags(f),same as u8));
 let result=if variant!=0 {
  let text=if variant==1{["","\nfirst","a\n\nb\n"][slot%3]}else{["\n","λ\n","\n\n"][slot%3]};f.write_str(text)
 }else{
  let first=f.write_str("K");if !catch{first?;}fmt::Debug::fmt(&self.0,f)
 };
 if catch{Ok(())}else{result}
}}
#[derive(Default)]struct SinkState {calls:usize,fail_at:usize,text:String}
struct Sink<const CHAR:bool>(Rc<RefCell<SinkState>>);
impl<const CHAR:bool> fmt::Write for Sink<CHAR>{
 fn write_str(&mut self,text:&str)->fmt::Result{
  let call={let mut s=self.0.borrow_mut();s.calls+=1;s.calls};event(2,format!("write:{call}:s:{}",hex(text)));
  let mut s=self.0.borrow_mut();if call==s.fail_at{return Err(fmt::Error)}s.text.push_str(text);Ok(())
 }
 fn write_char(&mut self,c:char)->fmt::Result{
  if !CHAR{return self.write_str(c.encode_utf8(&mut [0;4]))}
  let call={let mut s=self.0.borrow_mut();s.calls+=1;s.calls};event(3,format!("write:{call}:c:{}",c as u32));
  let mut s=self.0.borrow_mut();if call==s.fail_at{return Err(fmt::Error)}s.text.push(c);Ok(())
 }
}
fn options(style:usize)->fmt::FormattingOptions{
 let mut o=fmt::FormattingOptions::new();
 match style{
  1=>{o.alternate(true);},2=>{o.debug_as_hex(Some(fmt::DebugAsHex::Lower));},3=>{o.debug_as_hex(Some(fmt::DebugAsHex::Upper));},
  4=>{o.debug_as_hex(Some(fmt::DebugAsHex::Lower)).alternate(true);},5=>{o.debug_as_hex(Some(fmt::DebugAsHex::Upper)).alternate(true);},
  6=>{o.width(Some(10));},7=>{o.width(Some(10)).fill('*').align(Some(fmt::Alignment::Left));},8=>{o.width(Some(10)).fill('λ').align(Some(fmt::Alignment::Center));},
  9=>{o.width(Some(10)).sign(Some(fmt::Sign::Plus)).sign_aware_zero_pad(true);},10=>{o.sign(Some(fmt::Sign::Minus));},11=>{o.precision(Some(3));},
  12|13=>{o.width(Some(10)).precision(Some(3)).fill('*').align(Some(fmt::Alignment::Right)).alternate(style==13);},14=>{o.width(Some(10)).fill('\0').align(Some(fmt::Alignment::Center));},_=>{}
 };o
}
fn replay<const CHAR:bool>(p:&[usize])->String{
 let mut sets=UnionFind::<Key>::new(p[2]);if p[3]!=0{for i in 1..p[2]{sets.unify(Key::new(0),Key::new(i));}}
 HOOKS.with(|h|*h.borrow_mut()=Hooks{warm:true,..Hooks::default()});
 let mut discard=String::new();let mut warm=fmt::Formatter::new(&mut discard,fmt::FormattingOptions::new());fmt::Debug::fmt(&sets,&mut warm).unwrap();
 HOOKS.with(|h|{let mut h=h.borrow_mut();h.warm=false;h.panic_kind=p[6];h.panic_at=p[7];h.catch=p[8]!=0;h.variant=p[9];});
 let state=Rc::new(RefCell::new(SinkState{fail_at:p[5],..SinkState::default()}));let mut sink=Sink::<CHAR>(state.clone());let mut f=fmt::Formatter::new(&mut sink,options(p[1]));let mut results=Vec::new();
 for _ in 0..p[10]{
  HOOKS.with(|h|h.borrow_mut().slot=0);
  let status=match catch_unwind(AssertUnwindSafe(||fmt::Debug::fmt(&sets,&mut f))){Err(_)=>"panic",Ok(Err(_))=>"error",Ok(Ok(_))=>"ok"};
  results.push(status);HOOKS.with(|h|h.borrow_mut().events.push(format!("result:{status}:{}",flags(&f))));
 }
 let events=HOOKS.with(|h|h.borrow().events.join("\n"));let state=state.borrow();let roots=sets.snapshot().raw.iter().map(|k|k.0.to_string()).collect::<Vec<_>>().join(",");
 format!("{}\t{}\t{}\t{}\t{}",hex(&events),hex(&state.text),state.calls,results.join(","),roots)
}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));
 for line in io::stdin().lock().lines(){let line=line.unwrap();let p=line.split_whitespace().map(|x|x.parse::<usize>().unwrap()).collect::<Vec<_>>();assert_eq!(p.len(),11);let result=if p[4]!=0{replay::<true>(&p)}else{replay::<false>(&p)};println!("{result}");}
}
