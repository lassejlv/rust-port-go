// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual rustc_serialize trait clients, not a replacement codec implementation.
use super::*;
use storage_reference::{Encodable,Decodable,Encoder,Decoder};

struct C<const SPECIAL:bool>{values:Vec<i64>,position:usize,length:usize}
macro_rules! reject_emit {($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self,_:$ty){panic!("unexpected encoder primitive")})*};}
macro_rules! reject_read {($($method:ident:$ty:ty),*)=>{$(fn $method(&mut self)->$ty{panic!("unexpected decoder primitive")})*};}
impl<const B:bool> Encoder for C<B>{
 fn emit_usize(&mut self,n:usize){ev!("emit_len",n);self.values.push(n as i64)}
 fn emit_i64(&mut self,n:i64){ev!("emit_value",n);self.values.push(n)}
 reject_emit!(emit_u128:u128,emit_u64:u64,emit_u32:u32,emit_u16:u16,emit_u8:u8,emit_isize:isize,emit_i128:i128,emit_i32:i32,emit_i16:i16);
 fn emit_raw_bytes(&mut self,_:&[u8]){panic!("unexpected raw encoding")}
}
impl<const B:bool> Decoder for C<B>{
 fn read_usize(&mut self)->usize{ev!("read_len",self.length);self.length}
 fn read_i64(&mut self)->i64{let value=self.values[self.position];ev!("read_value",value);self.position+=1;value}
 reject_read!(read_u128:u128,read_u64:u64,read_u32:u32,read_u16:u16,read_u8:u8,read_isize:isize,read_i128:i128,read_i32:i32,read_i16:i16);
 fn read_raw_bytes(&mut self,_:usize)->&[u8]{panic!("unexpected raw decoding")}
 fn peek_byte(&self)->u8{panic!("unexpected decoder peek")}
 fn position(&self)->usize{self.position}
}
impl<const B:bool> Encodable<C<B>> for E{fn encode(&self,s:&mut C<B>){ev!("encode",self.0);s.emit_i64(self.0)}}
impl<const B:bool> Decodable<C<B>> for E{fn decode(d:&mut C<B>)->Self{ev!("decode");E(d.read_i64())}}
impl Encodable<C<true>> for [E]{fn encode(&self,s:&mut C<true>){ev!("encode_slice",self.len());s.values.extend(self.iter().map(|e|e.0))}}
impl Decodable<C<true>> for Vec<E>{fn decode(d:&mut C<true>)->Self{ev!("decode_vec");d.values.iter().map(|n|E(*n)).collect()}}

pub(super)fn replay(r:&Value)->Value{
 HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:n(r,"limit"),enabled:false,..Hooks::default()});let mut v=make_vec(values(r,"values"),n(r,"capacity"));let mut steps=vec![];
 for a in r["actions"].as_array().unwrap(){HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.fail=text(a,"panic").to_owned();h.nth=n(a,"panic_at").max(1);h.count=0;h.enabled=true});let mut result=Value::Null;
 let caught=catch_unwind(AssertUnwindSafe(||match text(a,"op"){
 "encode"=>{let mut c=C::<false>{values:vec![],position:0,length:0};v.encode(&mut c);result=json!(c.values)},
 "encode_override"=>{let mut c=C::<true>{values:vec![],position:0,length:0};v.encode(&mut c);result=json!(c.values)},
 "decode"=>{let mut c=C::<false>{values:values(a,"values"),position:0,length:n(a,"n")};v=IndexVec::decode(&mut c)},
 "decode_override"=>{let mut c=C::<true>{values:values(a,"values"),position:0,length:n(a,"n")};v=IndexVec::decode(&mut c)},
 "extend_one"=>std::iter::Extend::extend_one(&mut v,E(integer(a,"value"))),"extend_reserve"=>std::iter::Extend::extend_reserve(&mut v,n(a,"n")),_=>panic!("codec action")
 }));let events=HOOKS.with(|h|h.borrow().events.clone());steps.push(json!({"value":result,"panic":caught.is_err(),"events":events,"v":snapshot(&v),"w":[],"capacity":v.raw.capacity(),"other_capacity":0}));
 }HOOKS.with(|h|h.borrow_mut().enabled=false);json!({"steps":steps})
}
