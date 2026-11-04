// Deliberate caller override: the real std map must dispatch hash_one without
// constructing a hasher or calling either owned/borrowed key Hash implementation.
#![feature(map_try_insert)]
use std::{borrow::Borrow,cell::RefCell,collections::HashMap,hash::{BuildHasher,Hash,Hasher}};
thread_local!{static EVENTS:RefCell<Vec<String>>=RefCell::new(vec![]);}
fn event(text:String){EVENTS.with(|e|e.borrow_mut().push(text))}
struct Key(String);
impl Hash for Key{fn hash<H:Hasher>(&self,_:&mut H){panic!("key Hash must be bypassed")}}
impl PartialEq for Key{fn eq(&self,other:&Self)->bool{self.0==other.0}}impl Eq for Key{}
impl Borrow<str>for Key{fn borrow(&self)->&str{&self.0}}
impl Drop for Key{fn drop(&mut self){event(format!("drop:{}",self.0))}}
#[derive(Clone)]struct Builder;
impl Drop for Builder{fn drop(&mut self){event("builder_drop".into())}}
struct NeverHasher;
impl Hasher for NeverHasher{fn write(&mut self,_:&[u8]){panic!("hasher Write must be bypassed")}fn finish(&self)->u64{panic!("hasher Finish must be bypassed")}}
impl BuildHasher for Builder{type Hasher=NeverHasher;fn build_hasher(&self)->NeverHasher{panic!("BuildHasher must be bypassed")}fn hash_one<T:Hash>(&self,_:T)->u64{event("one".into());0}}
fn main(){let mut map=HashMap::<Key,u64,Builder>::with_hasher(Builder);map.insert(Key("a".into()),1);map.insert(Key("b".into()),2);assert_eq!(map.get("a"),Some(&1));assert_eq!(*map.entry(Key("a".into())).or_insert(8),1);let e=map.try_insert(Key("a".into()),9).unwrap_err();assert_eq!(*e.entry.get(),1);drop(e);assert_eq!(map.remove("a"),Some(1));*map.entry(Key("c".into())).or_insert(3)+=1;let snapshot=map.iter().map(|(k,v)|format!("{}:{v},",k.0)).collect::<String>();drop(map);println!("{snapshot}\t{}",EVENTS.with(|e|e.borrow().join("|")));}
