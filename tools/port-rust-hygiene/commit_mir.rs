// This caller imports the actual generic default; it does not copy its body.
extern crate rustc_span;
extern crate rustc_data_structures;
use rustc_span::hygiene::HygieneRemapper;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher};
use std::panic::{catch_unwind,AssertUnwindSafe};
struct Key(u32);
impl From<u32> for Key{fn from(n:u32)->Self{println!("from:{n}");Self(n)}}
impl Drop for Key{fn drop(&mut self){println!("drop:{}:{}",self.0,std::thread::panicking())}}
struct Remapper{slot:Option<Key>,length:usize}
impl HygieneRemapper<Key> for Remapper{
 fn stable_hash(&self,_key:Key,_ctx:&mut impl StableHashCtxt,_hasher:&mut StableHasher){}
 fn len(&self)->usize{println!("len:{}",self.length);self.length}
 fn first_non_det_index_mut(&mut self)->&mut Option<Key>{println!("slot");&mut self.slot}
 fn first_non_det_index(&self)->u32{self.slot.as_ref().unwrap().0}
}
fn main(){std::panic::set_hook(Box::new(|_|println!("hook")));let mut r=Remapper{slot:None,length:1};r.commit_end_of_determinism();println!("after-first:{}",r.first_non_det_index());r.length=2;let ok=catch_unwind(AssertUnwindSafe(||r.commit_end_of_determinism())).is_ok();println!("after-second:{ok}:{}",r.first_non_det_index());drop(r);}
