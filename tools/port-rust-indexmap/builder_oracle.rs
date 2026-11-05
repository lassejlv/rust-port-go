// Observer of actual unconstrained constructors and conditional S traits.
use indexmap::IndexMap;
use serde_json::{json,Value};
use std::{cell::{Cell,RefCell},hash::{BuildHasher,Hasher},io::{self,BufRead},panic::{self,AssertUnwindSafe}};
thread_local!{static EVENTS:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};static SCOPE:RefCell<String>=RefCell::new(String::new());static FAILURE:RefCell<String>=RefCell::new(String::new());}
fn event(s:String){EVENTS.with(|e|e.borrow_mut().push(s))}
fn scope()->String{SCOPE.with(|s|s.borrow().clone())}
fn fail(name:&str)->bool{FAILURE.with(|f|*f.borrow()==name)}
struct State{id:u64,counter:Cell<u64>,data:RefCell<Vec<Vec<u8>>>}
impl State{fn new(id:u64)->Self{Self{id,counter:Cell::new(0),data:RefCell::new(vec![vec![id as u8],vec![2,3]])}}fn snapshot(&self)->Value{json!({"id":self.id,"counter":self.counter.get(),"data":*self.data.borrow()})}fn copy(&self)->Self{self.counter.set(self.counter.get()+1);event(format!("clone:{}:{}:{}",self.id,scope(),std::thread::panicking()));if fail("clone"){panic!("clone")};Self{id:self.id+100,counter:Cell::new(self.counter.get()),data:RefCell::new(self.data.borrow().clone())}}}
impl Drop for State{fn drop(&mut self){let id=self.id;event(format!("drop:{id}:{}:{}",scope(),std::thread::panicking()));self.id+=10000;if id==1&&fail("drop"){panic!("drop")}}}
trait Access{fn state(&self)->&State;fn make(id:u64)->Self;}
macro_rules! state_type{($name:ident)=>{struct $name(State);impl Access for $name{fn state(&self)->&State{&self.0}fn make(id:u64)->Self{Self(State::new(id))}}};}
state_type!(Opaque);state_type!(Cloning);state_type!(Custom);state_type!(Building);
impl Clone for Cloning{fn clone(&self)->Self{Self(self.0.copy())}}
impl Clone for Custom{fn clone(&self)->Self{Self(self.0.copy())}fn clone_from(&mut self,other:&Self){self.0.id=other.0.id+1000;self.0.counter.set(self.0.counter.get()+1);event(format!("clone_from:{}:{}:{}",self.0.id,scope(),std::thread::panicking()));if fail("clone_from"){panic!("clone_from")};*self.0.data.borrow_mut()=other.0.data.borrow().clone()}}
struct H{state:u64}
impl Hasher for H{fn write(&mut self,_:&[u8]){panic!("unexpected bytes")}fn write_u64(&mut self,value:u64){event(format!("u64:{value}:{}",scope()));self.state=value}fn finish(&self)->u64{event(format!("finish:{}:{}",self.state,scope()));self.state}}
impl Drop for H{fn drop(&mut self){event(format!("hdrop:{}:{}:{}",self.state,scope(),std::thread::panicking()))}}
impl BuildHasher for Building{type Hasher=H;fn build_hasher(&self)->H{self.0.counter.set(self.0.counter.get()+1);event(format!("build:{}:{}:{}",self.0.id,scope(),std::thread::panicking()));H{state:0}}}
#[derive(Clone)]struct NoHash(Vec<u8>);
fn n(q:&Value,s:&str)->usize{q[s].as_u64().unwrap_or(0)as usize}
fn construct<S:Access>(q:&Value)->Value{let map=IndexMap::<NoHash,NoHash,S>::with_capacity_and_hasher(n(q,"capacity"),S::make(1));let r=json!({"capacity":map.capacity(),"len":map.len(),"iter":map.iter().count(),"keys":map.keys().count(),"values":map.values().count(),"slice":map.as_slice().len(),"state":map.hasher().state().snapshot()});drop(map);r}
fn cloning<S:Access+Clone>(q:&Value)->Value{let mut target=IndexMap::<NoHash,NoHash,S>::with_capacity_and_hasher(n(q,"capacity"),S::make(1));let source=IndexMap::<NoHash,NoHash,S>::with_capacity_and_hasher(n(q,"source_capacity"),S::make(2));let anchor=target.hasher().state()as*const State;
 let mut cloned=None;let failed=panic::catch_unwind(AssertUnwindSafe(||if q["mode"]=="clone"{cloned=Some(source.clone())}else{target.clone_from(&source)})).is_err();
 if let Some(other)=cloned.as_ref(){other.hasher().state().data.borrow_mut()[0][0]+=1;}
 let snapshot=|m:&IndexMap<NoHash,NoHash,S>|json!({"capacity":m.capacity(),"state":m.hasher().state().snapshot()});let r=json!({"failed":failed,"target":snapshot(&target),"source":snapshot(&source),"cloned":cloned.as_ref().map(snapshot),"same_cell":anchor==target.hasher().state()as*const State});drop(cloned);drop(source);drop(target);r}
fn building(q:&Value)->Value{let mut map=IndexMap::<u64,u64,Building>::with_capacity_and_hasher(n(q,"capacity"),Building::make(1));for i in 0..3{map.insert(i as u64*17+1,i as u64);}let result=map.get(&18).copied();let r=json!({"capacity":map.capacity(),"len":map.len(),"get":result,"state":map.hasher().state().snapshot(),"entries":map.into_iter().collect::<Vec<_>>()});r}
fn run(q:&Value)->Value{EVENTS.with(|e|e.borrow_mut().clear());SCOPE.with(|s|*s.borrow_mut()=q["scope"].as_str().unwrap().into());FAILURE.with(|s|*s.borrow_mut()=q["failure"].as_str().unwrap_or("").into());let value=match q["kind"].as_str().unwrap(){"opaque"=>construct::<Opaque>(q),"default"=>cloning::<Cloning>(q),"custom"=>cloning::<Custom>(q),"building"=>building(q),_=>panic!("bad kind")};json!({"value":value,"events":EVENTS.with(|e|e.borrow().clone())})}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
