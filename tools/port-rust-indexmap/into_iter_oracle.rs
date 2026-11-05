// Original owning iterator/drop/clone observer, linked to unchanged indexmap.
use indexmap::{IndexMap,map::IntoIter};use serde_json::{json,Value};
use std::{cell::RefCell,rc::Rc,hash::{Hash,Hasher,BuildHasher},io::{self,BufRead},panic::{self,AssertUnwindSafe}};
struct Log{events:Vec<String>,fail:String,failed:bool}type L=Rc<RefCell<Log>>;
fn step(l:&L,s:String){let event=format!("{s}:{}",std::thread::panicking());let fail={let mut log=l.borrow_mut();log.events.push(event.clone());let yes=!log.failed&&log.fail==event;if yes{log.failed=true};yes};if fail{panic!("fail")}}
struct K{id:u64,log:L}impl Hash for K{fn hash<H:Hasher>(&self,h:&mut H){h.write_u64(self.id)}}impl PartialEq for K{fn eq(&self,r:&Self)->bool{self.id==r.id}}impl Eq for K{}
impl Clone for K{fn clone(&self)->Self{step(&self.log,format!("kclone:{}",self.id));Self{id:self.id,log:self.log.clone()}}}impl Drop for K{fn drop(&mut self){step(&self.log,format!("kdrop:{}",self.id))}}
struct V{id:u64,log:L}impl Clone for V{fn clone(&self)->Self{step(&self.log,format!("vclone:{}",self.id));Self{id:self.id,log:self.log.clone()}}}impl Drop for V{fn drop(&mut self){step(&self.log,format!("vdrop:{}",self.id))}}
struct B(L);struct H(u64);impl Hasher for H{fn write(&mut self,_:&[u8]){panic!("unexpected write")}fn write_u64(&mut self,n:u64){self.0=n}fn finish(&self)->u64{self.0}}impl BuildHasher for B{type Hasher=H;fn build_hasher(&self)->H{H(0)}}impl Drop for B{fn drop(&mut self){step(&self.0,"bdrop".into())}}
fn map(n:usize,base:u64,l:&L)->IndexMap<K,V,B>{let mut m=IndexMap::with_capacity_and_hasher(n,B(l.clone()));for i in 0..n{m.insert(K{id:base+i as u64,log:l.clone()},V{id:base+10+i as u64,log:l.clone()});}m}
fn n(q:&Value,k:&str)->usize{q[k].as_u64().unwrap_or(0)as usize}
fn take(value:Option<(K,V)>)->Value{match value{Some(pair)=>{let result=json!([pair.0.id,pair.1.id]);drop(pair);result},None=>Value::Null}}
fn remaining(it:&IntoIter<K,V>)->Value{json!(it.as_slice().iter().map(|(k,v)|(k.id,v.id)).collect::<Vec<_>>())}
fn action(q:&Value)->Value{
 let l=Rc::new(RefCell::new(Log{events:Vec::new(),fail:String::new(),failed:false}));let m=map(n(q,"n"),0,&l);{let mut log=l.borrow_mut();log.events.clear();log.fail=q["fail"].as_str().unwrap_or("").into();}
 let mut holder=None;let result=panic::catch_unwind(AssertUnwindSafe(||{
  holder=Some(m.into_iter());let it=holder.as_mut().unwrap();for _ in 0..n(q,"front"){drop(it.next())}for _ in 0..n(q,"back"){drop(it.next_back())}
  match q["op"].as_str().unwrap(){
   "next"=>take(it.next()),"back"=>take(it.next_back()),"nth"=>take(it.nth(n(q,"nth"))),"nth_back"=>take(it.nth_back(n(q,"nth"))),
   "count"=>json!(holder.take().unwrap().count()),"last"=>take(holder.take().unwrap().last()),"close"=>{drop(holder.take());Value::Null},
   "clone"=>{let other=it.clone();let result=remaining(&other);drop(other);result},
   "clone_from"=>{let other=map(n(q,"other"),100,&l).into_iter();it.clone_from(&other);drop(other);Value::Null},
   "mutate"=>{for(_,v)in it.as_mut_slice().iter_mut(){v.id+=1000}remaining(it)},"inspect"=>remaining(it),_=>panic!("bad op")}
 }));let(value,error)=match result{Ok(v)=>(v,Value::Null),Err(_)=>(Value::Null,json!("fail"))};let current=holder.as_ref().map(remaining);let len=holder.as_ref().map(|it|it.len());let hint=holder.as_ref().map(|it|it.size_hint());let events=l.borrow().events.clone();{let mut log=l.borrow_mut();log.fail.clear();log.events.clear();}drop(holder);let dropped=l.borrow().events.clone();json!({"value":value,"panic":error,"remaining":current,"len":len,"hint":hint,"events":events,"drop":dropped})
}
struct OnDrop<F:FnOnce()>(Option<F>);impl<F:FnOnce()>Drop for OnDrop<F>{fn drop(&mut self){self.0.take().unwrap()()}}
fn run(q:&Value)->Value{if q["inherited"].as_bool().unwrap_or(false){let mut result=None;let _=panic::catch_unwind(AssertUnwindSafe(||{let _g=OnDrop(Some(||result=Some(action(q))));panic!("outer")}));result.unwrap()}else{action(q)}}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
