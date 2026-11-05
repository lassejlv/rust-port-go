// Actual unchanged eight UnordMap/UnordSet sorting methods and concrete
// Vec collection. Only the observing types/owned callbacks are authored here.
#![feature(fn_traits,unboxed_closures,panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::{unord::{UnordMap,UnordSet},stable_hash::{StableHash,StableHashCtxt,StableHasher,ToStableHashKey,StableCompare,DummyStableHashCtxt}};
use std::{borrow::{Borrow,BorrowMut},cell::Cell,cmp::Ordering,io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind}};
#[derive(Clone,Copy)]struct Config{length:usize,action:usize,cache:bool,consumer_fail:isize,skip:usize,key_fail:isize,cmp_fail:isize,key_drop_fail:isize,item_drop_fail:isize,input_drop_fail:bool,borrow_fail:bool,collection_drop_fail:bool,project_drop_fail:bool,token_fail:bool,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{length:0,action:0,cache:false,consumer_fail:-1,skip:0,key_fail:-1,cmp_fail:-1,key_drop_fail:-1,item_drop_fail:-1,input_drop_fail:false,borrow_fail:false,collection_drop_fail:false,project_drop_fail:false,token_fail:false,resume:false})};static KEYS:Cell<usize>=const{Cell::new(0)};static COMPARES:Cell<usize>=const{Cell::new(0)};static KEY_DROPS:Cell<usize>=const{Cell::new(0)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Item<const UNSTABLE:bool>{key:i32,id:u32}
impl<const B:bool>Drop for Item<B>{fn drop(&mut self){event(format!("item:{}:{}",self.id,std::thread::panicking()));if self.id as isize==CONFIG.get().item_drop_fail{raise("item_drop")}}}
struct Key{key:i32,id:u32}
impl Drop for Key{fn drop(&mut self){let n=KEY_DROPS.get();KEY_DROPS.set(n+1);event(format!("key_drop:{}:{}",self.id,std::thread::panicking()));if n as isize==CONFIG.get().key_drop_fail{raise("key_drop")}}}
fn compare(label:&str,a_key:i32,a_id:u32,b_key:i32,b_id:u32)->Ordering{let n=COMPARES.get();COMPARES.set(n+1);event(format!("{label}:{a_id}:{b_id}:{}",std::thread::panicking()));if n as isize==CONFIG.get().cmp_fail{raise("compare")};a_key.cmp(&b_key)}
impl PartialEq for Key{fn eq(&self,b:&Self)->bool{self.key==b.key}}impl Eq for Key{}
impl PartialOrd for Key{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(compare("partial",self.key,self.id,b.key,b.id))}}
impl Ord for Key{fn cmp(&self,b:&Self)->Ordering{compare("ord",self.key,self.id,b.key,b.id)}}
impl StableHash for Key{fn stable_hash<H:StableHashCtxt>(&self,hcx:&mut H,hasher:&mut StableHasher){self.key.stable_hash(hcx,hasher)}}
impl<const B:bool>ToStableHashKey for Item<B>{type KeyType=Key;fn to_stable_hash_key<H:StableHashCtxt>(&self,_:&mut H)->Key{let n=KEYS.get();KEYS.set(n+1);event(format!("key:{}:{}",self.id,std::thread::panicking()));if n as isize==CONFIG.get().key_fail{raise("key")};Key{key:self.key,id:self.id}}}
impl<const B:bool>StableCompare for Item<B>{const CAN_USE_UNSTABLE_SORT:bool=B;fn stable_cmp(&self,b:&Self)->Ordering{compare("stable",self.key,self.id,b.key,b.id)}}
impl<const B:bool> PartialEq for Item<B>{fn eq(&self,b:&Self)->bool{self.id==b.id}}
impl<const B:bool> Eq for Item<B>{}
impl<const B:bool> std::hash::Hash for Item<B>{fn hash<H:std::hash::Hasher>(&self,h:&mut H){std::hash::Hash::hash(&self.id,h)}}
struct Value(u32);impl Drop for Value{fn drop(&mut self){event(format!("value:{}:{}",self.0,std::thread::panicking()));if self.0 as isize==CONFIG.get().item_drop_fail{raise("value_drop")}}}
use std::{alloc::{GlobalAlloc,Layout,System},sync::atomic::{AtomicBool,AtomicUsize,Ordering as AtomicOrdering}};
static TRACK:AtomicBool=AtomicBool::new(false);static FIRST:AtomicUsize=AtomicUsize::new(0);static FIRST_SIZE:AtomicUsize=AtomicUsize::new(0);static FIRST_ALIGN:AtomicUsize=AtomicUsize::new(0);
struct Observer;unsafe impl GlobalAlloc for Observer{unsafe fn alloc(&self,l:Layout)->*mut u8{let ptr=unsafe{System.alloc(l)};if TRACK.load(AtomicOrdering::Relaxed)&&FIRST.compare_exchange(0,ptr as usize,AtomicOrdering::Relaxed,AtomicOrdering::Relaxed).is_ok(){FIRST_SIZE.store(l.size(),AtomicOrdering::Relaxed);FIRST_ALIGN.store(l.align(),AtomicOrdering::Relaxed);}ptr}unsafe fn dealloc(&self,p:*mut u8,l:Layout){unsafe{System.dealloc(p,l)}}}
#[global_allocator]static ALLOCATOR:Observer=Observer;
fn display(values:Vec<&Value>){event(format!("returned:{}:{}",values.len(),values.capacity()));event(format!("ids:{}",values.iter().map(|v|v.0.to_string()).collect::<Vec<_>>().join(",")));event(format!("reuse:{}",FIRST.load(AtomicOrdering::Relaxed)!=0&&FIRST.load(AtomicOrdering::Relaxed)==values.as_ptr()as usize));drop(values)}
fn consume(v:&Value,calls:&mut usize){event(format!("consume:{}:{}",v.0,std::thread::panicking()));let n=*calls;*calls+=1;if n as isize==CONFIG.get().consumer_fail{raise("consume")}}
fn execute<const B:bool>(){
 let c=CONFIG.get();let mut map=UnordMap::default();for i in 0..c.length{map.insert(Item::<B>{key:((i*7+3)%5)as i32,id:i as u32},Value(i as u32));}
 let mut hcx=DummyStableHashCtxt;TRACK.store(true,AtomicOrdering::Relaxed);let mut iter=map.values_sorted(&mut hcx,c.cache);TRACK.store(false,AtomicOrdering::Relaxed);
 for _ in 0..c.skip{event(format!("step:{}",iter.next().map(|v|v.0.to_string()).unwrap_or_else(||"none".into())))}
 let(lo,hi)=iter.size_hint();event(format!("hint:{lo}:{}",hi.unwrap()));
 match c.action{
  0=>display(iter.collect()),
  1=>{display(iter.by_ref().collect());let(lo,hi)=iter.size_hint();event(format!("after_hint:{lo}:{}",hi.unwrap()));},
  2=>{let mut calls=0;iter.for_each(|v|{consume(v,&mut calls)});event(format!("consumed:{calls}"));},
  3=>{let mut calls=0;let result=catch_unwind(AssertUnwindSafe(||iter.by_ref().for_each(|v|{consume(v,&mut calls)})));let message=result.err().map(|v|v.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("inner_caught:{}:{}",hex(&message),std::thread::panicking()));let(lo,hi)=iter.size_hint();event(format!("after_hint:{lo}:{}",hi.unwrap()));display(iter.collect());},_=>unreachable!()
 }
}
fn body(unstable:bool){FIRST.store(0,AtomicOrdering::Relaxed);FIRST_SIZE.store(0,AtomicOrdering::Relaxed);FIRST_ALIGN.store(0,AtomicOrdering::Relaxed);event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(||if unstable{execute::<true>()}else{execute::<false>()}));let message=result.err().map(|value|value.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();TRACK.store(false,AtomicOrdering::Relaxed);event(format!("ALLOC:{}:{}",FIRST_SIZE.load(AtomicOrdering::Relaxed),FIRST_ALIGN.load(AtomicOrdering::Relaxed)));event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer(bool);impl Drop for Outer{fn drop(&mut self){body(self.0)}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let unstable=args[2]=="true";let n:Vec<i64>=args[3].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{length:n[0]as usize,action:n[1]as usize,cache:n[12]!=0,consumer_fail:n[13]as isize,skip:n[14]as usize,key_fail:n[2]as isize,cmp_fail:n[3]as isize,key_drop_fail:n[4]as isize,item_drop_fail:n[5]as isize,input_drop_fail:n[6]!=0,borrow_fail:n[7]!=0,collection_drop_fail:n[8]!=0,project_drop_fail:n[9]!=0,token_fail:n[10]!=0,resume:n[11]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{TRACK.store(false,AtomicOrdering::Relaxed);let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body(unstable)}}));match mode.as_str(){"normal"=>body(unstable),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(unstable);panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
