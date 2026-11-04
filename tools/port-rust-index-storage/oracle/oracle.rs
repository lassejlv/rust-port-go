// SPDX-License-Identifier: MIT OR Apache-2.0
// Validation client only: all storage algorithms come from the unchanged
// verified pinned storage_reference modules assembled by reference-build.
#![feature(trusted_len)]
#![cfg_attr(feature = "nightly", feature(extend_one, specialization))]
use serde_json::{json, Value};
use std::cell::RefCell;
use std::fmt;
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead};
use std::ops::{Bound, RangeBounds};
use std::panic::{catch_unwind, AssertUnwindSafe};
use storage_reference::{Idx, IndexVec};

#[derive(Default)]
struct Hooks { events: Vec<String>, fail: String, nth: usize, count: usize, limit: usize, custom_clone_from: bool, enabled: bool, stream: bool, format_probe: bool, invert_probe: bool, local_debug: bool }
thread_local! { static HOOKS: RefCell<Hooks> = RefCell::new(Hooks::default()); }
fn event(kind: &str, args: &[String]) {
    let (panic,line,stream) = HOOKS.with(|h| {
        let mut h=h.borrow_mut(); if !h.enabled {return (false,String::new(),false)}
        let mut text=kind.to_owned();for arg in args {text.push(':');text.push_str(arg)}h.events.push(text.clone());
        let panic=if h.fail==kind {h.count+=1;h.count==h.nth} else {false};(panic,text,h.stream)
    });
    if stream {println!("ORACLE_EVENT:{}",serde_json::to_string(&line).unwrap())}
    if panic {panic!("oracle injection")}
}
macro_rules! ev {($kind:expr $(,$arg:expr)* $(,)?)=>{event($kind,&[$($arg.to_string()),*])};}
#[cfg(feature="nightly")] mod codec;
mod format;

#[derive(Copy, Clone, PartialEq, Eq, Hash)]
struct K { n: usize, metadata: &'static [u8] }
impl K {fn raw(n:usize)->Self{Self{n,metadata:b"key"}}}
struct InvertLocal(usize);impl Drop for InvertLocal{fn drop(&mut self){ev!("key_debug_local_drop",self.0,std::thread::panicking())}}
impl fmt::Debug for K {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{if HOOKS.with(|h|h.borrow().format_probe){return format::key(self,f)}let _local=if HOOKS.with(|h|h.borrow().local_debug){Some(InvertLocal(self.n))}else{None};if HOOKS.with(|h|h.borrow().invert_probe){ev!("key_debug",self.n,std::thread::panicking())}else{ev!("key_debug",self.n)}write!(f,"{}",self.n)}}
impl Idx for K {
    fn new(n:usize)->Self{ev!("new",n);let limit=HOOKS.with(|h|h.borrow().limit);assert!(n<=limit,"index domain");Self::raw(n)}
    fn index(self)->usize{ev!("index",self.n);self.n}
}
struct E(i64);
impl Clone for E {
    fn clone(&self)->Self{ev!("clone",self.0);E(self.0)}
    fn clone_from(&mut self,source:&Self){
        if HOOKS.with(|h|h.borrow().custom_clone_from){ev!("clone_from",self.0,source.0);self.0=source.0}
        else {*self=source.clone()}
    }
}
impl Drop for E {fn drop(&mut self){ev!("drop",self.0,std::thread::panicking())}}
impl PartialEq for E {
    fn eq(&self,b:&Self)->bool{ev!("eq",self.0,b.0);self.0==b.0}
    fn ne(&self,b:&Self)->bool{ev!("ne",self.0,b.0);self.0!=b.0}
}
impl Eq for E {}
impl PartialOrd for E {fn partial_cmp(&self,b:&Self)->Option<std::cmp::Ordering>{Some(self.cmp(b))}}
impl Ord for E {fn cmp(&self,b:&Self)->std::cmp::Ordering{ev!("cmp",self.0,b.0);self.0.cmp(&b.0)}}
impl fmt::Debug for E {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{ev!("debug",self.0);write!(f,"{}",self.0)}}
impl Hash for E {fn hash<H:Hasher>(&self,_:&mut H){ev!("hash",self.0)}}
struct H;
impl Hasher for H {fn finish(&self)->u64{0}fn write(&mut self,_:&[u8]){panic!("unexpected hash bytes")}fn write_usize(&mut self,n:usize){ev!("hash_len",n)}}

fn n(v:&Value,k:&str)->usize{v.get(k).and_then(Value::as_u64).unwrap_or(0) as usize}
fn integer(v:&Value,k:&str)->i64{v.get(k).and_then(Value::as_i64).unwrap_or(0)}
fn text<'a>(v:&'a Value,k:&str)->&'a str{v.get(k).and_then(Value::as_str).unwrap_or("")}
fn values(v:&Value,k:&str)->Vec<i64>{v.get(k).and_then(Value::as_array).map(|v|v.iter().map(|x|x.as_i64().unwrap()).collect()).unwrap_or_default()}
fn snapshot(v:&IndexVec<K,E>)->Vec<i64>{v.raw.iter().map(|e|e.0).collect()}
fn make_vec(xs:Vec<i64>,cap:usize)->IndexVec<K,E>{let mut raw=Vec::with_capacity(cap);raw.extend(xs.into_iter().map(E));IndexVec::from_raw(raw)}

