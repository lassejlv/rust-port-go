// Actual source admission-bound effects; no replacement Unord methods.
use rustc_data_structures::unord::{UnordItems,UnordBag,UnordMap,UnordSet};
use rustc_data_structures::stable_hash::{DummyStableHashCtxt,StableHash,StableHashCtxt,StableHasher,ToStableHashKey};
use std::{alloc::{GlobalAlloc,Layout,System},cmp::Ordering,hash::Hash,iter::{Sum,Product},panic::{catch_unwind,AssertUnwindSafe},sync::atomic::{AtomicBool,AtomicUsize,Ordering as AtomicOrdering}};
fn event(s:impl AsRef<str>){println!("{}",s.as_ref())}
static OBSERVE:AtomicBool=AtomicBool::new(false);static ALLOCS:AtomicUsize=AtomicUsize::new(0);static MAX:AtomicUsize=AtomicUsize::new(0);
struct Allocator;unsafe impl GlobalAlloc for Allocator{unsafe fn alloc(&self,l:Layout)->*mut u8{if OBSERVE.load(AtomicOrdering::Relaxed){ALLOCS.fetch_add(1,AtomicOrdering::Relaxed);MAX.fetch_max(l.size(),AtomicOrdering::Relaxed);}unsafe{System.alloc(l)}}unsafe fn dealloc(&self,p:*mut u8,l:Layout){unsafe{System.dealloc(p,l)}}unsafe fn realloc(&self,p:*mut u8,l:Layout,n:usize)->*mut u8{if OBSERVE.load(AtomicOrdering::Relaxed){ALLOCS.fetch_add(1,AtomicOrdering::Relaxed);MAX.fetch_max(n,AtomicOrdering::Relaxed);}unsafe{System.realloc(p,l,n)}}}
#[global_allocator]static ALLOCATOR:Allocator=Allocator;
struct DefaultValue(u64);impl Default for DefaultValue{fn default()->Self{panic!("UNEXPECTED Default")}}impl Drop for DefaultValue{fn drop(&mut self){event(format!("item:{}",self.0))}}
struct Value(u64);impl Clone for Value{fn clone(&self)->Self{panic!("UNEXPECTED Clone")}}impl Copy for Value{}
impl PartialEq for Value{fn eq(&self,_:&Self)->bool{panic!("UNEXPECTED Eq")}}impl Eq for Value{}
impl PartialOrd for Value{fn partial_cmp(&self,_:&Self)->Option<Ordering>{panic!("UNEXPECTED PartialOrd")}}
impl Ord for Value{fn cmp(&self,_:&Self)->Ordering{panic!("UNEXPECTED Ord")}}
struct Input(u64);impl Clone for Input{fn clone(&self)->Self{event(format!("clone:{}",self.0));Input(self.0+100)}}impl Drop for Input{fn drop(&mut self){event(format!("input:{}",self.0))}}
impl Iterator for Input{type Item=Value;fn next(&mut self)->Option<Value>{panic!("UNEXPECTED next")}
 fn max(self)->Option<Value>{event("max_override");Some(Value(7))}fn min(self)->Option<Value>{event("min_override");Some(Value(7))}
 fn sum<S:Sum<Value>>(self)->S{event("sum_override");panic!("sum_override")}fn product<P:Product<Value>>(self)->P{event("product_override");panic!("product_override")}}
