// Development-only unchanged semver FromIterator/std Vec collection observer.
#![feature(trusted_len)]
use semver::{Comparator,VersionReq,Op,Prerelease,BuildMetadata,Version};
use serde_json::{Value,json};
use std::{cell::{Cell,RefCell},iter::TrustedLen,io::{self,BufRead},panic::{self,AssertUnwindSafe}};
thread_local!{static EVENTS:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};static RESULT:RefCell<Option<Value>>=const{RefCell::new(None)};static LIVE:Cell<bool>=const{Cell::new(false)};}
fn event(text:String){if LIVE.with(|v|v.get()){println!("EVENT\t{text}")};EVENTS.with(|v|v.borrow_mut().push(text));}
fn bit(q:&Value,k:&str)->bool{q[k].as_bool().unwrap_or(false)}fn num(q:&Value,k:&str)->u64{q[k].as_u64().unwrap_or(0)}fn text<'a>(q:&'a Value,k:&str)->&'a str{q[k].as_str().unwrap_or("")}
fn value(major:u64)->Comparator{Comparator{op:Op::Exact,major,minor:None,patch:None,pre:Prerelease::EMPTY}}
struct Input{q:Value}
impl Drop for Input{fn drop(&mut self){event(format!("input-drop:panic={}",std::thread::panicking()));if bit(&self.q,"drop_input"){panic!("input drop")}}}
impl IntoIterator for Input{type Item=Comparator;type IntoIter=Iter;fn into_iter(self)->Iter{event(format!("into-iter:panic={}",std::thread::panicking()));if bit(&self.q,"panic_into"){panic!("into_iter")};Iter{q:self.q.clone(),position:0,hints:Cell::new(0)}}}
struct Iter{q:Value,position:usize,hints:Cell<usize>}
impl Drop for Iter{fn drop(&mut self){event(format!("iterator-drop:{}:panic={}",self.position,std::thread::panicking()));if bit(&self.q,"drop_iterator"){panic!("iterator drop")}}}
impl Iterator for Iter{type Item=Comparator;
 fn next(&mut self)->Option<Comparator>{event(format!("next:{}:panic={}",self.position,std::thread::panicking()));if self.q["panic_next"].as_u64()==Some(self.position as u64){panic!("next")};let xs=self.q["values"].as_array().unwrap();if self.position==xs.len(){return None};let v=xs[self.position].as_u64().unwrap();self.position+=1;Some(value(v))}
 fn size_hint(&self)->(usize,Option<usize>){let h=self.hints.get();self.hints.set(h+1);event(format!("hint:{}:{}:panic={}",h,self.position,std::thread::panicking()));if self.q["panic_hint"].as_u64()==Some(h as u64){panic!("hint")};if text(&self.q,"hint")=="remaining"{let n=self.q["values"].as_array().unwrap().len()-self.position;(n,Some(n))}else{(num(&self.q,"lower")as usize,self.q["upper"].as_u64().map(|v|v as usize))}}
}
struct Trusted(Iter);impl Iterator for Trusted{type Item=Comparator;fn next(&mut self)->Option<Comparator>{self.0.next()}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}}unsafe impl TrustedLen for Trusted{}
struct Fancy<const EACH:bool>(Iter);impl<const EACH:bool>Iterator for Fancy<EACH>{type Item=Comparator;fn next(&mut self)->Option<Comparator>{self.0.next()}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}
 fn for_each<F:FnMut(Comparator)>(self,mut f:F){if EACH{event(format!("for-each:panic={}",std::thread::panicking()));let mut iter=self;while let Some(v)=iter.next(){f(v)}}else{self.fold((),|(),v|f(v));}}
 fn fold<B,F:FnMut(B,Comparator)->B>(mut self,mut init:B,mut f:F)->B{event(format!("fold:panic={}",std::thread::panicking()));while let Some(v)=self.next(){init=f(init,v)};init}
}unsafe impl<const EACH:bool>TrustedLen for Fancy<EACH>{}
struct Mapper{q:Value}impl Mapper{fn apply(&self,mut value:Comparator)->Comparator{event(format!("map:{}:panic={}",value.major,std::thread::panicking()));if self.q["panic_map"].as_u64()==Some(value.major){panic!("map")};value.major+=10;value}}impl Drop for Mapper{fn drop(&mut self){event(format!("mapper-drop:panic={}",std::thread::panicking()));if bit(&self.q,"drop_iterator"){panic!("mapper drop")}}}
fn collect(q:&Value)->Value{
 let result=panic::catch_unwind(AssertUnwindSafe(||{
  let mut reused=Value::Null;
  let req=match text(q,"kind"){
   "trusted"=>VersionReq::from_iter(Trusted(Input{q:q.clone()}.into_iter())),
   "each"=>VersionReq::from_iter(Fancy::<true>(Input{q:q.clone()}.into_iter())),
   "fold"=>VersionReq::from_iter(Fancy::<false>(Input{q:q.clone()}.into_iter())),
   "map"=>{let mut values=Vec::with_capacity(num(q,"capacity")as usize);for n in q["values"].as_array().unwrap(){values.push(value(n.as_u64().unwrap()))};let pointer=values.as_ptr();let mut it=values.into_iter();for _ in 0..num(q,"front"){it.next();}for _ in 0..num(q,"back"){it.next_back();}let mapper=Mapper{q:q.clone()};let req=VersionReq::from_iter(it.map(move |value|mapper.apply(value)));reused=json!(pointer==req.comparators.as_ptr());req},
   "vec"=>{let mut values=Vec::with_capacity(num(q,"capacity")as usize);for n in q["values"].as_array().unwrap(){values.push(value(n.as_u64().unwrap()))};let pointer=values.as_ptr();let mut it=values.into_iter();for _ in 0..num(q,"front"){it.next();}for _ in 0..num(q,"back"){it.next_back();}let req=VersionReq::from_iter(it);reused=json!(pointer==req.comparators.as_ptr());req},
   _=>VersionReq::from_iter(Input{q:q.clone()}),
  };
  json!({"panic":false,"values":req.comparators.iter().map(|v|v.major).collect::<Vec<_>>(),"capacity":req.comparators.capacity(),"reused":reused})
 }));match result{Ok(v)=>v,Err(p)=>{let message=if let Some(v)=p.downcast_ref::<&str>(){(*v).to_owned()}else if let Some(v)=p.downcast_ref::<String>(){v.clone()}else{"opaque panic".into()};json!({"panic":true,"message":message})}}
}
struct Guard{q:Value}impl Drop for Guard{fn drop(&mut self){let result=collect(&self.q);RESULT.with(|v|*v.borrow_mut()=Some(result))}}
fn run(q:Value)->Value{EVENTS.with(|v|v.borrow_mut().clear());RESULT.with(|v|*v.borrow_mut()=None);panic::set_hook(Box::new(|_|{}));
 let mut result=match text(&q,"context"){
  "inherited"=>{let _=panic::catch_unwind(AssertUnwindSafe(||{let _guard=Guard{q:q.clone()};panic!("outer")}));RESULT.with(|v|v.borrow_mut().take().unwrap())},
  "processing"=>{let hook=q.clone();panic::set_hook(Box::new(move |_|{let result=collect(&hook);RESULT.with(|v|*v.borrow_mut()=Some(result))}));let _=panic::catch_unwind(||panic!("outer"));RESULT.with(|v|v.borrow_mut().take().unwrap())},
  _=>collect(&q),
 };
 result["events"]=json!(EVENTS.with(|v|v.borrow().clone()));json!({"request":q,"result":result})
}
fn clones(q:&Value)->Value{
 if text(q,"kind")=="req" {
  let n=num(q,"length")as usize;
  let mut values=Vec::with_capacity(num(q,"source_capacity")as usize);
  for i in 0..n {values.push(value(i as u64+1))}
  let source=VersionReq{comparators:values};
  let mut cloned=source.clone();
  let mut destination=VersionReq{comparators:Vec::with_capacity(num(q,"destination_capacity")as usize)};
  destination.clone_from(&source);
  let mut result=json!({"clone_eq":source==cloned,"clone_from_eq":source==destination,"clone_capacity":cloned.comparators.capacity(),"from_capacity":destination.comparators.capacity(),"source_capacity":source.comparators.capacity(),"length":n});
  let independent=if n>0 {cloned.comparators[0].major=900;destination.comparators[0].major=800;source.comparators[0].major==1&&cloned.comparators[0].major==900&&destination.comparators[0].major==800}else{true};
  result["independent"]=json!(independent);return result
 }
 macro_rules! same {($source:expr,$old:expr)=>{{let source=$source;let cloned=source.clone();let mut destination=$old;destination.clone_from(&source);json!({"clone_eq":cloned==source,"clone_from_eq":destination==source})}}}
 match text(q,"kind") {
  "version"=>same!(Version::parse(text(q,"text")).unwrap(),Version::new(8,9,10)),
  "comparator"=>same!(Comparator::parse(text(q,"text")).unwrap(),value(900)),
  "pre"=>same!(Prerelease::new(text(q,"text")).unwrap(),Prerelease::new("old").unwrap()),
  "build"=>same!(BuildMetadata::new(text(q,"text")).unwrap(),BuildMetadata::new("old").unwrap()),
  "op"=>{let mut clone_eq=true;let mut clone_from_eq=true;for source in [Op::Exact,Op::Greater,Op::GreaterEq,Op::Less,Op::LessEq,Op::Tilde,Op::Caret,Op::Wildcard]{let cloned=source.clone();let mut destination=Op::Less;destination.clone_from(&source);clone_eq&=cloned==source;clone_from_eq&=destination==source};json!({"clone_eq":clone_eq,"clone_from_eq":clone_from_eq})},
  _=>panic!("unknown clone case"),
 }
}
fn main(){if std::env::args().any(|a|a=="--layout"){println!("{} {}",std::mem::size_of::<Comparator>(),std::mem::align_of::<Comparator>());return};let clone=std::env::args().any(|a|a=="--clones");let live=std::env::args().any(|a|a=="--live");LIVE.with(|v|v.set(live));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();if clone{println!("{}",json!({"request":q,"result":clones(&q)}))}else{println!("{}",run(q))}}}
