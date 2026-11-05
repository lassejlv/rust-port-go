// SPDX-License-Identifier: MIT OR Apache-2.0
// External client: all derives/defaults execute in the complete unchanged crate.
#![feature(rustc_private,panic_can_unwind,panic_internals)]
#![allow(internal_features,deprecated)]
extern crate rustc_span;
extern crate rustc_hashes;
extern crate rustc_serialize;
extern crate rustc_index;
use rustc_span::*;
use rustc_hashes::{Hash64,Hash128};
use rustc_serialize::{Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{cmp::Ordering,fmt::Debug,path::Path,sync::{Mutex,Arc}};
static HOOK:Mutex<String>=Mutex::new(String::new());
fn hex(value:&[u8])->String{value.iter().map(|v|format!("{v:02x}")).collect()}
fn order(value:Ordering)->i8{match value{Ordering::Less=>-1,Ordering::Equal=>0,Ordering::Greater=>1}}
fn describe<T:Debug>(value:&T)->String{hex(format!("{value:?}").as_bytes())}
fn set_hook(){std::panic::set_hook(Box::new(|p|{let location=p.location().unwrap();let kind=if p.payload().is::<&str>(){"static_str"}else if p.payload().is::<String>(){"owned_string"}else{"opaque"};*HOOK.lock().unwrap()=format!("{}:{}:{}:{}:{}:{}:{}",kind,hex(p.payload_as_str().unwrap_or("opaque").as_bytes()),hex(location.file().as_bytes()),location.line(),location.column(),p.can_unwind(),p.force_no_backtrace());}));}
fn run<T:Ord+Clone+Debug>(name:&str,values:&[T]){
 for(a_index,a)in values.iter().enumerate(){for(b_index,b)in values.iter().enumerate(){for(c_index,c)in values.iter().enumerate(){
  HOOK.lock().unwrap().clear();
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||a.clone().clamp(b.clone(),c.clone())));
  let clamp=result.as_ref().map(describe).unwrap_or_else(|_|"panic".to_owned());
  let partial=a.partial_cmp(b).map(order);
  println!("ord\t{name}\t{a_index}\t{b_index}\t{c_index}\t{} {} {} {:?} {} {} {} {} {} {} {}\t{}",a.eq(b),a.ne(b),order(a.cmp(b)),partial,a.lt(b),a.le(b),a.gt(b),a.ge(b),describe(&a.clone().min(b.clone())),describe(&a.clone().max(b.clone())),clamp,hex(HOOK.lock().unwrap().as_bytes()));
 }}}
}
fn stable(value:u128)->StableSourceFileId{let mut encoder=MemEncoder::new();Hash128::new(value).encode(&mut encoder);let mut bytes=encoder.finish();bytes.extend_from_slice(MAGIC_END_BYTES);StableSourceFileId::decode(&mut MemDecoder::new(&bytes,0).unwrap())}
fn real_values()->Vec<RealFileName>{let mut result=vec![RealFileName::empty()];for path in["","a","a//b","a/b","a/./b","../a","/a","é/β"]{result.push(RealFileName::from_virtual_path(Path::new(path)));}result}
fn file_values()->Vec<FileName>{vec![FileName::Real(RealFileName::empty()),FileName::Real(RealFileName::from_virtual_path(Path::new("a//b"))),FileName::Real(RealFileName::from_virtual_path(Path::new("a/b"))),FileName::CfgSpec(Hash64::ZERO),FileName::CfgSpec(Hash64::new(u64::MAX)),FileName::Anon(Hash64::ZERO),FileName::MacroExpansion(Hash64::new(7)),FileName::ProcMacroSourceCode(Hash64::new(3)),FileName::CliCrateAttr(Hash64::new(5)),FileName::Custom("a".to_owned()),FileName::Custom("β\0".to_owned()),FileName::DocTest(RealFileName::from_virtual_path(Path::new("a")),-1),FileName::InlineAsm(Hash64::new(9))]}
fn owner_ties(){
 for operation in["min","max","clamp"]{
  let a=FileName::Custom("same".to_owned());let b=a.clone();let c=a.clone();
  let ptr=|v:&FileName|match v{FileName::Custom(s)=>s.as_ptr(),_=>unreachable!()};let pointers=[ptr(&a),ptr(&b),ptr(&c)];
  let result=match operation{"min"=>a.min(b),"max"=>a.max(b),_=>a.clamp(b,c)};
  println!("owner\tcustom\t{operation}\t{}",pointers.iter().position(|p|*p==ptr(&result)).unwrap());
  let a=RealFileName::from_virtual_path(Path::new("same"));let b=a.clone();let c=a.clone();
  let ptr=|v:&RealFileName|v.path(RemapPathScopeComponents::MACRO).as_os_str().as_encoded_bytes().as_ptr();let pointers=[ptr(&a),ptr(&b),ptr(&c)];
  let result=match operation{"min"=>a.min(b),"max"=>a.max(b),_=>a.clamp(b,c)};
  println!("owner\treal\t{operation}\t{}",pointers.iter().position(|p|*p==ptr(&result)).unwrap());
 }
}
fn eq<T:Clone+PartialEq>(name:&str,values:&[T]){
 for(a_index,a)in values.iter().enumerate(){for(b_index,b)in values.iter().enumerate(){
  let mut copied=a.clone();copied.clone_from(b);
  println!("eq\t{name}\t{a_index}\t{b_index}\t{} {} {}",a.eq(b),a.ne(b),copied.eq(b));
 }}
}
fn equality(){
 let names=[FileName::Custom("a".to_owned()),FileName::Custom("b".to_owned())];
 let distinct=[DistinctSources{begin:(names[0].clone(),BytePos(1)),end:(names[1].clone(),BytePos(2))},DistinctSources{begin:(names[0].clone(),BytePos(1)),end:(names[1].clone(),BytePos(3))}];
 let malformed=[MalformedSourceMapPositions{name:names[0].clone(),source_len:3,begin_pos:BytePos(1),end_pos:BytePos(2)},MalformedSourceMapPositions{name:names[1].clone(),source_len:3,begin_pos:BytePos(1),end_pos:BytePos(2)}];
 eq("spandata",&[SpanData{lo:BytePos(0),hi:BytePos(1),ctxt:SyntaxContext::root(),parent:None},SpanData{lo:BytePos(0),hi:BytePos(2),ctxt:SyntaxContext::root(),parent:None}]);
 eq("multibyte",&[MultiByteChar{pos:RelativeBytePos(1),bytes:2},MultiByteChar{pos:RelativeBytePos(u32::MAX),bytes:255}]);
 eq("normalized",&[NormalizedPos{pos:RelativeBytePos(1),diff:2},NormalizedPos{pos:RelativeBytePos(u32::MAX),diff:u32::MAX}]);
 eq("external-kind",&[ExternalSourceKind::Present(Arc::new("a".to_owned())),ExternalSourceKind::AbsentOk,ExternalSourceKind::AbsentErr]);
 eq("external",&[ExternalSource::Unneeded,ExternalSource::Foreign{kind:ExternalSourceKind::Present(Arc::new("a".to_owned())),metadata_index:17},ExternalSource::Foreign{kind:ExternalSourceKind::AbsentErr,metadata_index:u32::MAX}]);
 eq("source-hash",&[SourceFileHash::new_in_memory(SourceFileHashAlgorithm::Md5,b"a"),SourceFileHash::new_in_memory(SourceFileHashAlgorithm::Md5,b"b"),SourceFileHash::new_in_memory(SourceFileHashAlgorithm::Sha256,b"a")]);
 eq("line-info",&[LineInfo{line_index:1,start_col:CharPos(2),end_col:CharPos(3)},LineInfo{line_index:usize::MAX,start_col:CharPos(2),end_col:CharPos(3)}]);
 eq("distinct",&distinct);eq("malformed",&malformed);
 eq("lines-error",&[SpanLinesError::DistinctSources(Box::new(distinct[0].clone())),SpanLinesError::DistinctSources(Box::new(distinct[1].clone()))]);
 eq("snippet-error",&[SpanSnippetError::IllFormedSpan(Span::with_root_ctxt(BytePos(1),BytePos(2))),SpanSnippetError::DistinctSources(Box::new(distinct[0].clone())),SpanSnippetError::MalformedForSourcemap(malformed[0].clone()),SpanSnippetError::SourceNotAvailable{filename:names[0].clone()}]);
 eq("inner",&[InnerSpan::new(0,1),InnerSpan::new(usize::MAX,1)]);
 eq("scope",&[RemapPathScopeComponents::empty(),RemapPathScopeComponents::MACRO,RemapPathScopeComponents::from_bits_retain(255)]);
}
fn main(){set_hook();
 run("algorithm",&[SourceFileHashAlgorithm::Md5,SourceFileHashAlgorithm::Sha1,SourceFileHashAlgorithm::Sha256,SourceFileHashAlgorithm::Blake3]);
 run("stable",&[stable(0),stable(1),stable(1<<64),stable(u128::MAX)]);
 run("byte",&[BytePos(0),BytePos(1),BytePos(42),BytePos(u32::MAX)]);
 run("relative",&[RelativeBytePos(0),RelativeBytePos(1),RelativeBytePos(42),RelativeBytePos(u32::MAX)]);
 run("char",&[CharPos(0),CharPos(1),CharPos(42),CharPos(usize::MAX)]);
 run("attr",&[AttrId::ZERO,AttrId::from_u32(1),AttrId::from_u32(42),AttrId::MAX]);
 run("scope",&[RemapPathScopeComponents::empty(),RemapPathScopeComponents::MACRO,RemapPathScopeComponents::from_bits_retain(4),RemapPathScopeComponents::all(),RemapPathScopeComponents::from_bits_retain(255)]);
 run("real",&real_values());run("filename",&file_values());
 run("guaranteed",&[ErrorGuaranteed::unchecked_error_guaranteed()]);owner_ties();equality();
}
