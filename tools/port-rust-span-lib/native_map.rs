#![feature(fn_traits,unboxed_closures)]
// External observer of the unchanged Encoder default methods and real std map.
extern crate rustc_serialize;
extern crate rustc_data_structures;
use rustc_serialize::{Encoder,Encodable,ContextOrd};
use rustc_serialize::opaque::mem_encoder::MemEncoder;
use rustc_data_structures::fx::FxHashMap;
use std::{cell::RefCell,rc::Rc,cmp::Ordering};
type Log=Rc<RefCell<Vec<String>>>;
struct E{data:MemEncoder,log:Log,mode:u8,fail:usize}
macro_rules! delegate{($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self,v:$ty){self.data.$method(v);})*}}
impl Encoder for E{
 delegate!(emit_u128:u128,emit_u64:u64,emit_u16:u16,emit_u8:u8,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);
 fn emit_usize(&mut self,n:usize){self.log.borrow_mut().push(format!("len:{n}"));if self.mode==2{panic!("length")};self.data.emit_usize(n)}
 fn emit_u32(&mut self,n:u32){self.log.borrow_mut().push(format!("num:{n}"));self.data.emit_u32(n)}
 fn emit_raw_bytes(&mut self,b:&[u8]){self.data.emit_raw_bytes(b)}
 const DETERMINISTIC_HASH_MAP_ENCODE:bool=true;
}
#[derive(Hash,Eq,PartialEq)]struct K(u32);
impl Encodable<E>for K{fn encode(&self,e:&mut E){e.log.borrow_mut().push(format!("key:{}",self.0));e.emit_u32(self.0)}}
struct Cmp{log:Log,calls:usize,fail:usize}
impl Cmp{fn compare(&mut self,a:&K,b:&K)->Ordering{let n=self.calls;self.calls+=1;self.log.borrow_mut().push(format!("cmp:{n}:{}:{}",a.0,b.0));if n==self.fail{panic!("comparison")};a.0.cmp(&b.0)}}
impl<'a,'b>FnOnce<(&'a K,&'b K)>for Cmp{type Output=Ordering;extern "rust-call" fn call_once(mut self,args:(&'a K,&'b K))->Ordering{self.compare(args.0,args.1)}}
impl<'a,'b>FnMut<(&'a K,&'b K)>for Cmp{extern "rust-call" fn call_mut(&mut self,args:(&'a K,&'b K))->Ordering{self.compare(args.0,args.1)}}
impl Drop for Cmp{fn drop(&mut self){self.log.borrow_mut().push(format!("drop:{}:{}",self.calls,std::thread::panicking()))}}
impl<'a>ContextOrd<&'a E>for K{fn stable_cmp_with_context(e:&'a E)->impl FnMut(&Self,&Self)->Ordering{e.log.borrow_mut().push("factory".into());if e.mode==1{panic!("factory")};Cmp{log:e.log.clone(),calls:0,fail:e.fail}}}
fn main(){std::panic::set_hook(Box::new(|_|{}));let mut random=0x37e8b901d54ccf63u64;for n in[0usize,1,2,3,5,16,20,21,32,48,64,100]{for _ in 0..8{let mut input=vec![];let mut map=FxHashMap::default();for _ in 0..n{random=random.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);let key=(random%137)as u32;input.push(key);map.insert(K(key),key*7);}for(mode,fail)in[(0,usize::MAX),(1,usize::MAX),(2,usize::MAX),(0,0),(0,1),(0,2),(0,5),(0,17)]{let log:Log=Default::default();let mut e=E{data:MemEncoder::new(),log:log.clone(),mode,fail};let panic=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||e.encode_hash_map(&map))).is_err();println!("{}\t{mode}\t{}\t{panic}\t{}",input.iter().map(u32::to_string).collect::<Vec<_>>().join(","),if fail==usize::MAX{-1i64}else{fail as i64},log.borrow().join(","));}}}}
