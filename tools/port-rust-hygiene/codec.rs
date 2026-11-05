// Public codec implementations from the whole unchanged hygiene.rs, using
// genuine user Encoder/SpanEncoder and Decoder/SpanDecoder implementations.
use rustc_span::{Span,BytePos,DUMMY_SP,SpanEncoder,SpanDecoder,BlobDecoder,AttrId};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::{Symbol,ByteSymbol};
use rustc_span::def_id::{CrateNum,DefId,DefIndex,ModId,LocalDefId};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use rustc_serialize::{Encoder,Decoder,Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::panic::{catch_unwind,AssertUnwindSafe};
#[derive(Clone)]pub(crate) enum Atom{Usize(usize),U32(u32),U8(u8),Bool(bool),Raw(Vec<u8>),Span(Span),Symbol(Symbol),Expn(ExpnId),Ctxt(SyntaxContext),Def(DefId)}
impl Atom{fn debug(&self)->String{match self{Self::Usize(v)=>format!("usize:{v}"),Self::U32(v)=>format!("u32:{v}"),Self::U8(v)=>format!("u8:{v}"),Self::Bool(v)=>format!("bool:{v}"),Self::Raw(v)=>format!("raw:{}",hex(v)),Self::Span(v)=>format!("span:{}/{}/{}",v.lo().0,v.hi().0,v.ctxt().as_u32()),Self::Symbol(v)=>format!("symbol:{}",v.as_u32()),Self::Expn(v)=>format!("expn:{}/{}",v.krate.as_u32(),v.local_id.as_u32()),Self::Ctxt(v)=>format!("ctxt:{}",v.as_u32()),Self::Def(v)=>format!("def:{}/{}",v.krate.as_u32(),v.index.as_u32())}}}
struct E<const BOOL:bool>{atoms:Vec<Atom>,events:Vec<String>,fail:usize}
impl<const B:bool>E<B>{fn put(&mut self,a:Atom){let n=self.events.len();self.events.push(a.debug());if n==self.fail{panic!("encoder")};self.atoms.push(a)}}
macro_rules! unused_emit{($($name:ident:$t:ty),*)=>{$(fn $name(&mut self,_:$t){panic!(concat!("unused ",stringify!($name)))})*}}
macro_rules! encoder{()=>{
 unused_emit!(emit_u128:u128,emit_u64:u64,emit_u16:u16,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);
 fn emit_usize(&mut self,v:usize){self.put(Atom::Usize(v))}fn emit_u32(&mut self,v:u32){self.put(Atom::U32(v))}fn emit_u8(&mut self,v:u8){self.put(Atom::U8(v))}fn emit_raw_bytes(&mut self,v:&[u8]){self.put(Atom::Raw(v.to_vec()))}
}}
impl Encoder for E<false>{encoder!();}
impl Encoder for E<true>{encoder!();fn emit_bool(&mut self,v:bool){self.put(Atom::Bool(v))}}
impl<const B:bool>SpanEncoder for E<B>where Self:Encoder{
 fn encode_span(&mut self,v:Span){self.put(Atom::Span(v))}fn encode_symbol(&mut self,v:Symbol){self.put(Atom::Symbol(v))}fn encode_expn_id(&mut self,v:ExpnId){self.put(Atom::Expn(v))}fn encode_syntax_context(&mut self,v:SyntaxContext){self.put(Atom::Ctxt(v))}fn encode_def_id(&mut self,v:DefId){self.put(Atom::Def(v))}
 fn encode_byte_symbol(&mut self,_:ByteSymbol){panic!("unused byte symbol")}fn encode_crate_num(&mut self,_:CrateNum){panic!("unused crate")}fn encode_def_index(&mut self,_:DefIndex){panic!("unused def index")}
}
pub(crate) struct D<const BOOL:bool>{pub atoms:Vec<Atom>,pub pos:usize,pub events:Vec<String>,pub fail:usize}
impl<const B:bool>D<B>{fn take(&mut self,kind:&str)->Atom{let n=self.events.len();self.events.push(kind.into());if n==self.fail{panic!("decoder")};let a=self.atoms[self.pos].clone();self.pos+=1;a}}
macro_rules! unused_read{($($name:ident:$t:ty),*)=>{$(fn $name(&mut self)->$t{panic!(concat!("unused ",stringify!($name)))})*}}
macro_rules! decoder{()=>{
 unused_read!(read_u128:u128,read_u64:u64,read_u16:u16,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);
 fn read_usize(&mut self)->usize{let Atom::Usize(v)=self.take("usize")else{panic!("atom")};v}fn read_u32(&mut self)->u32{let Atom::U32(v)=self.take("u32")else{panic!("atom")};v}fn read_u8(&mut self)->u8{let Atom::U8(v)=self.take("u8")else{panic!("atom")};v}
 fn read_raw_bytes(&mut self,n:usize)->&[u8]{let at=self.pos;let Atom::Raw(_)=self.take(&format!("raw:{n}"))else{panic!("atom")};let Atom::Raw(v)=&self.atoms[at]else{unreachable!()};assert_eq!(v.len(),n);v}
 fn position(&self)->usize{self.pos}fn peek_byte(&self)->u8{let Atom::U8(v)=self.atoms[self.pos]else{panic!("atom")};v}
}}
impl Decoder for D<false>{decoder!();}
impl Decoder for D<true>{decoder!();fn read_bool(&mut self)->bool{let Atom::Bool(v)=self.take("bool")else{panic!("atom")};v}}
impl<const B:bool>BlobDecoder for D<B>where Self:Decoder{
 fn decode_symbol(&mut self)->Symbol{let Atom::Symbol(v)=self.take("symbol")else{panic!("atom")};v}
 fn decode_byte_symbol(&mut self)->ByteSymbol{panic!("unused byte symbol")}fn decode_def_index(&mut self)->DefIndex{panic!("unused index")}fn decode_local_def_id(&mut self)->LocalDefId{panic!("unused local")}
}
impl<const B:bool>SpanDecoder for D<B>where Self:Decoder{
 fn decode_span(&mut self)->Span{let Atom::Span(v)=self.take("span")else{panic!("atom")};v}fn decode_expn_id(&mut self)->ExpnId{let Atom::Expn(v)=self.take("expn")else{panic!("atom")};v}fn decode_syntax_context(&mut self)->SyntaxContext{let Atom::Ctxt(v)=self.take("ctxt")else{panic!("atom")};v}fn decode_def_id(&mut self)->DefId{let Atom::Def(v)=self.take("def")else{panic!("atom")};v}
 fn decode_crate_num(&mut self)->CrateNum{panic!("unused crate")}fn decode_attr_id(&mut self)->AttrId{panic!("unused attr")}
}
fn desugar(i:usize)->DesugaringKind{match i{0=>DesugaringKind::Async,1=>DesugaringKind::Await,2=>DesugaringKind::QuestionMark,3=>DesugaringKind::TryBlock,4=>DesugaringKind::YeetExpr,5=>DesugaringKind::OpaqueTy,6=>DesugaringKind::ForLoop,7=>DesugaringKind::WhileLoop,8=>DesugaringKind::BoundModifier,9=>DesugaringKind::Contract,10=>DesugaringKind::PatTyRange,11=>DesugaringKind::FormatLiteral{source:false},12=>DesugaringKind::FormatLiteral{source:true},13=>DesugaringKind::RangeExpr,_=>panic!("desugar")}}
fn kind(i:usize)->ExpnKind{match i{0=>ExpnKind::Root,1..=3=>ExpnKind::Macro([MacroKind::Bang,MacroKind::Attr,MacroKind::Derive][i-1],Symbol::intern("codec🦀")),4..=6=>ExpnKind::AstPass([AstPass::StdImports,AstPass::TestHarness,AstPass::ProcMacroHarness][i-4]),_=>ExpnKind::Desugaring(desugar(i-7))}}
fn hash(a:u64,b:u64)->ExpnHash{let mut e=MemEncoder::new();Fingerprint::new(Hash64::new(a),Hash64::new(b)).encode(&mut e);let mut data=e.finish();data.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&data,0).unwrap())}
fn encode<const B:bool>(n:usize,e:&mut E<B>)where E<B>:Encoder{
 match n{
  0..=2=>[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][n].encode(e),
  3..=5=>[MacroKind::Bang,MacroKind::Attr,MacroKind::Derive][n-3].encode(e),
  6..=8=>[AstPass::StdImports,AstPass::TestHarness,AstPass::ProcMacroHarness][n-6].encode(e),
  9..=22=>desugar(n-9).encode(e),23..=43=>kind(n-23).encode(e),
  44..=46=>{let(a,b)=[(0,0),(111,777),(u64::MAX,1<<63)][n-44];hash(a,b).encode(e)},
  47..=49=>DecodedSyntaxContext::from_u32([0,17,0x1fff_ffff][n-47]).encode(e),
  50..=52=>LocalExpnId::from_u32([0,17,0xffff_ff00][n-50]).encode(e),
  53..=60=>{
   let i=(n-53)as u32;let id=DefId{krate:CrateNum::from_u32(2),index:DefIndex::from_u32(99)};let sym=Symbol::intern("codec🦀");
   let features=if i%3==0{None}else{Some(std::sync::Arc::from(if i%3==1{vec![]}else{vec![sym,Symbol::intern("feature")]}))};
   ExpnData::new(kind(i as usize+1),ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(7)},Span::new(BytePos(i+10),BytePos(i+20),SyntaxContext::root(),None),DUMMY_SP,features,match i%5{0=>Edition::Edition2015,1=>Edition::Edition2018,2=>Edition::Edition2021,3=>Edition::Edition2024,_=>Edition::EditionFuture},if i%2==0{None}else{Some(id)},if i%2==0{Some(ModId::new_unchecked(id))}else{None},i&1!=0,i&2!=0,i&4!=0,i&3==0).encode(e)
  },
  61..=63=>(SyntaxContext::from_u32(17),ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(99)},[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][n-61]).encode(e),
  64..=66=>(ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(99)},[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][n-64]).encode(e),
  _=>panic!("case")
 }
}
pub(crate) fn decode<const B:bool>(n:usize,d:&mut D<B>)->String where D<B>:Decoder{
 match n{
 0..=2=>format!("{:?}",Transparency::decode(d)),3..=5=>format!("{:?}",MacroKind::decode(d)),6..=8=>format!("{:?}",AstPass::decode(d)),9..=22=>format!("{:?}",DesugaringKind::decode(d)),23..=43=>format!("{:?}",ExpnKind::decode(d)),44..=46=>format!("{:?}",ExpnHash::decode(d)),47..=49=>format!("{:?}",DecodedSyntaxContext::decode(d)),50..=52=>format!("{:?}",LocalExpnId::decode(d)),53..=60=>{let value=ExpnData::decode(d);format!("{value:?};refs={}",value.allow_internal_unstable.as_ref().map(std::sync::Arc::strong_count).unwrap_or(0))},61..=63=>format!("{:?}",<(SyntaxContext,ExpnId,Transparency)>::decode(d)),64..=66=>format!("{:?}",<(ExpnId,Transparency)>::decode(d)),_=>panic!("case")}
}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn run<const B:bool>(n:usize)where E<B>:Encoder,D<B>:Decoder{
 let mut success=E::<B>{atoms:vec![],events:vec![],fail:usize::MAX};encode(n,&mut success);
 for fail in std::iter::once(usize::MAX).chain(0..success.events.len()){
  let mut e=E::<B>{atoms:vec![],events:vec![],fail};let bad=catch_unwind(AssertUnwindSafe(||encode(n,&mut e))).is_err();
  let output=format!("encode {bad} {}\n",e.events.join(","));let mode=usize::from(B)+if fail==usize::MAX{0}else{(fail+1)*4};println!("{n}\t{mode}\t{}",hex(output));
  let mut d=D::<B>{atoms:success.atoms.clone(),pos:0,events:vec![],fail};let value=catch_unwind(AssertUnwindSafe(||decode(n,&mut d)));let output=format!("decode {} {} {}\nvalue {}\n",value.is_err(),d.pos,d.events.join(","),value.unwrap_or_default());println!("{n}\t{}\t{}",mode+2,hex(output));
 }
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{if args[1]=="--count"{println!("67");return}let n=args[1].parse().unwrap();run::<false>(n);run::<true>(n);});}

pub(crate) fn encoded<const B:bool>(n:usize)->Vec<Atom>where E<B>:Encoder{let mut e=E::<B>{atoms:vec![],events:vec![],fail:usize::MAX};encode(n,&mut e);e.atoms}
