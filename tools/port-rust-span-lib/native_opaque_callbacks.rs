#![feature(fn_traits,unboxed_closures)]
extern crate rustc_serialize;
use rustc_serialize::{Encoder,Decoder};
use rustc_serialize::opaque::{mem_encoder::MemEncoder,MemDecoder,MAGIC_END_BYTES,FileEncoder};
use std::{cell::{Cell,RefCell},rc::Rc};
type Log=Rc<RefCell<Vec<String>>>;
fn event(log:&Log,text:String){if std::env::var_os("FERRUGO_TRACE_EVENTS").is_some(){eprintln!("EVENT:{text}")};log.borrow_mut().push(text)}
struct Token(Log);impl Drop for Token{fn drop(&mut self){event(&self.0,format!("token:{}",std::thread::panicking()))}}
struct Visitor {log:Log,calls:Cell<u32>,mode:u8,_token:Token}
impl Visitor{fn invoked(&self,name:&str){let calls=self.calls.get();self.calls.set(calls+1);event(&self.log,format!("{name}:{calls}:{}",std::thread::panicking()))}}
impl<'a>FnOnce<(&'a mut[u8;4],)>for Visitor{type Output=usize;extern "rust-call" fn call_once(self,(window,):(&'a mut[u8;4],))->usize{self.invoked("once");event(&self.log,format!("window:{}",window.iter().map(|v|format!("{v:02x}")).collect::<String>()));window.fill(0x7b);if self.mode==1||self.mode==3{panic!("call")};if self.mode==4{5}else{3}}}
impl Drop for Visitor{fn drop(&mut self){event(&self.log,format!("drop:{}:{}",self.calls.get(),std::thread::panicking()));if self.mode==2||self.mode==3{panic!("drop")}}}
struct Borrower(Visitor);
impl<'a,'b>FnOnce<(&'b mut MemDecoder<'a>,)>for Borrower{type Output=u8;extern "rust-call" fn call_once(self,_:(&'b mut MemDecoder<'a>,))->u8{panic!("unexpected once")}}
impl<'a,'b>FnMut<(&'b mut MemDecoder<'a>,)>for Borrower{extern "rust-call" fn call_mut(&mut self,_:(&'b mut MemDecoder<'a>,))->u8{panic!("unexpected mut")}}
impl<'a,'b>Fn<(&'b mut MemDecoder<'a>,)>for Borrower{extern "rust-call" fn call(&self,(d,):(&'b mut MemDecoder<'a>,))->u8{self.0.invoked("fn");event(&self.0.log,format!("position:{}",d.position()));let out=d.read_u8();if self.0.mode==1||self.0.mode==3{panic!("call")};out}}
fn execute(kind:&str,mode:u8,pos:usize)->String{let log:Log=Default::default();let visitor=Visitor{log:log.clone(),calls:Cell::new(0),mode,_token:Token(log.clone())};if kind=="mem"{
 let mut encoder=MemEncoder::new();encoder.emit_u8(17);let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||encoder.write_with::<4>(visitor)));event(&log,format!("panic:{}",result.is_err()));event(&log,format!("position:{}",encoder.position()));event(&log,format!("bytes:{}",encoder.finish().iter().map(|v|format!("{v:02x}")).collect::<String>()));
 }else if kind=="file"{let path=std::path::Path::new(&std::env::var("FERRUGO_OPAQUE_DIRECTORY").unwrap()).join("file");let mut encoder=FileEncoder::new(&path).unwrap();encoder.emit_u8(17);let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||encoder.write_with::<4>(visitor)));event(&log,format!("panic:{}",result.is_err()));event(&log,format!("position:{}",encoder.position()));encoder.finish().unwrap();drop(encoder);let bytes=std::fs::read(&path).unwrap();event(&log,format!("bytes:{}",bytes.iter().map(|v|format!("{v:02x}")).collect::<String>()));
 }else{let mut bytes=vec![17,34,51,68];bytes.extend_from_slice(MAGIC_END_BYTES);let mut decoder=MemDecoder::new(&bytes,1).unwrap();let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||decoder.with_position(pos,Borrower(visitor))));match result{Ok(v)=>event(&log,format!("result:{v}")),Err(_)=>event(&log,"panic".into())};event(&log,format!("position:{}",decoder.position()));}
 let out=log.borrow().join(",");out}
fn main(){std::panic::set_hook(Box::new(|_|{}));if let Some(kind)=std::env::args().nth(1){let pos=std::env::args().nth(2).unwrap().parse().unwrap();let mode=std::env::args().nth(3).map(|v|v.parse().unwrap()).unwrap_or(3);let out=execute(&kind,mode,pos);println!("{out}");return};for kind in["mem","file","position"]{for mode in[0,1,2,4]{for pos in[0,1,3,4,usize::MAX]{if kind=="position"&&mode==2&&pos>=4{continue};println!("{kind}\t{mode}\t{pos}\t{}",execute(kind,mode,pos));}}}}
