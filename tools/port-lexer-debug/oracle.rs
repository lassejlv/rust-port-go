// Development-only wrapper compiled against the entire unchanged pinned parser.
// Each source Debug method is invoked directly, with native std builders/sinks.
#![feature(fn_traits, unboxed_closures)]
#[path="upstream/rustc_lexer/src/lib.rs"]
mod lexer;
use rustc_literal_escaper as lit;
mod owned;
mod profile;
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

fn literal_values()->Vec<Box<dyn Debug>> {
 use lit::EscapeError::*; use lit::Mode;
 let mut v:Vec<Box<dyn Debug>>=vec![];
 for e in [ZeroChars,MoreThanOneChar,LoneSlash,InvalidEscape,BareCarriageReturn,BareCarriageReturnInRawString,EscapeOnlyChar,TooShortHexEscape,InvalidCharInHexEscape,OutOfRangeHexEscape,NoBraceInUnicodeEscape,InvalidCharInUnicodeEscape,EmptyUnicodeEscape,UnclosedUnicodeEscape,LeadingUnderscoreUnicodeEscape,OverlongUnicodeEscape,LoneSurrogateUnicodeEscape,OutOfRangeUnicodeEscape,UnicodeEscapeInByte,NonAsciiCharInByte,NulInCStr,UnskippedWhitespaceWarning,MultipleSkippedLinesWarning] {v.push(Box::new(e));}
 for m in [Mode::Char,Mode::Byte,Mode::Str,Mode::RawStr,Mode::ByteStr,Mode::RawByteStr,Mode::CStr,Mode::RawCStr] {v.push(Box::new(m));}
 for c in ['a','\n','\'','\\','🦀','\u{301}','\u{7f}','\u{10ffff}'] {v.push(Box::new(lit::MixedUnit::Char(std::num::NonZero::new(c).unwrap())));v.push(Box::new(std::num::NonZero::new(c).unwrap()));}
 for b in [1u8,26,127,128,255] {v.push(Box::new(lit::MixedUnit::HighByte(std::num::NonZero::new(b).unwrap())));v.push(Box::new(std::num::NonZero::new(b).unwrap()));}
 v
}
fn literals()->Vec<lexer::LiteralKind> {
 use lexer::{LiteralKind::*,Base::*}; let mut v=vec![];
 for base in [Binary,Octal,Decimal,Hexadecimal] {for b in [false,true] {v.push(Int{base,empty_int:b});v.push(Float{base,empty_exponent:b});}}
 for b in [false,true] {v.extend([Char{terminated:b},Byte{terminated:b},Str{terminated:b},ByteStr{terminated:b},CStr{terminated:b}]);}
 for h in [None,Some(0),Some(1),Some(26),Some(255)] {v.extend([RawStr{n_hashes:h},RawByteStr{n_hashes:h},RawCStr{n_hashes:h}]);} v
}
fn errors()->Vec<lexer::RawStrError> {
 use lexer::RawStrError::*;let mut v=vec![];
 for c in ['\0','\n','\'','\\','a','🦀','\u{301}','\u{7f}','\u{10ffff}'] {v.push(InvalidStarter{bad_char:c});}
 for n in [0,26,u32::MAX] {for h in [None,Some(0),Some(26),Some(u32::MAX)] {v.push(NoTerminator{expected:n,found:u32::MAX-n,possible_terminator_offset:h});}v.push(TooManyDelimiters{found:n});}v
}
fn guards()->Vec<lexer::GuardedStr> {
 let mut v=vec![];for n in [0,26,u32::MAX] {for b in [false,true] {for len in [0,26,u32::MAX] {v.push(lexer::GuardedStr{n_hashes:n,terminated:b,token_len:len});}}}v
}
fn lexer_values()->Vec<Box<dyn Debug>> {
 use lexer::{TokenKind::*,DocStyle,Base};let mut v:Vec<Box<dyn Debug>>=vec![];
 for b in [Base::Binary,Base::Octal,Base::Decimal,Base::Hexadecimal] {v.push(Box::new(b));}
 v.push(Box::new(DocStyle::Outer));v.push(Box::new(DocStyle::Inner));
 for h in [None,Some(DocStyle::Outer),Some(DocStyle::Inner)] {v.push(Box::new(LineComment{doc_style:h}));for b in [false,true] {v.push(Box::new(BlockComment{doc_style:h,terminated:b}));}}
 for a in [false,true] {for b in [false,true] {v.push(Box::new(Frontmatter{has_invalid_preceding_whitespace:a,invalid_infostring:b}));}v.push(Box::new(Lifetime{starts_with_number:a}));}
 for k in [Whitespace,Ident,InvalidIdent,RawIdent,ForcedKeywordIdent,UnknownPrefix,UnknownPrefixLifetime,RawLifetime,GuardedStrPrefix,Semi,Comma,Dot,OpenParen,CloseParen,OpenBrace,CloseBrace,OpenBracket,CloseBracket,At,Pound,Tilde,Question,Colon,Dollar,Eq,Bang,Lt,Gt,Minus,And,Or,Plus,Star,Slash,Caret,Percent,Unknown,Eof] {v.push(Box::new(k));v.push(Box::new(lexer::Token{kind:k,len:u32::MAX}));}
 for k in literals() {v.push(Box::new(k));v.push(Box::new(Literal{kind:k,suffix_start:26}));v.push(Box::new(lexer::Token{kind:Literal{kind:k,suffix_start:u32::MAX},len:26}));}
 for e in errors() {v.push(Box::new(e));}for g in guards() {v.push(Box::new(g));}v
}
fn debug(scope:&str)->io::Result<()> {
 let values=if scope=="literal" {literal_values()} else {lexer_values()};
 let mut out=io::BufWriter::new(io::stdout().lock());
 for (case,value) in values.iter().enumerate() {for pattern in 0..19 {for chars in [false,true] {for reuse in [false,true] {
  let mut baseline=Sink{events:vec![],accepted:String::new(),fail:usize::MAX,chars};assert!(render(&mut baseline,value.as_ref(),pattern,reuse).is_ok());
  for fail in std::iter::once(usize::MAX).chain(0..=baseline.events.len()) {
   let mut sink=Sink{events:vec![],accepted:String::new(),fail,chars};let ok=render(&mut sink,value.as_ref(),pattern,reuse).is_ok();let state=STATE.lock().unwrap().take().unwrap();
   writeln!(out,"{case}\t{pattern}\t{chars}\t{reuse}\t{}\t{ok}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",if fail==usize::MAX{-1}else{fail as isize},state.first,state.reused,state.before,state.after,state.final_state,hex(sink.accepted.as_bytes()),sink.events.join("|"))?;
  }
 }}}}out.flush()
}
fn order()->io::Result<()> {
 let mut out=io::BufWriter::new(io::stdout().lock());
 macro_rules! compare {($name:expr,$values:expr)=>{let values=$values;for(i,a)in values.iter().enumerate(){for(j,b)in values.iter().enumerate(){let ord=match a.cmp(b){std::cmp::Ordering::Less=>-1,std::cmp::Ordering::Equal=>0,std::cmp::Ordering::Greater=>1};writeln!(out,"{}\t{i}\t{j}\t{ord}\t{}",$name,a.partial_cmp(b)==Some(a.cmp(b)))?;}}};}
 compare!("literal",literals());compare!("error",errors());compare!("guard",guards());out.flush()
}
fn main()->io::Result<()> {std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();match args.get(1).map(|s|s.as_str()){Some("literal")=>debug("literal"),Some("lexer")=>debug("lexer"),Some("order")=>order(),Some("cursor")=>{profile::cursor();Ok(())},Some("iterator")=>{profile::iterator();Ok(())},Some("internal")=>{owned::internal(args[2].parse().unwrap());Ok(())},Some("fold-next")=>{owned::fold_next(args[2].parse().unwrap());Ok(())},Some("processing")=>{owned::processing(&args[2],args[3].parse().unwrap());Ok(())},Some("owned-count")=>{println!("{}",owned::count(&args[2]));Ok(())},Some("owned")=>{owned::run(&args[2],args[3].parse().unwrap());Ok(())},_=>panic!("unknown contract scope")}}
