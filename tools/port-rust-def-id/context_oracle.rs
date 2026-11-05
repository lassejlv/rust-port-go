// Whole def_id ContextOrd with custom non-Copy Fn environment and all supertrait overrides.
#![feature(fn_traits,unboxed_closures)]
use def_id_reference::{*,fingerprint::Fingerprint};
use rustc_hashes::Hash64;
use rustc_serialize::{Encoder,ContextOrd};
use serde_json::{Value,json};
use std::{cell::{Cell,RefCell},rc::Rc,panic::{self,AssertUnwindSafe},io::{self,BufRead,Write}};
thread_local!{static EVENTS:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};static RESULT:RefCell<Option<Value>>=const{RefCell::new(None)};static LIVE:Cell<bool>=const{Cell::new(false)};}
fn event(s:String){if LIVE.with(|v|v.get()){println!("EVENT\t{s}");io::stdout().flush().unwrap()};EVENTS.with(|v|v.borrow_mut().push(s));}
fn text<'a>(q:&'a Value,k:&str)->&'a str{q[k].as_str().unwrap_or("")}fn bit(q:&Value,k:&str)->bool{q[k].as_bool().unwrap_or(false)}
struct State{q:Value,calls:Cell<u64>}
struct Token(Rc<State>);impl Drop for Token{fn drop(&mut self){event(format!("field-drop:panic={}",std::thread::panicking()));if bit(&self.0.q,"field_drop"){panic!("field drop")}}}
struct H{state:Rc<State>,token:Token,non_copy:Vec<u8>,bump:u64}
impl H{fn key(&self,id:DefId,extra:u64)->DefPathHash{DefPathHash(Fingerprint::new(Hash64::new(id.krate.as_u32()as u64),Hash64::new(id.index.as_u32()as u64+self.bump+extra)))}fn call_event(&self,kind:&str,id:DefId){let n=self.state.calls.get();self.state.calls.set(n+1);event(format!("call:{kind}:{}:{}:panic={}",id.krate.as_u32(),id.index.as_u32(),std::thread::panicking()));if self.state.q["panic_call"].as_u64()==Some(n){panic!("call")}}}
impl Drop for H{fn drop(&mut self){event(format!("env-drop:panic={}",std::thread::panicking()));if bit(&self.state.q,"drop"){panic!("env drop")}}}
impl FnOnce<(DefId,)> for H{type Output=DefPathHash;extern "rust-call" fn call_once(self,(id,):(DefId,))->DefPathHash{self.call_event("once",id);let result=self.key(id,100);if bit(&self.state.q,"forget"){std::mem::forget(self)};result}}
impl FnMut<(DefId,)> for H{extern "rust-call" fn call_mut(&mut self,(id,):(DefId,))->DefPathHash{self.bump+=10;self.call_event("mut",id);self.key(id,0)}}
impl Fn<(DefId,)> for H{extern "rust-call" fn call(&self,(id,):(DefId,))->DefPathHash{self.call_event("fn",id);self.key(id,0)}}
struct E<const OVERRIDE:bool>{state:Rc<State>}
macro_rules! emit{($($name:ident:$t:ty),*)=>{$(fn $name(&mut self,_:$t){panic!(concat!("unused encoder ",stringify!($name)))})*}}
impl<const B:bool>Encoder for E<B>{emit!(emit_usize:usize,emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_u8:u8,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);fn emit_raw_bytes(&mut self,_:&[u8]){panic!("unused encoder raw")}}
macro_rules! required{()=>{fn encode_def_id(&mut self,_:DefId){panic!("unused encode def") }fn encode_def_index(&mut self,_:DefIndex){panic!("unused encode index")}fn encode_crate_num(&mut self,_:CrateNum){panic!("unused encode crate")}}}
impl SpanEncoder for E<false>{required!();}
impl SpanEncoder for E<true>{required!();fn def_path_hasher(&self)->impl Fn(DefId)->DefPathHash{event(format!("new:panic={}",std::thread::panicking()));if bit(&self.state.q,"get"){panic!("get")};H{state:self.state.clone(),token:Token(self.state.clone()),non_copy:vec![1,2,3],bump:0}}}
fn id(c:u32,i:u32)->DefId{DefId{krate:CrateNum::from_u32(c),index:DefIndex::from_u32(i)}}
fn action<E:SpanEncoder>(e:&E,q:&Value)->Value{let result=panic::catch_unwind(AssertUnwindSafe(||{
 match text(q,"kind"){
 "raw"=>{let mut h=e.def_path_hasher();let a=Fn::call(&h,(id(1,2),));event(format!("value:{}",a.local_hash().as_u64()));let b=FnMut::call_mut(&mut h,(id(1,3),));event(format!("value:{}",b.local_hash().as_u64()));let c=FnOnce::call_once(h,(id(1,4),));event(format!("value:{}",c.local_hash().as_u64()));},
 "crate"=>{let mut cmp=<CrateNum as ContextOrd<&E>>::stable_cmp_with_context(e);if bit(q,"once"){let result=FnOnce::call_once(cmp,(&CrateNum::from_u32(2),&CrateNum::from_u32(1)));event(format!("value:{result:?}"))}else{let result=panic::catch_unwind(AssertUnwindSafe(||cmp(&CrateNum::from_u32(2),&CrateNum::from_u32(1))));event(format!("inner-caught:{}",result.is_err()));drop(cmp);}},
 "local"=>{let a=LocalDefId{local_def_index:DefIndex::from_u32(2)};let b=LocalDefId{local_def_index:DefIndex::from_u32(1)};let mut cmp=<LocalDefId as ContextOrd<&E>>::stable_cmp_with_context(e);if bit(q,"once"){let result=FnOnce::call_once(cmp,(&a,&b));event(format!("value:{result:?}"))}else{let result=panic::catch_unwind(AssertUnwindSafe(||cmp(&a,&b)));event(format!("inner-caught:{}",result.is_err()));drop(cmp);}},
 _=>{let mut cmp=<DefId as ContextOrd<&E>>::stable_cmp_with_context(e);if bit(q,"once"){let result=FnOnce::call_once(cmp,(&id(2,2),&id(1,1)));event(format!("value:{result:?}"))}else{let result=panic::catch_unwind(AssertUnwindSafe(||cmp(&id(2,2),&id(1,1))));event(format!("inner-caught:{}",result.is_err()));drop(cmp);}},
 };}));event(format!("caught:{}",result.is_err()));json!({"panic":result.is_err()})}
