#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
use std::fmt::{self,Write};
use rustc_span::fatal_error::FatalError;
#[derive(Default)]struct Sink{calls:usize,fail:usize,events:Vec<String>,out:String}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.calls+=1;self.events.push(format!("S:{}",hex(s)));if self.calls==self.fail{return Err(fmt::Error)};self.out.push_str(s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{self.calls+=1;self.events.push(format!("C:{:x}",c as u32));if self.calls==self.fail{return Err(fmt::Error)};self.out.push(c);Ok(())}
}
struct Probe(bool);
impl fmt::Debug for Probe{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let first=if self.0{fmt::Display::fmt(&FatalError,f)}else{fmt::Debug::fmt(&FatalError,f)};let second=fmt::Debug::fmt(&7u16,f);if first.is_ok()&&second.is_ok(){Ok(())}else{Err(fmt::Error)}}}
fn format(sink:&mut Sink,style:usize,value:&Probe)->fmt::Result{match style{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:12?}"),3=>write!(sink,"{value:+012?}"),4=>write!(sink,"{value:*<12?}"),5=>write!(sink,"{value:λ^12.3?}"),6=>write!(sink,"{value:x?}"),7=>write!(sink,"{value:#X?}"),8=>write!(sink,"{value:\0>12?}"),_=>unreachable!()}}
fn main(){for display in [false,true]{for style in 0..9{for fail in 0..8{let mut sink=Sink{fail,..Default::default()};let result=format(&mut sink,style,&Probe(display));println!("{}\t{}\t{}\t{}\t{}\t{}",display as u8,style,fail,result.is_ok(),hex(sink.out),hex(sink.events.join("\n")));}}}}
