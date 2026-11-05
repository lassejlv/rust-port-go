// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual matching std Windows io::Error; both reference and Go run under Wine.
use std::{fmt,fmt::Write,io::{self,BufRead}};
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{out:String,events:Vec<String>,n:usize,fail:usize}
impl Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.n+=1;self.events.push(format!("S:{}",hex(s)));if self.n==self.fail{return Err(fmt::Error)}self.out.push_str(s);Ok(())}fn write_char(&mut self,c:char)->fmt::Result{self.n+=1;self.events.push(format!("C:{:x}",c as u32));if self.n==self.fail{return Err(fmt::Error)}self.out.push(c);Ok(())}}
fn debug(sink:&mut Sink,style:u32,error:&io::Error)->fmt::Result{match style{0=>write!(sink,"{error:?}"),1=>write!(sink,"{error:#?}"),2=>write!(sink,"{error:*^12.3?}"),3=>write!(sink,"{error:+012?}"),4=>write!(sink,"{error:*<12?}"),5=>write!(sink,"{error:λ^12.3?}"),6=>write!(sink,"{error:x?}"),7=>write!(sink,"{error:#X?}"),8=>write!(sink,"{error:\0>12?}"),_=>unreachable!()}}
fn main(){for line in io::stdin().lock().lines(){let line=line.unwrap();let p:Vec<_>=line.split('\t').collect();let code:i32=p[1].parse().unwrap();let error=io::Error::from_raw_os_error(code);let mut sink=Sink{out:String::new(),events:vec![],n:0,fail:p[4].parse().unwrap()};let ok=if p[2]=="display"{write!(&mut sink,"{error}").is_ok()}else{debug(&mut sink,p[3].parse().unwrap(),&error).is_ok()};println!("{}\t{}\t{}\t{}\t{}",p[0],hex(format!("{:?}",error.kind())),ok,hex(sink.out),hex(sink.events.join("|")));}}
