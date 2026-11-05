// Development-only caller of the unchanged locked indexmap2.14.0 crate.
use indexmap::{IndexMap,Equivalent};
use serde_json::{json,Value};
use std::{cell::RefCell,rc::Rc,hash::{BuildHasher,Hasher,Hash},io::{self,BufRead},panic::{self,AssertUnwindSafe}};
type Events=Rc<RefCell<Vec<String>>>;
fn event(log:&Events,s:String){log.borrow_mut().push(s)}
struct B{log:Events,seed:u64,mask:u64}
struct H{log:Events,state:u64,mask:u64}
impl BuildHasher for B{type Hasher=H;fn build_hasher(&self)->H{event(&self.log,format!("build:{}",std::thread::panicking()));H{log:self.log.clone(),state:self.seed,mask:self.mask}}}
impl Hasher for H{fn write(&mut self,b:&[u8]){event(&self.log,format!("bytes:{b:?}"));panic!("unexpected byte write")}fn write_u64(&mut self,n:u64){event(&self.log,format!("u64:{n}:{}",std::thread::panicking()));self.state=self.state.rotate_left(5)^n}fn finish(&self)->u64{event(&self.log,format!("finish:{}",std::thread::panicking()));self.state&self.mask}}
impl Drop for H{fn drop(&mut self){event(&self.log,format!("hdrop:{}",std::thread::panicking()))}}
impl Clone for B{fn clone(&self)->Self{event(&self.log,format!("bclone:{}",std::thread::panicking()));Self{log:self.log.clone(),seed:self.seed,mask:self.mask}}fn clone_from(&mut self,b:&Self){event(&self.log,format!("bclonefrom:{}",std::thread::panicking()));self.seed=b.seed;self.mask=b.mask}}
impl Drop for B{fn drop(&mut self){event(&self.log,format!("bdrop:{}",std::thread::panicking()))}}
struct Q{key:u64,log:Events}
impl Hash for Q{fn hash<T:Hasher>(&self,h:&mut T){self.key.hash(h)}}
impl Equivalent<u64> for Q{fn equivalent(&self,k:&u64)->bool{event(&self.log,format!("equivalent:{}:{k}",self.key));self.key==*k}}
fn payload(p:Box<dyn std::any::Any+Send>)->String{if let Some(v)=p.downcast_ref::<&str>(){(*v).into()}else if let Some(v)=p.downcast_ref::<String>(){v.clone()}else{"opaque".into()}}
fn number(v:&Value,n:&str)->u64{v[n].as_u64().unwrap_or(0)}
fn run(q:&Value)->Value{
 let log=Rc::new(RefCell::new(Vec::new()));let b=B{log:log.clone(),seed:number(q,"seed"),mask:number(q,"mask")};let mut map=IndexMap::<u64,i64,B>::with_capacity_and_hasher(number(q,"capacity")as usize,b);let mut rows=Vec::new();
 for op in q["ops"].as_array().unwrap(){
  log.borrow_mut().clear();let key=number(op,"key");let n=number(op,"n")as usize;let kind=op["op"].as_str().unwrap();
  let result=panic::catch_unwind(AssertUnwindSafe(||match kind{
   "insert"=>{let(i,old)=map.insert_full(key,op["value"].as_i64().unwrap());json!([i,old])},
   "entry"=>{let e=map.entry(key);json!([e.index(),*e.key(),matches!(e,indexmap::map::Entry::Occupied(_))])},
   "entry_or_insert"=>json!(*map.entry(key).or_insert(op["value"].as_i64().unwrap())),
   "entry_or_default"=>json!(*map.entry(key).or_default()),
   "entry_with"=>json!(*map.entry(key).or_insert_with(||{event(&log,"factory".into());op["value"].as_i64().unwrap()})),
   "entry_with_key"=>json!(*map.entry(key).or_insert_with_key(|k|{event(&log,format!("factory:{k}"));op["value"].as_i64().unwrap()})),
   "entry_modify"=>{let e=map.entry(key).and_modify(|v|{event(&log,format!("modify:{v}"));*v=op["value"].as_i64().unwrap()});json!([e.index(),*e.key()])},
   "entry_insert"=>{let e=map.entry(key).insert_entry(op["value"].as_i64().unwrap());json!([e.index(),*e.key(),*e.get()])},
   "occupied_replace"=>match map.entry(key){indexmap::map::Entry::Occupied(mut e)=>json!([e.index(),e.insert(op["value"].as_i64().unwrap()),*e.get_mut()]),indexmap::map::Entry::Vacant(e)=>json!([e.index(),e.into_key()])},
   "entry_swap_remove"=>match map.entry(key){indexmap::map::Entry::Occupied(e)=>json!(e.swap_remove_entry()),indexmap::map::Entry::Vacant(e)=>{drop(e);Value::Null}},
   "entry_shift_remove"=>match map.entry(key){indexmap::map::Entry::Occupied(e)=>json!(e.shift_remove_entry()),indexmap::map::Entry::Vacant(e)=>{drop(e);Value::Null}},
   "vacant_insert"=>match map.entry(key){indexmap::map::Entry::Occupied(e)=>json!([e.index(),*e.into_mut()]),indexmap::map::Entry::Vacant(e)=>{let i=e.index();json!([i,*e.insert(op["value"].as_i64().unwrap())])}},
   "get"=>json!(map.get_full(&key).map(|(i,k,v)|(i,*k,*v))),
   "equivalent"=>json!(map.get_full(&Q{key,log:log.clone()}).map(|(i,k,v)|(i,*k,*v))),
   "swap_remove"=>json!(map.swap_remove(&key)),"shift_remove"=>json!(map.shift_remove(&key)),
   "swap_index"=>json!(map.swap_remove_index(n)),"shift_index"=>json!(map.shift_remove_index(n)),
   "reserve"=>{map.reserve(n);Value::Null},"reserve_exact"=>{map.reserve_exact(n);Value::Null},
   "reverse"=>{map.reverse();Value::Null},"clear"=>{map.clear();Value::Null},
   "pop"=>json!(map.pop()),"index"=>json!(map.get_index(n).map(|(k,v)|(*k,*v))),
   "clone"=>{let other=map.clone();let result=json!({"capacity":other.capacity(),"entries":other.iter().map(|(k,v)|(*k,*v)).collect::<Vec<_>>()});drop(other);result},
   "equal_clone"=>{let other=map.clone();let same=map==other;drop(other);json!(same)},
   "clone_from"=>{let mut other=IndexMap::with_capacity_and_hasher(n,B{log:log.clone(),seed:key,mask:map.hasher().mask});for i in 0..n{other.insert(i as u64*17+1,-(i as i64)-1);}map.clone_from(&other);drop(other);Value::Null},
   "debug"=>json!(format!("{map:?}")),
   _=>panic!("unknown operation")}));
  let(value,error)=match result{Ok(v)=>(v,None),Err(e)=>(Value::Null,Some(payload(e)))};
  rows.push(json!({"value":value,"panic":error,"len":map.len(),"capacity":map.capacity(),"entries":map.iter().map(|(k,v)|(*k,*v)).collect::<Vec<_>>(),"events":log.borrow().clone()}));
 }
 log.borrow_mut().clear();drop(map);let dropped=log.borrow().clone();json!({"rows":rows,"drop":dropped})
}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
