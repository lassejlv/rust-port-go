// User Drop/Decodable witnesses around actual unchanged Unord/SDK algorithms.
#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::unord::{UnordBag,UnordMap,UnordSet};
use rustc_serialize::{Decoder,Decodable,opaque::{MemDecoder,MAGIC_END_BYTES}};
use std::{cell::{Cell,RefCell},hash::{Hash,Hasher},io::{self,Write},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{length:usize,body:u64,token:u64,moved:u64,read:isize,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{length:0,body:0,token:0,moved:0,read:-1,resume:false})};static CURRENT:Cell<&'static str>=const{Cell::new("B")};static MOVED:RefCell<Vec<Token>>=const{RefCell::new(Vec::new())};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
fn selected(mask:u64,id:u64)->bool{id<64&&mask&(1<<id)!=0}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Token(u64);impl Drop for Token{fn drop(&mut self){event(format!("token:{}:{}:{}",self.0,CURRENT.get(),std::thread::panicking()));if selected(CONFIG.get().token,self.0){raise("token")}}}
struct Value{id:u64,token:Option<Token>}
impl Value{fn new(id:u64)->Self{event(format!("new:{id}:{}:{}",CURRENT.get(),std::thread::panicking()));Self{id,token:Some(Token(id))}}}
impl Drop for Value{fn drop(&mut self){event(format!("drop:{}:{}:{}",self.id,CURRENT.get(),std::thread::panicking()));if selected(CONFIG.get().moved,self.id){if let Some(token)=self.token.take(){event(format!("move:{}:{}",self.id,std::thread::panicking()));MOVED.with(|m|m.borrow_mut().push(token));}}if selected(CONFIG.get().body,self.id){raise("drop")}}}
impl Hash for Value{fn hash<H:Hasher>(&self,h:&mut H){self.id.hash(h)}}impl PartialEq for Value{fn eq(&self,other:&Self)->bool{self.id==other.id}}impl Eq for Value{}
struct Reader{next:usize}
impl Decoder for Reader{fn read_usize(&mut self)->usize{event(format!("length:{}:{}:{}",CONFIG.get().length,CURRENT.get(),std::thread::panicking()));CONFIG.get().length}fn read_u64(&mut self)->u64{let id=self.next;self.next+=1;event(format!("read:{id}:{}:{}",CURRENT.get(),std::thread::panicking()));if CONFIG.get().read==id as isize{raise("read")};id as u64}
fn read_u128(&mut self)->u128{panic!("unused u128")}fn read_u32(&mut self)->u32{panic!("unused u32")}fn read_u16(&mut self)->u16{panic!("unused u16")}fn read_u8(&mut self)->u8{panic!("unused u8")}fn read_isize(&mut self)->isize{panic!("unused isize")}fn read_i128(&mut self)->i128{panic!("unused i128")}fn read_i64(&mut self)->i64{panic!("unused i64")}fn read_i32(&mut self)->i32{panic!("unused i32")}fn read_i16(&mut self)->i16{panic!("unused i16")}fn read_raw_bytes(&mut self,_:usize)->&[u8]{panic!("unused raw")}fn peek_byte(&self)->u8{panic!("unused peek")}fn position(&self)->usize{self.next}}
impl Decodable<Reader> for Value{fn decode(d:&mut Reader)->Self{Value::new(d.read_u64())}}
fn message(p:Box<dyn std::any::Any+Send>)->String{p.downcast_ref::<&str>().map(|s|(*s).to_owned()).or_else(||p.downcast_ref::<String>().cloned()).unwrap_or_else(||"opaque".into())}
fn body(action:&str){event(format!("body:{}",std::thread::panicking()));CURRENT.set("A");let mut reader=Reader{next:0};let mut bytes=vec![5,42];bytes.extend_from_slice(MAGIC_END_BYTES);let mut opaque=MemDecoder::new(&bytes,0).unwrap();let bag=if action=="get_only"{let mut b=UnordBag::new();for id in 0..CONFIG.get().length{b.push(Value::new(id as u64));}Some(b)}else{None};CURRENT.set("B");let result=catch_unwind(AssertUnwindSafe(||match action{
"decode_bytes"=>{let value=UnordBag::<u8>::decode(&mut opaque);event(format!("decoded:{}",value.len()));drop(value)},
"get_only"=>{let item=bag.unwrap().into_items().get_only();event(format!("returned:{}",item.as_ref().map(|v|v.id.to_string()).unwrap_or_else(||"none".into())));drop(item)},
"decode_vec"=>{let value=Vec::<Value>::decode(&mut reader);event(format!("decoded:{}",value.len()));drop(value)},
"decode_bag"=>{let value=UnordBag::<Value>::decode(&mut reader);event(format!("decoded:{}",value.len()));drop(value)},
"decode_map"=>{let value=UnordMap::<Value,Value>::decode(&mut reader);event(format!("decoded:{}",value.len()));drop(value)},
"decode_set"=>{let value=UnordSet::<Value>::decode(&mut reader);event(format!("decoded:{}",value.len()));drop(value)},_=>unreachable!()}));event(format!("caught:{}:{}:{}",hex(&result.err().map(message).unwrap_or_default()),if action=="decode_bytes"{opaque.position()}else{reader.next},std::thread::panicking()));let moved=MOVED.with(|m|std::mem::take(&mut*m.borrow_mut()));drop(moved);}
struct Outer(String);impl Drop for Outer{fn drop(&mut self){body(&self.0)}}
struct HookLocal;impl Drop for HookLocal{fn drop(&mut self){event(format!("hook_local:{}",std::thread::panicking()));}}
fn main(){assert_eq!(std::mem::size_of::<Value>(),24);assert_eq!(std::mem::align_of::<Value>(),8);let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let action=args[2].clone();let values:Vec<i64>=args[3].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{length:values[0]as usize,body:values[1]as u64,token:values[2]as u64,moved:values[3]as u64,read:values[4]as isize,resume:values[5]!=0});let processing=mode=="processing";let hook_action=action.clone();panic::set_hook(Box::new(move|info|{let(kind,payload)=if let Some(s)=info.payload().downcast_ref::<&str>(){("static_str",(*s).to_owned())}else if let Some(s)=info.payload().downcast_ref::<String>(){("owned_string",s.clone())}else{("opaque",String::new())};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(&payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));let _local=HookLocal;drop(_local);if processing{body(&hook_action)}}));
 match mode.as_str(){"normal"=>body(&action),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(action);panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>panic!("bad mode")}}
