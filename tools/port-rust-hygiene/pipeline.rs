use super::*;
#[path="codec.rs"]mod client;
#[path="invalid_codec.rs"]mod invalid;
use rustc_serialize::Decoder;
use rustc_span::{BlobDecoder,SpanDecoder};
struct Observed(client::D<false>);
impl Drop for Observed{fn drop(&mut self){println!("decoder_state:{}:{}:{}:{}:{}",self.0.pos,self.0.events.join(","),std::thread::panicking(),runtime(),refs())}}
struct ContextFn(Env);
struct ExpansionFn(Env);
impl Drop for ContextFn{fn drop(&mut self){println!("encoder_owner:ctxt:{}:{}:{}",std::thread::panicking(),runtime(),refs())}}
impl Drop for ExpansionFn{fn drop(&mut self){println!("encoder_owner:expn:{}:{}:{}",std::thread::panicking(),runtime(),refs())}}
impl<'a,'b>FnOnce<(&'a mut (),SyntaxContext,&'b SyntaxContextKey)>for ContextFn{type Output=();extern "rust-call" fn call_once(mut self,a:(&'a mut (),SyntaxContext,&'b SyntaxContextKey)){self.call_mut(a)}}
impl<'a,'b>FnMut<(&'a mut (),SyntaxContext,&'b SyntaxContextKey)>for ContextFn{extern "rust-call" fn call_mut(&mut self,(_,c,key):(&'a mut (),SyntaxContext,&'b SyntaxContextKey)){self.0.call("encode_ctxt",&format!("{}/{}/{}",c.as_u32(),key.0.as_u32(),key.1.local_id.as_u32()))}}
impl<'a,'b>FnOnce<(&'a mut (),ExpnId,Option<&'b ExpnData>,ExpnHash)>for ExpansionFn{type Output=();extern "rust-call" fn call_once(mut self,a:(&'a mut (),ExpnId,Option<&'b ExpnData>,ExpnHash)){self.call_mut(a)}}
impl<'a,'b>FnMut<(&'a mut (),ExpnId,Option<&'b ExpnData>,ExpnHash)>for ExpansionFn{extern "rust-call" fn call_mut(&mut self,(_,id,d,h):(&'a mut (),ExpnId,Option<&'b ExpnData>,ExpnHash)){self.0.call("encode_expn",&format!("{}/{}/{}/{}/{}",id.krate.as_u32(),id.local_id.as_u32(),d.is_some(),h.local_hash().as_u64(),d.and_then(|d|d.allow_internal_unstable.as_ref()).map(Arc::strong_count).unwrap_or(0)))}}
fn decoder(raw:u32,fail:bool)->client::D<false>{client::D{atoms:vec![client::Atom::U32(raw)],pos:0,events:vec![],fail:if fail{0}else{usize::MAX}}}
struct BadRaw(client::D<false>);
impl Drop for BadRaw{fn drop(&mut self){println!("raw_state:{}:{}:{}",self.0.pos,self.0.events.join(","),std::thread::panicking())}}
macro_rules! forward_read{($($name:ident:$ty:ty),*)=>{$(fn $name(&mut self)->$ty{self.0.$name()})*}}
impl Decoder for BadRaw{
 forward_read!(read_usize:usize,read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);
 fn read_raw_bytes(&mut self,n:usize)->&[u8]{let at=self.0.pos;self.0.events.push(format!("raw:{n}"));self.0.pos+=1;let client::Atom::Raw(v)=&self.0.atoms[at]else{panic!("raw atom")};v}
 fn peek_byte(&self)->u8{self.0.peek_byte()}fn position(&self)->usize{self.0.position()}
}
impl BlobDecoder for BadRaw{
 fn decode_symbol(&mut self)->Symbol{self.0.decode_symbol()}
 fn decode_byte_symbol(&mut self)->rustc_span::symbol::ByteSymbol{self.0.decode_byte_symbol()}
 fn decode_def_index(&mut self)->rustc_span::def_id::DefIndex{self.0.decode_def_index()}
 fn decode_local_def_id(&mut self)->rustc_span::def_id::LocalDefId{self.0.decode_local_def_id()}
}
impl SpanDecoder for BadRaw{
 fn decode_span(&mut self)->Span{self.0.decode_span()}
 fn decode_expn_id(&mut self)->ExpnId{self.0.decode_expn_id()}
 fn decode_syntax_context(&mut self)->SyntaxContext{self.0.decode_syntax_context()}
 fn decode_def_id(&mut self)->rustc_span::def_id::DefId{self.0.decode_def_id()}
 fn decode_crate_num(&mut self)->CrateNum{self.0.decode_crate_num()}
 fn decode_attr_id(&mut self)->rustc_span::AttrId{self.0.decode_attr_id()}
}
pub fn build(operation:&str,core:Rc<Core>)->Box<dyn FnOnce()>{
 if let Some(number)=operation.strip_prefix("raw_hash_"){let n:usize=number.parse().unwrap();return Box::new(move||{let mut d=BadRaw(client::D{atoms:vec![client::Atom::Raw(vec![0;n])],pos:0,events:vec![],fail:usize::MAX});let _=ExpnHash::decode(&mut d);println!("returned:raw");});}
 if let Some(number)=operation.strip_prefix("codec_"){let n:usize=number.parse().unwrap();return Box::new(move||invalid::observe(n%32,n>=32));}
 if operation.starts_with("syntax_"){
  let cache=HygieneDecodeContext::default();let raw=if operation=="syntax_root_owned"{0}else if operation=="syntax_bound_owned"{0x2000_0000}else{17};let parent=SyntaxContext::from_u32(if operation=="syntax_parent_owned"{999}else{0});let expn=ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(17)};
  if operation=="syntax_cached_owned"{let mut d=decoder(raw,false);let _=decode_syntax_context(&mut d,&cache,|_,_|(SyntaxContext::root(),expn,Transparency::Opaque));}
  let reader_fail=operation=="syntax_reader_owned";let env=Env::new(core);let mut d=Observed(decoder(raw,reader_fail));
  return Box::new(move||{let c=decode_syntax_context(&mut d.0,&cache,move|d,c|{env.call("syntax_decode",&format!("{}/{}",c.as_u32(),d.position()));(parent,expn,Transparency::Opaque)});println!("returned:syntax:{}:{}",c.as_u32(),d.0.position());})
 }
 let id=LocalExpnId::fresh(data(),Quiet).to_expn_id();let ctxt=SyntaxContext::root().apply_mark(id,Transparency::Opaque);let cell=RefCell::new(HygieneEncodeContext::default());
 {let mut q=cell.borrow_mut();match operation{"encode_empty_owned"=>{},"encode_bad_context_owned"=>q.enqueue_syntax_context_for_encoding(SyntaxContext::from_u32(17)),"encode_bad_expn_owned"=>q.schedule_expn_data_for_encoding(LocalExpnId::from_u32(17).to_expn_id()),"encode_empty_expn_owned"=>q.schedule_expn_data_for_encoding(LocalExpnId::fresh_empty().to_expn_id()),"encode_foreign_missing_owned"=>q.schedule_expn_data_for_encoding(ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(17)}),_=>{q.enqueue_syntax_context_for_encoding(ctxt);q.schedule_expn_data_for_encoding(id);}}}
 let mut c=ContextFn(Env::new(core.clone()));let mut e=ExpansionFn(Env::new(core));let borrow=operation=="encode_borrowed";
 let locked=operation=="encode_locked_owned";let shared=operation=="encode_shared_owned";
 Box::new(move||{let _locked=if locked{Some(cell.borrow_mut())}else{None};let _shared=if shared{Some(cell.borrow())}else{None};if borrow{HygieneEncodeContext::encode(&cell,&mut (),&mut c,&mut e);println!("returned:encode_borrowed");drop(e);drop(c)}else{HygieneEncodeContext::encode(&cell,&mut (),c,e);println!("returned:encode_owned")}})
}
