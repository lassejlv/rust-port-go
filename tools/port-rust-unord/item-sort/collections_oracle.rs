// Actual unchanged eight UnordMap/UnordSet sorting methods and concrete
// Vec collection. Only the observing types/owned callbacks are authored here.
#![feature(fn_traits,unboxed_closures,panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::{unord::{UnordMap,UnordSet},stable_hash::{StableHash,StableHashCtxt,StableHasher,ToStableHashKey,StableCompare,DummyStableHashCtxt}};
use std::{borrow::{Borrow,BorrowMut},cell::Cell,cmp::Ordering,io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind}};
#[derive(Clone,Copy)]struct Config{length:usize,action:usize,cache:bool,key_fail:isize,cmp_fail:isize,key_drop_fail:isize,item_drop_fail:isize,input_drop_fail:bool,borrow_fail:bool,collection_drop_fail:bool,project_drop_fail:bool,token_fail:bool,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{length:0,action:0,cache:false,key_fail:-1,cmp_fail:-1,key_drop_fail:-1,item_drop_fail:-1,input_drop_fail:false,borrow_fail:false,collection_drop_fail:false,project_drop_fail:false,token_fail:false,resume:false})};static KEYS:Cell<usize>=const{Cell::new(0)};static COMPARES:Cell<usize>=const{Cell::new(0)};static KEY_DROPS:Cell<usize>=const{Cell::new(0)};}
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
fn execute<const B:bool>(){
 let c=CONFIG.get();
 if c.action<4{
  let mut map=UnordMap::default();for i in 0..c.length{map.insert(Item::<B>{key:((i*7+3)%5)as i32,id:i as u32},Value(i as u32));}
  match c.action{
   0|1=>{let result=if c.action==0{map.to_sorted(&mut DummyStableHashCtxt,c.cache)}else{map.to_sorted_stable_ord()};event(format!("returned:{}:{}",result.len(),result.capacity()));event(format!("ids:{}",result.iter().map(|(k,v)|format!("{}={}",k.id,v.0)).collect::<Vec<_>>().join(",")));drop(result);},
   2|3=>{let result=if c.action==2{map.into_sorted(&mut DummyStableHashCtxt,c.cache)}else{map.into_sorted_stable_ord()};event(format!("returned:{}:{}",result.len(),result.capacity()));event(format!("ids:{}",result.iter().map(|(k,v)|format!("{}={}",k.id,v.0)).collect::<Vec<_>>().join(",")));drop(result);},_=>unreachable!()
  }
 }else{
  let mut set=UnordSet::new();for i in 0..c.length{set.insert(Item::<B>{key:((i*7+3)%5)as i32,id:i as u32});}
  match c.action{
   4|5=>{let result=if c.action==4{set.to_sorted(&mut DummyStableHashCtxt,c.cache)}else{set.to_sorted_stable_ord()};event(format!("returned:{}:{}",result.len(),result.capacity()));event(format!("ids:{}",result.iter().map(|k|k.id.to_string()).collect::<Vec<_>>().join(",")));drop(result);},
   6|7=>{let result=if c.action==6{set.into_sorted(&mut DummyStableHashCtxt,c.cache)}else{set.into_sorted_stable_ord()};event(format!("returned:{}:{}",result.len(),result.capacity()));event(format!("ids:{}",result.iter().map(|k|k.id.to_string()).collect::<Vec<_>>().join(",")));drop(result);},_=>unreachable!()
  }
 }
}
fn body(unstable:bool){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(||if unstable{execute::<true>()}else{execute::<false>()}));let message=result.err().map(|value|value.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer(bool);impl Drop for Outer{fn drop(&mut self){body(self.0)}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let unstable=args[2]=="true";let n:Vec<i64>=args[3].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{length:n[0]as usize,action:n[1]as usize,cache:n[12]!=0,key_fail:n[2]as isize,cmp_fail:n[3]as isize,key_drop_fail:n[4]as isize,item_drop_fail:n[5]as isize,input_drop_fail:n[6]!=0,borrow_fail:n[7]!=0,collection_drop_fail:n[8]!=0,project_drop_fail:n[9]!=0,token_fail:n[10]!=0,resume:n[11]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body(unstable)}}));match mode.as_str(){"normal"=>body(unstable),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(unstable);panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
