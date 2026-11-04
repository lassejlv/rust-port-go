#![feature(ascii_char, ascii_char_variants)]
#![allow(deprecated)]
mod base_n;
use base_n::{BaseNString,ToBaseN};
use std::{fmt::{self,Debug,Display,Write},io::{self,Write as IoWrite},ops::Deref,sync::Mutex};

struct Width<const N:usize>(u128);
impl<const N:usize> From<Width<N>> for u128 {fn from(value:Width<N>)->u128{value.0}}
impl<const N:usize> ToBaseN for Width<N> {fn encoded_len(_:usize)->usize{N}}

fn values()->Vec<BaseNString>{
 let mut values=Vec::new();
 for base in [2,3,10,36,62,64] {for n in [0,1,35,63,64,u64::MAX as u128,u128::MAX] {
  values.push(n.to_base(base));values.push(n.to_base_fixed_len(base));
 }}
 values.extend([Width::<0>(7).to_base_fixed_len(10),Width::<1>(42).to_base_fixed_len(10),Width::<5>(7).to_base_fixed_len(10),Width::<128>(7).to_base_fixed_len(10)]);
 values
}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink {calls:Vec<String>,accepted:String,fail:usize,chars:bool,expected:*const u8}
impl fmt::Write for Sink {
 fn write_str(&mut self,s:&str)->fmt::Result{
  let same=s.as_ptr()==self.expected;
  self.calls.push(format!("S:{}:{same}",hex(s)));
  if self.calls.len()-1==self.fail {Err(fmt::Error)} else {self.accepted.push_str(s);Ok(())}
 }
 fn write_char(&mut self,c:char)->fmt::Result{
  if self.chars {self.calls.push(format!("C:{c}"));Err(fmt::Error)} else {self.write_str(c.encode_utf8(&mut[0;4]))}
 }
}
fn state(f:&fmt::Formatter<'_>)->String{
 let align=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
 format!("{}:{align}:{}:{}:{}",f.fill() as u32,f.width().map_or("-".into(),|v|v.to_string()),f.precision().map_or("-".into(),|v|v.to_string()),f.flags())
}
static STATES:Mutex<Option<(bool,bool,String,String,String)>>=Mutex::new(None);
struct Observed<'a>{value:&'a BaseNString,reuse:bool}
impl Debug for Observed<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let before=state(f);let first=Display::fmt(self.value,f);let after=state(f);
 let second=if self.reuse {Display::fmt(self.value,f)}else{Ok(())};let final_state=state(f);
 *STATES.lock().unwrap()=Some((first.is_ok(),second.is_ok(),before,after,final_state));first.and(second)
}}
fn render(out:&mut Sink,value:&BaseNString,pattern:usize,reuse:bool)->fmt::Result{
 let value=Observed{value,reuse};match pattern{
  0=>write!(out,"{value:?}"),1=>write!(out,"{value:#?}"),2=>write!(out,"{value:x?}"),3=>write!(out,"{value:X?}"),
  4=>write!(out,"{value:#x?}"),5=>write!(out,"{value:+08?}"),6=>write!(out,"{value:08x?}"),
  7=>write!(out,"{value:*>12?}"),8=>write!(out,"{value:*<12?}"),9=>write!(out,"{value:*^13?}"),
  10=>write!(out,"{value:.2?}"),11=>write!(out,"{value:+#08x?}"),12=>write!(out,"{value:-08?}"),
  13=>write!(out,"{value:🦀^12?}"),14=>write!(out,"{value:0>12?}"),15=>write!(out,"{value:\n>8?}"),
  16=>write!(out,"{value:\0>8?}"),17=>write!(out,"{value:#08?}"),18=>write!(out,"{value:#.2?}"),_=>unreachable!()
 }
}
fn main(){
 let mut out=io::BufWriter::new(io::stdout().lock());
 for (case,value) in values().iter().enumerate(){
  let expected=value.deref().as_ptr();assert_eq!(expected,value.as_ref().as_ptr());assert_eq!(expected,value.deref().as_ptr());
  for pattern in 0..19{for chars in [false,true]{for reuse in [false,true]{for fail in [usize::MAX,0,1,2]{
   let mut sink=Sink{calls:Vec::new(),accepted:String::new(),fail,chars,expected};let ok=render(&mut sink,value,pattern,reuse).is_ok();let (first,second,before,after,final_state)=STATES.lock().unwrap().take().unwrap();
   writeln!(out,"{case}\t{pattern}\t{chars}\t{reuse}\t{}\t{ok}\t{first}\t{second}\t{before}\t{after}\t{final_state}\t{}\t{}",if fail==usize::MAX{-1}else{fail as isize},hex(&sink.accepted),sink.calls.join("|" )).unwrap();
  }}}}
 }
}