fn start(q:&Value)->Value{let state=Rc::new(State{q:q.clone(),calls:Cell::new(0)});if bit(q,"default"){action(&E::<false>{state},q)}else{action(&E::<true>{state},q)}}
struct Guard(Value);impl Drop for Guard{fn drop(&mut self){event(format!("inherited:panic={}",std::thread::panicking()));let result=start(&self.0);RESULT.with(|v|*v.borrow_mut()=Some(result));}}
fn run(q:Value)->Value{EVENTS.with(|v|v.borrow_mut().clear());RESULT.with(|v|*v.borrow_mut()=None);panic::set_hook(Box::new(|_|{}));let result=match text(&q,"context"){"inherited"=>{let _=panic::catch_unwind(AssertUnwindSafe(||{let _guard=Guard(q.clone());panic!("outer")}));event("outer-caught:true".into());RESULT.with(|v|v.borrow_mut().take().unwrap())},"processing"=>{let query=q.clone();panic::set_hook(Box::new(move |_|{event(format!("processing:panic={}",std::thread::panicking()));let result=start(&query);RESULT.with(|v|*v.borrow_mut()=Some(result));}));let _=panic::catch_unwind(||panic!("outer"));event("outer-caught:true".into());RESULT.with(|v|v.borrow_mut().take().unwrap())},_=>start(&q)};json!({"request":q,"result":result,"events":EVENTS.with(|v|v.borrow().clone())})}
fn main(){let live=std::env::args().any(|a|a=="--live");LIVE.with(|v|v.set(live));for line in io::stdin().lock().lines(){let q=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",run(q))}}
