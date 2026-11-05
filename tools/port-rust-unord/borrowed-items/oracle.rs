// Calls actual unchanged borrowed Unord collection cloned/copied APIs.
#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::unord::{UnordItems,UnordBag,UnordSet,UnordMap};
use std::{cell::Cell,hash::{Hash,Hasher},io::{self,Write},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{family:usize,length:usize,clone_fail:isize,drop_fail:isize,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{family:0,length:0,clone_fail:-1,drop_fail:-1,resume:false})};static CLONES:Cell<usize>=const{Cell::new(0)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Value{value:u64,id:u64}
impl Clone for Value{fn clone(&self)->Self{let n=CLONES.get();CLONES.set(n+1);event(format!("clone:{}:{}",self.id,std::thread::panicking()));if n as isize==CONFIG.get().clone_fail{raise("clone")};Value{value:self.value,id:self.id+1000}}}
impl Drop for Value{fn drop(&mut self){event(format!("item:{}:{}",self.id,std::thread::panicking()));if self.id as isize==CONFIG.get().drop_fail{raise("item_drop")}}}
impl Hash for Value{fn hash<H:Hasher>(&self,h:&mut H){self.value.hash(h)}}impl PartialEq for Value{fn eq(&self,b:&Self)->bool{self.value==b.value}}impl Eq for Value{}
#[derive(Copy,Hash,Eq,PartialEq)]struct CopyValue(u64);impl Clone for CopyValue{fn clone(&self)->Self{event("UNEXPECTED copy_clone");*self}}
fn cloned<'a,I:Iterator<Item=&'a Value>>(items:UnordItems<&'a Value,I>,count:bool){if count{let n=items.cloned().count();event(format!("returned:{n}"))}else{let result=items.cloned().get_only();event(format!("returned:{}",result.as_ref().map(|v|v.id.to_string()).unwrap_or_else(||"none".into())));drop(result)}}
fn copied<'a,I:Iterator<Item=&'a CopyValue>>(items:UnordItems<&'a CopyValue,I>,count:bool){if count{let n=items.copied().count();event(format!("returned:{n}"))}else{let result=items.copied().get_only();event(format!("returned:{}",result.as_ref().map(|v|v.0.to_string()).unwrap_or_else(||"none".into())));drop(result)}}
struct RefInput<'a,T>(&'a T);
impl<'a,T>Iterator for RefInput<'a,T>{type Item=&'a T;
 fn next(&mut self)->Option<Self::Item>{raise("UNEXPECTED next")}
 fn count(self)->usize{event("count_override");777}
 fn fold<B,F:FnMut(B,Self::Item)->B>(self,mut init:B,mut f:F)->B{event("fold_override");init=f(init,self.0);f(init,self.0)}
}
fn execute(){let c=CONFIG.get();match c.family{
 0|1=>{let mut bag=UnordBag::new();for n in 0..c.length{bag.push(Value{value:n as u64,id:n as u64})};cloned(bag.items(),c.family==1)},
 2=>{let mut set=UnordSet::new();for n in 0..c.length{set.insert(Value{value:n as u64,id:n as u64});};cloned(set.items(),false)},
 3=>{let mut map=UnordMap::default();for n in 0..c.length{map.insert(Value{value:n as u64,id:n as u64},n as u64);};cloned(map.keys(),false)},
 4|5=>{let mut bag=UnordBag::new();for n in 0..c.length{bag.push(CopyValue(n as u64))};copied(bag.items(),c.family==5)},
 6=>{let mut set=UnordSet::new();for n in 0..c.length{set.insert(CopyValue(n as u64));};copied(set.items(),false)},
 7=>{let mut map=UnordMap::default();for n in 0..c.length{map.insert(CopyValue(n as u64),n as u64);};copied(map.keys(),false)},
 8=>{let value=Value{value:99,id:99};cloned(UnordItems::new(RefInput(&value)),true)},
 9=>{let value=CopyValue(99);copied(UnordItems::new(RefInput(&value)),true)},_=>unreachable!()
}}
fn body(){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(execute));let message=result.err().map(|v|v.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer;impl Drop for Outer{fn drop(&mut self){body()}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let n:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{family:n[0]as usize,length:n[1]as usize,clone_fail:n[2]as isize,drop_fail:n[3]as isize,resume:n[4]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body()}}));match mode.as_str(){"normal"=>body(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