struct Bounds {a:usize,b:usize,sk:usize,ek:usize}
impl RangeBounds<usize> for Bounds {
    fn start_bound(&self)->Bound<&usize>{ev!("start");match self.sk{0=>Bound::Unbounded,1=>Bound::Included(&self.a),2=>Bound::Excluded(&self.a),_=>panic!("bound kind")}}
    fn end_bound(&self)->Bound<&usize>{ev!("end");match self.ek{0=>Bound::Unbounded,1=>Bound::Included(&self.b),2=>Bound::Excluded(&self.b),_=>panic!("bound kind")}}
}
// The ordinary descriptor has no owned destructor. Separate owned scenarios
// below use a wrapper so a nil Go Close does not create an extra Rust event.
fn bounds(a:&Value)->Bounds{Bounds{a:n(a,"a"),b:n(a,"b"),sk:n(a,"start_kind"),ek:n(a,"end_kind")}}

struct Values {xs:std::vec::IntoIter<E>}
impl Iterator for Values {
    type Item=E;
    fn next(&mut self)->Option<E>{ev!("next");self.xs.next()}
    fn size_hint(&self)->(usize,Option<usize>){ev!("hint");self.xs.size_hint()}
}
impl Drop for Values {fn drop(&mut self){ev!("iterator_close")}}
struct IntoValues(Values);
impl IntoIterator for IntoValues {type Item=E;type IntoIter=Values;fn into_iter(self)->Values{ev!("into_iter");self.0}}
struct Trusted(Values);
impl Iterator for Trusted {type Item=E;fn next(&mut self)->Option<E>{self.0.next()}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}}
// This adapter supplies the same source TrustedLen bound selected by Vec.
unsafe impl std::iter::TrustedLen for Trusted {}
struct IntoTrusted(Trusted);
impl IntoIterator for IntoTrusted {type Item=E;type IntoIter=Trusted;fn into_iter(self)->Trusted{ev!("into_iter");self.0}}

struct NoHint(Values);
impl Iterator for NoHint {type Item=E;fn next(&mut self)->Option<E>{self.0.next()}}
struct IntoNoHint(NoHint);
impl IntoIterator for IntoNoHint {type Item=E;type IntoIter=NoHint;fn into_iter(self)->NoHint{ev!("into_iter");self.0}}
struct Fancy<const EACH:bool,const FOLD:bool>(Values);
impl<const A:bool,const B:bool> Iterator for Fancy<A,B>{
 type Item=E;fn next(&mut self)->Option<E>{self.0.next()}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}
 fn fold<Acc,F:FnMut(Acc,E)->Acc>(mut self,init:Acc,mut f:F)->Acc{if B{ev!("fold")};let mut accum=init;while let Some(value)=self.next(){accum=f(accum,value)}accum}
 fn for_each<F:FnMut(E)>(mut self,mut f:F){if A{ev!("for_each");while let Some(value)=self.next(){f(value)}}else{self.fold((),|(),value|f(value))}}
}
unsafe impl<const A:bool,const B:bool> std::iter::TrustedLen for Fancy<A,B>{}
struct IntoFancy<const A:bool,const B:bool>(Fancy<A,B>);
impl<const A:bool,const B:bool> IntoIterator for IntoFancy<A,B>{type Item=E;type IntoIter=Fancy<A,B>;fn into_iter(self)->Self::IntoIter{ev!("into_iter");self.0}}

