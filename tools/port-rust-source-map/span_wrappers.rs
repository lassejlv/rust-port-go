#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Full original Span wrappers + SourceMap + local/foreign hygiene graphs.
use std::{hash::Hasher,cell::RefCell,io::{self,BufRead},path::{Path,PathBuf},sync::{Arc,atomic::{AtomicUsize,Ordering}},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{BytePos,Span,Symbol,DUMMY_SP,SPAN_TRACK,FileName,RealFileName,SourceFile,SourceFileHashAlgorithm,with_metavar_spans};
use rustc_span::def_id::{DefIndex,LocalDefId,CrateNum};
use rustc_span::edition::Edition;
use rustc_span::hygiene::*;
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::stable_hash::{RawDefId,RawSpan,StableHashCtxt,StableHashControls,StableHasher};
use rustc_hashes::Hash64;
use rustc_serialize::{Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
thread_local!{static TRACK:RefCell<Vec<u32>>=const{RefCell::new(Vec::new())};}
fn track(id:LocalDefId){TRACK.with(|v|v.borrow_mut().push(id.local_def_index.as_u32()))}
static TRACK_FN:fn(LocalDefId)=track;
static READS:AtomicUsize=AtomicUsize::new(0);
const EXTERNAL:&str="external λ file\n";
struct Loader{present:bool}
impl FileLoader for Loader{
 fn file_exists(&self,_:&Path)->bool{self.present}
 fn read_file(&self,_:&Path)->io::Result<String>{READS.fetch_add(1,Ordering::SeqCst);if self.present{Ok(EXTERNAL.into())}else{Err(io::Error::from(io::ErrorKind::NotFound))}}
 fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(EXTERNAL.as_bytes()))}
 fn current_directory(&self)->io::Result<PathBuf>{Ok(PathBuf::from("/proof"))}
}
struct Hcx;
impl StableHashCtxt for Hcx{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){h.write_u64(s.0)}
 fn def_path_hash(&self,d:RawDefId)->Fingerprint{Fingerprint::new(u64::from(d.0)+100,u64::from(d.1)+1000)}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
