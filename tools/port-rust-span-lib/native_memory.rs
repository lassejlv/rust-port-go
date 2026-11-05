// Primitive memory-codec observer linked to the complete unchanged original.
extern crate rustc_serialize;
use rustc_serialize::{Encoder,Decoder};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::fmt::Write;
fn hex(bytes:&[u8])->String{let mut s=String::new();for b in bytes{write!(s,"{b:02x}").unwrap();}s}
fn main(){
 let mut values=vec![0u128,1,63,64,127,128,255,256,0xffff,0x1fffff,u32::MAX as u128,u64::MAX as u128,(u64::MAX as u128)+1,1u128<<127,u128::MAX];let mut random=0x3813c2a965eab38bu64;
 for _ in 0..256{random=random.wrapping_mul(6364136223846793005).wrapping_add(1);let lo=random;random=random.wrapping_mul(6364136223846793005).wrapping_add(1);values.push((random as u128)<<64|lo as u128);}
 for value in values{for ty in ["usize","u128","u64","u32","u16","u8","isize","i128","i64","i32","i16","i8"]{
  let mut e=MemEncoder::new();match ty{"usize"=>e.emit_usize(value as usize),"u128"=>e.emit_u128(value),"u64"=>e.emit_u64(value as u64),"u32"=>e.emit_u32(value as u32),"u16"=>e.emit_u16(value as u16),"u8"=>e.emit_u8(value as u8),"isize"=>e.emit_isize(value as isize),"i128"=>e.emit_i128(value as i128),"i64"=>e.emit_i64(value as i64),"i32"=>e.emit_i32(value as i32),"i16"=>e.emit_i16(value as i16),"i8"=>e.emit_i8(value as i8),_=>unreachable!()};let capacity=e.data.capacity();let mut bytes=e.finish();let encoded=hex(&bytes);bytes.extend_from_slice(MAGIC_END_BYTES);let mut d=MemDecoder::new(&bytes,0).unwrap();let decoded=match ty{"usize"=>d.read_usize()as u128,"u128"=>d.read_u128(),"u64"=>d.read_u64()as u128,"u32"=>d.read_u32()as u128,"u16"=>d.read_u16()as u128,"u8"=>d.read_u8()as u128,"isize"=>d.read_isize()as i128 as u128,"i128"=>d.read_i128()as u128,"i64"=>d.read_i64()as i128 as u128,"i32"=>d.read_i32()as i128 as u128,"i16"=>d.read_i16()as i128 as u128,"i8"=>d.read_i8()as i128 as u128,_=>unreachable!()};println!("{ty}\t{value:032x}\t{encoded}\t{capacity}\t{decoded:032x}\t{}",d.position());
 }}
 for text in ["","a","é\n\0😀", "\u{323af}"]{let mut e=MemEncoder::new();e.emit_str(text);let mut bytes=e.finish();let encoded=hex(&bytes);bytes.extend_from_slice(MAGIC_END_BYTES);let mut d=MemDecoder::new(&bytes,0).unwrap();let out=d.read_str();println!("str\t{}\t{encoded}\t0\t{}\t{}",hex(text.as_bytes()),hex(out.as_bytes()),d.position());}
}
