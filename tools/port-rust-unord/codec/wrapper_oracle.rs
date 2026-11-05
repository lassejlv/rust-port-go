// The original generic slice/Vec/UnordBag Encodable and exact opaque concrete
// specializations are linked unchanged. This wrapper is a distinct source type.
#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::unord::UnordBag;
use rustc_serialize::{Encoder,Encodable,opaque::{FileEncoder,mem_encoder::MemEncoder}};
use std::{cell::{Cell,RefCell},io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind}};
#[derive(Clone,Copy)]struct Config{kind:usize,bag:bool,length:usize,fail:isize,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{kind:0,bag:false,length:0,fail:-1,resume:false})};static PATH:RefCell<String>=const{RefCell::new(String::new())};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}
fn hex(s:&[u8])->String{s.iter().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise()->!{if CONFIG.get().resume{panic::resume_unwind(Box::new("emit_u8"))}else{panic::panic_any("emit_u8")}}
struct Local(usize);impl Drop for Local{fn drop(&mut self){event(format!("local:{}:{}",self.0,std::thread::panicking()));}}
struct Wrapped<E>{inner:E,calls:usize}
macro_rules! delegate{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self,v:$ty){self.inner.$name(v)})*}}
impl<E:Encoder>Encoder for Wrapped<E>{
 delegate!(emit_usize:usize,emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);
 fn emit_raw_bytes(&mut self,v:&[u8]){self.inner.emit_raw_bytes(v)}
 fn emit_u8(&mut self,v:u8){let call=self.calls;self.calls+=1;event(format!("emit:{call}:{v}:{}",std::thread::panicking()));let _local=Local(call);if call as isize==CONFIG.get().fail{raise()};self.inner.emit_u8(v)}
}
fn encode<E:Encoder>(encoder:&mut E){let values:Vec<u8>=(0..CONFIG.get().length).map(|i|((i*73+42)%256)as u8).collect();if CONFIG.get().bag{let mut bag=UnordBag::new();for value in values{bag.push(value)}bag.encode(encoder)}else{values.encode(encoder)}}
fn caught(f:impl FnOnce())->String{match catch_unwind(AssertUnwindSafe(f)){Ok(())=>String::new(),Err(value)=>value.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()}}
fn body(){event(format!("body:{}",std::thread::panicking()));let c=CONFIG.get();match c.kind{
0=>{let mut e=MemEncoder::new();let message=caught(||encode(&mut e));let position=e.position();let data=e.finish();event(format!("state:mem:0:{position}:{}:{}",data.capacity(),hex(&data)));event(format!("caught:{}:{}",hex(message.as_bytes()),std::thread::panicking()));},
1=>{let mut e=Wrapped{inner:MemEncoder::new(),calls:0};let message=caught(||encode(&mut e));let position=e.inner.position();let calls=e.calls;let data=e.inner.finish();event(format!("state:mem:{calls}:{position}:{}:{}",data.capacity(),hex(&data)));event(format!("caught:{}:{}",hex(message.as_bytes()),std::thread::panicking()));},
2=>{let path=PATH.with(|p|p.borrow().clone());let mut e=FileEncoder::new(&path).unwrap();let message=caught(||encode(&mut e));let position=e.position();e.finish().unwrap();drop(e);let data=std::fs::read(path).unwrap();event(format!("state:file:0:{position}:{}",hex(&data)));event(format!("caught:{}:{}",hex(message.as_bytes()),std::thread::panicking()));},
3=>{let path=PATH.with(|p|p.borrow().clone());let mut e=Wrapped{inner:FileEncoder::new(&path).unwrap(),calls:0};let message=caught(||encode(&mut e));let position=e.inner.position();let calls=e.calls;e.inner.finish().unwrap();drop(e);let data=std::fs::read(path).unwrap();event(format!("state:file:{calls}:{position}:{}",hex(&data)));event(format!("caught:{}:{}",hex(message.as_bytes()),std::thread::panicking()));},_=>unreachable!()}}
struct Outer;impl Drop for Outer{fn drop(&mut self){body()}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let n:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{kind:n[0]as usize,bag:n[1]!=0,length:n[2]as usize,fail:n[3]as isize,resume:n[4]!=0});PATH.with(|p|*p.borrow_mut()=args[3].clone());let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let value=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(value.as_bytes()),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file.as_bytes())));if processing{body()}}));match mode.as_str(){"normal"=>body(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