struct Total;impl Sum<Value>for Total{fn sum<I:Iterator<Item=Value>>(_:I)->Self{panic!("UNEXPECTED Sum")}}impl Product<Value>for Total{fn product<I:Iterator<Item=Value>>(_:I)->Self{panic!("UNEXPECTED Product")}}
struct RefInput<'a>(&'a Value);impl Drop for RefInput<'_>{fn drop(&mut self){event("ref_input")}}
impl<'a>Iterator for RefInput<'a>{type Item=&'a Value;fn next(&mut self)->Option<Self::Item>{panic!("UNEXPECTED ref next")}
 fn cloned<'b,T:'b>(self)->std::iter::Cloned<Self>where Self:Sized+Iterator<Item=&'b T>,T:Clone{event("cloned_override");panic!("cloned_override")}
 fn copied<'b,T:'b>(self)->std::iter::Copied<Self>where Self:Sized+Iterator<Item=&'b T>,T:Copy{event("copied_override");panic!("copied_override")}}
#[derive(Clone,Copy,Eq,PartialEq,Hash)]struct Item(u64);
struct Key(u64);impl PartialEq for Key{fn eq(&self,_:&Self)->bool{panic!("UNEXPECTED key Eq")}}impl Eq for Key{}
impl PartialOrd for Key{fn partial_cmp(&self,b:&Self)->Option<Ordering>{event(format!("partial:{}:{}",self.0,b.0));Some(self.0.cmp(&b.0))}fn lt(&self,b:&Self)->bool{event(format!("lt:{}:{}",self.0,b.0));self.0<b.0}}
impl Ord for Key{fn cmp(&self,_:&Self)->Ordering{panic!("UNEXPECTED key Ord")}}
impl StableHash for Key{fn stable_hash<H:StableHashCtxt>(&self,_:&mut H,_:&mut StableHasher){panic!("UNEXPECTED key StableHash")}}
impl ToStableHashKey for Item{type KeyType=Key;fn to_stable_hash_key<H:StableHashCtxt>(&self,_:&mut H)->Key{event(format!("project:{}",self.0));Key(self.0)}}
fn show(values:impl Iterator<Item=u64>){event(format!("returned:{}",values.map(|v|v.to_string()).collect::<Vec<_>>().join(",")))}
fn primitive(family:usize){match family{
 0=>{OBSERVE.store(true,AtomicOrdering::Relaxed);let mut bag:UnordBag<DefaultValue>=Default::default();OBSERVE.store(false,AtomicOrdering::Relaxed);let empty_allocs=ALLOCS.swap(0,AtomicOrdering::Relaxed);event(format!("empty:{}",bag.len()));OBSERVE.store(true,AtomicOrdering::Relaxed);bag.push(DefaultValue(7));OBSERVE.store(false,AtomicOrdering::Relaxed);let allocations=ALLOCS.load(AtomicOrdering::Relaxed);let bytes=MAX.load(AtomicOrdering::Relaxed);assert_eq!((allocations,bytes),(1,32));event(format!("push:{}:capacity{}",bag.len(),bytes/8));event(format!("ALLOC:{empty_allocs}:{allocations}:{bytes}"));drop(bag)},
 1=>{let source=UnordItems::new(Input(1));let mut destination=UnordItems::new(Input(2));drop(source.clone());destination.clone_from(&source);drop(destination);drop(source)},
 2=>{let value=UnordItems::new(Input(0)).max();event(format!("returned:{}",value.unwrap().0))},
 3=>{let value=UnordItems::new(Input(0)).min();event(format!("returned:{}",value.unwrap().0))},
 4=>{let _:Total=UnordItems::new(Input(0)).sum();},5=>{let _:Total=UnordItems::new(Input(0)).product();},
 6=>{let value=Value(0);drop(UnordItems::new(RefInput(&value)).cloned())},7=>{let value=Value(0);drop(UnordItems::new(RefInput(&value)).copied())},_=>unreachable!()}}
fn keys(family:usize,cache:bool){let mut hcx=DummyStableHashCtxt;match family{
 0=>{let v:Vec<Item>=UnordItems::new([Item(2),Item(1)].into_iter()).collect_sorted(&mut hcx,cache);show(v.into_iter().map(|v|v.0))},
 1=>{let v=UnordItems::new([Item(2),Item(1)].into_iter()).into_sorted(&mut hcx);show(v.into_iter().map(|v|v.0))},
 2|3|4=>{let mut map=UnordMap::default();map.insert(Item(2),102u64);map.insert(Item(1),101u64);match family{2=>{let v=map.to_sorted(&mut hcx,cache);show(v.into_iter().map(|(k,_)|k.0))},3=>{let v=map.into_sorted(&mut hcx,cache);show(v.into_iter().map(|(k,_)|k.0))},4=>show(map.values_sorted(&mut hcx,cache).copied()),_=>unreachable!()}},
 5|6=>{let mut set=UnordSet::new();set.insert(Item(2));set.insert(Item(1));if family==5{let v=set.to_sorted(&mut hcx,cache);show(v.into_iter().map(|v|v.0))}else{let v=set.into_sorted(&mut hcx,cache);show(v.into_iter().map(|v|v.0))}},_=>unreachable!()}}
fn main(){std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();let family=args[2].parse().unwrap();let result=catch_unwind(AssertUnwindSafe(||if args[1]=="primitive"{primitive(family)}else{keys(family,args[3]=="1")}));if let Err(payload)=result{event(format!("caught:{}",payload.downcast_ref::<&str>().copied().unwrap_or("opaque")))}}
