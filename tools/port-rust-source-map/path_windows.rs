#![feature(hasher_prefixfree_extras)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Real pinned std Path/OsStr public contracts, under explicit Windows64 target; emulated Wine execution is identified separately.
use std::{fmt,fmt::Write,hash::{Hash,Hasher},io::{self,BufRead},path::{Path,Component},ffi::OsString};
use std::os::windows::ffi::OsStringExt;
fn unhex(s:&str)->Vec<u8>{(0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect()}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn component(c:Component<'_>)->String{let kind=match c{Component::Prefix(_)=>0,Component::RootDir=>1,Component::CurDir=>2,Component::ParentDir=>3,Component::Normal(_)=>4};format!("{kind}:{}",hex(c.as_os_str().as_encoded_bytes()))}
fn alias(p:*const u8,n:usize,base:&[u8])->String{if n==0{return "empty".into()}let x=p as usize;let lo=base.as_ptr() as usize;if x>=lo&&x.saturating_add(n)<=lo+base.len(){format!("{}",x-lo)}else{"other".into()}}
struct H<'a>{events:Vec<String>,base:&'a[u8]}
impl Hasher for H<'_>{fn finish(&self)->u64{0}fn write(&mut self,b:&[u8]){self.events.push(format!("bytes:{}:{}",hex(b),alias(b.as_ptr(),b.len(),self.base)))}fn write_usize(&mut self,x:usize){self.events.push(format!("usize:{x}"))}fn write_isize(&mut self,x:isize){self.events.push(format!("isize:{x}"))}fn write_u8(&mut self,x:u8){self.events.push(format!("u8:{x}"))}fn write_length_prefix(&mut self,x:usize){self.events.push(format!("length:{x}"))}}
struct Sink<'a>{events:Vec<String>,out:String,n:usize,fail:usize,base:&'a[u8]}
impl fmt::Write for Sink<'_>{fn write_str(&mut self,s:&str)->fmt::Result{self.n+=1;self.events.push(format!("S:{}:{}",hex(s),alias(s.as_ptr(),s.len(),self.base)));if self.n==self.fail{return Err(fmt::Error)}self.out.push_str(s);Ok(())}fn write_char(&mut self,c:char)->fmt::Result{self.n+=1;self.events.push(format!("C:{:x}",c as u32));if self.n==self.fail{return Err(fmt::Error)}self.out.push(c);Ok(())}}
fn debug(sink:&mut Sink<'_>,style:u32,value:&dyn fmt::Debug)->fmt::Result{match style{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:*^12.3?}"),3=>write!(sink,"{value:+012?}"),4=>write!(sink,"{value:*<12?}"),5=>write!(sink,"{value:λ^12.3?}"),6=>write!(sink,"{value:x?}"),7=>write!(sink,"{value:#X?}"),8=>write!(sink,"{value:\0>12?}"),_=>unreachable!()}}
fn run(parts:&[&str])->String{let wide=|s:&str|unhex(s).chunks_exact(2).map(|b|u16::from_le_bytes([b[0],b[1]])).collect::<Vec<_>>();let po=OsString::from_wide(&wide(parts[2]));let qo=OsString::from_wide(&wide(parts[3]));let pb=po.as_encoded_bytes();let qb=qo.as_encoded_bytes();let p=Path::new(&po);let q=Path::new(&qo);match parts[1]{
 "view"=>format!("{}:{}:{}:{}",p.has_root(),p.is_absolute(),p.file_name().map_or("none".into(),|v|hex(v.as_encoded_bytes())),p.components().map(component).collect::<Vec<_>>().join(",")),
 "iter"=>{let mut c=p.components();let mut out=vec![];for step in parts[4].chars(){match step{'F'=>out.push(c.next().map_or("none".into(),component)),'B'=>out.push(c.next_back().map_or("none".into(),component)),'A'=>out.push(format!("path:{}",hex(c.as_path().as_os_str().as_encoded_bytes()))),'C'=>out.push(format!("clone:{}",c.clone().map(component).collect::<Vec<_>>().join(","))),_=>panic!("bad iterator step")}}out.join("|")},
 "hash"|"oshash"=>{let mut h=H{events:vec![],base:&pb};if parts[1]=="hash"{p.hash(&mut h)}else{p.as_os_str().hash(&mut h)}h.events.join("|")},
 "lossy"=>{let text=p.to_string_lossy();format!("{}:{}",matches!(text,std::borrow::Cow::Owned(_)),hex(text.as_bytes()))},
 "compare"=>format!("{:?}",p.cmp(q)),
 "strip"=>p.strip_prefix(q).map_or("none".into(),|p|hex(p.as_os_str().as_encoded_bytes())),
 "join"=>{let joined=p.join(q);let b=joined.as_os_str().as_encoded_bytes();format!("{}:{}:{}",hex(b),alias(b.as_ptr(),b.len(),&pb),alias(b.as_ptr(),b.len(),&qb))},
 "clone"=>{let cloned=p.to_path_buf();let b=cloned.as_os_str().as_encoded_bytes();format!("{}:{}",hex(b),alias(b.as_ptr(),b.len(),&pb))},
 "debug"=>{let mut sink=Sink{events:vec![],out:String::new(),n:0,fail:parts[6].parse().unwrap(),base:&pb};let result=debug(&mut sink,parts[5].parse().unwrap(),&p);format!("{}:{}:{}",result.is_ok(),hex(sink.out),sink.events.join("|"))},
 _=>panic!("unknown path operation")}}
fn main(){for line in io::stdin().lock().lines(){let line=line.unwrap();let parts:Vec<_>=line.split('\t').collect();println!("{}\t{}",parts[0],hex(run(&parts)));}}
