// Observer only: indexmap2.14.0 is linked unchanged from its verified archive.
#![feature(fn_traits,unboxed_closures)]
use indexmap::{IndexMap,map::Entry};
use serde_json::{json,Value};
use std::{cell::RefCell,rc::Rc,hash::{Hash,Hasher,BuildHasher},io::{self,BufRead},panic::{self,AssertUnwindSafe}};
struct Log{events:Vec<String>,fail:String,failed:bool,from_override:bool}
type L=Rc<RefCell<Log>>;
fn event(l:&L,s:String){let fail={let mut b=l.borrow_mut();b.events.push(s.clone());let fail=!b.failed&&b.fail==s;if fail{b.failed=true}fail};if fail{panic!("fail")}}
fn step(l:&L,s:String){event(l,format!("{s}:{}",std::thread::panicking()))}
struct K{id:u64,tag:u64,log:L}
impl Hash for K{fn hash<H:Hasher>(&self,h:&mut H){step(&self.log,format!("hash:{}:{}",self.id,self.tag));h.write_u64(self.id)}}
impl PartialEq for K{fn eq(&self,b:&Self)->bool{step(&self.log,format!("eq:{}:{}",self.id,b.id));self.id==b.id}}impl Eq for K{}
impl Clone for K{fn clone(&self)->Self{step(&self.log,format!("kclone:{}:{}",self.id,self.tag));Self{id:self.id,tag:self.tag,log:self.log.clone()}}fn clone_from(&mut self,source:&Self){if self.log.borrow().from_override{self.tag=source.tag;step(&self.log,format!("kclonefrom:{}:{}",self.id,source.id));self.id=source.id}else{*self=source.clone()}}}
impl Drop for K{fn drop(&mut self){step(&self.log,format!("kdrop:{}:{}",self.id,self.tag))}}
struct V{id:i64,tag:u64,log:L}
impl Clone for V{fn clone(&self)->Self{step(&self.log,format!("vclone:{}:{}",self.id,self.tag));Self{id:self.id,tag:self.tag,log:self.log.clone()}}fn clone_from(&mut self,source:&Self){if self.log.borrow().from_override{self.tag=source.tag;step(&self.log,format!("vclonefrom:{}:{}",self.id,source.id));self.id=source.id}else{*self=source.clone()}}}
impl Drop for V{fn drop(&mut self){step(&self.log,format!("vdrop:{}:{}",self.id,self.tag))}}
struct B{log:L,mask:u64}struct H{log:L,mask:u64,state:u64}
impl Clone for B{fn clone(&self)->Self{step(&self.log,"bclone".into());Self{log:self.log.clone(),mask:self.mask}}fn clone_from(&mut self,source:&Self){step(&self.log,"bclonefrom".into());self.mask=source.mask}}
impl BuildHasher for B{type Hasher=H;fn build_hasher(&self)->H{step(&self.log,"build".into());H{log:self.log.clone(),mask:self.mask,state:0}}}
impl Hasher for H{fn write(&mut self,_:&[u8]){panic!("unexpected write")}fn write_u64(&mut self,n:u64){step(&self.log,format!("u64:{n}"));self.state=n}fn finish(&self)->u64{step(&self.log,"finish".into());self.state&self.mask}}
impl Drop for H{fn drop(&mut self){step(&self.log,"hdrop".into())}}impl Drop for B{fn drop(&mut self){step(&self.log,"bdrop".into())}}
struct F{log:L,value:Option<V>}
impl Drop for F{fn drop(&mut self){step(&self.log,"fdrop".into())}}
impl FnOnce<()> for F{type Output=V;extern "rust-call" fn call_once(mut self,_:())->V{step(&self.log,"fcall".into());self.value.take().unwrap()}}
impl FnOnce<(&K,)> for F{type Output=V;extern "rust-call" fn call_once(mut self,(key,):(&K,))->V{step(&self.log,format!("fcall-key:{}:{}",key.id,key.tag));self.value.take().unwrap()}}
impl FnOnce<(&mut V,)> for F{type Output=();extern "rust-call" fn call_once(self,(value,):(&mut V,)){step(&self.log,format!("fcall-mut:{}:{}",value.id,value.tag));value.id=self.value.as_ref().unwrap().id}}
fn num(q:&Value,s:&str)->u64{q[s].as_u64().unwrap_or(0)}
fn operation(map:&mut IndexMap<K,V,B>,q:&Value,l:&L)->Value{
 let op=q["op"].as_str().unwrap();
 if op=="clone"{let other=map.clone();let result=json!({"capacity":other.capacity(),"entries":other.iter().map(|(k,v)|(k.id,k.tag,v.id,v.tag)).collect::<Vec<_>>()});drop(other);return result}
 if op=="clone_from"{let count=num(q,"key")%5;let mut other=IndexMap::with_capacity_and_hasher(count as usize,B{log:l.clone(),mask:7});for i in 0..count{other.insert(K{id:10+i,tag:1100+i,log:l.clone()},V{id:10+i as i64,tag:1200+i,log:l.clone()});}map.clone_from(&other);drop(other);return Value::Null}
 let entry=map.entry(K{id:num(q,"key"),tag:900,log:l.clone()});let value=||V{id:777,tag:901,log:l.clone()};let factory=||F{log:l.clone(),value:Some(value())};let op=q["op"].as_str().unwrap();
 match op{
 "close"=>{drop(entry);Value::Null},
 "or_insert"=>{let v=entry.or_insert(value());json!([v.id,v.tag])},
 "with"=>{let v=entry.or_insert_with(factory());json!([v.id,v.tag])},
 "with_key"=>{let v=entry.or_insert_with_key(factory());json!([v.id,v.tag])},
 "modify"=>{let e=entry.and_modify(factory());json!([e.index(),e.key().id,e.key().tag])},
 "insert_entry"=>{let e=entry.insert_entry(value());json!([e.index(),e.get().id,e.get().tag])},
 "occupied_insert"=>match entry{Entry::Occupied(mut e)=>{let old=e.insert(value());let r=json!([old.id,old.tag,e.get().id,e.get().tag]);drop(old);r},Entry::Vacant(e)=>{drop(e);Value::Null}},
 "swap_remove"=>match entry{Entry::Occupied(e)=>{let(k,v)=e.swap_remove_entry();let r=json!([k.id,k.tag,v.id,v.tag]);drop(k);drop(v);r},Entry::Vacant(e)=>{drop(e);Value::Null}},
 "shift_remove"=>match entry{Entry::Occupied(e)=>{let(k,v)=e.shift_remove_entry();let r=json!([k.id,k.tag,v.id,v.tag]);drop(k);drop(v);r},Entry::Vacant(e)=>{drop(e);Value::Null}},
 "into_key"=>match entry{Entry::Vacant(e)=>{let key=e.into_key();let r=json!([key.id,key.tag]);drop(key);r},Entry::Occupied(e)=>{drop(e);Value::Null}},
 "vacant_insert"=>match entry{Entry::Vacant(e)=>{let v=e.insert(value());json!([v.id,v.tag])},Entry::Occupied(e)=>{drop(e);Value::Null}},
 _=>panic!("bad op")}
}
struct OnDrop<F:FnOnce()>(Option<F>);impl<F:FnOnce()>Drop for OnDrop<F>{fn drop(&mut self){self.0.take().unwrap()()}}
fn run(q:&Value)->Value{
 let l=Rc::new(RefCell::new(Log{events:Vec::new(),fail:String::new(),failed:false,from_override:q["from_override"].as_bool().unwrap_or(false)}));let n=num(q,"n");let mut map=IndexMap::with_capacity_and_hasher(num(q,"capacity")as usize,B{log:l.clone(),mask:num(q,"mask")});
 for i in 0..n{map.insert(K{id:i,tag:100+i,log:l.clone()},V{id:i as i64,tag:200+i,log:l.clone()});}
 {let mut log=l.borrow_mut();log.events.clear();log.fail=q["fail"].as_str().unwrap_or("").into();}
 let mut result=None;
 let mut call=||result=Some(panic::catch_unwind(AssertUnwindSafe(||operation(&mut map,q,&l))));
 if q["inherited"].as_bool().unwrap_or(false){let _=panic::catch_unwind(AssertUnwindSafe(||{let _g=OnDrop(Some(call));panic!("outer")}));}else{call()}
 let(value,error)=match result.unwrap(){Ok(v)=>(v,Value::Null),Err(_)=>(Value::Null,json!("fail"))};
 let entries=map.iter().map(|(k,v)|(k.id,k.tag,v.id,v.tag)).collect::<Vec<_>>();let capacity=map.capacity();l.borrow_mut().fail.clear();let events=l.borrow().events.clone();l.borrow_mut().events.clear();drop(map);let dropped=l.borrow().events.clone();
 json!({"value":value,"panic":error,"entries":entries,"capacity":capacity,"events":events,"drop":dropped})
}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
