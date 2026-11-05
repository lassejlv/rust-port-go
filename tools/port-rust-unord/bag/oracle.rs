#![allow(unused_imports)]
// Development-only driver linked to the whole unchanged rustc_data_structures
// reference crate. Expected results are produced only by original Rust code.
use rustc_data_structures::stable_hash::{DummyStableHashCtxt,StableHash,StableHashCtxt,StableHasher};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::unord::{UnordBag,UnordItems};
use std::{cell::RefCell,fmt::{self,Write},hash::Hasher,io::{self,Write as IoWrite},panic::{catch_unwind,AssertUnwindSafe}};
fn random(s:&mut u64)->u64{*s^=*s<<13;*s^=*s>>7;*s^=*s<<17;*s}
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
thread_local!{static HASH:RefCell<(Vec<u64>,isize)>=const{RefCell::new((Vec::new(),-1))};}
#[derive(Clone,Eq,PartialEq,Hash)]struct Tracked(u64);
impl StableHash for Tracked{fn stable_hash<H:StableHashCtxt>(&self,_:&mut H,h:&mut StableHasher){let fail=HASH.with(|s|{let mut s=s.borrow_mut();let index=s.0.len()as isize;s.0.push(self.0);index==s.1});h.write_u64(self.0^0xc0ffee1234567890);if fail{std::panic::panic_any("tracked_hash")};h.write_u8(self.0 as u8);}}
struct Sink{calls:Vec<String>,fail:isize}
impl Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{let index=self.calls.len()as isize;self.calls.push(format!("S{}",hex(s)));if index==self.fail{Err(fmt::Error)}else{Ok(())}}fn write_char(&mut self,c:char)->fmt::Result{let index=self.calls.len()as isize;self.calls.push(format!("C{}",hex(&c.to_string())));if index==self.fail{Err(fmt::Error)}else{Ok(())}}}
struct Observe<'a,T:?Sized>{value:&'a T,after:&'a RefCell<String>}
impl<T:fmt::Debug+?Sized>fmt::Debug for Observe<'_,T>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let r=fmt::Debug::fmt(self.value,f);*self.after.borrow_mut()=format!("{:x},{:?}",f.fill()as u32,f.align());r}}
fn debug<T:fmt::Debug>(value:&T,pattern:usize,fail:isize)->(bool,String,Vec<String>){let after=RefCell::new(String::new());let value=Observe{value,after:&after};let mut sink=Sink{calls:vec![],fail};let result=match pattern{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:08?}"),3=>write!(sink,"{value:+010x?}"),4=>write!(sink,"{value:💫>12.3?}"),5=>write!(sink,"{value:#010X?}"),_=>unreachable!()};(result.is_err(),after.into_inner(),sink.calls)}
thread_local!{static EVENTS:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};}
fn event(s:String){EVENTS.with(|e|e.borrow_mut().push(s));}
struct Probe{value:u64,id:u64}
impl Default for Probe{fn default()->Self{event("default".into());Self{value:0,id:777}}}
impl Clone for Probe{fn clone(&self)->Self{event(format!("clone:{}",self.id));Self{value:self.value,id:self.id+100}}}
impl Drop for Probe{fn drop(&mut self){event(format!("drop:{}:{}",self.id,std::thread::panicking()));}}
impl PartialEq for Probe{fn eq(&self,b:&Self)->bool{event(format!("eq:{}:{}",self.id,b.id));self.value==b.value}fn ne(&self,b:&Self)->bool{event(format!("ne:{}:{}",self.id,b.id));self.value!=b.value}}impl Eq for Probe{}
fn probes(length:usize,offset:u64)->UnordBag<Probe>{let mut b=UnordBag::new();for i in 0..length{b.push(Probe{value:i as u64*17+3,id:i as u64+offset})}b}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));let mut out=io::BufWriter::new(io::stdout().lock());
for kind in 0..9{for length in [0,1,3,9]{EVENTS.with(|e|e.borrow_mut().clear());let result=match kind{
0=>{let b=probes(length,0);let clone=b.clone();let result=format!("{}:{}",b==clone,b!=clone);drop(clone);drop(b);result},
1=>{let source=probes(length,0);let mut destination=probes(1,999);destination.clone_from(&source);let result=format!("{}:{}",destination==source,destination!=source);drop(destination);drop(source);result},
2=>{let owned=probes(length,0).into_items();let item=owned.get_only();let result=item.as_ref().map(|v|v.id.to_string()).unwrap_or_else(||"none".into());drop(item);result},
3=>{let b=probes(length,0);let count=b.items().count();drop(b);count.to_string()},
4=>{let a=probes(length,0);let b=probes(length+1,100);let result=format!("{}:{}",a==b,a!=b);drop(a);drop(b);result},
5=>{let mut values=Vec::with_capacity(length+12);for i in 0..length{values.push(Probe{value:i as u64*17+3,id:i as u64})}let b=UnordItems::new(values.into_iter()).collect::<UnordBag<_>>();let result=b.len().to_string();drop(b);result},
6=>{let b:UnordBag<Probe>=Default::default();let result=b.len().to_string();drop(b);result},7=>{let b=probes(length,0);let result=b.items().get_only().map(|v|v.id.to_string()).unwrap_or_else(||"none".into());drop(b);result},8=>{probes(length,0).into_items().count().to_string()},_=>unreachable!()};let events=EVENTS.with(|e|e.borrow().join("\n"));writeln!(out,"C\t{kind}\t{length}\t{result}\t{}",hex(&events))?;}}
for length in [0,1,2,3,10,31,64]{for order in 0..3{let mut values:Vec<u64>=(0..length).map(|v|v*17+3).collect();if order==1{values.reverse()}else if order==2{values.rotate_left(length as usize/2)}let bag=UnordItems::new(values.iter().map(|v|Tracked(*v))).collect::<UnordBag<_>>();for fail in -1..=length as isize{HASH.with(|s|*s.borrow_mut()=(vec![],fail));let mut hasher=StableHasher::new();hasher.write_u64(0xfeed0123456789ab);let panic=catch_unwind(AssertUnwindSafe(||bag.stable_hash(&mut DummyStableHashCtxt,&mut hasher))).is_err();let fingerprint:Fingerprint=hasher.finish();let(a,b)=fingerprint.split();let events=HASH.with(|s|s.borrow().0.iter().map(u64::to_string).collect::<Vec<_>>().join(","));writeln!(out,"H\t{length}\t{order}\t{fail}\t{panic}\t{}\t{}\t{events}",a.as_u64(),b.as_u64())?;}}}
for length in [0,1,3,9]{let bag=UnordItems::new((0..length).map(|v|v*17+3u64)).collect::<UnordBag<_>>();for pattern in 0..6{let count=debug(&bag,pattern,-1).2.len();for fail in -1..=count as isize{let(error,after,calls)=debug(&bag,pattern,fail);writeln!(out,"D\t{length}\t{pattern}\t{fail}\t{error}\t{after}\t{}",calls.join(","))?;}}}Ok(())}
