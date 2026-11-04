// SPDX-License-Identifier: MIT OR Apache-2.0
// Compile the complete pinned idx.rs unchanged in standard/nightly profiles.
mod original { include!("idx.rs"); }
use original::{Idx,IntoSliceIdx};
use std::{fmt,hash::{Hash,Hasher},io::{self,BufRead},ops,panic::{catch_unwind,AssertUnwindSafe},sync::Mutex};

#[derive(Default)]struct Context{limit:usize,fail:[usize;5],calls:[usize;5],trace:Vec<String>}
static CONTEXT:Mutex<Context>=Mutex::new(Context{limit:0,fail:[0;5],calls:[0;5],trace:Vec::new()});
fn event(kind:usize,a:usize,b:usize)->bool{
 let mut ctx=CONTEXT.lock().unwrap();ctx.calls[kind]+=1;
 let name=["n","i","p","m","c"][kind];
 ctx.trace.push(if kind<2||kind==4{format!("{name}:{a}")}else{format!("{name}:{a}:{b}")});
 (ctx.fail[kind]!=0&&ctx.calls[kind]==ctx.fail[kind])||(kind==0&&a>ctx.limit)
}
#[derive(Clone,Copy)]struct Key{value:usize,metadata:&'static[u8]}
impl PartialEq for Key{fn eq(&self,b:&Self)->bool{self.value==b.value}}
impl Eq for Key{}
impl Hash for Key{fn hash<H:Hasher>(&self,h:&mut H){self.value.hash(h)}}
impl fmt::Debug for Key{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.value.fmt(f)}}
fn new_key(value:usize)->Key{if event(0,value,0){panic!("new");}Key{value,metadata:&[1,2]}}
fn index_key(key:Key)->usize{if event(1,key.value,0){panic!("index");}key.value}
trait HarnessKey:Idx{fn raw(self)->usize;fn untracked(value:usize)->Self;}
impl HarnessKey for usize{fn raw(self)->usize{self}fn untracked(v:usize)->Self{v}}
impl HarnessKey for u32{fn raw(self)->usize{self as usize}fn untracked(v:usize)->Self{v as u32}}
fn plus_override<K:HarnessKey>(key:K,amount:usize)->K{
 if event(2,key.raw(),amount){panic!("plus");}
 K::new(key.index().wrapping_add(amount).wrapping_add(7))
}
fn increment_override<K:HarnessKey>(key:&mut K,amount:usize){
 let value=key.raw();let fail=event(3,value,amount);*key=K::untracked(value.wrapping_add(11));
 if fail{panic!("increment");}*key=key.plus(amount);
}
macro_rules! key_type{($name:ident{$($methods:tt)*})=>{
 #[derive(Clone,Copy,Eq,PartialEq,Hash,Debug)]struct $name(Key);
 impl HarnessKey for $name{fn raw(self)->usize{self.0.value}fn untracked(value:usize)->Self{Self(Key{value,metadata:&[1,2]})}}
 impl Idx for $name{fn new(v:usize)->Self{Self(new_key(v))}fn index(self)->usize{index_key(self.0)}$($methods)*}
}}
key_type!(DefaultKey{});
key_type!(PlusKey{fn plus(self,n:usize)->Self{plus_override(self,n)}});
key_type!(IncrementKey{fn increment_by(&mut self,n:usize){increment_override(self,n)}});
key_type!(BothKey{fn plus(self,n:usize)->Self{plus_override(self,n)}fn increment_by(&mut self,n:usize){increment_override(self,n)}});

struct Custom<K>(K);
impl<K:HarnessKey,T> IntoSliceIdx<K,[T]> for Custom<K>{
 type Output=ops::RangeTo<usize>;
 fn into_slice_idx(self)->Self::Output{if event(4,self.0.raw(),0){panic!("convert");}..self.0.index().wrapping_add(9)}
}

