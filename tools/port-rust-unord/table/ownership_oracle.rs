// Development-only owned HashMap/Entry probe. Every operation below executes
// the real std type with ordinary Rust Hash/Eq/Clone/Drop/FnOnce implementations.
#![feature(map_try_insert)]
use rustc_hash::FxHasher;
use std::{borrow::Borrow,cell::RefCell,collections::{HashMap,hash_map::Entry},hash::{BuildHasher,Hash,Hasher},io::{self,BufRead,Write},panic::{catch_unwind,AssertUnwindSafe}};
#[derive(Default)]struct Control{events:Vec<String>,counter:usize,fail:i64,second:i64,fired:bool,enabled:bool,live:bool}
thread_local!{static CONTROL:RefCell<Control>=RefCell::new(Control::default());}
fn event(kind:&str,id:u64){let(fail,live,text)=CONTROL.with(|cell|{let mut c=cell.borrow_mut();if !c.enabled{return(None,false,String::new())}let text=format!("{kind}:{id}:{}",std::thread::panicking());c.events.push(text.clone());let index=c.counter;c.counter+=1;let fail=if index as i64==c.fail&&!c.fired{c.fired=true;Some(index)}else if index as i64==c.second&&c.fired{Some(index)}else{None};(fail,c.live,text)});if live{eprintln!("PROBE_EVENT\t{}",hex(&text));}if let Some(index)=fail{std::panic::panic_any(index)}}
fn mark(text:&str){let live=CONTROL.with(|cell|{let mut c=cell.borrow_mut();c.events.push(text.into());c.live});if live{eprintln!("PROBE_EVENT\t{}",hex(text));}}
struct Key{slot:u64,token:u64}
impl Hash for Key{fn hash<H:Hasher>(&self,h:&mut H){event("hash",self.token);h.write_u64(self.slot)}}
impl PartialEq for Key{fn eq(&self,other:&Self)->bool{event("eq",self.token*1000+other.token);self.slot==other.slot}}
impl Eq for Key{}
impl Borrow<u64>for Key{fn borrow(&self)->&u64{&self.slot}}
impl Clone for Key{fn clone(&self)->Self{event("kclone",self.token);Self{slot:self.slot,token:self.token+100}}}
impl Drop for Key{fn drop(&mut self){event("kdrop",self.token)}}
struct Value(u64);
impl Clone for Value{fn clone(&self)->Self{event("vclone",self.0);Self(self.0+100)}}
impl Drop for Value{fn drop(&mut self){event("vdrop",self.0)}}
struct AuditHasher(FxHasher);
impl Hasher for AuditHasher{fn finish(&self)->u64{event("finish",0);self.0.finish()}fn write(&mut self,b:&[u8]){event("bytes",b.len()as u64);self.0.write(b)}fn write_u64(&mut self,v:u64){event("write",v);self.0.write_u64(v)}}
impl Drop for AuditHasher{fn drop(&mut self){event("hdrop",0)}}
struct Builder;
impl BuildHasher for Builder{type Hasher=AuditHasher;fn build_hasher(&self)->AuditHasher{event("build",0);AuditHasher(FxHasher::default())}}
impl Clone for Builder{fn clone(&self)->Self{event("bclone",0);Self}fn clone_from(&mut self,_:&Self){event("bclonefrom",0)}}
impl Drop for Builder{fn drop(&mut self){event("bdrop",0)}}
type Map=HashMap<Key,Value,Builder>;
fn initial(offset:u64)->Map{let mut m=Map::with_hasher(Builder);for slot in 1..=3{m.insert(Key{slot,token:slot+10+offset},Value(slot+20+offset));}m}
struct Env{token:u64,calls:u64}
impl Drop for Env{fn drop(&mut self){event("envdrop",self.token*1000+self.calls)}}
impl Env{fn next(&mut self)->Value{self.calls+=1;event("call",self.token);Value(90+self.calls)}fn modify(&mut self,value:&mut Value){self.calls+=1;value.0+=1;event("call",self.token)}}
fn snapshot(map:&Option<Map>)->String{match map{None=>"none".into(),Some(m)=>{let mut text=format!("{}:{}:",m.len(),m.capacity());for(k,v)in m{text.push_str(&format!("{}/{}/{},",k.slot,k.token,v.0));}text}}}
fn outcome(result:Result<(),Box<dyn std::any::Any+Send>>)->String{match result{Ok(())=>"ok".into(),Err(e)=>if let Some(index)=e.downcast_ref::<usize>(){format!("event:{index}")}else{"capacity".into()}}}
fn work(scenario:usize)->(String,String,String){
 let mut map=if scenario==24{None}else{Some(initial(0))};let mut other=if scenario==14{Some(initial(200))}else{None};
 CONTROL.with(|c|{let mut c=c.borrow_mut();c.enabled=true;c.counter=0;c.events.clear()});
 let action=catch_unwind(AssertUnwindSafe(||{match scenario{
 0=>{drop(map.as_mut().unwrap().insert(Key{slot:4,token:44},Value(54)));},
 1=>{drop(map.as_mut().unwrap().insert(Key{slot:1,token:41},Value(51)));},
 2=>{drop(map.as_mut().unwrap().remove(&1u64));},
 3=>{drop(map.as_mut().unwrap().entry(Key{slot:1,token:41}));},
 4=>{drop(map.as_mut().unwrap().entry(Key{slot:4,token:44}));},
 5|6=>{let slot=if scenario==5{1}else{4};let _=map.as_mut().unwrap().entry(Key{slot,token:slot+40}).or_insert(Value(slot+50));},
 7|8=>{let slot=if scenario==7{1}else{4};drop(map.as_mut().unwrap().try_insert(Key{slot,token:slot+40},Value(slot+50)));},
 9=>{if let Entry::Occupied(mut e)=map.as_mut().unwrap().entry(Key{slot:1,token:41}){drop(e.insert(Value(51)));}},
 10=>{if let Entry::Occupied(e)=map.as_mut().unwrap().entry(Key{slot:1,token:41}){drop(e.remove());}},
 11=>{if let Entry::Occupied(e)=map.as_mut().unwrap().entry(Key{slot:1,token:41}){drop(e.remove_entry());}},
 12=>{if let Entry::Vacant(e)=map.as_mut().unwrap().entry(Key{slot:4,token:44}){drop(e.into_key());}},
 13=>{drop(map.as_ref().unwrap().clone());},
 14=>{map.as_mut().unwrap().clone_from(other.as_ref().unwrap());},
 15=>{map.as_mut().unwrap().clear();},
 16=>{drop(map.take().unwrap().into_iter());},
 17=>{let mut i=map.take().unwrap().into_iter();let first=i.next();drop(i);drop(first);},
 18|19=>{let slot=if scenario==18{1}else{4};let mut env=Env{token:90,calls:0};let _=map.as_mut().unwrap().entry(Key{slot,token:slot+40}).or_insert_with_key(move|_key|env.next());},
 20|21=>{let slot=if scenario==20{1}else{4};let mut env=Env{token:90,calls:0};drop(map.as_mut().unwrap().entry(Key{slot,token:slot+40}).and_modify(move|value|env.modify(value)));},
 22|23=>{let slot=if scenario==22{1}else{4};drop(map.as_mut().unwrap().entry(Key{slot,token:slot+40}).insert_entry(Value(slot+50)));},
 24=>{drop(Map::with_capacity_and_hasher(usize::MAX,Builder));},
 25=>{let replacement=map.as_ref().unwrap().clone();*map.as_mut().unwrap()=replacement;},
 26=>{let _=map.as_ref().unwrap().get(&1u64);},
 27=>{let query=Key{slot:1,token:41};drop(map.as_mut().unwrap().remove(&query));},
 _=>panic!("unknown scenario")
 }}));
 let state=snapshot(&map);mark("final");let final_drop=catch_unwind(AssertUnwindSafe(||{drop(other.take());drop(map.take());}));
 (outcome(action),outcome(final_drop),state)
}
fn ambient<T>(name:&str,f:impl FnOnce()->T)->T{if name=="normal"{return f()}assert_eq!(name,"unwinding");struct Guard<'a,F:FnOnce()->T,T>{call:Option<F>,out:&'a RefCell<Option<T>>}impl<F:FnOnce()->T,T>Drop for Guard<'_,F,T>{fn drop(&mut self){*self.out.borrow_mut()=Some((self.call.take().unwrap())());}}let result=RefCell::new(None);let _=catch_unwind(AssertUnwindSafe(||{let _guard=Guard{call:Some(f),out:&result};panic!("ambient")}));result.into_inner().unwrap()}
thread_local!{static PROCESSING_SCENARIO:RefCell<usize>=RefCell::new(0);static PROCESSING_RESULT:RefCell<Option<(String,String,String)>>=RefCell::new(None);}
fn processing(scenario:usize)->(String,String,String){PROCESSING_SCENARIO.with(|s|*s.borrow_mut()=scenario);PROCESSING_RESULT.with(|r|*r.borrow_mut()=None);let old=std::panic::take_hook();std::panic::set_hook(Box::new(|_|{let scenario=PROCESSING_SCENARIO.with(|s|*s.borrow());let result=work(scenario);PROCESSING_RESULT.with(|r|*r.borrow_mut()=Some(result));}));let _=catch_unwind(||panic!("processing"));std::panic::set_hook(old);PROCESSING_RESULT.with(|r|r.borrow_mut().take().unwrap())}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));let mut out=io::BufWriter::new(io::stdout().lock());for line in io::stdin().lock().lines(){let line=line?;let fields:Vec<_>=line.split('\t').collect();assert!(fields.len()==3||fields.len()==4);let scenario=fields[0].parse().unwrap();let name=fields[1];let fail=fields[2].parse().unwrap();let second=fields.get(3).map(|s|s.parse().unwrap()).unwrap_or(-1);CONTROL.with(|c|*c.borrow_mut()=Control{fail,second,live:fields.len()==4||name=="processing_panic",..Default::default()});let(action,final_drop,state)=if name=="processing_panic"{processing(scenario)}else{ambient(name,||work(scenario))};let(counters,events)=CONTROL.with(|c|{let c=c.borrow();(c.counter,c.events.join("|"))});writeln!(out,"{line}\t{action}\t{final_drop}\t{counters}\t{}\t{}",hex(&state),hex(&events))?;}Ok(())}
