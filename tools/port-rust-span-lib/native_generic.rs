// Custom non-Copy source witnesses exercise the unchanged aggregate derives
// and scoped session FnOnce functions. No original body is reproduced here.
#![feature(fn_traits,unboxed_closures)]
extern crate rustc_span;
extern crate rustc_serialize;
extern crate rustc_data_structures;
use rustc_data_structures::stable_hash::{StableHash,StableHashCtxt,StableHasher,StableHashControls,RawSpan,RawDefId};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_span::{Spanned,Span,BytePos,respan,dummy_spanned,SessionGlobals,edition::Edition};
use rustc_serialize::{Encoder,Decoder,Encodable,Decodable};
use rustc_serialize::opaque::{mem_encoder::MemEncoder,MemDecoder,MAGIC_END_BYTES};
use rustc_span::{SpanEncoder,SpanDecoder,BlobDecoder,Symbol,ByteSymbol,ExpnId,SyntaxContext,AttrId};
use rustc_span::def_id::{CrateNum,DefIndex,DefId,LocalDefId};
use std::{cell::{Cell,RefCell},rc::Rc,fmt,hash::{Hash,Hasher}};
type Log=Rc<RefCell<Vec<String>>>;
struct Node {id:u32,calls:Cell<u32>,log:Log,mode:Rc<Cell<u8>>}
impl Node {fn new(id:u32,log:&Log,mode:&Rc<Cell<u8>>)->Self{Self{id,calls:Cell::new(0),log:log.clone(),mode:mode.clone()}}fn event(&self,name:&str){let n=self.calls.get();self.calls.set(n+1);self.log.borrow_mut().push(format!("{name}:{}:{n}:{}",self.id,std::thread::panicking()))}}
impl Clone for Node {fn clone(&self)->Self{self.event("clone");if self.mode.get()==1{panic!("clone")};Self::new(self.id,&self.log,&self.mode)}fn clone_from(&mut self,_:&Self){self.event("unexpected_clone_from");panic!("unexpected node clone_from")}}
impl PartialEq for Node {fn eq(&self,other:&Self)->bool{self.event("eq");if self.mode.get()==2{panic!("eq")};self.id==other.id}fn ne(&self,_:&Self)->bool{self.event("unexpected_ne");panic!("unexpected node ne")}}
impl Hash for Node {fn hash<H:Hasher>(&self,h:&mut H){self.event("hash");if self.mode.get()==3{panic!("hash")};h.write_u32(self.id)}}
impl StableHash for Node {fn stable_hash<Hcx:StableHashCtxt>(&self,_:&mut Hcx,h:&mut StableHasher){self.event("stable");if self.mode.get()==5{panic!("stable")};h.write_u32(self.id)}}
struct Hcx(Log);impl StableHashCtxt for Hcx {fn stable_hash_span(&mut self,span:RawSpan,h:&mut StableHasher){self.0.borrow_mut().push(format!("span:{:016x}",span.0));h.write_u64(span.0)}fn def_path_hash(&self,_:RawDefId)->Fingerprint{panic!("unexpected def_path_hash")}fn stable_hash_controls(&self)->StableHashControls{panic!("unexpected controls")}fn assert_default_stable_hash_controls(&self,_:&str){panic!("unexpected assert controls")}}
impl fmt::Debug for Node {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.event("debug");if self.mode.get()==7{return Err(fmt::Error)};if self.mode.get()==8{panic!("debug")};self.id.fmt(f)}}
impl<S:rustc_span::SpanEncoder>Encodable<S>for Node{fn encode(&self,e:&mut S){self.event("encode");if self.mode.get()==4{panic!("encode")};e.emit_u32(self.id)}}
impl Drop for Node {fn drop(&mut self){self.event("drop");if self.mode.get()==6&&self.id==1{panic!("drop")}}}
thread_local!{static DECODE:RefCell<Option<(Log,Rc<Cell<u8>>)>>=const{RefCell::new(None)};}
impl<D:SpanDecoder>Decodable<D>for Node{fn decode(d:&mut D)->Self{DECODE.with(|settings|{let borrow=settings.borrow();let(log,mode)=borrow.as_ref().unwrap();log.borrow_mut().push(format!("decode:{}",std::thread::panicking()));if mode.get()==10{panic!("node decode")};Self::new(d.read_u32(),log,mode)})}}
struct E{data:MemEncoder,log:Log,mode:u8}
macro_rules! encode_primitives{($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self,v:$ty){self.data.$method(v);})*}}
impl Encoder for E{encode_primitives!(emit_usize:usize,emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_u8:u8,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);fn emit_raw_bytes(&mut self,b:&[u8]){self.data.emit_raw_bytes(b)}}
#[allow(unused_variables)]impl SpanEncoder for E{
 fn encode_span(&mut self,span:Span){self.log.borrow_mut().push("enc_span".into());if self.mode==9{panic!("span encode")};span.lo().encode(self);span.hi().encode(self)}
 fn encode_symbol(&mut self,v:Symbol){panic!("unexpected symbol")}fn encode_byte_symbol(&mut self,v:ByteSymbol){panic!("unexpected byte symbol")}fn encode_expn_id(&mut self,v:ExpnId){panic!("unexpected expn")}fn encode_syntax_context(&mut self,v:SyntaxContext){panic!("unexpected context")}fn encode_crate_num(&mut self,v:CrateNum){panic!("unexpected crate")}fn encode_def_index(&mut self,v:DefIndex){self.log.borrow_mut().push(format!("def_index:{}",v.as_u32()));if self.mode==12{panic!("parent encode")};self.emit_u32(v.as_u32())}fn encode_def_id(&mut self,v:DefId){panic!("unexpected definition")}
}
struct D<'a>{data:MemDecoder<'a>,log:Log,mode:u8}
struct Override(E);
macro_rules! wrap_primitives{($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self,v:$ty){self.0.$method(v);})*}}
impl Encoder for Override{wrap_primitives!(emit_usize:usize,emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_u8:u8,emit_isize:isize,emit_i128:i128,emit_i64:i64,emit_i32:i32,emit_i16:i16);fn emit_raw_bytes(&mut self,b:&[u8]){self.0.emit_raw_bytes(b)}}
impl SpanEncoder for Override{
 fn encode_span(&mut self,v:Span){self.0.encode_span(v)}fn encode_symbol(&mut self,v:Symbol){self.0.encode_symbol(v)}fn encode_byte_symbol(&mut self,v:ByteSymbol){self.0.encode_byte_symbol(v)}fn encode_expn_id(&mut self,v:ExpnId){self.0.encode_expn_id(v)}fn encode_syntax_context(&mut self,v:SyntaxContext){self.0.encode_syntax_context(v)}fn encode_crate_num(&mut self,v:CrateNum){self.0.encode_crate_num(v)}fn encode_def_index(&mut self,v:DefIndex){self.0.encode_def_index(v)}fn encode_def_id(&mut self,v:DefId){self.0.encode_def_id(v)}
 fn encode_def_key(&mut self,_:Option<DefIndex>,value:impl Encodable<Self>){self.0.log.borrow_mut().push(format!("override:{}",std::any::type_name_of_val(&value)));if self.0.mode==13{panic!("override")};drop(value)}
}
macro_rules! decode_primitives{($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self)->$ty{self.data.$method()})*}}
impl Decoder for D<'_>{decode_primitives!(read_usize:usize,read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i64:i64,read_i32:i32,read_i16:i16);fn read_raw_bytes(&mut self,n:usize)->&[u8]{self.data.read_raw_bytes(n)}fn peek_byte(&self)->u8{self.data.peek_byte()}fn position(&self)->usize{self.data.position()}}
impl BlobDecoder for D<'_>{fn decode_symbol(&mut self)->Symbol{panic!("unexpected symbol")}fn decode_byte_symbol(&mut self)->ByteSymbol{panic!("unexpected byte symbol")}fn decode_def_index(&mut self)->DefIndex{panic!("unexpected index")}fn decode_local_def_id(&mut self)->LocalDefId{panic!("unexpected local")}}
impl SpanDecoder for D<'_>{fn decode_span(&mut self)->Span{self.log.borrow_mut().push("dec_span".into());if self.mode==9{panic!("span decode")};Span::with_root_ctxt(BytePos(7),BytePos(9))}fn decode_expn_id(&mut self)->ExpnId{panic!("unexpected expn")}fn decode_syntax_context(&mut self)->SyntaxContext{panic!("unexpected context")}fn decode_crate_num(&mut self)->CrateNum{panic!("unexpected crate")}fn decode_def_id(&mut self)->DefId{panic!("unexpected definition")}fn decode_attr_id(&mut self)->AttrId{panic!("unexpected attr")}}
struct H(Log);impl Hasher for H{fn finish(&self)->u64{self.0.borrow_mut().push("finish".into());0}fn write(&mut self,b:&[u8]){self.0.borrow_mut().push(format!("write:{}",hex(b)))}fn write_u32(&mut self,n:u32){self.0.borrow_mut().push(format!("u32:{n}"))}fn write_u64(&mut self,n:u64){self.0.borrow_mut().push(format!("u64:{n:016x}"))}}
fn hex(b:&[u8])->String{b.iter().map(|b|format!("{b:02x}")).collect()}
fn generic(op:&str,mode_value:u8,variant:u32)->String{
 let log:Log=Default::default();let mode=Rc::new(Cell::new(mode_value));let span=Span::with_root_ctxt(BytePos(variant),BytePos(variant+3));
 let mut a=respan(span,Node::new(1,&log,&mode));let b=if variant%2==0{respan(span,Node::new(1,&log,&mode))}else{dummy_spanned(Node::new(2,&log,&mode))};
 let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||match op{
  "clone"=>{let c=a.clone();log.borrow_mut().push(format!("result:{}:{}",c.node.id,c.node.calls.get()));drop(c)},
  "clone_from"=>{a.clone_from(&b);log.borrow_mut().push(format!("result:{}:{}:{}",a.node.id,a.node.calls.get(),a.span==b.span))},
  "eq"=>{let v=a==b;log.borrow_mut().push(format!("result:{v}"))},
  "ne"=>{let v=a!=b;log.borrow_mut().push(format!("result:{v}"))},
  "hash"=>a.hash(&mut H(log.clone())),
  "stable"=>{let mut h=StableHasher::new();a.stable_hash(&mut Hcx(log.clone()),&mut h);let value:Fingerprint=h.finish();log.borrow_mut().push(format!("result:{}",hex(&value.to_le_bytes())))},
  "encode"=>{let mut e=MemEncoder::new();a.encode(&mut e);log.borrow_mut().push(format!("result:{}",hex(&e.finish())))},
  "encode_custom"=>{let mut e=E{data:MemEncoder::new(),log:log.clone(),mode:mode_value};a.encode(&mut e);log.borrow_mut().push(format!("result:{}",hex(&e.data.finish())))},
  "def_key"=>{let mut e=E{data:MemEncoder::new(),log:log.clone(),mode:mode_value};e.encode_def_key(Some(DefIndex::from_u32(7)),Node::new(3,&log,&mode));log.borrow_mut().push(format!("result:{}",hex(&e.data.finish())))},
  "def_key_override"=>{let mut e=Override(E{data:MemEncoder::new(),log:log.clone(),mode:mode_value});e.encode_def_key(Some(DefIndex::from_u32(7)),Node::new(3,&log,&mode));log.borrow_mut().push(format!("result:{}",hex(&e.0.data.finish())))},
  "decode"=>{DECODE.with(|settings|*settings.borrow_mut()=Some((log.clone(),mode.clone())));let mut bytes=vec![3];bytes.extend_from_slice(MAGIC_END_BYTES);let mut d=D{data:MemDecoder::new(&bytes,0).unwrap(),log:log.clone(),mode:mode_value};let c=Spanned::<Node>::decode(&mut d);log.borrow_mut().push(format!("result:{}:{}:{}",c.node.id,c.node.calls.get(),d.position()));drop(c)},
  "debug"=>{use std::fmt::Write;let mut text=String::new();let ok=write!(&mut text,"{a:?}").is_ok();log.borrow_mut().push(format!("result:{ok}:{}",hex(text.as_bytes())))},
  _=>unreachable!()
 }));
 log.borrow_mut().push(format!("panic:{}",result.is_err()));mode.set(0);drop(a);drop(b);let out=log.borrow().join(",");out
}
struct Callback {log:Log,calls:u32,panic_call:bool,panic_drop:bool}
fn event(log:&Log,text:String){if std::env::var_os("FERRUGO_TRACE_EVENTS").is_some(){eprintln!("EVENT:{text}")};log.borrow_mut().push(text)}
impl Callback {fn invoke(&mut self){event(&self.log,format!("call:{}:{}",self.calls,std::thread::panicking()));self.calls+=1;if self.panic_call{panic!("callback")}}}
impl FnOnce<()>for Callback{type Output=();extern "rust-call" fn call_once(mut self,_:()){self.invoke()}}
impl<'a>FnOnce<(&'a SessionGlobals,)>for Callback{type Output=();extern "rust-call" fn call_once(mut self,_:(&SessionGlobals,)){self.invoke()}}
impl Drop for Callback{fn drop(&mut self){let set=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||rustc_span::with_session_globals(|_|()))).is_ok();event(&self.log,format!("callback_drop:{}:{set}:{}",self.calls,std::thread::panicking()));if self.panic_drop{panic!("drop")}}}
fn session(op:&str,panic_call:bool,panic_drop:bool,observe_final:bool)->String{let log:Log=Default::default();let f=Callback{log:log.clone(),calls:0,panic_call,panic_drop};let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||match op{
 "create"=>rustc_span::create_session_globals_then(Edition::Edition2024,&[],None,f),
 "default"=>rustc_span::create_default_session_globals_then(f),
 "with_absent"=>rustc_span::with_session_globals(f),
 "if_absent"=>rustc_span::create_session_if_not_set_then(Edition::Edition2018,f),
 "set"=>{let g=SessionGlobals::new(Edition::Edition2021,&[],None);rustc_span::set_session_globals_then(&g,f)},
 "with_present"=>rustc_span::create_default_session_globals_then(||rustc_span::with_session_globals(f)),
 "if_present"=>rustc_span::create_default_session_globals_then(||rustc_span::create_session_if_not_set_then(Edition::Edition2015,f)),
 "create_present"=>rustc_span::create_default_session_globals_then(||rustc_span::create_session_globals_then(Edition::Edition2018,&[],None,f)),
 "set_present"=>rustc_span::create_default_session_globals_then(||{let g=SessionGlobals::new(Edition::Edition2018,&[],None);rustc_span::set_session_globals_then(&g,f)}),
 _=>unreachable!()
 }));log.borrow_mut().push(format!("panic:{}",result.is_err()));if observe_final{let set=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||rustc_span::with_session_globals(|_|()))).is_ok();log.borrow_mut().push(format!("final_scope:{set}"));}let out=log.borrow().join(",");out}