fn hash(i:usize)->ExpnHash{let mut e=MemEncoder::new();Fingerprint::new(Hash64::new(111),Hash64::new(1000+i as u64)).encode(&mut e);let mut b=e.finish();b.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&b,0).unwrap())}
fn repr(s:Span)->String{let d=s.data_untracked();format!("{},{},{},{}",d.lo.0,d.hi.0,d.ctxt.as_u32(),d.parent.map(|p|p.local_def_index.as_u32().to_string()).unwrap_or("_".into()))}
fn option(s:Option<Span>)->String{s.map(repr).unwrap_or("_".into())}
fn emit(out:&mut Vec<String>,tag:String,f:impl FnOnce()->String){TRACK.with(|v|v.borrow_mut().clear());let result=catch_unwind(AssertUnwindSafe(f)).unwrap_or("panic".into());let tracks=TRACK.with(|v|v.borrow().iter().map(u32::to_string).collect::<Vec<_>>().join(","));out.push(format!("{tag}={result}@{tracks}#{}",READS.load(Ordering::SeqCst)));}
fn request(depth:usize,mode:usize,metas:usize,edition:usize)->String{
 READS.store(0,Ordering::SeqCst);
 let sm=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader{present:mode&8!=0}),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});
 let local=sm.new_source_file(FileName::Custom("local.rs".into()),"local text\n".repeat(32));
 let template=SourceFile::new(FileName::Real(RealFileName::from_virtual_path(Path::new("foreign.rs"))),EXTERNAL.into(),SourceFileHashAlgorithm::Md5,None).unwrap();
 let imported=sm.new_imported_source_file(template.name,template.src_hash,template.checksum_hash,template.stable_id,template.normalized_source_len.0,template.unnormalized_source_len,CrateNum::from_u32(1),template.lines,template.multibyte_chars,template.normalized_pos,0);
 let foreign=imported.start_pos.0;
 let mut ids=vec![ExpnId::root()];let mut contexts=vec![SyntaxContext::root()];let mut current=SyntaxContext::root();
 for i in 0..depth{
  let kind=match(i+mode)%8{0=>ExpnKind::Macro(MacroKind::Bang,Symbol::intern("m")),1=>ExpnKind::Macro(MacroKind::Attr,Symbol::intern("a")),2=>ExpnKind::Macro(MacroKind::Derive,Symbol::intern("d")),3=>ExpnKind::Desugaring(DesugaringKind::Async),4=>ExpnKind::Desugaring(DesugaringKind::Await),5=>ExpnKind::Desugaring(DesugaringKind::FormatLiteral{source:true}),6=>ExpnKind::AstPass(AstPass::TestHarness),_=>ExpnKind::Root};
  let at=if mode&1!=0&&i%2==0{foreign+1}else{10+i as u32};
  let call=Span::new(BytePos(at),BytePos(at+1),if mode&2!=0{current}else{SyntaxContext::root()},Some(LocalDefId{local_def_index:DefIndex::from_u32(10+i as u32)}));
  let def=match(i+mode)%3{0=>DUMMY_SP,1=>Span::with_root_ctxt(BytePos(5),BytePos(6)),_=>Span::with_root_ctxt(BytePos(foreign+2),BytePos(foreign+3))};
  let foreign_id=mode&16!=0&&i%2==0;
  let parent=if ids[i].krate!=CrateNum::from_u32(if foreign_id{1}else{0}){ExpnId::root()}else{ids[i]};
  let data=ExpnData::new(kind,parent,call,def,None,[Edition::Edition2015,Edition::Edition2018,Edition::Edition2021,Edition::Edition2024,Edition::EditionFuture][edition%5],None,None,false,false,false,mode&4!=0&&i%2==0);
  let id=if foreign_id{register_expn_id(CrateNum::from_u32(1),ExpnIndex::from_u32(i as u32+1),data,hash(i))}else{LocalExpnId::fresh(data,Hcx).to_expn_id()};
  ids.push(id);current=current.apply_mark(id,[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][(i+mode)%3]);contexts.push(current);
 }
 let mut spans=vec![DUMMY_SP];
 for (i,&context)in contexts.iter().enumerate(){for(base,length)in [(20,2),(foreign+1,2),(foreign+9,1)]{spans.push(Span::new(BytePos(base),BytePos(base+length),context,Some(LocalDefId{local_def_index:DefIndex::from_u32(30+i as u32)})));}}
 let a=Span::new(BytePos(2),BytePos(3),SyntaxContext::root(),Some(LocalDefId{local_def_index:DefIndex::from_u32(80)}));
 let b=Span::new(BytePos(if mode&1!=0{foreign+1}else{30}),BytePos(if mode&1!=0{foreign+3}else{35}),current,Some(LocalDefId{local_def_index:DefIndex::from_u32(81)}));
 let meta_a=Span::new(BytePos(60),BytePos(64),if metas%3==2{SyntaxContext::root()}else{current},None);
 let meta_b=Span::new(BytePos(70),BytePos(74),if metas%3==0{current}else{SyntaxContext::root()},None);
 with_metavar_spans(|map|{if metas!=0{if metas!=2{assert!(map.insert(a,meta_a))}if metas!=1{assert!(map.insert(b,meta_b))}}if metas>=6{if metas&1!=0{map.get(a);}if metas&2!=0{map.get(b);}let _=map.freeze_and_get_read_spans();}});
 spans.push(a);spans.push(b);
 let mut out=vec![];
 for(i,&s)in spans.iter().enumerate(){
  emit(&mut out,format!("visible{i}"),||s.is_visible(&sm).to_string());
  emit(&mut out,format!("external{i}"),||s.in_external_macro(&sm).to_string());
  emit(&mut out,format!("ancestor{i}"),||option(s.find_ancestor_not_from_extern_macro(&sm)));
  for(j,&within)in spans.iter().enumerate(){emit(&mut out,format!("within{i},{j}"),||option(s.within_macro(within,&sm)));}
 }
 with_metavar_spans(|map|{let frozen=map.freeze_and_get_read_spans();out.push(format!("read={},{};len{}",map.get(a).is_some(),map.get(b).is_some(),frozen.len()));});
 out.push(format!("owners={},{}",Arc::strong_count(&local),Arc::strong_count(&imported)));out.join("|")
}
fn main(){std::panic::set_hook(Box::new(|p|{if std::env::var_os("FERRUGO_SPAN_WRAPPER_PANICS").is_some(){eprintln!("{p}")}}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);let old=SPAN_TRACK.swap(&TRACK_FN);for line in io::stdin().lock().lines(){let line=line.unwrap();let p:Vec<usize>=line.split('\t').map(|s|s.parse().unwrap()).collect();if std::env::var_os("FERRUGO_SPAN_WRAPPER_PANICS").is_some(){eprintln!("REQUEST {line}");}let result=rustc_span::create_session_globals_then([Edition::Edition2015,Edition::Edition2018,Edition::Edition2021,Edition::Edition2024,Edition::EditionFuture][p[3]],&[],None,||request(p[0],p[1],p[2],p[3]));println!("{}",result);}SPAN_TRACK.swap(old);}
