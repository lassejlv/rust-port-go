// Compile and run the actual unchanged UnordBag StableHash impl and its bounds.
use rustc_data_structures::stable_hash::{DummyStableHashCtxt,StableHash,StableHashCtxt,StableHasher};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::unord::UnordBag;
use std::hash::{Hash,Hasher};
struct Value(u64);
#[cfg(not(no_hash))]
impl Hash for Value{fn hash<H:Hasher>(&self,_:&mut H){panic!("UNEXPECTED Hash")}}
#[cfg(not(no_eq))]
impl PartialEq for Value{fn eq(&self,_:&Self)->bool{panic!("UNEXPECTED Eq")}}
#[cfg(not(no_eq))]
impl Eq for Value{}
#[cfg(not(no_stable_hash))]
impl StableHash for Value{fn stable_hash<H:StableHashCtxt>(&self,_:&mut H,h:&mut StableHasher){println!("stable:{}",self.0);h.write_u64(self.0)}}
fn requires<T:StableHash>(){}
fn main(){requires::<UnordBag<Value>>();for length in [0usize,1,2,4]{println!("length:{length}");let mut bag=UnordBag::new();for n in 0..length{bag.push(Value(n as u64))};let mut h=StableHasher::new();bag.stable_hash(&mut DummyStableHashCtxt,&mut h);let f:Fingerprint=h.finish();let(a,b)=f.split();println!("fingerprint:{}:{}",a.as_u64(),b.as_u64());}}