fn catch_step(f:impl FnOnce()->Value)->Value{let mut value=Value::Null;let panic=catch_unwind(AssertUnwindSafe(||value=f()));json!({"value":value,"panic":panic.is_err()})}
fn visit_ref<'a,I:Iterator<Item=&'a E>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value],mutate:bool)->Value{assert!(!mutate);let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|e|json!(e.0)).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|e|json!(e.0)).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|e|json!(e.0)).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_mut<'a,I:Iterator<Item=&'a mut E>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|e|{e.0+=1;json!(e.0)}).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_enum_ref<'a,I:Iterator<Item=(K,&'a E)>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_enum_mut<'a,I:Iterator<Item=(K,&'a mut E)>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|(k,e)|{e.0+=1;json!([k.n,e.0])}).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_indices<I:Iterator<Item=K>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|k|json!(k.n)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|k|json!(k.n)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|k|json!(k.n)).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|k|json!(k.n)).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|k|json!(k.n)).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|k|json!(k.n)).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|k|json!(k.n)).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_owned<I:Iterator<Item=E>>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){continue}
match command{"next"=>out.push(it.next().map(|e|json!(e.0)).unwrap_or(Value::Null)),"back"=>{},"len"=>{let(a,b)=it.size_hint();assert_eq!(Some(a),b);out.push(json!(a))},"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|e|json!(e.0)).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_owned_back<I:Iterator<Item=E>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|e|json!(e.0)).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|e|json!(e.0)).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|e|json!(e.0)).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|e|json!(e.0)).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_enum_owned<I:Iterator<Item=(K,E)>>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){continue}
match command{"next"=>out.push(it.next().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)),"back"=>{},"len"=>{let(a,b)=it.size_hint();assert_eq!(Some(a),b);out.push(json!(a))},"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
fn visit_enum_owned_back<I:Iterator<Item=(K,E)>+DoubleEndedIterator+ExactSizeIterator>(mut it:I,commands:&[Value])->Value{let mut out=vec![];for c in commands{let command=c.as_str().unwrap();
if let Some(n)=command.strip_prefix("catch_nth:"){out.push(catch_step(||it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("catch_nth_back:"){out.push(catch_step(||it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)));continue}
if let Some(n)=command.strip_prefix("nth:"){out.push(it.nth(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));continue}
if let Some(n)=command.strip_prefix("nth_back:"){out.push(it.nth_back(n.parse::<usize>().unwrap()).map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));continue}
match command{"next"=>out.push(it.next().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null)),"len"=>out.push(json!(it.len())),"hint"=>{let(a,b)=it.size_hint();out.push(json!([a,b.unwrap()]))},"count"=>{out.push(json!(it.count()));return json!(out)},"last"=>{out.push(it.last().map(|(k,e)|json!([k.n,e.0])).unwrap_or(Value::Null));return json!(out)},"close"=>break,_=>panic!("command")}}json!(out)}
struct SourceRange{start:usize,end:usize}
impl Drop for SourceRange{fn drop(&mut self){ev!("input_close",std::thread::panicking())}}
impl<T> storage_reference::IntoSliceIdx<K,[T]> for SourceRange{type Output=std::ops::Range<usize>;fn into_slice_idx(self)->Self::Output{ev!("convert");self.start..self.end}}

struct FactoryToken;
impl Drop for FactoryToken {fn drop(&mut self){ev!("factory_close",std::thread::panicking())}}
struct OwnedBounds(Bounds);
impl RangeBounds<usize> for OwnedBounds{fn start_bound(&self)->Bound<&usize>{self.0.start_bound()}fn end_bound(&self)->Bound<&usize>{self.0.end_bound()}}
impl Drop for OwnedBounds{fn drop(&mut self){ev!("bounds_close",std::thread::panicking())}}

fn replay_option(r:&Value)->Value {
 HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:n(r,"limit"),custom_clone_from:r["custom_clone_from"].as_bool().unwrap(),enabled:false,..Hooks::default()});
 let make=|xs:Vec<i64>,cap:usize|{let mut raw=Vec::with_capacity(cap);raw.extend(xs.into_iter().map(|n|if n== -1000000{None}else{Some(E(n))}));IndexVec::<K,Option<E>>::from_raw(raw)};
 let snap=|v:&IndexVec<K,Option<E>>|v.raw.iter().map(|e|e.as_ref().map(|e|e.0)).collect::<Vec<_>>();
 let mut v=make(values(r,"values"),n(r,"capacity"));let mut w=make(values(r,"other"),n(r,"other_capacity"));let mut steps=vec![];
 for a in r["actions"].as_array().unwrap(){
  HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.fail=text(a,"panic").to_owned();h.nth=n(a,"panic_at").max(1);h.count=0;h.enabled=true});let mut result=Value::Null;
  let caught=catch_unwind(AssertUnwindSafe(||match text(a,"op"){
   "insert"=>result=v.insert(K::raw(n(a,"a")),E(integer(a,"value"))).map(|e|{let value=e.0;std::mem::forget(e);json!(value)}).unwrap_or(Value::Null),
   "remove"=>result=v.remove(K::raw(n(a,"a"))).map(|e|{let value=e.0;std::mem::forget(e);json!(value)}).unwrap_or(Value::Null),
   "contains"=>result=json!(v.contains(K::raw(n(a,"a")))),
   "get_or_insert"=>result=json!(v.get_or_insert_with(K::raw(n(a,"a")),||{ev!("factory");E(integer(a,"value"))}).0),
   "get_or_insert_owned"=>{let token=FactoryToken;result=json!(v.get_or_insert_with(K::raw(n(a,"a")),move||{let _=&token;ev!("factory");E(integer(a,"value"))}).0)},
   "debug"=>result=json!(format!("{v:?}")),"map"=>result=json!(format!("{:?}",v.debug_map_view())),"compact"=>result=json!(format!("{:?}",v.debug_map_view_compact())),
   "clone_into"=>v.as_slice().clone_into(&mut w),"clone_from"=>w.clone_from(&v),"eq"=>result=json!(v==w),"ne"=>result=json!(v!=w),"close"=>{let old=std::mem::replace(&mut v,IndexVec::new());drop(old)},_=>panic!("option action")
  }));
  let events=HOOKS.with(|h|h.borrow().events.clone());steps.push(json!({"value":result,"panic":caught.is_err(),"events":events,"v":snap(&v),"w":snap(&w),"capacity":v.raw.capacity(),"other_capacity":w.raw.capacity()}));
 }
 HOOKS.with(|h|h.borrow_mut().enabled=false);json!({"steps":steps})
}

