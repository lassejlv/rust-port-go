#![feature(rustc_private)]
// Original decode helpers: actual MemDecoder, recursively reentered caches,
// full FnOnce capture cleanup, and state after caught callback/allocation panic.
use rustc_span::{DUMMY_SP};
use rustc_span::hygiene::*;
use rustc_span::edition::Edition;
use rustc_span::symbol::Symbol;
use rustc_span::def_id::CrateNum;
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_hashes::Hash64;
use rustc_serialize::{Encoder,Encodable,Decodable,Decoder};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};
use std::{cell::Cell,panic::{catch_unwind,AssertUnwindSafe}};
struct Token(&'static str,u32);
impl Token{fn touch(&self){}}
impl Drop for Token{fn drop(&mut self){println!("drop {} {} {}",self.0,self.1,std::thread::panicking())}}
struct State{mode:Cell<usize>,reentered:Cell<bool>}
fn hash(raw:u32)->ExpnHash{
 let mut e=MemEncoder::new();Fingerprint::new(Hash64::new(111),Hash64::new(raw as u64+1000)).encode(&mut e);let mut bytes=e.finish();bytes.extend_from_slice(MAGIC_END_BYTES);ExpnHash::decode(&mut MemDecoder::new(&bytes,0).unwrap())
}
fn expansion(raw:u32,state:&State)->ExpnId{
 let token=Token("expn",raw);
 decode_expn_id(CrateNum::from_u32(1),raw,move|id|{
  token.touch();println!("call expn {} {}",raw,std::thread::panicking());
  if state.mode.get()&1!=0&&raw==2{panic!("expn")}
  let mut data=ExpnData::default(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("decoded")),DUMMY_SP,Edition::Edition2024,None,None);
  data.parent=expansion(raw-1,state);
  assert_eq!(id.krate.as_u32(),1);
  (data,hash(raw))
 })
}
fn syntax(raw:u32,state:&State,cache:&HygieneDecodeContext)->SyntaxContext{
 let mut e=MemEncoder::new();e.emit_u32(raw);let mut bytes=e.finish();bytes.extend_from_slice(MAGIC_END_BYTES);
 let mut d=MemDecoder::new(&bytes,0).unwrap();let addr=&mut d as*mut _;let token=Token("ctxt",raw);
 let value=decode_syntax_context(&mut d,cache,move|d,id|{
  token.touch();println!("call ctxt {} {} {}",raw,std::ptr::eq(d,addr),std::thread::panicking());
  if state.mode.get()&1!=0&&raw==2{panic!("ctxt")}
  if state.mode.get()&4!=0&&!state.reentered.replace(true){let nested=syntax(raw,state,cache);println!("reentered {} {}",raw,nested.as_u32());}
  let parent=if state.mode.get()&2!=0&&raw==3{SyntaxContext::from_u32(999)}else{syntax(raw-1,state,cache)};
  assert_eq!(id.as_u32(),raw);
  (parent,ExpnId{krate:CrateNum::from_u32(1),local_id:ExpnIndex::from_u32(raw)},Transparency::Opaque)
 });
 println!("read {} {}",raw,d.position());value
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let a:Vec<usize>=std::env::args().skip(1).map(|s|s.parse().unwrap()).collect();let(n,mode)=(a[0]as u32,a[1]);
 rustc_span::create_session_globals_then(Edition::Edition2015,&[],None,||{
  let state=State{mode:Cell::new(mode),reentered:Cell::new(false)};let cache=HygieneDecodeContext::default();
  for pass in 0..2{
   for raw in [n,0,n,n.saturating_sub(1)]{
    let result=catch_unwind(AssertUnwindSafe(||expansion(raw,&state)));println!("expn result {pass} {raw} {} {}",result.is_err(),result.map(|id|id.local_id.as_u32()).unwrap_or(0));
    let result=catch_unwind(AssertUnwindSafe(||syntax(raw,&state,&cache)));println!("ctxt result {pass} {raw} {} {}",result.is_err(),result.map(|c|c.as_u32()).unwrap_or(0));
   }
   state.mode.set(0);
  }
  println!("--final--\n{}",debug_hygiene_data(false));
 });
}
