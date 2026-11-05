#![feature(rustc_private)]
// Complete public hygiene Debug derives/custom implementations, delegated to
// the actual unchanged crate and mutable standard Formatter/writer operations.
use rustc_span::{Span,BytePos,DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_span::def_id::{CrateNum,DefId,DefIndex,ModId};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use rustc_serialize::{Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{fmt::{self,Write},cell::RefCell};
fn hash(a:u64,b:u64)->ExpnHash{let mut e=MemEncoder::new();Fingerprint::new(Hash64::new(a),Hash64::new(b)).encode(&mut e);let mut data=e.finish();data.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&data,0).unwrap())}
fn desugar(i:usize)->DesugaringKind{match i{0=>DesugaringKind::Async,1=>DesugaringKind::Await,2=>DesugaringKind::QuestionMark,3=>DesugaringKind::TryBlock,4=>DesugaringKind::YeetExpr,5=>DesugaringKind::OpaqueTy,6=>DesugaringKind::ForLoop,7=>DesugaringKind::WhileLoop,8=>DesugaringKind::BoundModifier,9=>DesugaringKind::Contract,10=>DesugaringKind::PatTyRange,11=>DesugaringKind::FormatLiteral{source:false},12=>DesugaringKind::FormatLiteral{source:true},13=>DesugaringKind::RangeExpr,_=>panic!("kind")}}
fn values()->Vec<Box<dyn fmt::Debug>>{
 let mut out:Vec<Box<dyn fmt::Debug>>=vec![];
 for n in [0,17,0x1fff_ffff]{out.push(Box::new(SyntaxContext::from_u32(n)))}
 for n in [0,17,0xffff_ff00]{out.push(Box::new(ExpnIndex::from_u32(n)))}
 for n in [0,17,0xffff_ff00]{out.push(Box::new(LocalExpnId::from_u32(n)))}
 for n in [0,17,0x1fff_ffff]{out.push(Box::new(DecodedSyntaxContext::from_u32(n)))}
 for (c,n)in[(0,0),(0,17),(3,99)]{out.push(Box::new(ExpnId{krate:CrateNum::from_u32(c),local_id:ExpnIndex::from_u32(n)}))}
 out.push(Box::new(hash(0,0)));out.push(Box::new(hash(111,777)));out.push(Box::new(hash(u64::MAX,0x8000000000000000)));
 for t in [Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque]{out.push(Box::new(t))}
 for t in [MacroKind::Bang,MacroKind::Attr,MacroKind::Derive]{out.push(Box::new(t))}
 for t in [AstPass::StdImports,AstPass::TestHarness,AstPass::ProcMacroHarness]{out.push(Box::new(t))}
 for i in 0..14{out.push(Box::new(desugar(i)))}
 out.push(Box::new(ExpnKind::Root));
 let name=Symbol::intern("macro\n\0é🦀\u{301}");
 for t in [MacroKind::Bang,MacroKind::Attr,MacroKind::Derive]{out.push(Box::new(ExpnKind::Macro(t,name)))}
 for t in [AstPass::StdImports,AstPass::TestHarness,AstPass::ProcMacroHarness]{out.push(Box::new(ExpnKind::AstPass(t)))}
 for i in 0..14{out.push(Box::new(ExpnKind::Desugaring(desugar(i))))}
 for i in 0..8{
  let call=Span::new(BytePos(i+10),BytePos(i+20),SyntaxContext::root(),None);
  let def=if i%2==0{DUMMY_SP}else{Span::new(BytePos(30),BytePos(40),SyntaxContext::root(),None)};
  let features=if i%3==0{None}else{Some(std::sync::Arc::from(if i%3==1{vec![]}else{vec![name,Symbol::intern("featureβ"),Symbol::intern("feature🦀")]}))};
  let id=DefId{krate:CrateNum::from_u32(2),index:DefIndex::from_u32(99)};
  let data=ExpnData::new(ExpnKind::Macro(MacroKind::Attr,name),ExpnId::root(),call,def,features,match i%5{0=>Edition::Edition2015,1=>Edition::Edition2018,2=>Edition::Edition2021,3=>Edition::Edition2024,_=>Edition::EditionFuture},if i%2==0{None}else{Some(id)},if i%2==0{Some(ModId::new_unchecked(id))}else{None},i&1!=0,i&2!=0,i&4!=0,i&3==0);
  out.push(Box::new(data));
 }
 for t in [Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque]{out.push(Box::new((SyntaxContext::from_u32(17),ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(99)},t)));}
 out
}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{calls:Vec<String>,text:String,fail:usize,chars:bool}
impl Sink{fn add(&mut self,kind:&str,text:&str)->fmt::Result{let n=self.calls.len();self.calls.push(format!("{kind}:{}",hex(text)));if n==self.fail{Err(fmt::Error)}else{self.text.push_str(text);Ok(())}}}
impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.add("S",s)}fn write_char(&mut self,c:char)->fmt::Result{let mut b=[0;4];if self.chars{self.add("C",c.encode_utf8(&mut b))}else{self.write_str(c.encode_utf8(&mut b))}}}
struct Observed<'a>{value:&'a dyn fmt::Debug,result:&'a RefCell<Option<(bool,bool,u32,u8)>>}
impl fmt::Debug for Observed<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let first=self.value.fmt(f);let second=fmt::Debug::fmt(&7u16,f);
 *self.result.borrow_mut()=Some((first.is_err(),second.is_err(),f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3}));
 if first.is_ok()&&second.is_ok(){Ok(())}else{Err(fmt::Error)}
}}
fn render(sink:&mut Sink,v:&Observed<'_>,style:usize)->fmt::Result{match style{
 0=>write!(sink,"{v:?}"),1=>write!(sink,"{v:#?}"),2=>write!(sink,"{v:x?}"),3=>write!(sink,"{v:X?}"),4=>write!(sink,"{v:#x?}"),5=>write!(sink,"{v:+08?}"),6=>write!(sink,"{v:08x?}"),7=>write!(sink,"{v:*>12?}"),8=>write!(sink,"{v:*<12?}"),9=>write!(sink,"{v:*^13?}"),10=>write!(sink,"{v:.2?}"),11=>write!(sink,"{v:+#08x?}"),12=>write!(sink,"{v:-08?}"),13=>write!(sink,"{v:🦀^12?}"),14=>write!(sink,"{v:\n>8?}"),15=>write!(sink,"{v:\0>8?}"),_=>panic!("style")}}