fn replay_zst(r:&Value)->Value {
 HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:usize::MAX,enabled:false,..Hooks::default()});let mut v=IndexVec::<K,()>::from_elem_n((),n(r,"capacity"));let mut w=IndexVec::<K,()>::from_elem_n((),n(r,"other_capacity"));let mut steps=vec![];
 for a in r["actions"].as_array().unwrap(){HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.enabled=true});let mut result=Value::Null;let caught=catch_unwind(AssertUnwindSafe(||match text(a,"op"){
 "raw_nth"|"raw_nth_back"=>{let mut it=v.iter();let ok=if text(a,"op")=="raw_nth"{it.nth(n(a,"a")).is_some()}else{it.nth_back(n(a,"a")).is_some()};result=json!([ok,it.len()])},"raw_count"=>result=json!(v.iter().count()),"raw_last"=>result=json!(v.iter().last().is_some()),
 "owned_nth"|"owned_nth_back"=>{let old=std::mem::replace(&mut v,IndexVec::new());let mut it=old.into_iter();let ok=if text(a,"op")=="owned_nth"{it.nth(n(a,"a")).is_some()}else{it.nth_back(n(a,"a")).is_some()};result=json!([ok,it.len()])},"owned_count"=>{let old=std::mem::replace(&mut v,IndexVec::new());result=json!(old.into_iter().count())},"owned_last"=>{let old=std::mem::replace(&mut v,IndexVec::new());result=json!(old.into_iter().last().is_some())},
 "len"=>result=json!([v.len(),v.is_empty()]),"next_index"=>result=json!(v.next_index().n),"last_index"=>result=v.last_index().map(|k|json!(k.n)).unwrap_or(Value::Null),"get"=>result=json!(v.get(K::raw(n(a,"a"))).is_some()),"at"=>{let _=&v[K::raw(n(a,"a"))];result=json!(true)},"push"=>result=json!(v.push(()).n),"pop"=>result=json!(v.pop().is_some()),"truncate"=>v.truncate(n(a,"n")),"resize"=>v.resize(n(a,"n"),()),
 "ensure"=>{let _=v.ensure_contains_elem(K::raw(n(a,"a")),||{ev!("factory");()});result=json!(true)},"resize_elem"=>v.resize_to_elem(K::raw(n(a,"a")),||{ev!("factory");()}),"shrink"=>v.shrink_to_fit(),"clone"=>result=json!(v.clone().len()),"clone_into"=>v.as_slice().clone_into(&mut w),"append"=>v.append(&mut w),"binary"=>result=match v.binary_search(&()){Ok(k)=>json!([k.n,true]),Err(k)=>json!([k.n,false])},
 "indices"=>{let mut it=v.indices();let mut out=vec![];for c in a["commands"].as_array().unwrap(){match c.as_str().unwrap(){"next"=>out.push(it.next().map(|k|json!(k.n)).unwrap_or(Value::Null)),"back"=>out.push(it.next_back().map(|k|json!(k.n)).unwrap_or(Value::Null)),"hint"=>{let(x,y)=it.size_hint();out.push(json!([x,y.unwrap()]))},_=>{}}}result=json!(out)},
 "owned"=>{let old=std::mem::replace(&mut v,IndexVec::new());let mut it=old.into_iter();let mut out=vec![];for c in a["commands"].as_array().unwrap(){match c.as_str().unwrap(){"next"=>out.push(json!(it.next().is_some())),"back"=>out.push(json!(it.next_back().is_some())),"hint"=>{let(x,y)=it.size_hint();out.push(json!([x,y.unwrap()]))},_=>{}}}result=json!(out)},
 "drain"=>{let mut it=v.drain(n(a,"a")..n(a,"b"));let ok=it.next().is_some();let(x,y)=it.size_hint();result=json!([ok,[x,y.unwrap()]])},"from_iter_owned"=>{let old=std::mem::replace(&mut v,IndexVec::new());v=old.into_iter().collect()},"range"=>result=v.get(K::raw(n(a,"a"))..K::raw(n(a,"b"))).map(|s|json!(s.len())).unwrap_or(Value::Null),"copy"=>v.copy_within(K::raw(n(a,"a"))..K::raw(n(a,"b")),K::raw(0)),"close"=>v=IndexVec::new(),_=>panic!("zst action")
 }));let events=HOOKS.with(|h|h.borrow().events.clone());steps.push(json!({"value":result,"panic":caught.is_err(),"events":events,"length":v.len(),"other_length":w.len(),"capacity":v.raw.capacity(),"other_capacity":w.raw.capacity()}));}
 HOOKS.with(|h|h.borrow_mut().enabled=false);json!({"steps":steps})
}

