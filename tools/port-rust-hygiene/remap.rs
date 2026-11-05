#![feature(trait_alias)]
extern crate rustc_span;extern crate rustc_data_structures;
use rustc_span::hygiene::{HygieneRemapper,HygieneRemappingResults,RemappableIdx};
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher,DummyStableHashCtxt};
use std::{cell::{RefCell,Cell},rc::Rc,hash::{Hash,Hasher},panic::{catch_unwind,AssertUnwindSafe}};
thread_local!{static LOG:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};}
fn log(s:String){LOG.with(|l|l.borrow_mut().push(s));}
#[derive(Clone,Copy)]struct K{raw:u32,metadata:&'static[u8]}
impl PartialEq for K{fn eq(&self,_:&Self)->bool{panic!("unexpected key eq")}}impl Eq for K{}
impl Hash for K{fn hash<H:Hasher>(&self,_:&mut H){panic!("unexpected key hash")}}
impl From<u32> for K{fn from(raw:u32)->Self{log(format!("keyfrom:{raw}"));Self{raw,metadata:&[1,2]}}}
impl From<K> for u32{fn from(k:K)->u32{let _=k.metadata;log(format!("into:{}",k.raw));k.raw}}
#[derive(Clone,Copy,PartialEq,Eq,Hash)]struct A(u32);
impl From<u32> for A{fn from(v:u32)->Self{log(format!("afrom:{v}"));A(v)}}
#[derive(Clone,Copy,PartialEq,Eq,Hash)]struct B(u32,&'static[u8]);
impl From<u32> for B{fn from(v:u32)->Self{log(format!("bfrom:{v}"));B(v+100,&[3,4])}}
struct R{length:Rc<Cell<usize>>,first:Option<K>,fail:Option<u32>}
impl HygieneRemapper<K> for R{
 fn stable_hash(&self,k:K,_:&mut impl StableHashCtxt,h:&mut StableHasher){log(format!("hash:{}",k.raw));if self.fail==Some(k.raw){panic!("hash failed")};h.write_u64((k.raw%3)as u64)}
 fn len(&self)->usize{log("len".into());self.length.get()}
 fn first_non_det_index_mut(&mut self)->&mut Option<K>{log("firstmut".into());&mut self.first}
 fn first_non_det_index(&self)->u32{log("first".into());self.first.expect("not committed").raw}
}
struct Override<'a>(&'a mut R);
impl HygieneRemapper<K> for Override<'_>{
 fn stable_hash(&self,k:K,c:&mut impl StableHashCtxt,h:&mut StableHasher){self.0.stable_hash(k,c,h)}
 fn len(&self)->usize{self.0.len()}
 fn first_non_det_index_mut(&mut self)->&mut Option<K>{self.0.first_non_det_index_mut()}
 fn first_non_det_index(&self)->u32{self.0.first_non_det_index()}
 fn commit_end_of_determinism(&mut self)where K:From<u32>{log("overridecommit".into());self.0.commit_end_of_determinism()}
 fn remapping_results<O>(&self,hcx:&mut impl StableHashCtxt)->HygieneRemappingResults<K,O>where K:RemappableIdx+From<u32>{log(format!("overrideremap:{}",if std::any::type_name::<O>().ends_with("::A"){ "A"}else{"B"}));self.0.remapping_results(hcx)}
}
fn exercise<T:HygieneRemapper<K>>(r:&mut T,length:&Cell<usize>,old:usize,extra:usize,mode:usize){
 r.commit_end_of_determinism();length.set(old+extra);
 if mode&1!=0{let ok=catch_unwind(AssertUnwindSafe(||r.commit_end_of_determinism())).is_ok();log(format!("commitagain:{ok}"));}
 log(format!("boundary:{}",r.first_non_det_index()));
 let a=catch_unwind(AssertUnwindSafe(||r.remapping_results::<A>(&mut DummyStableHashCtxt)));
 match a{Ok(a)=>{for v in 0..(old+extra+3)as u32{let result=a.map_id(K{raw:v,metadata:&[1,2]});log(format!("aresult:{v}:{}",result.0))}},Err(_)=>log("a:panic".into())}
 let b=catch_unwind(AssertUnwindSafe(||r.remapping_results::<B>(&mut DummyStableHashCtxt)));
 match b{Ok(b)=>{for v in 0..(old+extra+3)as u32{let result=b.map_id(K{raw:v,metadata:&[1,2]});log(format!("bresult:{v}:{}",result.0))}},Err(_)=>log("b:panic".into())}
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let a:Vec<_>=std::env::args().skip(1).map(|s|s.parse::<usize>().unwrap()).collect();let(old,extra,mode)=(a[0],a[1],a[2]);let len=Rc::new(Cell::new(old));let mut r=R{length:len.clone(),first:None,fail:if mode&4!=0{Some(old as u32+1)}else{None}};
 if mode&2!=0{exercise(&mut Override(&mut r),&len,old,extra,mode)}else{exercise(&mut r,&len,old,extra,mode)};
 LOG.with(|l|for item in l.borrow().iter(){println!("{item}")});
}
