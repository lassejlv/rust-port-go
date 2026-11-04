// SPDX-License-Identifier: MIT OR Apache-2.0
// Original Rust standard char Debug; no hand-written escape classification.
use std::{fmt,io::{self,Write}};
const OFFSET:u64=14695981039346656037;
fn mix(hash:&mut u64,bytes:&[u8]){for b in bytes{*hash=(*hash^u64::from(*b)).wrapping_mul(1099511628211)}}
struct Sink{hash:u64}
impl Sink{fn string(&mut self,s:&str){mix(&mut self.hash,b"S");mix(&mut self.hash,&(s.len()as u32).to_le_bytes());mix(&mut self.hash,s.as_bytes())}}
impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.string(s);Ok(())}}
struct CharSink(Sink);
impl fmt::Write for CharSink{fn write_str(&mut self,s:&str)->fmt::Result{self.0.write_str(s)}fn write_char(&mut self,c:char)->fmt::Result{mix(&mut self.0.hash,b"C");mix(&mut self.0.hash,&(c as u32).to_le_bytes());Ok(())}}
fn main()->io::Result<()>{
 assert_eq!(std::char::UNICODE_VERSION,(17,0,0));
 let mut flags=vec![0u8;0x110000];let mut hashes=[vec![],vec![]];
 for mode in 0..2{let mut sink=Sink{hash:OFFSET};let mut character=CharSink(Sink{hash:OFFSET});let mut count=0;
  for value in 0..=0x10ffff{let Some(c)=char::from_u32(value)else{continue};
   if mode==0{flags[value as usize]=u8::from(format!("{c:?}").contains("\\u{"));mix(&mut sink.hash,b"V");mix(&mut sink.hash,&value.to_le_bytes());fmt::write(&mut sink,format_args!("{c:?}")).unwrap()}else{mix(&mut character.0.hash,b"V");mix(&mut character.0.hash,&value.to_le_bytes());fmt::write(&mut character,format_args!("{c:?}")).unwrap()}
   count+=1;if count%256==0{hashes[mode].push(if mode==0{sink.hash}else{character.0.hash});sink.hash=OFFSET;character.0.hash=OFFSET}
  }
  assert_eq!(count,1112064);assert_eq!(count%256,0);
 }
 let mut out=io::BufWriter::new(io::stdout().lock());out.write_all(b"RDC1")?;for v in [17u32,1112064,0x110000,hashes[0].len()as u32]{out.write_all(&v.to_le_bytes())?};out.write_all(&flags)?;for mode in hashes{for hash in mode{out.write_all(&hash.to_le_bytes())?}}Ok(())
}
