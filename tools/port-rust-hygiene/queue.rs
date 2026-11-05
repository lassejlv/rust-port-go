#![feature(rustc_private)]
// Observer of unchanged HygieneEncodeContext, including owned/borrowed FnMut.
use rustc_span::{Span,BytePos,DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_span::def_id::CrateNum;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher,RawSpan,RawDefId,StableHashControls};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use rustc_serialize::{Encodable,Decodable};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{hash::Hasher,cell::RefCell,panic::{catch_unwind,AssertUnwindSafe}};
struct Hcx;
impl StableHashCtxt for Hcx {
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Fingerprint::new(Hash64::new(id.0 as u64+100),Hash64::new(id.1 as u64+1000))}
 fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
 fn assert_default_stable_hash_controls(&self,_:&str){}
}
struct Token(&'static str);
impl Token{fn touch(&mut self){}}
impl Drop for Token{fn drop(&mut self){println!("drop {} {}",self.0,std::thread::panicking())}}
struct State{ctxt_calls:usize,expn_calls:usize,fail_ctxt:bool,fail_expn:bool}
fn round(cell:&RefCell<HygieneEncodeContext>,state:&mut State,borrow:bool){
 let mut ctoken=Token("ctxt");let mut etoken=Token("expn");
 let mut ctxt=move|s:&mut State,c:SyntaxContext,key:&SyntaxContextKey|{
  ctoken.touch();s.ctxt_calls+=1;
  println!("ctxt {} {} {} {} {:?}",s.ctxt_calls,c.as_u32(),key.0.as_u32(),key.1.local_id.as_u32(),key.2);
  let mut h=cell.borrow_mut();h.enqueue_syntax_context_for_encoding(key.0);h.schedule_expn_data_for_encoding(key.1);drop(h);
  if s.fail_ctxt&&s.ctxt_calls==2{panic!("ctxt")}
 };
 let mut expn=move|s:&mut State,id:ExpnId,data:Option<&ExpnData>,hash:ExpnHash|{
  etoken.touch();s.expn_calls+=1;
  println!("expn {} {} {} {} {} {}",s.expn_calls,id.krate.as_u32(),id.local_id.as_u32(),data.is_some(),hash.local_hash().as_u64(),data.and_then(|d|d.allow_internal_unstable.as_ref()).map(std::sync::Arc::strong_count).unwrap_or(0));
  if let Some(d)=data{let mut h=cell.borrow_mut();h.schedule_expn_data_for_encoding(d.parent);h.enqueue_syntax_context_for_encoding(d.call_site.ctxt());}
  if s.fail_expn&&s.expn_calls==1{panic!("expn")}
 };
 if borrow{
  let failed=catch_unwind(AssertUnwindSafe(||HygieneEncodeContext::encode(cell,state,&mut ctxt,&mut expn))).is_err();
  println!("caught {failed} {} {}",state.ctxt_calls,state.expn_calls);
  drop(expn);drop(ctxt);
 }else{
  let failed=catch_unwind(AssertUnwindSafe(||HygieneEncodeContext::encode(cell,state,ctxt,expn))).is_err();
  println!("caught {failed} {} {}",state.ctxt_calls,state.expn_calls);
 }
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let args:Vec<usize>=std::env::args().skip(1).map(|s|s.parse().unwrap()).collect();let(n,mode)=(args[0],args[1]);
 rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{
  let mut ids=vec![ExpnId::root()];let mut ctxts=vec![SyntaxContext::root()];
  for i in 0..n{
   let call=Span::new(BytePos(i as u32+1),BytePos(i as u32+2),ctxts[i],None);
   let mut data=ExpnData::default(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("queue")),call,Edition::Edition2024,None,None);data.parent=ids[i];
   if i%2==0{data.allow_internal_unstable=Some(std::sync::Arc::from([Symbol::intern("feature")]))}
   let id=LocalExpnId::fresh(data,Hcx).to_expn_id();ids.push(id);ctxts.push(ctxts[i].apply_mark(id,Transparency::Opaque));
  }
  let mut hash_encoder=MemEncoder::new();Fingerprint::new(Hash64::new(111),Hash64::new(777)).encode(&mut hash_encoder);let mut hash_bytes=hash_encoder.finish();hash_bytes.extend_from_slice(MAGIC_END_BYTES);let foreign_hash=ExpnHash::decode(&mut MemDecoder::new(&hash_bytes,0).unwrap());
  let foreign=register_expn_id(CrateNum::from_u32(1),ExpnIndex::from_u32(2),ExpnData::default(ExpnKind::Macro(MacroKind::Derive,Symbol::intern("foreign")),DUMMY_SP,Edition::Edition2021,None,None),foreign_hash);
  let cell=RefCell::new(HygieneEncodeContext::default());
  {let mut h=cell.borrow_mut();h.enqueue_syntax_context_for_encoding(ctxts[n]);h.enqueue_syntax_context_for_encoding(ctxts[n]);h.schedule_expn_data_for_encoding(ids[n]);h.schedule_expn_data_for_encoding(foreign);}
  let mut state=State{ctxt_calls:0,expn_calls:0,fail_ctxt:mode&1!=0,fail_expn:mode&2!=0};round(&cell,&mut state,mode&4!=0);
  state.fail_ctxt=false;state.fail_expn=false;round(&cell,&mut state,mode&4!=0);
  // Set membership remains after an interrupted mem::take round.
  {let mut h=cell.borrow_mut();for &c in &ctxts{h.enqueue_syntax_context_for_encoding(c)}for &id in &ids{h.schedule_expn_data_for_encoding(id)}}
  round(&cell,&mut state,false);
  println!("--final--\n{}",debug_hygiene_data(false));
 });
}
