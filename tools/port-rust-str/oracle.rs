#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
// Uses the unchanged selected SDK str/Range/char implementations directly.
use std::{io::{self,BufRead},sync::{Arc,Mutex}};
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
fn unhex(text:&str)->Vec<u8>{text.as_bytes().chunks_exact(2).map(|b|u8::from_str_radix(std::str::from_utf8(b).unwrap(),16).unwrap()).collect()}
fn observe(text:&str,op:&str,begin:usize,end:usize)->String{
 let view=match op{
 "range"=>Some(&text[begin..end]),
 "from"=>Some(&text[begin..]),
 "to"=>Some(&text[..end]),
 "inclusive"=>Some(&text[begin..=end]),
 "to_inclusive"=>Some(&text[..=end]),
 "exhausted"=>{let mut range=begin..=begin;range.next();Some(&text[range])},
 "get"=>text.get(begin..end),
 "get_inclusive"=>text.get(begin..=end),
 "get_exhausted"=>{let mut range=begin..=begin;range.next();text.get(range)},
 "boundaries"=>return format!("{}:{}:{}",text.is_char_boundary(begin),text.floor_char_boundary(begin),text.ceil_char_boundary(begin)),
 _=>panic!("unknown query")};
 view.map(|s|format!("some:{}",hex(s.as_bytes()))).unwrap_or("none".into())
}
fn main(){let hooks:Arc<Mutex<Vec<String>>>=Default::default();let hooklog=hooks.clone();std::panic::set_hook(Box::new(move|info|{
 let(kind,payload)=if let Some(s)=info.payload().downcast_ref::<&str>(){("static_str",*s)}else if let Some(s)=info.payload().downcast_ref::<String>(){("owned_string",s.as_str())}else{("opaque","")};let loc=info.location().unwrap();let file=loc.file().strip_prefix(concat!(env!("FERRUGO_ORACLE_ROOT"),"/")).unwrap_or(loc.file());hooklog.lock().unwrap().push(format!("{kind}:{}:{}:{}:{}:{}:{}:{}",hex(payload.as_bytes()),hex(file.as_bytes()),loc.line(),loc.column(),info.can_unwind(),info.force_no_backtrace(),std::thread::panicking()));
 }));for line in io::stdin().lock().lines(){let line=line.unwrap();let fields:Vec<_>=line.split('\t').collect();let bytes=unhex(fields[0]);let text=std::str::from_utf8(&bytes).unwrap();let begin:usize=fields[2].parse().unwrap();let end:usize=fields[3].parse().unwrap();hooks.lock().unwrap().clear();let result=std::panic::catch_unwind(||observe(text,fields[1],begin,end));let value=match result{Ok(value)=>format!("ok:{value}"),Err(_)=>"panic".into()};println!("{}\t{}",value,hooks.lock().unwrap().join(","));}}
