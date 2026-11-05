#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use fingerprint_reference::*;
use std::io::{self,Write};
fn hex(value:&str)->String{value.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn event(value:&str){eprintln!("EVENT:{value}");io::stderr().flush().unwrap();}
fn location(file:&str)->String{
 if file.ends_with("/wrapper/fingerprint.rs")||file=="wrapper/fingerprint.rs"{return "original/compiler/rustc_data_structures/src/fingerprint.rs".into()}
 file.strip_prefix(concat!(env!("FERRUGO_FP_METADATA_ROOT"),"/")).unwrap_or(file).to_owned()
}
struct AuditDecoder{inner:MemDecoder<'static>,bytes:Vec<u8>,length:usize}
macro_rules! reads{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self)->$ty{self.inner.$name()})*}}
impl Decoder for AuditDecoder{
 reads!(read_usize:usize,read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);
 fn read_raw_bytes(&mut self,n:usize)->&[u8]{event(&format!("read:{n}:{}:{}",self.length,std::thread::panicking()));&self.bytes[..self.length]}
 fn peek_byte(&self)->u8{self.inner.peek_byte()}
 fn position(&self)->usize{self.inner.position()}
}
impl Drop for AuditDecoder{fn drop(&mut self){event(&format!("decoder_drop:{}",std::thread::panicking()));}}
fn decode(length:usize,packed:bool){
 let mut decoder=AuditDecoder{inner:MemDecoder::new(MAGIC_END_BYTES,0).unwrap(),bytes:(0..32).collect(),length};
 let value=if packed{PackedFingerprint::decode(&mut decoder).into()}else{Fingerprint::decode(&mut decoder)};
 let(a,b)=value.split();event(&format!("value:{}:{}",a.as_u64(),b.as_u64()));
}
fn attempt(length:usize,packed:bool){let caught=std::panic::catch_unwind(||decode(length,packed)).is_err();event(&format!("caught:{caught}"));}
struct Ambient{length:usize,packed:bool}impl Drop for Ambient{fn drop(&mut self){event("ambient_drop");attempt(self.length,self.packed);}}
fn trigger(){panic!("ambient")}
fn main(){let args:Vec<_>=std::env::args().collect();let ambient=&args[1];let packed=args[2].parse().unwrap();let length=args[3].parse().unwrap();let processing=ambient=="processing";
 std::panic::set_hook(Box::new(move|info|{
  let(kind,payload)=if let Some(text)=info.payload().downcast_ref::<String>(){("owned_string",text.as_str())}else if let Some(text)=info.payload().downcast_ref::<&str>(){("static_str",*text)}else{("opaque","")};let l=info.location().unwrap();event(&format!("hook:{kind}:{}:{}:{}:{}:{}:{}:{}:{}",hex(payload),hex(&location(l.file())),l.line(),l.column(),info.can_unwind(),info.force_no_backtrace(),std::thread::panicking(),hex(l.file())));
  if processing&&payload=="ambient"{event("ambient_hook");attempt(length,packed);event("hook_return");}
 }));
 if ambient=="normal"{attempt(length,packed)}else if ambient=="inherited"{let caught=std::panic::catch_unwind(||{let _guard=Ambient{length,packed};trigger()}).is_err();event(&format!("ambient_caught:{caught}"));}else{let caught=std::panic::catch_unwind(trigger).is_err();event(&format!("ambient_caught:{caught}"));}
}
