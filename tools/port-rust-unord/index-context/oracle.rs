// Caller-owned observer types; all Bag/Vec algorithms and cleanup are native.
#![feature(panic_can_unwind,panic_internals,trusted_len)]
#![allow(internal_features)]
use rustc_data_structures::unord::{UnordBag,UnordItems};
use std::{cell::Cell,io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind},ptr::NonNull};
thread_local!{static CURRENT:Cell<&'static str>=const{Cell::new("none")};static ACTION:Cell<&'static str>=const{Cell::new("")};static RESUME:Cell<bool>=const{Cell::new(false)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if RESUME.get(){panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Element(u8);
impl Clone for Element{fn clone(&self)->Self{event(format!("clone:{}:{}:{}",self.0,CURRENT.get(),std::thread::panicking()));if self.0==2&&(ACTION.get().starts_with("clone_fail")||ACTION.get()=="clone_from_fail"){raise("clone")};Self(self.0)}}
impl Drop for Element{fn drop(&mut self){event(format!("drop:{}:{}:{}",self.0,CURRENT.get(),std::thread::panicking()));if (self.0==10&&ACTION.get()=="clone_from_old_drop")||self.0==1&&(ACTION.get()=="drop_fail"||ACTION.get()=="clone_fail_drop"||ACTION.get()=="iter_drop_fail"){raise("element_drop")}}}
struct Zst;
impl Drop for Zst{fn drop(&mut self){event(format!("zst_drop:{}:{}",CURRENT.get(),std::thread::panicking()));}}
struct Input(u8);
impl Iterator for Input{type Item=Element;fn next(&mut self)->Option<Element>{event(format!("next:{}:{}:{}",self.0,CURRENT.get(),std::thread::panicking()));self.0+=1;if self.0==2{raise("next")};Some(Element(self.0))}fn size_hint(&self)->(usize,Option<usize>){event(format!("hint:{}:{}",CURRENT.get(),std::thread::panicking()));(0,None)}}
impl Drop for Input{fn drop(&mut self){event(format!("input_drop:{}:{}",CURRENT.get(),std::thread::panicking()));if ACTION.get()=="extend_double"||ACTION.get()=="collect_double"{raise("iterator_drop")}}}
struct ZInput(u8);
impl Iterator for ZInput{type Item=Zst;fn next(&mut self)->Option<Zst>{event(format!("znext:{}:{}:{}",self.0,CURRENT.get(),std::thread::panicking()));if self.0==0{self.0=1;Some(Zst)}else{None}}fn size_hint(&self)->(usize,Option<usize>){event(format!("zhint:{}:{}",CURRENT.get(),std::thread::panicking()));let n=1-self.0 as usize;(n,Some(n))}}
impl Drop for ZInput{fn drop(&mut self){event(format!("zinput_drop:{}:{}",CURRENT.get(),std::thread::panicking()));}}
struct TrustedZInput(ZInput);impl Iterator for TrustedZInput{type Item=Zst;fn next(&mut self)->Option<Zst>{self.0.next()}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}}unsafe impl std::iter::TrustedLen for TrustedZInput{}
fn message(p:Box<dyn std::any::Any+Send>)->String{p.downcast_ref::<&str>().map(|s|(*s).to_owned()).or_else(||p.downcast_ref::<String>().cloned()).unwrap_or_else(||"opaque".into())}
fn bag()->UnordBag<Element>{let mut b=UnordBag::new();for i in 1..=3{b.push(Element(i))};b}
fn body(target:&'static str){event(format!("body:{}",std::thread::panicking()));CURRENT.set("A");let action=ACTION.get();
 if action=="extend_trusted_none"||action=="collect_trusted_none"{let iterator=(0..usize::MAX).map(|_|Zst).chain(std::iter::once(Zst));CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||if action=="extend_trusted_none"{let mut v:Vec<Zst>=Vec::new();v.extend(iterator)}else{drop(UnordItems::new(iterator).collect::<UnordBag<_>>())}));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));return}
 if action.starts_with("collect"){CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||drop(UnordItems::new(Input(0)).collect::<UnordBag<_>>())));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));return}
 if action.starts_with("clone_from"){let source=bag();let mut destination=UnordBag::new();destination.push(Element(10));CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||destination.clone_from(&source)));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));drop(destination);drop(source);return}
 if action=="clone_zst_max"{let v:Vec<()>=unsafe{Vec::from_raw_parts(NonNull::dangling().as_ptr(),usize::MAX,usize::MAX)};CURRENT.set(target);let cloned=v.clone();event(format!("sizes:{}:{}",cloned.len(),cloned.capacity()));return}
 if action.starts_with("extend_capacity"){let mut v:Vec<Zst>=unsafe{Vec::from_raw_parts(NonNull::dangling().as_ptr(),usize::MAX,usize::MAX)};CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||match action{"extend_capacity_generic"=>v.extend(ZInput(0)),"extend_capacity_trusted"=>v.extend(TrustedZInput(ZInput(0))),"extend_capacity_owned"=>v.extend(vec![Zst]),_=>unreachable!()}));unsafe{v.set_len(0)};event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));return}
 if action=="capacity"{let mut v:Vec<Zst>=unsafe{Vec::from_raw_parts(NonNull::dangling().as_ptr(),usize::MAX,usize::MAX)};CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||v.push(Zst)));unsafe{v.set_len(0)};event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));return}
 if action.starts_with("extend"){let mut b=UnordBag::new();b.push(Element(10));CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||b.extend(Input(0))));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));drop(b);return}
 if action=="iter_drop_fail"{let v=vec![Element(1),Element(2),Element(3)];CURRENT.set(target);let result=catch_unwind(AssertUnwindSafe(||drop(v.into_iter())));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));return}
 let b=bag();CURRENT.set(target);if action=="drop_fail"{let result=catch_unwind(AssertUnwindSafe(||drop(b)));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));}else{let result=catch_unwind(AssertUnwindSafe(||drop(b.clone())));event(format!("result:{}:{}",hex(&result.err().map(message).unwrap_or_default()),std::thread::panicking()));drop(b)}
}
struct Outer(&'static str);impl Drop for Outer{fn drop(&mut self){body(self.0)}}
struct HookLocal;impl Drop for HookLocal{fn drop(&mut self){event(format!("hook_local:{}",std::thread::panicking()));}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let target=if args[2]=="B"{"B"}else{"none"};let action: &'static str=Box::leak(args[3].clone().into_boxed_str());ACTION.set(action);RESUME.set(args[4]=="true");CURRENT.set(target);let processing=mode=="processing";
 panic::set_hook(Box::new(move|info|{if target=="B"{let(kind,payload)=if let Some(s)=info.payload().downcast_ref::<&str>(){("static_str",(*s).to_owned())}else if let Some(s)=info.payload().downcast_ref::<String>(){("owned_string",s.clone())}else{("opaque",String::new())};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(&payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));let _local=HookLocal;}if processing{body(target)}}));
 match mode.as_str(){"normal"=>body(target),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(target);panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>panic!("unknown mode")}}
