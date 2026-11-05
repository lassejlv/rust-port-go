// Test caller for the six actual aliases in the whole unchanged def_id.rs.
use def_id_reference::{*,unord::{UnordMap,UnordSet}};
use serde_json::{json,Value};
use std::{cell::RefCell,rc::Rc,hash::Hash,fmt,panic::{self,AssertUnwindSafe},io::{self,BufRead}};
trait Key:Copy+Eq+Hash+fmt::Debug{fn key(n:u32)->Self;}
impl Key for DefId{fn key(n:u32)->Self{DefId{krate:CrateNum::from_u32(n%3),index:DefIndex::from_u32(n*7)}}}
impl Key for LocalDefId{fn key(n:u32)->Self{LocalDefId{local_def_index:DefIndex::from_u32(n*7)}}}
#[derive(Default)]struct Log{events:Vec<String>,fail_clone:i64,fail_drop:i64,dropped:bool}
type Shared=Rc<RefCell<Log>>;
fn event(log:&Shared,s:String){log.borrow_mut().events.push(s)}
struct Item{tag:u64,log:Shared}
impl Item{fn new(tag:u64,log:&Shared)->Self{Self{tag,log:log.clone()}}}
impl Clone for Item{fn clone(&self)->Self{event(&self.log,format!("clone:{}:{}",self.tag,std::thread::panicking()));if self.log.borrow().fail_clone==self.tag as i64{panic!("clone")};Self::new(self.tag,&self.log)}}
impl PartialEq for Item{fn eq(&self,other:&Self)->bool{event(&self.log,format!("equal:{}:{}:{}",self.tag,other.tag,std::thread::panicking()));self.tag==other.tag}}
impl Eq for Item{}
impl fmt::Debug for Item{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.tag.fmt(f)}}
impl Drop for Item{fn drop(&mut self){event(&self.log,format!("drop:{}:{}",self.tag,std::thread::panicking()));let fail={let mut s=self.log.borrow_mut();let fail=s.fail_drop==self.tag as i64&&!s.dropped;s.dropped|=fail;fail};if fail{panic!("drop")}}}
fn payload(p:Box<dyn std::any::Any+Send>)->String{if let Some(v)=p.downcast_ref::<&str>(){(*v).into()}else if let Some(v)=p.downcast_ref::<String>(){v.clone()}else{"opaque".into()}}
fn snapshot<K:Key>(map:&UnordMap<K,Item>)->Value{json!({"len":map.len(),"values":(0..10).map(|n|map.get(&K::key(n)).map(|v|v.tag)).collect::<Vec<_>>()})}
fn maps<K:Key>(q:&Value,mut source:UnordMap<K,Item>,mut dest:UnordMap<K,Item>,log:Shared)->Value{
 if q["constructor"]!="iterator"{for n in 0..6{source.insert(K::key(n),Item::new(10+u64::from(n),&log));}}
 for n in 0..3{dest.insert(K::key(n),Item::new(100+u64::from(n),&log));}
 drop(source.insert(K::key(2),Item::new(42,&log)));
 source.entry(K::key(1)).and_modify(|v|{event(&v.log,format!("modify:{}",v.tag));v.tag+=1000}).or_insert(Item::new(999,&log));
 source.entry(K::key(9)).and_modify(|v|{event(&v.log,format!("modify:{}",v.tag));v.tag+=1000}).or_insert(Item::new(99,&log));
 drop(source.remove(&K::key(3)));
 {let mut s=log.borrow_mut();s.fail_clone=q["clone"].as_i64().unwrap();s.fail_drop=q["drop"].as_i64().unwrap();}
 let mut cloned=None;
 let error=panic::catch_unwind(AssertUnwindSafe(||{if q["action"]=="clone_from"{dest.clone_from(&source)}else{cloned=Some(source.clone())}})).err().map(payload);
 {let mut s=log.borrow_mut();s.fail_clone=-1;s.fail_drop=-1;}
 let equal=cloned.as_ref().map(|v|source==*v);
 let result=json!({"source":snapshot(&source),"dest":snapshot(&dest),"cloned":cloned.as_ref().map(snapshot),"equal":equal,"panic":error,"debug":format!("{source:?}"),"pair_layout":[std::mem::size_of::<(K,Item)>(),std::mem::align_of::<(K,Item)>()]});
 drop(cloned);drop(dest);drop(source);
 json!({"state":result,"events":log.borrow().events.clone()})
}
fn sets<K:Key>(mut source:UnordSet<K>,mut dest:UnordSet<K>)->Value{
 let inserted=(0..10).map(|n|source.insert(K::key(n))).collect::<Vec<_>>();let duplicate=source.insert(K::key(2));let removed=source.remove(&K::key(4));let missing=source.remove(&K::key(11));
 let cloned=source.clone();dest.insert(K::key(20));dest.clone_from(&source);let equal=dest==cloned;let text=format!("{source:?}");source.clear();source.insert(K::key(9));let only=source.get_only().map(|v|*v==K::key(9));
 json!({"inserted":inserted,"duplicate":duplicate,"removed":removed,"missing":missing,"len":dest.len(),"equal":equal,"only":only,"debug":text,"contains":(0..12).map(|n|dest.contains(&K::key(n))).collect::<Vec<_>>(),"key_layout":[std::mem::size_of::<K>(),std::mem::align_of::<K>()]})
}
fn run(q:&Value)->Value{
 let cap=q["capacity"].as_u64().unwrap()as usize;
 if q["kind"]=="set"{return if q["local"]==true{let source:LocalDefIdSet=if q["constructor"]=="iterator"{(0..10).map(LocalDefId::key).collect::<Vec<_>>().into_iter().collect()}else{LocalDefIdSet::with_capacity(cap)};sets(source,LocalDefIdSet::new())}else{let source:DefIdSet=if q["constructor"]=="iterator"{(0..10).map(DefId::key).collect::<Vec<_>>().into_iter().collect()}else{DefIdSet::with_capacity(cap)};sets(source,DefIdSet::new())}}
 let log=Rc::new(RefCell::new(Log{fail_clone:-1,fail_drop:-1,..Log::default()}));
 if q["local"]==true{let source:LocalDefIdMap<Item>=if q["constructor"]=="iterator"{(0..6).map(|n|(LocalDefId::key(n),Item::new(10+u64::from(n),&log))).collect::<Vec<_>>().into_iter().collect()}else{LocalDefIdMap::with_capacity(cap)};maps(q,source,LocalDefIdMap::with_capacity(cap),log)}
 else{let source:DefIdMap<Item>=if q["constructor"]=="iterator"{(0..6).map(|n|(DefId::key(n),Item::new(10+u64::from(n),&log))).collect::<Vec<_>>().into_iter().collect()}else{DefIdMap::with_capacity(cap)};maps(q,source,DefIdMap::with_capacity(cap),log)}
}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
