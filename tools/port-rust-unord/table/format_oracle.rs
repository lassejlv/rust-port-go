// Actual std Map/Set/Entry Debug delegation, observed through fmt::Write.
#![feature(map_try_insert)]
use rustc_hash::{FxHashMap,FxHashSet};
use std::{cell::RefCell,collections::hash_map::Entry,fmt::{self,Debug,Write},io::{self,BufRead,Write as IoWrite},rc::Rc};
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{events:Rc<RefCell<Vec<String>>>,writes:usize,fail:i64,chars:bool}
impl Sink{fn record(&mut self,kind:&str,text:&str)->fmt::Result{self.events.borrow_mut().push(format!("{kind}:{}",hex(text)));let index=self.writes;self.writes+=1;if index as i64==self.fail{Err(fmt::Error)}else{Ok(())}}}
impl Write for Sink{fn write_str(&mut self,text:&str)->fmt::Result{self.record("S",text)}fn write_char(&mut self,c:char)->fmt::Result{let mut bytes=[0;4];self.record(if self.chars{"C"}else{"S"},c.encode_utf8(&mut bytes))}}
struct Observed<'a,T>{value:&'a T,reuse:bool,events:Rc<RefCell<Vec<String>>>}
fn state(f:&fmt::Formatter<'_>)->String{format!("{}:{}",f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3})}
impl<T:Debug>Debug for Observed<'_,T>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let result=self.value.fmt(f);self.events.borrow_mut().push(format!("R:{}:{}",result.is_ok(),state(f)));if self.reuse{let second=self.value.fmt(f);self.events.borrow_mut().push(format!("R:{}:{}",second.is_ok(),state(f)));second}else{result}}}
fn render<T:Debug>(out:&mut Sink,value:&T,reuse:bool,events:Rc<RefCell<Vec<String>>>,pattern:usize)->fmt::Result{let v=Observed{value,reuse,events};match pattern{
0=>write!(out,"{v:?}"),1=>write!(out,"{v:#?}"),2=>write!(out,"{v:030?}"),3=>write!(out,"{v:#030?}"),4=>write!(out,"{v:+030?}"),5=>write!(out,"{v:*>+030?}"),6=>write!(out,"{v:*<30?}"),7=>write!(out,"{v:_^30?}"),8=>write!(out,"{v:030x?}"),9=>write!(out,"{v:#030x?}"),10=>write!(out,"{v:030X?}"),11=>write!(out,"{v:#030X?}"),12=>write!(out,"{v:.0?}"),13=>write!(out,"{v:é>30.3?}"),14=>write!(out,"{v:\n>+030?}"),15=>write!(out,"{v:\n>+#030?}"),_=>panic!("unknown pattern")}}
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());for line in io::stdin().lock().lines(){let line=line?;let f:Vec<_>=line.split('\t').collect();assert_eq!(f.len(),5);let kind:usize=f[0].parse().unwrap();let pattern=f[1].parse().unwrap();let chars=f[2]=="true";let fail=f[3].parse().unwrap();let reuse=f[4]=="true";let events=Rc::new(RefCell::new(vec![]));let mut sink=Sink{events:events.clone(),writes:0,fail,chars};let mut map=FxHashMap::default();map.insert(1u64,u64::MAX);map.insert(u64::MAX,1);map.insert(16,2);let set:FxHashSet<_>=[1u64,u64::MAX,16].into_iter().collect();let result=match kind{
0=>render(&mut sink,&map,reuse,events.clone(),pattern),1=>render(&mut sink,&set,reuse,events.clone(),pattern),
2=>render(&mut sink,&map.entry(1),reuse,events.clone(),pattern),3=>render(&mut sink,&map.entry(2),reuse,events.clone(),pattern),
4=>if let Entry::Occupied(entry)=map.entry(1){render(&mut sink,&entry,reuse,events.clone(),pattern)}else{unreachable!()},
5=>if let Entry::Vacant(entry)=map.entry(2){render(&mut sink,&entry,reuse,events.clone(),pattern)}else{unreachable!()},
6=>render(&mut sink,&map.try_insert(1,999).unwrap_err(),reuse,events.clone(),pattern),_=>panic!("unknown kind")};writeln!(out,"{line}\t{}\t{}",result.is_ok(),events.borrow().join("|"))?;}Ok(())}
