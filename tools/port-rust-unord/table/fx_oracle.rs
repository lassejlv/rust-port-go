// Direct calls into unchanged rustc-hash2.1.1, including std's typed defaults.
#![feature(hasher_prefixfree_extras)]
use rustc_hash::FxHasher;
use std::{hash::{Hash,Hasher},io::{self,Write}};
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());let mut lengths:Vec<usize>=(0..=64).collect();lengths.extend([127,128,129,255,256,257,4096,65536]);let texts=["","a","\0","é","中文","🦀","a\0é🦀","\n\r\t"];
for seed in [0u64,1,u64::MAX,0x123456789abcdef0]{for length in &lengths{let n=*length;let data:Vec<u8>=(0..n).map(|i|seed.wrapping_add((i as u64).wrapping_mul(131))as u8).collect();for mode in 0..19{let mut h=FxHasher::with_seed(seed as usize);match mode{
0=>h.write(&data),1=>data.hash(&mut h),2=>{h.write(&data[..n/2]);h.write(&data[n/2..]);},
3=>h.write_u8(seed as u8),4=>h.write_u16(seed as u16),5=>h.write_u32(seed as u32),6=>h.write_u64(seed),7=>h.write_u128((seed as u128)<<64|n as u128),8=>h.write_usize(seed as usize),
9=>(seed as i8).hash(&mut h),10=>(seed as i16).hash(&mut h),11=>(seed as i32).hash(&mut h),12=>(seed as i64).hash(&mut h),13=>(((seed as u128)<<64|n as u128)as i128).hash(&mut h),14=>(seed as isize).hash(&mut h),
15=>texts[n%texts.len()].hash(&mut h),16=>h.write_length_prefix(n),17=>{h.write(&[]);h.write_u64(seed);h.write(&data)},18=>{let s=texts[n%texts.len()];h.write_str(s);h.write_u128((n as u128)<<64|seed as u128)},_=>unreachable!()};let mut clone=FxHasher::default();clone.clone_from(&h.clone());clone.write_u64(n as u64);writeln!(out,"{seed}\t{n}\t{mode}\t{}\t{}",h.finish(),clone.finish())?;}}}Ok(())}
