// The unchanged hashbrown crate is compiled with cfg(miri) solely to select its
// original generic control-group implementation. This does not run Miri.
use std::{cell::RefCell,hash::{Hash,Hasher,BuildHasher}};
use hashbrown::HashMap;
thread_local!{static EVENTS:RefCell<Vec<String>>=RefCell::new(vec![]);}
struct Key{identity:u64,hash:u64}
impl Hash for Key{fn hash<H:Hasher>(&self,h:&mut H){h.write_u64(self.hash)}}
impl PartialEq for Key{fn eq(&self,other:&Self)->bool{EVENTS.with(|e|e.borrow_mut().push(format!("{}:{}",self.identity,other.identity)));self.identity==other.identity}}impl Eq for Key{}
#[derive(Clone)]struct Builder;
struct Direct(u64);impl Hasher for Direct{fn write(&mut self,_:&[u8]){panic!("unexpected bytes")}fn write_u64(&mut self,v:u64){self.0=v}fn finish(&self)->u64{self.0}}
impl BuildHasher for Builder{type Hasher=Direct;fn build_hasher(&self)->Direct{Direct(0)}}
fn main(){for tags in [[0u64,1,0,1],[1,0,1,0],[127,126,127,126]]{let mut map=HashMap::with_hasher(Builder);for index in 0..4{map.insert(Key{identity:index as u64+1,hash:tags[index]<<57|index as u64},index as u64);}EVENTS.with(|e|e.borrow_mut().clear());let query=Key{identity:99,hash:tags[0]<<57};assert!(map.get(&query).is_none());println!("{}\t{}",map.capacity(),EVENTS.with(|e|e.borrow().join("|")));}}
