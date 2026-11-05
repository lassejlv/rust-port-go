// Observe actual unchanged Unord collection conversions and ExtendUnord.
#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::{unord::{UnordItems,UnordMap,UnordSet,UnordBag,ExtendUnord},fx::FxHashSet};
use std::{cell::Cell,hash::{Hash,Hasher},io::{self,Write},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{family:usize,length:usize,next_fail:isize,call_fail:bool,item_fail:isize,input_fail:bool,hash_fail:isize,resume:bool,nested:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{family:0,length:0,next_fail:-1,call_fail:false,item_fail:-1,input_fail:false,hash_fail:-1,resume:false,nested:false})};static OBSERVE:Cell<bool>=const{Cell::new(false)};static HASHES:Cell<usize>=const{Cell::new(0)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Value{value:u64,id:u64}
impl Drop for Value{fn drop(&mut self){event(format!("item:{}:{}",self.id,std::thread::panicking()));if self.id as isize==CONFIG.get().item_fail{raise("item_drop")}}}
impl Hash for Value{fn hash<H:Hasher>(&self,h:&mut H){if OBSERVE.get(){let n=HASHES.get();HASHES.set(n+1);event(format!("hash:{}:{}",self.id,std::thread::panicking()));if n as isize==CONFIG.get().hash_fail{raise("hash")}}self.value.hash(h)}}
impl PartialEq for Value{fn eq(&self,b:&Self)->bool{if OBSERVE.get(){event(format!("eq:{}:{}:{}",self.id,b.id,std::thread::panicking()));}self.value==b.value}}impl Eq for Value{}
trait Ident{fn id(&self)->u64;}impl Ident for Value{fn id(&self)->u64{self.id}}impl Ident for(Value,Value){fn id(&self)->u64{self.0.id}}
struct Input<T:Ident>{values:std::vec::IntoIter<T>,calls:usize}
impl<T:Ident>Drop for Input<T>{fn drop(&mut self){event(format!("input_drop:{}:{}:{}",self.calls,self.values.len(),std::thread::panicking()));if CONFIG.get().input_fail{raise("input_drop")}}}
impl<T:Ident>Iterator for Input<T>{type Item=T;
 fn next(&mut self)->Option<T>{let n=self.calls;self.calls+=1;event(format!("next:{n}:{}",std::thread::panicking()));if n as isize==CONFIG.get().next_fail{raise("next")};self.values.next()}
 fn size_hint(&self)->(usize,Option<usize>){event(format!("hint:{}",self.values.len()));(self.values.len(),Some(self.values.len()))}
 fn for_each<F:FnMut(T)>(mut self,mut f:F){event("for_each");if CONFIG.get().call_fail{raise("for_each")};if CONFIG.get().nested{if let Some(v)=self.next(){nested(&mut f,v)}};while let Some(v)=self.next(){f(v)}}
 fn fold<B,F:FnMut(B,T)->B>(mut self,mut acc:B,mut f:F)->B{event("fold");if CONFIG.get().call_fail{raise("fold")};if CONFIG.get().nested{if let Some(v)=self.next(){let mut slot=Some(acc);nested(&mut|v|{slot=Some(f(slot.take().unwrap(),v))},v);acc=slot.unwrap();}};while let Some(v)=self.next(){acc=f(acc,v)};acc}
 fn collect<C:FromIterator<T>>(self)->C{raise("UNEXPECTED collect")}
}
struct DuringDrop<'a,F:FnMut(T),T>{function:&'a mut F,value:Option<T>}
impl<F:FnMut(T),T>Drop for DuringDrop<'_,F,T>{fn drop(&mut self){event(format!("nested_consume:{}",std::thread::panicking()));(self.function)(self.value.take().unwrap())}}
fn nested<F:FnMut(T),T>(function:&mut F,value:T){let _=catch_unwind(AssertUnwindSafe(||{let _guard=DuringDrop{function,value:Some(value)};panic::panic_any("nested")}));}
fn value(n:usize,key:bool)->Value{Value{value:(n%3)as u64,id:(n+if key{100}else{200})as u64}}
fn input<T:Ident>(values:Vec<T>)->UnordItems<T,Input<T>>{UnordItems::new(Input{values:values.into_iter(),calls:0})}
fn execute(){let c=CONFIG.get();OBSERVE.set(false);
 match c.family{
  0|3=>{let values=(0..c.length).map(|n|(value(n,true),value(n,false))).collect::<Vec<_>>();let items=input(values);let mut map=UnordMap::default();if c.family==3{map.insert(Value{value:0,id:90},Value{value:0,id:190});}OBSERVE.set(true);if c.family==0{map=UnordMap::from(items)}else{map.extend_unord(items)};event(format!("returned:{}",map.len()));drop(map)},
  1|4=>{let values=(0..c.length).map(|n|value(n,true)).collect::<Vec<_>>();let items=input(values);let mut set=UnordSet::new();if c.family==4{set.insert(Value{value:0,id:90});}OBSERVE.set(true);if c.family==1{set=UnordSet::from(items)}else{set.extend_unord(items)};event(format!("returned:{}",set.len()));drop(set)},
  2|5=>{let values=(0..c.length).map(|n|value(n,true)).collect::<Vec<_>>();let items=input(values);let mut bag=UnordBag::new();if c.family==5{bag.push(Value{value:0,id:90});}OBSERVE.set(true);if c.family==2{bag=UnordBag::from(items)}else{bag.extend_unord(items)};event(format!("returned:{}",bag.len()));drop(bag)},
  6=>{let mut raw=FxHashSet::default();for n in 0..c.length{raw.insert(value(n,true));}OBSERVE.set(true);let set=UnordSet::from(raw);event(format!("returned:{}",set.len()));drop(set)},
  7|8=>{let mut source=UnordBag::new();for n in 0..c.length{source.push(value(n,true));}let items=source.into_items();let mut bag=UnordBag::new();if c.family==8{bag.push(Value{value:0,id:90});}OBSERVE.set(true);if c.family==7{bag=UnordBag::from(items)}else{bag.extend_unord(items)};event(format!("returned:{}",bag.len()));drop(bag)},_=>unreachable!()
 };event("after_drop")
}
fn body(){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(execute));let message=result.err().map(|v|v.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer;impl Drop for Outer{fn drop(&mut self){body()}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let n:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{family:n[0]as usize,length:n[1]as usize,next_fail:n[2]as isize,call_fail:n[3]!=0,item_fail:n[4]as isize,input_fail:n[5]!=0,hash_fail:n[6]as isize,resume:n[7]!=0,nested:n[8]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body()}}));match mode.as_str(){"normal"=>body(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
