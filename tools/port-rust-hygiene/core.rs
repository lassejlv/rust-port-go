extern crate rustc_span;
extern crate rustc_data_structures;
extern crate rustc_hashes;
use rustc_span::{Span,BytePos,DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher,RawSpan,RawDefId,StableHashControls};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use std::hash::Hasher;
struct Hcx;
impl StableHashCtxt for Hcx {
 fn stable_hash_span(&mut self,span:RawSpan,h:&mut StableHasher){h.write_u64(span.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Fingerprint::new(Hash64::new(id.0 as u64+100),Hash64::new(id.1 as u64+1000))}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
fn main(){let args:Vec<_>=std::env::args().collect();let n:usize=args[1].parse().unwrap();let mode:usize=args[2].parse().unwrap();
 rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{
 let mut ctxt=SyntaxContext::root();let mut parent=ExpnId::root();
 for i in 0..n {
  let call_ctxt=if mode&1!=0 {ctxt}else{SyntaxContext::root()};
  let call=Span::new(BytePos(i as u32*2+1),BytePos(i as u32*2+2),call_ctxt,None);
  let kind=ExpnKind::Macro(match (i+mode)%3{0=>MacroKind::Bang,1=>MacroKind::Attr,_=>MacroKind::Derive},Symbol::intern(if i%2==0{"macro🦀"}else{"macroé"}));
  let mut data=ExpnData::default(kind,call,match i%5{0=>Edition::Edition2015,1=>Edition::Edition2018,2=>Edition::Edition2021,3=>Edition::Edition2024,_=>Edition::EditionFuture},None,None);
  data.parent=parent;data.local_inner_macros=i%2==0;data.diagnostic_opaque=i%3==0;
  if i%3==0 {data.allow_internal_unstable=Some(std::sync::Arc::from([Symbol::intern("feature")]));}
  let id=if mode&2==0 {LocalExpnId::fresh(data,Hcx)}else{let id=LocalExpnId::fresh_empty();id.set_expn_data(data,Hcx);id};
  let hash=id.to_expn_id().expn_hash();
  println!("expn {} {} {}",id.as_u32(),hash.stable_crate_id().as_u64(),hash.local_hash().as_u64());
  let transparency=match (i+mode)%3{0=>Transparency::Transparent,1=>Transparency::SemiOpaque,_=>Transparency::Opaque};
  ctxt=ctxt.apply_mark(id.to_expn_id(),transparency);
  println!("ctxt {} {} {} {:?} {:?}",ctxt.as_u32(),ctxt.normalize_to_macros_2_0().as_u32(),ctxt.normalize_to_macro_rules().as_u32(),ctxt.edition(),ctxt.marks());
  if i%2==0 {update_dollar_crate_names(|c|{let _=c.edition();Symbol::intern(&format!("crate{}",c.as_u32()))});}
  parent=id.to_expn_id();
 }
 println!("--compact--\n{}",debug_hygiene_data(false));println!("--pretty--\n{}",debug_hygiene_data(true));
 let _=DUMMY_SP;
 });
}
