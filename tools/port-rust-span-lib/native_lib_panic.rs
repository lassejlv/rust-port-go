#![feature(panic_can_unwind,panic_internals,step_trait)]
#![allow(internal_features)]
extern crate rustc_span;
extern crate rustc_serialize;
extern crate rustc_hashes;
extern crate rustc_data_structures;
use rustc_span::*;
use rustc_data_structures::sync::FreezeLock;
use rustc_hashes::Hash64;
use std::{io::{self,Read},sync::{Arc,Mutex}};
thread_local!{static HOOK_FILE:std::cell::Cell<*const SourceFile>=const{std::cell::Cell::new(std::ptr::null())};static REENTRY:std::cell::RefCell<Vec<String>>=const{std::cell::RefCell::new(Vec::new())};}
struct ClearHookFile;impl Drop for ClearHookFile{fn drop(&mut self){HOOK_FILE.with(|p|p.set(std::ptr::null()));}}
struct ShortRaw<'a>(rustc_serialize::opaque::MemDecoder<'a>);
macro_rules! reads{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self)->$ty{self.0.$name()})*}}
impl rustc_serialize::Decoder for ShortRaw<'_>{reads!(read_usize:usize,read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);fn read_raw_bytes(&mut self,n:usize)->&[u8]{let value=self.0.read_raw_bytes(n);if n==1{&value[..0]}else{value}}fn peek_byte(&self)->u8{self.0.peek_byte()}fn position(&self)->usize{self.0.position()}}
impl BlobDecoder for ShortRaw<'_>{fn decode_symbol(&mut self)->Symbol{self.0.decode_symbol()}fn decode_byte_symbol(&mut self)->ByteSymbol{self.0.decode_byte_symbol()}fn decode_def_index(&mut self)->rustc_span::def_id::DefIndex{self.0.decode_def_index()}fn decode_local_def_id(&mut self)->rustc_span::def_id::LocalDefId{self.0.decode_local_def_id()}}
impl SpanDecoder for ShortRaw<'_>{fn decode_span(&mut self)->Span{self.0.decode_span()}fn decode_expn_id(&mut self)->ExpnId{self.0.decode_expn_id()}fn decode_syntax_context(&mut self)->SyntaxContext{self.0.decode_syntax_context()}fn decode_crate_num(&mut self)->rustc_span::def_id::CrateNum{self.0.decode_crate_num()}fn decode_def_id(&mut self)->rustc_span::def_id::DefId{self.0.decode_def_id()}fn decode_attr_id(&mut self)->AttrId{self.0.decode_attr_id()}}
mod source_helpers { include!("native_source_helpers.rs"); }
fn normalized_file(file:&str)->String{if let Some((_,rest))=file.split_once("/original/"){return format!("original/{rest}")};if let Some((_,rest))=file.split_once("/registry/src/"){if let Some((_,rest))=rest.split_once('/') {return format!("registry/{rest}")}};file.to_owned()}
fn hex(text:&str)->String{text.as_bytes().iter().map(|v|format!("{v:02x}")).collect()}
fn sourcefile()->SourceFile{SourceFile::new(FileName::Anon(Hash64::ZERO),"hi".into(),SourceFileHashAlgorithm::Md5,None).unwrap()}
struct Overrun;impl Read for Overrun{fn read(&mut self,b:&mut[u8])->io::Result<usize>{Ok(b.len()+1)}}
fn action(op:&str){match op{
 "decode_remap_bits"|"decode_filename_tag"|"decode_hashalgorithm_tag"|"decode_hash_len"|"decode_option_tag"=>{use rustc_serialize::Decodable;let mut bytes=match op{"decode_remap_bits"=>vec![4],"decode_hash_len"=>vec![0,31],"decode_option_tag"=>vec![2],_=>vec![255]};bytes.extend_from_slice(rustc_serialize::opaque::MAGIC_END_BYTES);let mut d=rustc_serialize::opaque::MemDecoder::new(&bytes,0).unwrap();match op{"decode_remap_bits"=>{let _=RemapPathScopeComponents::decode(&mut d);},"decode_filename_tag"=>{let _=FileName::decode(&mut d);},"decode_hashalgorithm_tag"=>{let _=SourceFileHashAlgorithm::decode(&mut d);},"decode_hash_len"=>{let _=SourceFileHash::decode(&mut d);},_=>{let _=Option::<u8>::decode(&mut d);}}},
 "encode_non_utf8_path"=>{#[cfg(unix)]{use std::os::unix::ffi::OsStrExt;use rustc_serialize::Encodable;let path=std::path::Path::new(std::ffi::OsStr::from_bytes(b"\xff"));let value=RealFileName::from_virtual_path(path);value.encode(&mut rustc_serialize::opaque::mem_encoder::MemEncoder::new());}},
 "original_relative_add"=>{let mut sf=sourcefile();sf.normalized_pos=vec![NormalizedPos{pos:RelativeBytePos(0),diff:u32::MAX}];std::hint::black_box(sf.original_relative_byte_pos(BytePos(1)));},
 "normalized_cmp_add"=>{let mut sf=sourcefile();sf.normalized_pos=vec![NormalizedPos{pos:RelativeBytePos(u32::MAX),diff:1}];std::hint::black_box(sf.normalized_byte_pos(0));},
 "normalized_return_add"=>{let mut sf=sourcefile();sf.start_pos=BytePos(u32::MAX);std::hint::black_box(sf.normalized_byte_pos(1));},
 "multibyte_zero"|"multibyte_end_add"=>{let mut sf=sourcefile();sf.multibyte_chars=vec![MultiByteChar{pos:RelativeBytePos(if op=="multibyte_zero"{0}else{u32::MAX-1}),bytes:if op=="multibyte_zero"{0}else{4}}];std::hint::black_box(sf.lookup_file_pos_with_col_display(BytePos(if op=="multibyte_zero"{1}else{u32::MAX})));},
 "split_debug"|"split_add"=>create_default_session_globals_then(||{let span=if op=="split_debug"{Span::with_root_ctxt(BytePos(1),BytePos(3))}else{Span::with_root_ctxt(BytePos(u32::MAX-1),BytePos(u32::MAX))};std::hint::black_box(span.split_at(3));}),
 "diff_short_raw"=>{use rustc_serialize::{Encodable,Decodable};let sf=SourceFile::new(FileName::Anon(Hash64::ZERO),"a\nb".to_owned(),SourceFileHashAlgorithm::Md5,None).unwrap();let mut e=rustc_serialize::opaque::mem_encoder::MemEncoder::new();sf.encode(&mut e);let mut bytes=e.finish();bytes.extend_from_slice(rustc_serialize::opaque::MAGIC_END_BYTES);let sf=SourceFile::decode(&mut ShortRaw(rustc_serialize::opaque::MemDecoder::new(&bytes,0).unwrap()));std::hint::black_box(sf.lines());},
 "encode_diffs"|"encode_first_nonzero"=>{use rustc_serialize::Encodable;let mut sf=source_helpers::source_with_encoded_line_diffs(2,1,&[1]);if op=="encode_first_nonzero"{sf.lines=FreezeLock::frozen(SourceFileLines::Lines(vec![RelativeBytePos(1)]));}HOOK_FILE.with(|p|p.set(&sf));let _clear=ClearHookFile;sf.encode(&mut rustc_serialize::opaque::mem_encoder::MemEncoder::new());},
 "mem_expn"|"mem_ctxt"|"mem_index"|"guaranteed_encode"=>{use rustc_serialize::Encodable;let mut e=rustc_serialize::opaque::mem_encoder::MemEncoder::new();match op{"mem_expn"=>e.encode_expn_id(ExpnId::root()),"mem_ctxt"=>e.encode_syntax_context(SyntaxContext::root()),"mem_index"=>e.encode_def_index(rustc_span::def_id::DefIndex::from_u32(0)),_=>{#[allow(deprecated)]ErrorGuaranteed::unchecked_error_guaranteed().encode(&mut e);}}},
 "file_expn"|"file_ctxt"|"file_index"=>{let path=std::path::Path::new(&std::env::args().nth(1).unwrap()).join("panic-codec");let mut e=rustc_serialize::opaque::FileEncoder::new(&path).unwrap();match op{"file_expn"=>e.encode_expn_id(ExpnId::root()),"file_ctxt"=>e.encode_syntax_context(SyntaxContext::root()),_=>e.encode_def_index(rustc_span::def_id::DefIndex::from_u32(0))}},
 "decode_expn"|"decode_ctxt"|"decode_index"|"decode_attr"|"guaranteed_decode"=>{use rustc_serialize::Decodable;let mut d=rustc_serialize::opaque::MemDecoder::new(rustc_serialize::opaque::MAGIC_END_BYTES,0).unwrap();match op{"decode_expn"=>{d.decode_expn_id();},"decode_ctxt"=>{d.decode_syntax_context();},"decode_index"=>{d.decode_def_index();},"decode_attr"=>{d.decode_attr_id();},_=>{let _=ErrorGuaranteed::decode(&mut d);}}},
 "path_zero"=>{RealFileName::empty().path(RemapPathScopeComponents::empty());},
 "path_multi"=>{RealFileName::empty().path(RemapPathScopeComponents::MACRO|RemapPathScopeComponents::DIAGNOSTICS);},
 "embeddable_zero"=>{RealFileName::empty().embeddable_name(RemapPathScopeComponents::empty());},
 "embeddable_multi"=>{RealFileName::empty().embeddable_name(RemapPathScopeComponents::MACRO|RemapPathScopeComponents::DIAGNOSTICS);},
 "def_path_default"=>{let _=rustc_span::SpanEncoder::def_path_hasher(&rustc_serialize::opaque::mem_encoder::MemEncoder::new());},
 "byte_add"=>{std::hint::black_box(BytePos(u32::MAX)+BytePos(1));},
 "byte_sub"=>{std::hint::black_box(BytePos(0)-BytePos(1));},
 "relative_add"=>{std::hint::black_box(RelativeBytePos(u32::MAX)+RelativeBytePos(1));},
 "relative_sub"=>{std::hint::black_box(RelativeBytePos(0)-RelativeBytePos(1));},
 "char_add"=>{std::hint::black_box(CharPos(usize::MAX)+CharPos(1));},
 "char_sub"=>{std::hint::black_box(CharPos(0)-CharPos(1));},
 "attr_u32"=>{std::hint::black_box(AttrId::from_u32(u32::MAX));},
 "attr_usize"=>{std::hint::black_box(AttrId::from_usize(usize::MAX));},
 "attr_add"=>{std::hint::black_box(AttrId::MAX+1);},
 "attr_step"=>{std::hint::black_box(<AttrId as std::iter::Step>::forward_checked(AttrId::MAX,1));},
 "attr_forward_none"=>{std::hint::black_box(<AttrId as std::iter::Step>::forward(AttrId::from_u32(1),usize::MAX));},
 "attr_backward_none"=>{std::hint::black_box(<AttrId as std::iter::Step>::backward(AttrId::from_u32(0),1));},
 "session_absent"=>with_session_globals(|_|()),
 "session_present"=>create_default_session_globals_then(||create_default_session_globals_then(||())),
 "line_index"=>{std::hint::black_box(sourcefile().line_bounds(usize::MAX));},
 "line_begin_missing"=>{let mut sf=sourcefile();sf.lines=FreezeLock::frozen(SourceFileLines::Lines(vec![]));std::hint::black_box(sf.line_begin_pos(BytePos(0)));},
 "bytepos_middle"|"bytepos_total_extra"=>{let mut sf=sourcefile();sf.multibyte_chars=if op=="bytepos_middle"{vec![MultiByteChar{pos:RelativeBytePos(0),bytes:2}]}else{vec![MultiByteChar{pos:RelativeBytePos(0),bytes:2};3]};std::hint::black_box(sf.lookup_file_pos_with_col_display(BytePos(if op=="bytepos_middle"{1}else{2})));},
 "line_local_boundary"|"line_local_oob"|"line_external_boundary"|"line_external_oob"=>{let mut sf=sourcefile();let text=Arc::new("é".to_owned());if op.starts_with("line_local"){sf.src=Some(text);}else{sf.src=None;sf.external_src=FreezeLock::frozen(ExternalSource::Foreign{kind:ExternalSourceKind::Present(text),metadata_index:0});}sf.lines=FreezeLock::frozen(SourceFileLines::Lines(vec![RelativeBytePos(if op.ends_with("oob"){3}else{1})]));std::hint::black_box(sf.get_line(0));},
 "absolute_overflow"=>{let mut sf=sourcefile();sf.start_pos=BytePos(u32::MAX);std::hint::black_box(sf.absolute_position(RelativeBytePos(1)));},
 "relative_underflow"=>{let mut sf=sourcefile();sf.start_pos=BytePos(2);std::hint::black_box(sf.relative_position(BytePos(1)));},
 "diff_zero"=>{let sf=source_helpers::source_with_encoded_line_diffs(1,0,&[]);std::hint::black_box(sf.lines());},
 "diff_width"=>{let sf=source_helpers::source_with_encoded_line_diffs(1,3,&[]);std::hint::black_box(sf.lines());},
 "external_state"=>{let mut sf=sourcefile();sf.external_src=FreezeLock::new(ExternalSource::Unneeded);std::hint::black_box(sf.add_external_src(||Some("hi".into())));},
 "md5_overrun"=>{let _=SourceFileHash::new(SourceFileHashAlgorithm::Md5,Overrun);},
 "sha1_overrun"=>{let _=SourceFileHash::new(SourceFileHashAlgorithm::Sha1,Overrun);},
 "sha256_overrun"=>{let _=SourceFileHash::new(SourceFileHashAlgorithm::Sha256,Overrun);},
 "blake3_overrun"=>{let _=SourceFileHash::new(SourceFileHashAlgorithm::Blake3,Overrun);},
 _=>panic!("unknown action")
}}
fn main(){for op in["decode_remap_bits","decode_filename_tag","decode_hashalgorithm_tag","decode_hash_len","decode_option_tag","encode_non_utf8_path","original_relative_add","normalized_cmp_add","normalized_return_add","multibyte_zero","multibyte_end_add","split_debug","split_add","diff_short_raw","encode_diffs","encode_first_nonzero","mem_expn","mem_ctxt","mem_index","guaranteed_encode","file_expn","file_ctxt","file_index","decode_expn","decode_ctxt","decode_index","decode_attr","guaranteed_decode","path_zero","path_multi","embeddable_zero","embeddable_multi","def_path_default","byte_add","byte_sub","relative_add","relative_sub","char_add","char_sub","attr_u32","attr_usize","attr_add","attr_step","attr_forward_none","attr_backward_none","session_absent","session_present","line_index","line_begin_missing","bytepos_middle","bytepos_total_extra","line_local_boundary","line_local_oob","line_external_boundary","line_external_oob","absolute_overflow","relative_underflow","diff_zero","diff_width","external_state","md5_overrun","sha1_overrun","sha256_overrun","blake3_overrun"]{
 let log:Arc<Mutex<Vec<String>>>=Default::default();let hooks=log.clone();REENTRY.with(|r|r.borrow_mut().clear());std::panic::set_hook(Box::new(move|info|{let(kind,payload)=if let Some(value)=info.payload().downcast_ref::<&str>(){("static_str",*value)}else if let Some(value)=info.payload().downcast_ref::<String>(){("owned_string",value.as_str())}else{("opaque","")};let loc=info.location().unwrap();let file=normalized_file(loc.file());hooks.lock().unwrap().push(format!("{kind}:{}:{}:{}:{}:{}:{}:{}",hex(payload),hex(&file),loc.line(),loc.column(),info.can_unwind(),info.force_no_backtrace(),hex(loc.file())));HOOK_FILE.with(|p|if let Some(sf)=unsafe{p.get().as_ref()}{let result=sf.lines.try_write().is_some();REENTRY.with(|r|r.borrow_mut().push(result.to_string()));});}));let panicked=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||action(op))).is_err();println!("{op}\t{panicked}\t{}\t{}",log.lock().unwrap().join(","),REENTRY.with(|r|r.borrow().join(",")));
}}
