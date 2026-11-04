// Development-only probe of the recorded Rust standard-library DebugStruct.
// No implementation body is substituted into the reference path.
use std::{cell::RefCell,fmt::{self,Debug,Write},io::{self,BufRead,Write as IoWrite},rc::Rc};
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{events:Rc<RefCell<Vec<String>>>,writes:usize,fail:i64,chars:bool}
impl Sink{fn record(&mut self,kind:&str,s:&str)->fmt::Result{self.events.borrow_mut().push(format!("{kind}:{}",hex(s)));let n=self.writes;self.writes+=1;if n as i64==self.fail{Err(fmt::Error)}else{Ok(())}}}
impl Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.record("S",s)}fn write_char(&mut self,c:char)->fmt::Result{let mut b=[0;4];let s=c.encode_utf8(&mut b);self.record(if self.chars{"C"}else{"S"},s)}}
struct Value{kind:usize,events:Rc<RefCell<Vec<String>>>}
impl Debug for Value{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.events.borrow_mut().push(format!("D:{}",self.kind));match self.kind%3{
 0=>Debug::fmt(&u64::MAX,f),
 1=>{f.write_char('\n')?;let _=f.write_str("attempt");f.write_char('é')?;f.write_str("x\ny")},
 _=>f.debug_struct("Child").field("n",&15u64).finish_non_exhaustive(),
}}}
struct Record{count:usize,events:Rc<RefCell<Vec<String>>>}
impl Debug for Record{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let mut b=f.debug_struct("Record");for i in 0..self.count{self.events.borrow_mut().push(format!("E:{i}"));b.field(if i==0{"number"}else{"field"},&Value{kind:i,events:self.events.clone()});}b.finish_non_exhaustive()}}
struct Observed{count:usize,reuse:bool,events:Rc<RefCell<Vec<String>>>}
fn state(f:&fmt::Formatter<'_>)->String{format!("{}:{}",f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3})}
impl Debug for Observed{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let result=Debug::fmt(&Record{count:self.count,events:self.events.clone()},f);self.events.borrow_mut().push(format!("R:{}:{}",result.is_ok(),state(f)));if self.reuse{let second=Debug::fmt(&Record{count:1,events:self.events.clone()},f);self.events.borrow_mut().push(format!("R:{}:{}",second.is_ok(),state(f)));second}else{result}}}
fn render(out:&mut Sink,v:&Observed,pattern:usize)->fmt::Result{match pattern{
0=>write!(out,"{v:?}"),1=>write!(out,"{v:#?}"),2=>write!(out,"{v:030?}"),3=>write!(out,"{v:#030?}"),
4=>write!(out,"{v:+030?}"),5=>write!(out,"{v:*>+030?}"),6=>write!(out,"{v:*<30?}"),7=>write!(out,"{v:_^30?}"),
8=>write!(out,"{v:030x?}"),9=>write!(out,"{v:#030x?}"),10=>write!(out,"{v:030X?}"),11=>write!(out,"{v:#030X?}"),
12=>write!(out,"{v:.0?}"),13=>write!(out,"{v:é>30.3?}"),14=>write!(out,"{v:\n>+030?}"),15=>write!(out,"{v:\n>+#030?}"),
_=>panic!("unknown pattern")}}
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());for line in io::stdin().lock().lines(){let line=line?;let p:Vec<_>=line.split('\t').collect();assert_eq!(p.len(),5);let pattern=p[0].parse().unwrap();let count=p[1].parse().unwrap();let chars=p[2]=="true";let fail=p[3].parse().unwrap();let reuse=p[4]=="true";let events=Rc::new(RefCell::new(vec![]));let v=Observed{count,reuse,events:events.clone()};let mut sink=Sink{events:events.clone(),writes:0,fail,chars};let result=render(&mut sink,&v,pattern);writeln!(out,"{line}\t{}\t{}",result.is_ok(),events.borrow().join("|"))?;}Ok(())}
