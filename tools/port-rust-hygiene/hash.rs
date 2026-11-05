#![feature(rustc_private)]
use rustc_span::hygiene::*;
use rustc_span::def_id::CrateNum;
use rustc_data_structures::{fingerprint::Fingerprint,unhash::Unhasher};
use rustc_hashes::Hash64;
use rustc_serialize::{Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{hash::{Hash,Hasher},panic::{catch_unwind,AssertUnwindSafe}};
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn hash(a:u64,b:u64)->ExpnHash{let mut e=MemEncoder::new();Fingerprint::new(Hash64::new(a),Hash64::new(b)).encode(&mut e);let mut data=e.finish();data.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&data,0).unwrap())}
struct Raw{events:Vec<String>,value:u64,fail:usize}
impl Raw{fn record(&mut self,event:String){let n=self.events.len();self.events.push(event.clone());if n==self.fail{panic!("hasher")};for b in event.bytes(){self.value=self.value.wrapping_mul(17).wrapping_add(b as u64)}}}
impl Hasher for Raw{fn write(&mut self,b:&[u8]){self.record(format!("bytes:{}",hex(b)))}fn finish(&self)->u64{self.value}}
struct Typed(Raw);
impl Hasher for Typed{fn write(&mut self,b:&[u8]){self.0.write(b)}fn finish(&self)->u64{self.0.finish()}fn write_u8(&mut self,n:u8){self.0.record(format!("u8:{n}"))}fn write_u32(&mut self,n:u32){self.0.record(format!("u32:{n}"))}fn write_u64(&mut self,n:u64){self.0.record(format!("u64:{n}"))}fn write_isize(&mut self,n:isize){self.0.record(format!("isize:{n}"))}}
struct Wrapped(Unhasher);
impl Hasher for Wrapped{fn write(&mut self,b:&[u8]){self.0.write(b)}fn finish(&self)->u64{self.0.finish()}fn write_u64(&mut self,n:u64){self.0.write_u64(n)}}
fn values<T:Hash+Copy,H:Hasher>(value:T,count:usize,h:&mut H){T::hash_slice(&[value,value][..count],h)}
fn run<T:Hash+Copy>(n:usize,value:T){
 for count in 0..3{for seed in [0,7]{for kind in 0..5{for fail in std::iter::once(usize::MAX).chain(if kind<2{0..8}else{0..0}){
  let(mode,output)=(kind+count*5+usize::from(seed!=0)*15+if fail==usize::MAX{0}else{(fail+1)*30},match kind{
   0=>{let mut h=Raw{events:vec![],value:seed,fail};let bad=catch_unwind(AssertUnwindSafe(||values(value,count,&mut h))).is_err();format!("trace {bad} {} {}\n",h.finish(),h.events.join(","))},
   1=>{let mut h=Typed(Raw{events:vec![],value:seed,fail});let bad=catch_unwind(AssertUnwindSafe(||values(value,count,&mut h))).is_err();format!("trace {bad} {} {}\n",h.finish(),h.0.events.join(","))},
   2=>{let mut h=Unhasher::default();h.write_u64(seed);let bad=catch_unwind(AssertUnwindSafe(||values(value,count,&mut h))).is_err();format!("unhash {bad} {}\n",h.finish())},
   3=>{let mut h=Unhasher::default();h.write_u64(seed);let bad=catch_unwind(AssertUnwindSafe(||{let mut reference=&mut h;values(value,count,&mut reference)})).is_err();format!("unhash {bad} {}\n",h.finish())},
   _=>{let mut h=Wrapped(Unhasher::default());h.0.write_u64(seed);let bad=catch_unwind(AssertUnwindSafe(||values(value,count,&mut h))).is_err();format!("unhash {bad} {}\n",h.finish())}
  });println!("{n}\t{mode}\t{}",hex(output));
 }}}}
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();if args[1]=="--count"{println!("30");return}let n:usize=args[1].parse().unwrap();let variant=n/10;let raw=[0,17,0x1fff_ffff][variant];let wide=[0,17,0xffff_ff00][variant];let id=ExpnId{krate:CrateNum::from_u32([0,3,0xffff_ff00][variant]),local_id:ExpnIndex::from_u32(wide)};match n%10{
 0=>run(n,SyntaxContext::from_u32(raw)),1=>run(n,ExpnIndex::from_u32(wide)),2=>run(n,LocalExpnId::from_u32(wide)),3=>run(n,DecodedSyntaxContext::from_u32(raw)),4=>run(n,id),5=>{let(a,b)=[(0,0),(111,777),(u64::MAX,1<<63)][variant];run(n,hash(a,b))},6=>run(n,[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][variant]),7=>run(n,[MacroKind::Bang,MacroKind::Attr,MacroKind::Derive][variant]),8=>run(n,(SyntaxContext::from_u32(raw),id,[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][variant])),9=>run(n,(id,[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][variant])),_=>unreachable!()}}
