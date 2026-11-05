// SPDX-License-Identifier: MIT OR Apache-2.0
#![feature(rustc_private,panic_can_unwind,panic_internals)]
#![allow(internal_features)]
extern crate rustc_span;
extern crate rustc_serialize;
extern crate rustc_index;
use rustc_span::*;
use rustc_span::def_id::{CrateNum,DefId,DefIndex,LocalDefId};
use rustc_index::Idx;
use rustc_serialize::{Decoder,Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::sync::OnceLock;
static OP:OnceLock<String>=OnceLock::new();
static AMBIENT:OnceLock<String>=OnceLock::new();
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
fn outer_panic()->!{panic!("outer")}
fn read(name:&str){println!("read:{name}");}
struct Bad<'a>{inner:MemDecoder<'a>,lengths:usize,target:usize}
impl Drop for Bad<'_>{fn drop(&mut self){println!("decoder_drop:{}:{}:{}",self.inner.position(),self.lengths,std::thread::panicking());}}
macro_rules! reads{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self)->$ty{read(stringify!($name));self.inner.$name()})*}}
impl Decoder for Bad<'_>{
 reads!(read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);
 fn read_usize(&mut self)->usize{read("read_usize");self.lengths+=1;let value=self.inner.read_usize();if self.lengths==self.target{usize::MAX}else{value}}
 fn read_raw_bytes(&mut self,count:usize)->&[u8]{read("read_raw_bytes");self.inner.read_raw_bytes(count)}
 fn peek_byte(&self)->u8{self.inner.peek_byte()}
 fn position(&self)->usize{self.inner.position()}
}
impl BlobDecoder for Bad<'_>{
 fn decode_symbol(&mut self)->Symbol{self.inner.decode_symbol()}
 fn decode_byte_symbol(&mut self)->ByteSymbol{self.inner.decode_byte_symbol()}
 fn decode_def_index(&mut self)->DefIndex{self.inner.decode_def_index()}
 fn decode_local_def_id(&mut self)->LocalDefId{self.inner.decode_local_def_id()}
}
impl SpanDecoder for Bad<'_>{
 fn decode_span(&mut self)->Span{self.inner.decode_span()}
 fn decode_expn_id(&mut self)->ExpnId{self.inner.decode_expn_id()}
 fn decode_syntax_context(&mut self)->SyntaxContext{self.inner.decode_syntax_context()}
 fn decode_def_id(&mut self)->DefId{self.inner.decode_def_id()}
 fn decode_crate_num(&mut self)->CrateNum{self.inner.decode_crate_num()}
 fn decode_attr_id(&mut self)->AttrId{self.inner.decode_attr_id()}
}
fn operation(){
 let parts:Vec<_>=OP.get().unwrap().split('_').collect();
 match parts[0]{
 "vec"=>{
  assert_eq!((std::mem::size_of::<MultiByteChar>(),std::mem::size_of::<NormalizedPos>()),(8,8));
  let source=SourceFile::new(FileName::Custom(String::new()),String::new(),SourceFileHashAlgorithm::Md5,None).unwrap();
  let mut encoder=MemEncoder::new();source.encode(&mut encoder);let mut bytes=encoder.finish();bytes.extend_from_slice(MAGIC_END_BYTES);
  let mut decoder=Bad{inner:MemDecoder::new(&bytes,0).unwrap(),lengths:0,target:parts[1].parse().unwrap()};
  let _=SourceFile::decode(&mut decoder);println!("decoded");
 },
 "idx"=>{
  let mut value=AttrId::from_u32(parts[1].parse().unwrap());let amount=parts[2].parse().unwrap();
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||value.increment_by(amount)));
  println!("idx_after:{}",value.as_u32());
  if let Err(payload)=result{std::panic::resume_unwind(payload);}
 },
 "clamp"=>{let _=SourceFileHashAlgorithm::Sha1.clamp(SourceFileHashAlgorithm::Sha256,SourceFileHashAlgorithm::Md5);},
 _=>panic!("invalid operation")
 }
}
fn caught(){let result=std::panic::catch_unwind(operation);println!("caught:{}:{}",result.is_err(),std::thread::panicking());}
struct Outer;
impl Drop for Outer{fn drop(&mut self){caught();}}
fn main(){
 OP.set(std::env::args().nth(1).unwrap()).unwrap();AMBIENT.set(std::env::args().nth(2).unwrap()).unwrap();
 std::panic::set_hook(Box::new(|p|{
  let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};let l=p.location().unwrap();
  println!("hook:{kind}:{}:{}:{}:{}:{}:{}:{}",hex(p.payload_as_str().unwrap_or("opaque").as_bytes()),hex(l.file().as_bytes()),l.line(),l.column(),p.can_unwind(),p.force_no_backtrace(),std::thread::panicking());
  if p.payload_as_str()==Some("outer")&&AMBIENT.get().unwrap()=="processing"{operation();}
 }));
 rustc_data_structures::sync::set_dyn_thread_safe_mode(true);
 create_default_session_globals_then(||match AMBIENT.get().unwrap().as_str(){
  "normal"=>caught(),
  "unwinding"=>{let result=std::panic::catch_unwind(||{let _outer=Outer;outer_panic();});println!("outer_caught:{}",result.is_err());},
  "processing"=>{let result=std::panic::catch_unwind(outer_panic);println!("outer_caught:{}",result.is_err());},
  _=>panic!("bad ambient")
 });
}
