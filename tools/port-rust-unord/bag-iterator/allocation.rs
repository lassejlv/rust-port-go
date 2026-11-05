// Actual source Vec extension allocation/identity, with no output under observe.
use rustc_data_structures::unord::UnordBag;
use std::{alloc::{GlobalAlloc,Layout,System},cell::Cell,sync::atomic::{AtomicBool,AtomicUsize,Ordering}};
static OBSERVE:AtomicBool=AtomicBool::new(false);
static MAX:AtomicUsize=AtomicUsize::new(0);
static ALLOCATIONS:AtomicUsize=AtomicUsize::new(0);
struct Allocator;
fn record(size:usize,align:usize){if OBSERVE.load(Ordering::Relaxed)&&align==8&&size>=64{MAX.fetch_max(size,Ordering::Relaxed);ALLOCATIONS.fetch_add(1,Ordering::Relaxed);}}
unsafe impl GlobalAlloc for Allocator{
 unsafe fn alloc(&self,l:Layout)->*mut u8{record(l.size(),l.align());unsafe{System.alloc(l)}}
 unsafe fn dealloc(&self,p:*mut u8,l:Layout){unsafe{System.dealloc(p,l)}}
 unsafe fn realloc(&self,p:*mut u8,l:Layout,n:usize)->*mut u8{record(n,l.align());unsafe{System.realloc(p,l,n)}}
}
#[global_allocator]static ALLOCATOR:Allocator=Allocator;
struct Value{number:u64,_identity:u64}impl Drop for Value{fn drop(&mut self){}}
struct Input<'a>{values:std::vec::IntoIter<Value>,next:&'a Cell<usize>,hint:&'a Cell<usize>}
impl Iterator for Input<'_>{type Item=Value;fn next(&mut self)->Option<Value>{self.next.set(self.next.get()+1);self.values.next()}fn size_hint(&self)->(usize,Option<usize>){self.hint.set(self.hint.get()+1);self.values.size_hint()}}
struct Producer<I>{input:Option<I>}impl<I>Drop for Producer<I>{fn drop(&mut self){}}
impl<I:Iterator>IntoIterator for Producer<I>{type Item=I::Item;type IntoIter=I;fn into_iter(mut self)->I{self.input.take().unwrap()}}
fn main(){let args:Vec<_>=std::env::args().collect();let family:usize=args[1].parse().unwrap();let length:usize=args[2].parse().unwrap();let initial=args[3]=="1";
 assert_eq!((std::mem::size_of::<Value>(),std::mem::align_of::<Value>()),(16,8));
 let values=(0..length).map(|n|Value{number:n as u64,_identity:n as u64}).collect::<Vec<_>>();let input_pointer=values.as_ptr()as usize;let input_capacity=values.capacity();
 let next=Cell::new(0);let hint=Cell::new(0);let mut bag=UnordBag::new();
 OBSERVE.store(true,Ordering::Relaxed);if initial{bag.push(Value{number:90,_identity:90})};OBSERVE.store(false,Ordering::Relaxed);let initial_capacity=MAX.swap(0,Ordering::Relaxed)/16;ALLOCATIONS.store(0,Ordering::Relaxed);
 OBSERVE.store(true,Ordering::Relaxed);match family{0=>bag.extend(Producer{input:Some(Input{values:values.into_iter(),next:&next,hint:&hint})}),1=>bag.extend(Producer{input:Some(values.into_iter())}),_=>unreachable!()};OBSERVE.store(false,Ordering::Relaxed);
 let pointer=Cell::new(0usize);bag.items().all(|v|{if pointer.get()==0{pointer.set(v as*const Value as usize)};let _=v.number;true});
 // The separate all call observes only the actual allocation address. No
 // Rust vector header or private Bag field is read or reinterpreted.
 let reused=length>0&&pointer.get()==input_pointer;let maximum=MAX.load(Ordering::Relaxed);let capacity=if reused{input_capacity}else if maximum>0{maximum/16}else{initial_capacity};assert!(capacity>=bag.len());
 let length_after=bag.len();
 println!("{{\"family\":{family},\"length\":{length},\"initial\":{initial},\"capacity\":{capacity},\"reused\":{reused},\"next\":{},\"hint\":{},\"allocations\":{},\"maximum_size\":{maximum},\"rust_size\":16,\"rust_align\":8,\"len\":{length_after}}}",next.get(),hint.get(),ALLOCATIONS.load(Ordering::Relaxed));
}