fn replay_invert(r:&Value)->Value{
 HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:n(r,"limit"),enabled:false,stream:text(r,"kind").starts_with("abort"),invert_probe:true,local_debug:r.get("local_debug").and_then(Value::as_bool).unwrap_or(false),..Hooks::default()});let v=IndexVec::<K,K>::from_raw(values(r,"values").into_iter().map(|n|K::raw(n as usize)).collect());let mut steps=vec![];
 for a in r["actions"].as_array().unwrap(){HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.fail=text(a,"panic").to_owned();h.nth=n(a,"panic_at").max(1);h.count=0;h.enabled=true});let mut result=Value::Null;let caught=catch_unwind(AssertUnwindSafe(||{let x=v.invert_bijective_mapping();result=json!({"values":x.raw.iter().map(|k|k.n).collect::<Vec<_>>(),"capacity":x.raw.capacity()})}));let events=HOOKS.with(|h|h.borrow().events.clone());steps.push(json!({"value":result,"panic":caught.is_err(),"events":events,"v":v.raw.iter().map(|k|k.n).collect::<Vec<_>>(),"w":[],"capacity":v.raw.capacity(),"other_capacity":0}));}
 HOOKS.with(|h|h.borrow_mut().enabled=false);json!({"steps":steps})
}

fn replay(r:&Value)->Value {
    if text(r,"kind")=="format" {return format::replay(r)}
    if text(r,"kind")=="invert" || text(r,"kind")=="abort_invert" {return replay_invert(r)}
    #[cfg(feature="nightly")] if text(r,"kind")=="codec" {return codec::replay(r)}
    if text(r,"kind")=="option" {return replay_option(r)}
    if text(r,"kind")=="zst" {return replay_zst(r)}
    HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:n(r,"limit"),custom_clone_from:r["custom_clone_from"].as_bool().unwrap(),enabled:false,stream:text(r,"kind")=="abort",..Hooks::default()});
    let mut v=make_vec(values(r,"values"),n(r,"capacity"));let mut w=make_vec(values(r,"other"),n(r,"other_capacity"));let mut steps=vec![];
    for a in r["actions"].as_array().unwrap(){
        HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.fail=text(a,"panic").to_owned();h.nth=n(a,"panic_at").max(1);h.count=0;h.enabled=true});
        let mut result=Value::Null;
        let caught=catch_unwind(AssertUnwindSafe(||{match text(a,"op"){
            "custom_get"=>result=v.get(SourceRange{start:n(a,"a"),end:n(a,"b")}).map(|slice|json!(slice.iter().map(|e|e.0).collect::<Vec<_>>())).unwrap_or(Value::Null),
            "custom_get_mut"=>result=v.get_mut(SourceRange{start:n(a,"a"),end:n(a,"b")}).map(|slice|json!(slice.iter_mut().map(|e|{e.0+=1;e.0}).collect::<Vec<_>>())).unwrap_or(Value::Null),
            "custom_index"=>result=json!(v[SourceRange{start:n(a,"a"),end:n(a,"b")}].iter().map(|e|e.0).collect::<Vec<_>>()),
            "custom_index_mut"=>result=json!(v[SourceRange{start:n(a,"a"),end:n(a,"b")}].iter_mut().map(|e|{e.0+=1;e.0}).collect::<Vec<_>>()),
            "custom_copy"=>{let mut scalar=IndexVec::<K,i64>::from_raw(snapshot(&v));scalar.copy_within(SourceRange{start:n(a,"a"),end:n(a,"b")},K::raw(n(a,"c")));for(dst,x)in v.raw.iter_mut().zip(scalar.raw){dst.0=x}},
            "outer_panic"=>{let _old=std::mem::replace(&mut v,IndexVec::new());panic!("outer oracle panic")},
            "len"=>result=json!([v.len(),v.is_empty()]),"next_index"=>result=json!(v.next_index().n),"last_index"=>result=v.last_index().map(|k|json!(k.n)).unwrap_or(Value::Null),
            "get"=>result=v.get(K::raw(n(a,"a"))).map(|e|json!(e.0)).unwrap_or(Value::Null),"at"=>result=json!(v[K::raw(n(a,"a"))].0),"write"=>v[K::raw(n(a,"a"))].0=integer(a,"value"),
            "range"=>result=v.get(K::raw(n(a,"a"))..K::raw(n(a,"b"))).map(|s|json!(s.iter().map(|e|e.0).collect::<Vec<_>>())).unwrap_or(Value::Null),
            "swap"=>v.swap(K::raw(n(a,"a")),K::raw(n(a,"b"))),
            "copy"=>{let mut scalar=IndexVec::<K,i64>::from_raw(snapshot(&v));scalar.copy_within(K::raw(n(a,"a"))..K::raw(n(a,"b")),K::raw(n(a,"c")));for(dst,x)in v.raw.iter_mut().zip(scalar.raw){dst.0=x}},
            "pick2"=>{let(x,y)=v.pick2_mut(K::raw(n(a,"a")),K::raw(n(a,"b")));result=json!([x.0,y.0])},
            "pick3"=>{let(x,y,z)=v.pick3_mut(K::raw(n(a,"a")),K::raw(n(a,"b")),K::raw(n(a,"c")));result=json!([x.0,y.0,z.0])},
            "binary"=>{let q=E(integer(a,"value"));result=match v.binary_search(&q){Ok(k)=>json!([k.n,true]),Err(k)=>json!([k.n,false])};std::mem::forget(q)},
            "push"=>result=json!(v.push(E(integer(a,"value"))).n),"pop"=>result=v.pop().map(|e|{let x=e.0;std::mem::forget(e);json!(x)}).unwrap_or(Value::Null),
            "truncate"=>v.truncate(n(a,"n")),"resize"=>v.resize(n(a,"n"),E(integer(a,"value"))),
            "ensure_owned"=>{let token=FactoryToken;result=json!(v.ensure_contains_elem(K::raw(n(a,"a")),move||{let _=&token;ev!("factory");E(integer(a,"value"))}).0)},
            "resize_elem_owned"=>{let token=FactoryToken;v.resize_to_elem(K::raw(n(a,"a")),move||{let _=&token;ev!("factory");E(integer(a,"value"))})},
            "from_fn_owned"=>{let token=FactoryToken;v=IndexVec::from_fn_n(move |k:K|{let _=&token;ev!("factory",k.n);E(k.n as i64)},n(a,"n"))},
            "drain_owned"=>{let commands=a["commands"].as_array().unwrap();result=visit_owned(v.drain(OwnedBounds(bounds(a))),commands)},
            "drain_enum_owned"=>{let commands=a["commands"].as_array().unwrap();result=visit_enum_owned(v.drain_enumerated(OwnedBounds(bounds(a))),commands)},
            "ensure"=>result=json!(v.ensure_contains_elem(K::raw(n(a,"a")),||{ev!("factory");E(integer(a,"value"))}).0),
            "resize_elem"=>v.resize_to_elem(K::raw(n(a,"a")),||{ev!("factory");E(integer(a,"value"))}),"append"=>v.append(&mut w),"shrink"=>v.shrink_to_fit(),"close"=>{let old=std::mem::replace(&mut v,IndexVec::new());drop(old)},
            "clone"|"to_owned"=>{let x=if text(a,"op")=="to_owned"{v.as_slice().to_owned()}else{v.clone()};result=json!(snapshot(&x));drop(x)},"clone_into"=>v.as_slice().clone_into(&mut w),"clone_from"=>w.clone_from(&v),"eq"=>result=json!(v==w),"ne"=>result=json!(v!=w),"debug"=>result=json!(format!("{v:?}")),"map"=>result=json!(format!("{:?}",v.debug_map_view())),"hash"=>v.hash(&mut H),
            "from_elem"=>v=IndexVec::from_elem_n(E(integer(a,"value")),n(a,"n")),
            "from_fn"=>v=IndexVec::from_fn_n(|k:K|{ev!("factory",k.n);E(k.n as i64)},n(a,"n")),
            "from_array"=>{let xs=values(a,"values");v=match xs.len(){
 0=>IndexVec::from(std::array::from_fn::<_,0,_>(|i|E(xs[i]))),
 1=>IndexVec::from(std::array::from_fn::<_,1,_>(|i|E(xs[i]))),
 2=>IndexVec::from(std::array::from_fn::<_,2,_>(|i|E(xs[i]))),
 3=>IndexVec::from(std::array::from_fn::<_,3,_>(|i|E(xs[i]))),
 4=>IndexVec::from(std::array::from_fn::<_,4,_>(|i|E(xs[i]))),
 5=>IndexVec::from(std::array::from_fn::<_,5,_>(|i|E(xs[i]))),
 6=>IndexVec::from(std::array::from_fn::<_,6,_>(|i|E(xs[i]))),
 7=>IndexVec::from(std::array::from_fn::<_,7,_>(|i|E(xs[i]))),
 8=>IndexVec::from(std::array::from_fn::<_,8,_>(|i|E(xs[i]))),
 9=>IndexVec::from(std::array::from_fn::<_,9,_>(|i|E(xs[i]))),_=>panic!("fixed array case")}},
            "from_iter"|"extend_iter"=>{let it=Values{xs:values(a,"values").into_iter().map(E).collect::<Vec<_>>().into_iter()};if text(a,"op").starts_with("extend"){v.extend(IntoValues(it))}else{v=IntoValues(it).into_iter().collect()}},
            "from_iter_no_hint"|"extend_iter_no_hint"=>{let it=Values{xs:values(a,"values").into_iter().map(E).collect::<Vec<_>>().into_iter()};if text(a,"op").starts_with("extend"){v.extend(IntoNoHint(NoHint(it)))}else{v=IntoNoHint(NoHint(it)).into_iter().collect()}},
            "from_iter_trusted"|"extend_iter_trusted"=>{let it=Values{xs:values(a,"values").into_iter().map(E).collect::<Vec<_>>().into_iter()};if text(a,"op").starts_with("extend"){v.extend(IntoTrusted(Trusted(it)))}else{v=IntoTrusted(Trusted(it)).into_iter().collect()}},
            "from_iter_trusted_each"|"extend_iter_trusted_each"=>{let it=Values{xs:values(a,"values").into_iter().map(E).collect::<Vec<_>>().into_iter()};if text(a,"op").starts_with("extend"){v.extend(IntoFancy(Fancy::<true,false>(it)))}else{v=IntoFancy(Fancy::<true,false>(it)).into_iter().collect()}},
            "from_iter_trusted_fold"|"extend_iter_trusted_fold"=>{let it=Values{xs:values(a,"values").into_iter().map(E).collect::<Vec<_>>().into_iter()};if text(a,"op").starts_with("extend"){v.extend(IntoFancy(Fancy::<false,true>(it)))}else{v=IntoFancy(Fancy::<false,true>(it)).into_iter().collect()}},
            "from_iter_owned"=>{let old=std::mem::replace(&mut v,IndexVec::new());let mut it=old.into_iter();for _ in 0..n(a,"a"){drop(it.next())}for _ in 0..n(a,"b"){drop(it.next_back())}v=it.collect()},
            "iter"|"iter_mut"|"iter_enum"|"iter_enum_mut"|"indices"|"owned"|"owned_enum"|"drain"|"drain_enum"=>{
                let commands=a["commands"].as_array().unwrap();result=match text(a,"op"){
                    "iter"=>visit_ref(v.iter(),commands,false),"iter_mut"=>visit_mut(v.iter_mut(),commands),"iter_enum"=>visit_enum_ref(v.iter_enumerated(),commands),"iter_enum_mut"=>visit_enum_mut(v.iter_enumerated_mut(),commands),"indices"=>visit_indices(v.indices(),commands),
                    "owned"=>{let old=std::mem::replace(&mut v,IndexVec::new());visit_owned_back(old.into_iter(),commands)},
                    "owned_enum"=>{let old=std::mem::replace(&mut v,IndexVec::new());visit_enum_owned_back(old.into_iter_enumerated(),commands)},
                    "drain"=>visit_owned(v.drain(bounds(a)),commands),"drain_enum"=>visit_enum_owned(v.drain_enumerated(bounds(a)),commands),_=>unreachable!()};
            },
            op=>panic!("unknown operation {op}"),
        }}));
        let events=HOOKS.with(|h|h.borrow().events.clone());
        steps.push(json!({"value":result,"panic":caught.is_err(),"events":events,"v":snapshot(&v),"w":snapshot(&w),"capacity":v.raw.capacity(),"other_capacity":w.raw.capacity()}));
    }
    HOOKS.with(|h|h.borrow_mut().enabled=false);
    json!({"steps":steps})
}
fn main(){std::panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let line=line.unwrap();if line.is_empty(){continue}let request:Value=serde_json::from_str(&line).unwrap();println!("{}",replay(&request));}}
