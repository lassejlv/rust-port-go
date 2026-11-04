// Development-only wrapper compiled against the entire unchanged pinned parser.
// Each source Debug method is invoked directly, with native std builders/sinks.
use rustc_parse_format as pf;
use std::{fmt::{self,Debug,Write},io::{self,Write as IoWrite},sync::Mutex};
fn hex(bytes:&[u8])->String {bytes.iter().map(|b|format!("{b:02x}")).collect()}
struct Sink {events:Vec<String>,accepted:String,fail:usize,chars:bool}
impl Sink {fn event(&mut self,kind:char,text:&str)->fmt::Result {let n=self.events.len();self.events.push(format!("{kind}:{}",hex(text.as_bytes())));if n==self.fail {Err(fmt::Error)}else{self.accepted.push_str(text);Ok(())}}}
impl Write for Sink {
 fn write_str(&mut self,text:&str)->fmt::Result {self.event('S',text)}
 fn write_char(&mut self,c:char)->fmt::Result {if self.chars {self.event('C',c.encode_utf8(&mut[0;4]))} else {self.write_str(c.encode_utf8(&mut[0;4]))}}
}
#[derive(Default)]struct State {first:bool,reused:bool,before:String,after:String,final_state:String}
static STATE:Mutex<Option<State>>=Mutex::new(None);
fn state(f:&fmt::Formatter<'_>)->String {let align=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};format!("{}:{align}",f.fill() as u32)}
struct Observed<'a>{value:&'a dyn Debug,reuse:bool}
impl Debug for Observed<'_> {
 fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {
  let before=state(f);let result=Debug::fmt(self.value,f);let after=state(f);
  let second=if self.reuse {Debug::fmt(&26usize,f)}else{Ok(())};let final_state=state(f);
  *STATE.lock().unwrap()=Some(State{first:result.is_ok(),reused:second.is_ok(),before,after,final_state});result.and(second)
 }
}
fn render(out:&mut Sink,value:&dyn Debug,pattern:usize,reuse:bool)->fmt::Result {
 let value=Observed{value,reuse};match pattern {
  0=>write!(out,"{value:?}"),1=>write!(out,"{value:#?}"),2=>write!(out,"{value:x?}"),3=>write!(out,"{value:X?}"),
  4=>write!(out,"{value:#x?}"),5=>write!(out,"{value:+08?}"),6=>write!(out,"{value:08x?}"),
  7=>write!(out,"{value:*>12?}"),8=>write!(out,"{value:*<12?}"),9=>write!(out,"{value:*^13?}"),
  10=>write!(out,"{value:.2?}"),11=>write!(out,"{value:+#08x?}"),12=>write!(out,"{value:-08?}"),
  13=>write!(out,"{value:🦀^12?}"),14=>write!(out,"{value:0>12?}"),15=>write!(out,"{value:\n>8?}"),
  16=>write!(out,"{value:\0>8?}"),17=>write!(out,"{value:#08?}"),18=>write!(out,"{value:#.2?}"),
  _=>panic!("invalid pattern")
 }
}
fn spec(fill:char)->pf::FormatSpec<'static> {
 pf::FormatSpec{fill:Some(fill),fill_span:Some(26..usize::MAX),align:pf::Alignment::AlignCenter,sign:Some(pf::Sign::Minus),alternate:true,zero_pad:false,debug_hex:Some(pf::DebugHex::Upper),precision:pf::Count::CountIsName("n\n\"\\\0\u{301}",26..42),precision_span:Some(1..9),width:pf::Count::CountIs(u16::MAX),width_span:Some(0..26),ty:"x\n🦀\u{301}\0",ty_span:Some(26..42)}
}
fn argument(rich:bool)->pf::Argument<'static> {pf::Argument{position:if rich{pf::Position::ArgumentIs(26)}else{pf::Position::ArgumentNamed("name")},position_span:26..42,format:if rich{spec('\'')}else{pf::FormatSpec::default()}}}
fn values()->Vec<Box<dyn Debug>> {vec![
 Box::new(pf::ParseMode::Format),Box::new(pf::ParseMode::InlineAsm),Box::new(pf::ParseMode::Diagnostic),
 Box::new(pf::Alignment::AlignUnknown),Box::new(pf::Alignment::AlignLeft),Box::new(pf::Alignment::AlignRight),Box::new(pf::Alignment::AlignCenter),
 Box::new(pf::Sign::Plus),Box::new(pf::Sign::Minus),Box::new(pf::DebugHex::Lower),Box::new(pf::DebugHex::Upper),
 Box::new(pf::Position::ArgumentImplicitlyIs(0)),Box::new(pf::Position::ArgumentIs(26)),Box::new(pf::Position::ArgumentIs(usize::MAX)),
 Box::new(pf::Position::ArgumentNamed("")),Box::new(pf::Position::ArgumentNamed("plain")),Box::new(pf::Position::ArgumentNamed("a\n\"'\\\0")),Box::new(pf::Position::ArgumentNamed("\u{301}x🦀\u{301}")),
 Box::new(pf::Count::CountImplied),Box::new(pf::Count::CountIs(0)),Box::new(pf::Count::CountIs(u16::MAX)),
 Box::new(pf::Count::CountIsName("",0..0)),Box::new(pf::Count::CountIsName("n\n",26..usize::MAX)),Box::new(pf::Count::CountIsParam(26)),Box::new(pf::Count::CountIsStar(usize::MAX)),
 Box::new(pf::FormatSpec::default()),Box::new(spec('🦀')),Box::new(spec('\0')),Box::new(spec('\n')),Box::new(spec('\'')),Box::new(spec('\u{301}')),
 Box::new(argument(false)),Box::new(argument(true)),Box::new(pf::Piece::Lit("")),Box::new(pf::Piece::Lit("plain")),Box::new(pf::Piece::Lit("a\n\"'\\\0")),Box::new(pf::Piece::Lit("\u{301}x🦀\u{301}")),
 Box::new(pf::Piece::NextArgument(Box::new(argument(false)))),Box::new(pf::Piece::NextArgument(Box::new(argument(true)))),
 Box::new(pf::Count::CountIsName("range",0..26))
]}
fn main()->io::Result<()> {
 let mut out=io::BufWriter::new(io::stdout().lock());
 for (case,value) in values().iter().enumerate() {for pattern in 0..19 {for chars in [false,true] {for reuse in [false,true] {
  let mut baseline=Sink{events:vec![],accepted:String::new(),fail:usize::MAX,chars};assert!(render(&mut baseline,value.as_ref(),pattern,reuse).is_ok());
  for fail in std::iter::once(usize::MAX).chain(0..=baseline.events.len()) {
   let mut sink=Sink{events:vec![],accepted:String::new(),fail,chars};let ok=render(&mut sink,value.as_ref(),pattern,reuse).is_ok();let state=STATE.lock().unwrap().take().unwrap();
   writeln!(out,"{case}\t{pattern}\t{chars}\t{reuse}\t{}\t{ok}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",if fail==usize::MAX{-1}else{fail as isize},state.first,state.reused,state.before,state.after,state.final_state,hex(sink.accepted.as_bytes()),sink.events.join("|"))?;
  }
 }}}}out.flush()
}