fn run(value:&dyn fmt::Debug,style:usize,chars:bool,fail:usize)->(String,usize){
 let mut sink=Sink{calls:vec![],text:String::new(),fail,chars};let result=RefCell::new(None);let outer=render(&mut sink,&Observed{value,result:&result},style);
 let(first,second,fill,align)=result.into_inner().unwrap();
 (format!("result {} {first} {second} {fill} {align}\ntext {}\ncalls {}\n",outer.is_err(),hex(sink.text),sink.calls.join(",")),sink.calls.len())
}
fn main(){let args:Vec<_>=std::env::args().collect();rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{
 let values=values();if args[1]=="--count"{println!("{}",values.len());return}
 let n:usize=args[1].parse().unwrap();let only=args.get(2).map(|s|s.parse::<usize>().unwrap());for style in 0..16{for chars in [false,true]{if only.is_some_and(|v|v!=style+usize::from(chars)*16){continue}let (_,calls)=run(values[n].as_ref(),style,chars,usize::MAX);for fail in std::iter::once(usize::MAX).chain(0..calls){let mode=style+usize::from(chars)*16+if fail==usize::MAX{0}else{(fail+1)*32};let(output,_)=run(values[n].as_ref(),style,chars,fail);println!("{n}\t{mode}\t{}",hex(output));}}}
 });
}