fn ambient_session(op:&str,call:bool,drop:bool,ambient:&str)->String{
 match ambient{
  "normal"=>session(op,call,drop,true),
  "unwinding"=>{struct Outer<'a>{op:&'a str,call:bool,drop:bool,out:Rc<RefCell<String>>}impl Drop for Outer<'_>{fn drop(&mut self){*self.out.borrow_mut()=session(self.op,self.call,self.drop,true)}}let out:Rc<RefCell<String>>=Default::default();let _=std::panic::catch_unwind(std::panic::AssertUnwindSafe({let out=out.clone();move||{let _guard=Outer{op,call,drop,out};panic!("ambient")}}));let value=out.borrow().clone();value},
  "processing"=>{let out=std::sync::Arc::new(std::sync::Mutex::new(String::new()));let hookout=out.clone();let op=op.to_string();std::panic::set_hook(Box::new(move |_|{*hookout.lock().unwrap()=session(&op,call,drop,false)}));let _=std::panic::catch_unwind(||panic!("ambient"));let value=out.lock().unwrap().clone();value},
  _=>unreachable!()
 }
}
fn main(){std::panic::set_hook(Box::new(|_|{}));if let Some(op)=std::env::args().nth(1){let call=std::env::args().nth(2).unwrap()=="true";let drop=std::env::args().nth(3).map(|v|v=="true").unwrap_or(true);let ambient=std::env::args().nth(4).unwrap_or("normal".into());let out=ambient_session(&op,call,drop,&ambient);println!("{out}");return};rustc_span::create_default_session_globals_then(||{for op in["clone","clone_from","eq","ne","hash","stable","encode","debug"]{for mode in 0..9{for variant in 0..2{println!("generic\t{op}\t{mode}\t{variant}\t{}",generic(op,mode,variant))}}}for(op,modes)in[("encode_custom",[0,4,9]),("decode",[0,9,10]),("def_key",[0,4,12])]{for mode in modes{println!("generic\t{op}\t{mode}\t0\t{}",generic(op,mode,0))}}for mode in[0,13]{println!("generic\tdef_key_override\t{mode}\t0\t{}",generic("def_key_override",mode,0))}});for op in["create","default","with_absent","if_absent","set","with_present","if_present","create_present","set_present"]{for panic_call in[false,true]{for panic_drop in[false,true]{if panic_call&&panic_drop||panic_drop&&["with_absent","create_present","set_present"].contains(&op){continue};println!("session\t{op}\t{panic_call}\t{panic_drop}\t{}",session(op,panic_call,panic_drop,true))}}}}
