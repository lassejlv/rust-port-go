// SPDX-License-Identifier: MIT OR Apache-2.0
// Development oracle: the complete original union_find.rs and Idx source are
// compiled unchanged. The scoped IndexVec shim preserves only the operations
// the original union-find uses; it is not a replacement implementation of it.
extern crate self as rustc_index;
mod original_idx { include!("upstream/idx.rs"); }
pub use original_idx::Idx;
use std::{fmt, hash::Hash, io::{self,BufRead}, marker::PhantomData, ops::{Index,IndexMut}, panic::{catch_unwind,AssertUnwindSafe}, sync::Mutex};

pub struct IndexVec<I:Idx,T>{pub raw:Vec<T>,marker:PhantomData<fn(&I)>}
impl<I:Idx,T> IndexVec<I,T>{
 pub fn from_fn_n(mut func:impl FnMut(I)->T,n:usize)->Self{
  let _=I::new(n);
  Self{raw:(0..n).map(I::new).map(|i|func(i)).collect(),marker:PhantomData}
 }
 pub fn indices(&self)->impl DoubleEndedIterator<Item=I>+ExactSizeIterator+Clone+'static{
  let _=I::new(self.raw.len());
  (0..self.raw.len()).map(I::new)
 }
}
impl<I:Idx,T> Index<I> for IndexVec<I,T>{type Output=T;fn index(&self,i:I)->&T{&self.raw[i.index()]}}
impl<I:Idx,T> IndexMut<I> for IndexVec<I,T>{fn index_mut(&mut self,i:I)->&mut T{&mut self.raw[i.index()]}}
impl<I:Idx,T> FromIterator<T> for IndexVec<I,T>{fn from_iter<J:IntoIterator<Item=T>>(iter:J)->Self{Self{raw:Vec::from_iter(iter),marker:PhantomData}}}
impl<I:Idx,T:fmt::Debug> fmt::Debug for IndexVec<I,T>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.raw.fmt(f)}}

mod upstream { pub mod union_find; }

#[derive(Default)]struct Context{limit:usize,panic_new:usize,panic_index:usize,panic_equal:usize,panic_not_equal:usize,new_calls:usize,index_calls:usize,equal_calls:usize,not_equal_calls:usize,trace:Vec<String>}
static CONTEXT:Mutex<Context>=Mutex::new(Context{limit:0,panic_new:0,panic_index:0,panic_equal:0,panic_not_equal:0,new_calls:0,index_calls:0,equal_calls:0,not_equal_calls:0,trace:Vec::new()});

#[derive(Clone,Copy)]struct Key(usize,&'static[u8]);
impl Hash for Key{fn hash<H:std::hash::Hasher>(&self,state:&mut H){self.0.hash(state)}}
fn key(i:usize)->Key{Key(i,&[1,2])}
impl fmt::Debug for Key{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.0.fmt(f)}}
impl PartialEq for Key{
 fn eq(&self,b:&Self)->bool{
  let mut ctx=CONTEXT.lock().unwrap();ctx.trace.push(format!("eq:{}:{}",self.0,b.0));ctx.equal_calls+=1;
  let panic=ctx.panic_equal!=0&&ctx.equal_calls==ctx.panic_equal;drop(ctx);if panic{panic!("equal");}self.0==b.0
 }
 fn ne(&self,b:&Self)->bool{
  let mut ctx=CONTEXT.lock().unwrap();ctx.trace.push(format!("ne:{}:{}",self.0,b.0));ctx.not_equal_calls+=1;
  let panic=ctx.panic_not_equal!=0&&ctx.not_equal_calls==ctx.panic_not_equal;drop(ctx);if panic{panic!("not equal");}self.0!=b.0
 }
}
impl Eq for Key{}
impl Idx for Key{
 fn new(i:usize)->Self{
  let mut ctx=CONTEXT.lock().unwrap();ctx.trace.push(format!("n:{i}"));ctx.new_calls+=1;
  let panic=i>ctx.limit||(ctx.panic_new!=0&&ctx.new_calls==ctx.panic_new);drop(ctx);if panic{panic!("new");}key(i)
 }
 fn index(self)->usize{
  let mut ctx=CONTEXT.lock().unwrap();ctx.trace.push(format!("i:{}",self.0));ctx.index_calls+=1;
  let panic=ctx.panic_index!=0&&ctx.index_calls==ctx.panic_index;drop(ctx);if panic{panic!("index");}self.0
 }
}

#[derive(Clone,Copy,Hash)]struct DefaultNeKey(Key);
impl fmt::Debug for DefaultNeKey{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.0.fmt(f)}}
impl PartialEq for DefaultNeKey{fn eq(&self,other:&Self)->bool{self.0.eq(&other.0)}}
impl Eq for DefaultNeKey{}
impl Idx for DefaultNeKey{
 fn new(i:usize)->Self{Self(Key::new(i))}
 fn index(self)->usize{self.0.index()}
}
fn default_ne_key(i:usize)->DefaultNeKey{DefaultNeKey(key(i))}

fn execute(input:&str)->String{
 let mut fields=input.split('\t');
 let n=fields.next().unwrap().parse::<usize>().unwrap();
 let limit=fields.next().unwrap().parse().unwrap();
 let panic_new=fields.next().unwrap().parse().unwrap();
 let panic_index=fields.next().unwrap().parse().unwrap();
 let panic_equal=fields.next().unwrap().parse().unwrap();
 let panic_not_equal=fields.next().unwrap().parse().unwrap();
 let default_not_equal=fields.next().unwrap().parse::<bool>().unwrap();
 *CONTEXT.lock().unwrap()=Context{limit,panic_new,panic_index,panic_equal,panic_not_equal,..Default::default()};
 let operations=fields.next().unwrap_or("");
 if default_not_equal{execute_typed(n,operations,default_ne_key)}else{execute_typed(n,operations,key)}
}

fn execute_typed<K:Idx>(n:usize,operations:&str,key:fn(usize)->K)->String{
 let mut result=Vec::new();
 let forest=catch_unwind(AssertUnwindSafe(||upstream::union_find::UnionFind::<K>::new(n)));
 if let Ok(mut sets)=forest{
  result.push(format!("new ok {sets:?}"));
  let mut saved:Option<IndexVec<K,K>>=None;
  for op in operations.split(';').filter(|op|!op.is_empty()){
   let mut args=op.split(':');let kind=args.next().unwrap();let a=args.next().unwrap().parse::<usize>().unwrap();let b=args.next().unwrap().parse::<usize>().unwrap();
   let outcome=catch_unwind(AssertUnwindSafe(||match kind{
    "F"=>format!("find {:?}",sets.find(key(a))),
    "U"=>format!("unify {:?}",sets.unify(key(a),key(b))),
    "S"=>{let snapshot=sets.snapshot();let value=format!("snapshot {snapshot:?}");saved=Some(snapshot);value},
    "A"=>format!("at {:?}",saved.as_ref().unwrap()[key(a)]),
    "T"=>{saved.as_mut().unwrap()[key(a)]=key(b);"set".into()},
    "L"=>{CONTEXT.lock().unwrap().limit=a;format!("limit {a}")},
    _=>panic!("unknown operation")
   }));
   let value=outcome.unwrap_or_else(|_|"panic".into());result.push(format!("{value} {sets:?}"));
  }
 }else{result.push("new panic".into());}
 let ctx=CONTEXT.lock().unwrap();format!("{}\ttrace={}",result.join("|"),ctx.trace.join(","))
}

fn main(){std::panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){println!("{}",execute(&line.unwrap()));}}
