// Native formatting contract for the unchanged Cargo modules.
// SPDX-License-Identifier: MIT OR Apache-2.0
mod util;
use util::semver_ext::{OptVersionReq,VersionReqMatchMode};
use semver::{Version,VersionReq};
use std::fmt::{self,Write as _};
use std::io::{self,Write as _};

#[derive(Default)]
struct Sink{calls:Vec<String>,text:String,fail:usize}
impl fmt::Write for Sink{fn write_str(&mut self,text:&str)->fmt::Result{self.calls.push(text.into());if self.fail!=0&&self.calls.len()==self.fail{return Err(fmt::Error)};self.text.push_str(text);Ok(())}}
fn display(w:&mut Sink,value:&OptVersionReq,op:u8)->fmt::Result{match op{
 0=>write!(w,"{value}"),1=>write!(w,"{value:30}"),2=>write!(w,"{value:<30}"),3=>write!(w,"{value:>30}"),4=>write!(w,"{value:.2}"),5=>write!(w,"{value:*^30}"),6=>write!(w,"{value:#30}"),7=>write!(w,"{value:+30}"),8=>write!(w,"{value:030}"),_=>panic!("bad display operation")}}
fn debug<T:fmt::Debug>(value:&T,op:u8)->String{match op{9=>format!("{value:?}"),10=>format!("{value:x?}"),11=>format!("{value:X?}"),12=>format!("{value:#?}"),13=>format!("{value:#x?}"),14=>format!("{value:#X?}"),_=>panic!("bad debug operation")}}
fn number(out:&mut Vec<u8>,value:usize){out.extend((value as u32).to_le_bytes())}
fn text(out:&mut Vec<u8>,value:&str){number(out,value.len());out.extend(value.as_bytes())}
fn record(out:&mut Vec<u8>,constructor:u8,req:&str,version:&str,op:u8,fail:usize,sink:Sink,error:bool){out.push(constructor);text(out,req);text(out,version);out.push(op);number(out,fail);text(out,&sink.text);out.push(error as u8);number(out,sink.calls.len());for call in sink.calls{text(out,&call)}}
fn main(){
 let mut records=Vec::new();let mut count=0;
 for constructor in 0..7{for req_text in ["*","^10.26.31, <32","~0.2.255-alpha","=18446744073709551615.0.0"]{for version_text in ["10.26.31-rc.0+build.007","0.0.0"]{
  let req=VersionReq::parse(req_text).unwrap();let version=Version::parse(version_text).unwrap();let value=match constructor{0=>OptVersionReq::Any,1=>OptVersionReq::Req(req.clone()),2=>OptVersionReq::Locked(version.clone(),req.clone()),3=>OptVersionReq::Precise(version.clone(),req.clone()),4=>OptVersionReq::exact(&version),5=>OptVersionReq::lock_to_exact(&version),6=>OptVersionReq::from(req),_=>unreachable!()};
  for op in 0..9{let mut full=Sink::default();display(&mut full,&value,op).unwrap();let calls=full.calls.len();for fail in 0..=calls+1{let mut sink=Sink{fail,..Sink::default()};let error=display(&mut sink,&value,op).is_err();record(&mut records,constructor,req_text,version_text,op,fail,sink,error);count+=1}}
  for op in 9..15{let sink=Sink{text:debug(&value,op),..Sink::default()};record(&mut records,constructor,req_text,version_text,op,0,sink,false);count+=1}
 }}}
 for (constructor,mode) in [(7,VersionReqMatchMode::Default),(8,VersionReqMatchMode::Prerelease)]{for op in 9..15{let sink=Sink{text:debug(&mode,op),..Sink::default()};record(&mut records,constructor,"","",op,0,sink,false);count+=1}}
 let mut output=io::BufWriter::new(io::stdout().lock());output.write_all(b"CGSF").unwrap();output.write_all(&(count as u32).to_le_bytes()).unwrap();output.write_all(&records).unwrap();
}
