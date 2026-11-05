// Real arbitrary non-Copy Pos implementation and ordinary std ownership paths.
extern crate rustc_span;
use rustc_span::Pos;
use std::{cell::Cell,io::{self,Write}};
thread_local!{static NEXT:Cell<u32>=const{Cell::new(0)};static FAIL:Cell<u8>=const{Cell::new(0)};}
fn event(text:&str){eprintln!("EVENT:{text}");io::stderr().flush().unwrap();}
fn fail(which:u8){if FAIL.with(Cell::get)==which{panic!("pos")}}
struct P{value:Cell<usize>,id:u32,seen:Cell<usize>}
impl P{fn new(value:usize)->Self{let id=NEXT.with(|n|{n.set(n.get()+1);n.get()});Self{value:Cell::new(value),id,seen:Cell::new(0)}}fn same(&self)->bool{let pointer=self as *const Self as usize;let old=self.seen.replace(pointer);old==0||old==pointer}}
impl Drop for P{fn drop(&mut self){event(&format!("drop:{}:{}:{}",self.id,self.value.get(),std::thread::panicking()));}}
impl Pos for P{
 fn from_usize(n:usize)->Self{event(&format!("from_usize:{n}:{}",std::thread::panicking()));fail(1);Self::new(n)}
 fn to_usize(&self)->usize{event(&format!("to_usize:{}:{}:{}:{}",self.id,self.value.get(),std::thread::panicking(),self.same()));self.value.set(self.value.get()+1);fail(2);self.value.get()}
 fn from_u32(n:u32)->Self{event(&format!("from_u32:{n}:{}",std::thread::panicking()));fail(3);Self::new(n as usize)}
 fn to_u32(&self)->u32{event(&format!("to_u32:{}:{}:{}:{}",self.id,self.value.get(),std::thread::panicking(),self.same()));self.value.set(self.value.get()+1);fail(4);self.value.get()as u32}
}
fn run(){let a=P::from_usize(10);event(&format!("result_usize:{}",a.to_usize()));event(&format!("result_u32:{}",a.to_u32()));let b=P::from_u32(20);event(&format!("result_second:{}",b.to_u32()));}
fn attempt(){let caught=std::panic::catch_unwind(run).is_err();event(&format!("caught:{caught}"));}
struct Ambient;impl Drop for Ambient{fn drop(&mut self){event("ambient_drop");attempt();}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=&args[1];FAIL.with(|f|f.set(args[2].parse().unwrap()));
 if mode=="processing"{std::panic::set_hook(Box::new(|_|{event("ambient_hook");attempt();}));let caught=std::panic::catch_unwind(||panic!("ambient")).is_err();event(&format!("ambient_caught:{caught}"));}
 else{std::panic::set_hook(Box::new(|_|event("hook")));if mode=="normal"{attempt()}else{let caught=std::panic::catch_unwind(||{let _guard=Ambient;panic!("ambient")}).is_err();event(&format!("ambient_caught:{caught}"));}}
}
