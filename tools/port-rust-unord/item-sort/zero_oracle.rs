// Actual unchanged generic UnordItems sorting, with an inhabited no-Drop ZST
// and a legal C::from_iter implementation retaining the full usize domain.
#![feature(fn_traits,unboxed_closures,panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::{unord::UnordItems,stable_hash::{StableHash,StableHashCtxt,StableHasher,ToStableHashKey,StableCompare,DummyStableHashCtxt}};
use std::{borrow::{Borrow,BorrowMut},cell::Cell,cmp::Ordering,io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind}};
#[derive(Clone,Copy)]struct Config{length:usize,action:usize,key_fail:isize,cmp_fail:isize,key_drop_fail:isize,input_drop_fail:bool,borrow_fail:bool,collection_drop_fail:bool,project_drop_fail:bool,token_fail:bool,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{length:0,action:0,key_fail:-1,cmp_fail:-1,key_drop_fail:-1,input_drop_fail:false,borrow_fail:false,collection_drop_fail:false,project_drop_fail:false,token_fail:false,resume:false})};static KEYS:Cell<usize>=const{Cell::new(0)};static COMPARES:Cell<usize>=const{Cell::new(0)};static KEY_DROPS:Cell<usize>=const{Cell::new(0)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Zero<const B:bool>;
struct Key{key:u32,id:u32}
impl Drop for Key{fn drop(&mut self){let n=KEY_DROPS.get();KEY_DROPS.set(n+1);event(format!("key_drop:{}:{}",self.id,std::thread::panicking()));if n as isize==CONFIG.get().key_drop_fail{raise("key_drop")}}}
fn compare(label:&str,a:&Key,b:&Key)->Ordering{let n=COMPARES.get();COMPARES.set(n+1);event(format!("{label}:{}:{}:{}",a.id,b.id,std::thread::panicking()));if n as isize==CONFIG.get().cmp_fail{raise("compare")};a.key.cmp(&b.key)}
impl PartialEq for Key{fn eq(&self,b:&Self)->bool{self.key==b.key}}impl Eq for Key{}
impl PartialOrd for Key{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(compare("partial",self,b))}}
impl Ord for Key{fn cmp(&self,b:&Self)->Ordering{compare("ord",self,b)}}
impl StableHash for Key{fn stable_hash<H:StableHashCtxt>(&self,hcx:&mut H,hasher:&mut StableHasher){self.key.stable_hash(hcx,hasher)}}
impl<const B:bool>ToStableHashKey for Zero<B>{type KeyType=Key;fn to_stable_hash_key<H:StableHashCtxt>(&self,_:&mut H)->Key{let n=KEYS.get();KEYS.set(n+1);event(format!("key:{n}:{}",std::thread::panicking()));if n as isize==CONFIG.get().key_fail{raise("key")};Key{key:((n*7+3)%5)as u32,id:n as u32}}}
impl<const B:bool>StableCompare for Zero<B>{const CAN_USE_UNSTABLE_SORT:bool=B;fn stable_cmp(&self,_:&Self)->Ordering{event("UNEXPECTED stable_cmp");Ordering::Equal}}
struct Input<const B:bool>{length:usize}
impl<const B:bool>Drop for Input<B>{fn drop(&mut self){event(format!("input_drop:{}:{}",self.length,std::thread::panicking()));if CONFIG.get().input_drop_fail{raise("input_drop")}}}
impl<const B:bool>Iterator for Input<B>{type Item=Zero<B>;fn next(&mut self)->Option<Self::Item>{event("UNEXPECTED next");None}fn size_hint(&self)->(usize,Option<usize>){event(format!("hint:{}",self.length));(self.length,Some(self.length))}fn collect<C:FromIterator<Self::Item>>(self)->C{event("collect");C::from_iter(self)}}
struct Collection<const B:bool>{values:Vec<Zero<B>>}
impl<const B:bool>FromIterator<Zero<B>>for Collection<B>{fn from_iter<I:IntoIterator<Item=Zero<B>>>(input:I)->Self{event("collection_from");let input=input.into_iter();let length=input.size_hint().0;drop(input);let mut values=Vec::with_capacity(length);unsafe{values.set_len(length)};event(format!("collection_ready:{}:{}",values.len(),values.capacity()));Self{values}}}
impl<const B:bool>Borrow<[Zero<B>]>for Collection<B>{fn borrow(&self)->&[Zero<B>]{&self.values}}
impl<const B:bool>BorrowMut<[Zero<B>]>for Collection<B>{fn borrow_mut(&mut self)->&mut[Zero<B>]{event(format!("borrow:{}",self.values.len()));if CONFIG.get().borrow_fail{raise("borrow")};&mut self.values}}
impl<const B:bool>Drop for Collection<B>{fn drop(&mut self){event(format!("collection_drop:{}:{}",self.values.len(),std::thread::panicking()));if CONFIG.get().collection_drop_fail{raise("collection_drop")}}}
struct Token;impl Drop for Token{fn drop(&mut self){event(format!("token:{}",std::thread::panicking()));if CONFIG.get().token_fail{raise("token_drop")}}}
struct Projection{_token:Token}
impl Drop for Projection{fn drop(&mut self){event(format!("projection_drop:0:{}",std::thread::panicking()));if CONFIG.get().project_drop_fail{raise("projection_drop")}}}
impl<'a,const B:bool>FnOnce<(&'a Zero<B>,)>for Projection{type Output=&'a Zero<B>;extern "rust-call" fn call_once(self,(item,):(&'a Zero<B>,))->Self::Output{event("UNEXPECTED project_once");item}}
impl<'a,const B:bool>FnMut<(&'a Zero<B>,)>for Projection{extern "rust-call" fn call_mut(&mut self,(item,):(&'a Zero<B>,))->Self::Output{event("UNEXPECTED project_mut");item}}
impl<'a,const B:bool>Fn<(&'a Zero<B>,)>for Projection{extern "rust-call" fn call(&self,(item,):(&'a Zero<B>,))->Self::Output{event("UNEXPECTED project_shared");item}}
fn execute<const B:bool>(){let c=CONFIG.get();let input=UnordItems::new(Input::<B>{length:c.length});let result:Collection<B>=match c.action{0|1=>input.collect_sorted(&mut DummyStableHashCtxt,c.action==1),2=>input.collect_stable_ord_by_key(Projection{_token:Token}),_=>unreachable!()};event(format!("returned:{}",result.values.len()));drop(result)}
fn body(unstable:bool){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(||if unstable{execute::<true>()}else{execute::<false>()}));let message=result.err().map(|value|value.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer(bool);impl Drop for Outer{fn drop(&mut self){body(self.0)}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let unstable=args[2]=="true";let n:Vec<i128>=args[3].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{length:n[0]as usize,action:n[1]as usize,key_fail:n[2]as isize,cmp_fail:n[3]as isize,key_drop_fail:n[4]as isize,input_drop_fail:n[5]!=0,borrow_fail:n[6]!=0,collection_drop_fail:n[7]!=0,project_drop_fail:n[8]!=0,token_fail:n[9]!=0,resume:n[10]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body(unstable)}}));match mode.as_str(){"normal"=>body(unstable),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(unstable);panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
