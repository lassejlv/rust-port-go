// SPDX-License-Identifier: MIT OR Apache-2.0
#![feature(rustc_private)]
extern crate rustc_serialize;
extern crate rustc_span;
use rustc_serialize::{Encoder,Encodable,opaque::mem_encoder::MemEncoder};
use rustc_span::{SourceFileHash,SourceFileHashAlgorithm,SpanEncoder,Span,Symbol,ByteSymbol,ExpnId,SyntaxContext};
use rustc_span::def_id::{CrateNum,DefIndex,DefId};
struct Wrapped{inner:MemEncoder,bytes:usize,raw:usize}
macro_rules! delegate{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self,value:$ty){self.inner.$name(value)})*}}
impl Encoder for Wrapped{
 delegate!(emit_usize:usize,emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);
 fn emit_u8(&mut self,value:u8){self.bytes+=1;self.inner.emit_u8(value)}
 fn emit_raw_bytes(&mut self,value:&[u8]){self.raw+=1;self.inner.emit_raw_bytes(value)}
}
impl SpanEncoder for Wrapped{
 fn encode_span(&mut self,value:Span){self.inner.encode_span(value)}
 fn encode_symbol(&mut self,value:Symbol){self.inner.encode_symbol(value)}
 fn encode_byte_symbol(&mut self,value:ByteSymbol){self.inner.encode_byte_symbol(value)}
 fn encode_expn_id(&mut self,value:ExpnId){self.inner.encode_expn_id(value)}
 fn encode_syntax_context(&mut self,value:SyntaxContext){self.inner.encode_syntax_context(value)}
 fn encode_crate_num(&mut self,value:CrateNum){self.inner.encode_crate_num(value)}
 fn encode_def_index(&mut self,value:DefIndex){self.inner.encode_def_index(value)}
 fn encode_def_id(&mut self,value:DefId){self.inner.encode_def_id(value)}
}
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
fn span_projection<E:SpanEncoder>(bytes:&[u8],encoder:&mut E){bytes.encode(encoder)}
type Alias=MemEncoder;
fn main(){
 for length in[0,1,7,8,9,10,15,16,17,30,31,32,33,63,64,65,255]{
  let data:Vec<u8>=(0..length).map(|i|(i*19+42)as u8).collect();
  let mut direct=MemEncoder::new();data.as_slice().encode(&mut direct);let bytes=direct.finish();println!("slice\tdirect\t{length}\t{}\t{}",bytes.capacity(),hex(&bytes));
  let mut alias=Alias::new();data.as_slice().encode(&mut alias);let bytes=alias.finish();println!("slice\talias\t{length}\t{}\t{}",bytes.capacity(),hex(&bytes));
  let mut projected=MemEncoder::new();span_projection(&data,&mut projected);let bytes=projected.finish();println!("slice\tspan\t{length}\t{}\t{}",bytes.capacity(),hex(&bytes));
  let mut wrapped=Wrapped{inner:MemEncoder::new(),bytes:0,raw:0};data.as_slice().encode(&mut wrapped);let count=wrapped.bytes;let raw=wrapped.raw;let bytes=wrapped.inner.finish();println!("wrapped\t{length}\t{count}\t{raw}\t{}\t{}",bytes.capacity(),hex(&bytes));
 }
 let hash=SourceFileHash::new_in_memory(SourceFileHashAlgorithm::Md5,b"source");
 let mut direct=MemEncoder::new();hash.encode(&mut direct);let bytes=direct.finish();println!("hash\tdirect\t{}\t{}",bytes.capacity(),hex(&bytes));
 let mut wrapped=Wrapped{inner:MemEncoder::new(),bytes:0,raw:0};hash.encode(&mut wrapped);let count=wrapped.bytes;let raw=wrapped.raw;let bytes=wrapped.inner.finish();println!("hash\twrapped\t{count}\t{raw}\t{}\t{}",bytes.capacity(),hex(&bytes));
}
