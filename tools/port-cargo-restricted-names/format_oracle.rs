// Development-only original-source formatter probe. MIT OR Apache-2.0.
use std::cell::RefCell;
use std::fmt::{self, Write};
use std::io::{self, Read};

use super::restricted_names::*;
use super::{text_in, text_out, u32_in};

#[derive(Clone, Copy)]
struct State { fill: u32, align: u8, width: Option<usize>, precision: Option<usize>, flags: u8 }
fn state(f: &fmt::Formatter<'_>) -> State {
    State { fill:f.fill() as u32, align:match f.align() {None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3}, width:f.width(), precision:f.precision(), flags:u8::from(f.alternate())|u8::from(f.sign_plus())<<1|u8::from(f.sign_minus())<<2|u8::from(f.sign_aware_zero_pad())<<3 }
}
fn state_out(out: &mut impl io::Write, s: State) {
    out.write_all(&s.fill.to_le_bytes()).unwrap(); out.write_all(&[s.align,s.flags]).unwrap();
    for value in [s.width,s.precision] { out.write_all(&[u8::from(value.is_some())]).unwrap(); out.write_all(&(value.unwrap_or(0) as u64).to_le_bytes()).unwrap(); }
}
struct Sink { writes:Vec<(u8,String)>, accepted:String, chars:bool, fail:u32 }
impl Sink {
    fn record(&mut self, kind:u8, text:&str)->fmt::Result {
        let index=self.writes.len() as u32;
        self.writes.push((kind,text.to_owned()));
        if index==self.fail {return Err(fmt::Error)}
        self.accepted.push_str(text); Ok(())
    }
}
impl fmt::Write for Sink {
    fn write_str(&mut self,text:&str)->fmt::Result {self.record(0,text)}
    fn write_char(&mut self,ch:char)->fmt::Result {
        let mut storage=[0;4]; let text=ch.encode_utf8(&mut storage);
        if self.chars {self.record(1,text)} else {self.write_str(text)}
    }
}
struct Observation { states:Vec<State>, first:bool, reuse:bool }
struct Spy<'a> { error:&'a NameValidationError, debug:bool, reuse:bool, observation:&'a RefCell<Observation> }
impl Spy<'_> {
    fn render(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {
        self.observation.borrow_mut().states.push(state(f));
        let first=if self.debug {fmt::Debug::fmt(self.error,f)} else {fmt::Display::fmt(self.error,f)};
        { let mut observation=self.observation.borrow_mut(); observation.first=first.is_ok(); observation.states.push(state(f)); }
        let reuse=if self.reuse {fmt::Debug::fmt(&26usize,f)} else {Ok(())};
        {let mut observation=self.observation.borrow_mut(); observation.reuse=reuse.is_ok(); observation.states.push(state(f));}
        first.and(reuse)
    }
}
impl fmt::Debug for Spy<'_> {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {self.render(f)}}
impl fmt::Display for Spy<'_> {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {self.render(f)}}
fn render(sink:&mut Sink,spy:&Spy<'_>,pattern:u8)->fmt::Result {
    match pattern {
        0=>write!(sink,"{}",spy), 1=>write!(sink,"{:#?}",spy), 2=>write!(sink,"{:x?}",spy), 3=>write!(sink,"{:X?}",spy),
        4=>write!(sink,"{:#x?}",spy), 5=>write!(sink,"{:+08?}",spy), 6=>write!(sink,"{:08x?}",spy),
        7=>write!(sink,"{:*>12?}",spy), 8=>write!(sink,"{:*<12?}",spy), 9=>write!(sink,"{:*^13?}",spy),
        10=>write!(sink,"{:.2?}",spy), 11=>write!(sink,"{:+#08x?}",spy), 12=>write!(sink,"{:-08?}",spy),
        13=>write!(sink,"{:🦀^12?}",spy),14=>write!(sink,"{:0>12?}",spy),15=>write!(sink,"{:\n>8?}",spy),
        16=>write!(sink,"{:\0>8?}",spy),17=>write!(sink,"{:#08?}",spy),18=>write!(sink,"{:#.2?}",spy),
        _=>panic!("unknown format pattern"),
    }
}
pub fn run() {
    let mut data=Vec::new(); io::stdin().read_to_end(&mut data).unwrap(); let mut input=data.as_slice();
    assert_eq!(&input[..4],b"RNF1"); input=&input[4..]; let count=u32_in(&mut input);
    let mut output=io::BufWriter::new(io::stdout().lock()); io::Write::write_all(&mut output,b"RNG1").unwrap(); io::Write::write_all(&mut output,&count.to_le_bytes()).unwrap();
    for _ in 0..count {
        let op=input[0]; input=&input[1..]; let name=text_in(&mut input); let what=text_in(&mut input);
        let debug=input[0]!=0; let pattern=input[1]; let chars=input[2]!=0; let reuse=input[3]!=0; input=&input[4..]; let fail=u32_in(&mut input);
        let error=match op {0=>validate_package_name(&name),1=>validate_registry_name(&name),2=>validate_name(&name,Box::leak(what.into_boxed_str())),3=>validate_profile_name(&name),4=>validate_feature_name(&name),5=>validate_path_base_name(&name),_=>panic!("unknown validator")}.unwrap_err();
        let observation=RefCell::new(Observation{states:Vec::new(),first:false,reuse:false});
        let mut sink=Sink{writes:Vec::new(),accepted:String::new(),chars,fail};
        let ok=render(&mut sink,&Spy{error:&error,debug,reuse,observation:&observation},pattern).is_ok();
        let observation=observation.into_inner(); assert_eq!(observation.states.len(),3);
        io::Write::write_all(&mut output,&[u8::from(observation.first),u8::from(observation.reuse),u8::from(ok)]).unwrap();
        for state in observation.states {state_out(&mut output,state)}
        io::Write::write_all(&mut output,&(sink.writes.len() as u32).to_le_bytes()).unwrap();
        for (kind,text) in sink.writes {io::Write::write_all(&mut output,&[kind]).unwrap();text_out(&mut output,&text)}
        text_out(&mut output,&sink.accepted);
    }
    assert!(input.is_empty());
}
