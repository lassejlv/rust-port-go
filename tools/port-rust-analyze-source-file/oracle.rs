#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// The entire unchanged original file is compiled with actual rustc_span types.
use rustc_span::{RelativeBytePos,MultiByteChar,Pos};
use std::{io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe}};
mod original {
 include!(env!("FERRUGO_ANALYZE_SOURCE"));
 pub fn run(p:&[&str])->String {
  let bytes:Vec<u8>=(0..p[2].len()).step_by(2).map(|i|u8::from_str_radix(&p[2][i..i+2],16).unwrap()).collect();let src=String::from_utf8(bytes).unwrap();
  let mut lines:Vec<RelativeBytePos>=Vec::with_capacity(p[6].parse().unwrap());for i in 0..p[5].parse::<u32>().unwrap(){lines.push(RelativeBytePos(i))}
  let mut mb:Vec<MultiByteChar>=Vec::with_capacity(p[8].parse().unwrap());for i in 0..p[7].parse::<u32>().unwrap(){mb.push(MultiByteChar{pos:RelativeBytePos(i),bytes:2})}
  let result=catch_unwind(AssertUnwindSafe(||match p[1]{
   "full"=>{let (l,m)=analyze_source_file(&src);lines=l;mb=m;0},
   "generic"=>analyze_source_file_generic(&src,p[3].parse().unwrap(),RelativeBytePos(p[4].parse().unwrap()),&mut lines,&mut mb),
   "sse2"=>{assert!(is_x86_feature_detected!("sse2"));unsafe{analyze_source_file_sse2(&src,&mut lines,&mut mb)};0},
   _=>panic!("bad mode")
  }));
  format!("{}\t{}\t{}\t{}\t{}\t{}",if result.is_ok(){"ok"}else{"panic"},result.unwrap_or(0),lines.capacity(),mb.capacity(),lines.iter().map(|p|p.0.to_string()).collect::<Vec<_>>().join(","),mb.iter().map(|p|format!("{}/{}",p.pos.0,p.bytes)).collect::<Vec<_>>().join(","))
 }
}
fn main(){std::panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let line=line.unwrap();let parts:Vec<_>=line.split('\t').collect();println!("{}\t{}",parts[0],original::run(&parts));}}