fn execute<K:HarnessKey>(operation:&str,a:usize,b:usize,amount:usize)->String{
 let mut key=K::untracked(a);
 let result=catch_unwind(AssertUnwindSafe(||match operation{
  "new"=>{let out=K::new(a);format!("key:{}",out.raw())},
  "index"=>format!("index:{}",key.index()),
  "plus"=>format!("key:{}",key.plus(amount).raw()),
  "increment"=>{key.increment_by(amount);"unit".into()},
  "single"=>format!("single:{}",<K as IntoSliceIdx<K,[u8]>>::into_slice_idx(key)),
  "full"=>{let _=<ops::RangeFull as IntoSliceIdx<K,[u8]>>::into_slice_idx(..);"full".into()},
  "range"=>{let r=<ops::Range<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(key..K::untracked(b));format!("range:{}:{}",r.start,r.end)},
  "from"=>{let r=<ops::RangeFrom<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(key..);format!("from:{}",r.start)},
  "to"=>{let r=<ops::RangeTo<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(..K::untracked(b));format!("to:{}",r.end)},
  "inclusive"=>{let r=<ops::RangeInclusive<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(key..=K::untracked(b));format!("inclusive:{}:{}:{}",r.start(),r.end(),r.is_empty())},
  "to_inclusive"=>{let r=<ops::RangeToInclusive<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(..=K::untracked(b));format!("to_inclusive:{}",r.end)},
  "exhausted"=>{let mut r=a..=a;let _=r.next();let before=r.is_empty();let input_start=*r.start();let input_end=*r.end();let r=<ops::RangeInclusive<usize> as IntoSliceIdx<usize,[u8]>>::into_slice_idx(r);format!("exhausted:{}:{}:{}:{}:{}:{}",before,input_start,input_end,r.start(),r.end(),r.is_empty())},
  "custom"=>{let r=<Custom<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(Custom(key));format!("custom_to:{}",r.end)},
  #[cfg(feature="nightly")]
  "core_range"=>{let r=<core::range::Range<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(core::range::Range{start:key,end:K::untracked(b)});format!("core_range:{}:{}",r.start,r.end)},
  #[cfg(feature="nightly")]
  "core_from"=>{let r=<core::range::RangeFrom<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(core::range::RangeFrom{start:key});format!("core_from:{}",r.start)},
  #[cfg(feature="nightly")]
  "core_inclusive"=>{let r=<core::range::RangeInclusive<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(core::range::RangeInclusive{start:key,last:K::untracked(b)});format!("core_inclusive:{}:{}",r.start,r.last)},
  #[cfg(feature="nightly")]
  "core_to_inclusive"=>{let r=<core::range::RangeToInclusive<K> as IntoSliceIdx<K,[u8]>>::into_slice_idx(core::range::RangeToInclusive{last:K::untracked(b)});format!("core_to_inclusive:{}",r.last)},
  _=>panic!("operation unavailable")
 }));
 let value=result.unwrap_or_else(|_|"panic".into());
 let ctx=CONTEXT.lock().unwrap();format!("{value}\tkey={}\ttrace={}",key.raw(),ctx.trace.join(","))
}

fn main(){
 std::panic::set_hook(Box::new(|_|{}));
 for line in io::stdin().lock().lines(){let line=line.unwrap();let mut f=line.split('\t');let kind=f.next().unwrap();let operation=f.next().unwrap();let a=f.next().unwrap().parse().unwrap();let b=f.next().unwrap().parse().unwrap();let amount=f.next().unwrap().parse().unwrap();let limit=f.next().unwrap().parse().unwrap();let mut fail=[0;5];for v in &mut fail{*v=f.next().unwrap().parse().unwrap();}
  *CONTEXT.lock().unwrap()=Context{limit,fail,..Default::default()};
  let value=match kind{"usize"=>execute::<usize>(operation,a,b,amount),"u32"=>execute::<u32>(operation,a,b,amount),"default"=>execute::<DefaultKey>(operation,a,b,amount),"plus"=>execute::<PlusKey>(operation,a,b,amount),"increment"=>execute::<IncrementKey>(operation,a,b,amount),"both"=>execute::<BothKey>(operation,a,b,amount),_=>panic!("key type")};println!("{value}");
 }
}
