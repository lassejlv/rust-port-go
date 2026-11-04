use super::*;
use serde_core::{Deserialize,Serialize,de::{self,Visitor,Deserializer},ser::{self,Serializer}};
use std::{cell::RefCell,marker::PhantomData};
thread_local!{static TRACE:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};static PROBE:RefCell<Option<usize>>=const{RefCell::new(None)};}
fn log(s:String){TRACE.with(|v|v.borrow_mut().push(s));}
#[derive(Debug)]pub struct E<const O:bool>(String);impl<const O:bool>fmt::Display for E<O>{fn fmt(&self,w:&mut fmt::Formatter)->fmt::Result{w.write_str(&self.0)}}impl<const O:bool>std::error::Error for E<O>{}
impl<const O:bool>de::Error for E<O>{fn custom<T:fmt::Display>(v:T)->Self{
 if let Some(fail)=PROBE.with(|v|*v.borrow()){
 let mut sink=CustomSink{calls:vec![],accepted:String::new(),fail};let error=fmt::write(&mut sink,format_args!("{v}")).is_err();log(format!("custom_writer:{}",json!({"chunks":sink.calls,"accepted":sink.accepted,"error":error})));return Self("custom writer".into())
 }
 let s=v.to_string();log(format!("custom:{s}"));Self(s)}fn invalid_type(v:de::Unexpected,e:&dyn de::Expected)->Self{if O{let class=match v{de::Unexpected::Bool(_)=>"Bool",de::Unexpected::Signed(_)=>"Signed",de::Unexpected::Unsigned(_)=>"Unsigned",de::Unexpected::Float(_)=>"Float",de::Unexpected::Char(_)=>"Char",de::Unexpected::Str(_)=>"Str",de::Unexpected::Bytes(_)=>"Bytes",de::Unexpected::Unit=>"Unit",de::Unexpected::Option=>"Option",de::Unexpected::NewtypeStruct=>"NewtypeStruct",de::Unexpected::Seq=>"Seq",de::Unexpected::Map=>"Map",de::Unexpected::Enum=>"Enum",de::Unexpected::Other(_)=>"Other",_=>panic!()};let s=format!("invalid_type:{class}:{v}:{e}");log(s.clone());Self(s)}else{Self::custom(format_args!("invalid type: {v}, expected {e}"))}}}
impl<const O:bool>ser::Error for E<O>{fn custom<T:fmt::Display>(v:T)->Self{let s=v.to_string();log(format!("ser_custom:{s}"));Self(s)}}
struct CustomSink{calls:Vec<String>,accepted:String,fail:usize}
impl fmt::Write for CustomSink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.calls.push(format!("S:{s}"));if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push_str(s);Ok(())}}
 fn write_char(&mut self,c:char)->fmt::Result{self.calls.push(format!("C:{c}"));if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push(c);Ok(())}}
}
macro_rules! unsupported_ser{($($name:ident($($arg:ident:$ty:ty),*);)*)=>{$(fn $name(self,$($arg:$ty),*)->Result<Self::Ok,Self::Error>{$(let _=$arg;)*panic!("unexpected serializer method {}",stringify!($name))})*};}
macro_rules! ser_impl{()=>{
 type Ok=String;type Error=E<false>;type SerializeSeq=ser::Impossible<String,Self::Error>;type SerializeTuple=Self::SerializeSeq;type SerializeTupleStruct=Self::SerializeSeq;type SerializeTupleVariant=Self::SerializeSeq;type SerializeMap=Self::SerializeSeq;type SerializeStruct=Self::SerializeSeq;type SerializeStructVariant=Self::SerializeSeq;
 fn serialize_str(self,v:&str)->Result<String,Self::Error>{log(format!("serialize_str:{v}"));if self.reject{Err(E("serialize rejection".into()))}else{Ok(v.into())}}
 unsupported_ser!{serialize_bool(v:bool);serialize_i8(v:i8);serialize_i16(v:i16);serialize_i32(v:i32);serialize_i64(v:i64);serialize_u8(v:u8);serialize_u16(v:u16);serialize_u32(v:u32);serialize_u64(v:u64);serialize_f32(v:f32);serialize_f64(v:f64);serialize_char(v:char);serialize_bytes(v:&[u8]);serialize_none();serialize_unit();serialize_unit_struct(name:&'static str);serialize_unit_variant(name:&'static str,index:u32,variant:&'static str);}
 fn serialize_some<T:Serialize+?Sized>(self,_:&T)->Result<String,Self::Error>{panic!()}
 fn serialize_newtype_struct<T:Serialize+?Sized>(self,_:&'static str,_:&T)->Result<String,Self::Error>{panic!()}
 fn serialize_newtype_variant<T:Serialize+?Sized>(self,_:&'static str,_:u32,_:&'static str,_:&T)->Result<String,Self::Error>{panic!()}
 fn serialize_seq(self,_:Option<usize>)->Result<Self::SerializeSeq,Self::Error>{panic!()}
 fn serialize_tuple(self,_:usize)->Result<Self::SerializeTuple,Self::Error>{panic!()}
 fn serialize_tuple_struct(self,_:&'static str,_:usize)->Result<Self::SerializeTupleStruct,Self::Error>{panic!()}
 fn serialize_tuple_variant(self,_:&'static str,_:u32,_:&'static str,_:usize)->Result<Self::SerializeTupleVariant,Self::Error>{panic!()}
 fn serialize_map(self,_:Option<usize>)->Result<Self::SerializeMap,Self::Error>{panic!()}
 fn serialize_struct(self,_:&'static str,_:usize)->Result<Self::SerializeStruct,Self::Error>{panic!()}
 fn serialize_struct_variant(self,_:&'static str,_:u32,_:&'static str,_:usize)->Result<Self::SerializeStructVariant,Self::Error>{panic!()}
};}
struct Base{reject:bool}impl Serializer for Base{ser_impl!();}
struct Collect{reject:bool}impl Serializer for Collect{ser_impl!();fn collect_str<T:fmt::Display+?Sized>(self,v:&T)->Result<String,Self::Error>{log(format!("collect_str:{v}"));if self.reject{Err(E("collect rejection".into()))}else{Ok(format!("collected:{v}"))}}}
struct D<'a,const O:bool>{q:&'a Value}
struct Expected<'a,T>(&'a T);impl<'a,T:de::Expected>fmt::Display for Expected<'a,T>{fn fmt(&self,w:&mut fmt::Formatter)->fmt::Result{self.0.fmt(w)}}
impl<'de,const O:bool> Deserializer<'de> for D<'de,O>{type Error=E<O>;
 fn deserialize_any<V:Visitor<'de>>(self,_:V)->Result<V::Value,Self::Error>{panic!("child unexpectedly visited")}
 fn deserialize_str<V:Visitor<'de>>(self,v:V)->Result<V::Value,Self::Error>{log("deserialize_str".into());log(format!("expecting:{}",Expected(&v)));let q=self.q;let s=text(q,"a");let signed=q["signed"].as_i64().unwrap();let unsigned=q["unsigned"].as_u64().unwrap();let wide=q["wide"].as_array().unwrap();let wide=((wide[0].as_u64().unwrap() as u128)<<64)|wide[1].as_u64().unwrap() as u128;let f=text(q,"float").parse::<f64>().unwrap_or(0.0);match text(q,"input"){
 "str"=>v.visit_str(s),"borrowed_str"=>v.visit_borrowed_str(s),"string"=>v.visit_string(s.to_owned()),"char"=>v.visit_char(s.chars().next().unwrap()),"bool"=>v.visit_bool(q["collect"].as_bool().unwrap()),"i8"=>v.visit_i8(signed as i8),"i16"=>v.visit_i16(signed as i16),"i32"=>v.visit_i32(signed as i32),"i64"=>v.visit_i64(signed),"i128"=>v.visit_i128(wide as i128),"u8"=>v.visit_u8(unsigned as u8),"u16"=>v.visit_u16(unsigned as u16),"u32"=>v.visit_u32(unsigned as u32),"u64"=>v.visit_u64(unsigned),"u128"=>v.visit_u128(wide),"f32"=>v.visit_f32(f as f32),"f64"=>v.visit_f64(f),"bytes"=>v.visit_bytes(s.as_bytes()),"borrowed_bytes"=>v.visit_borrowed_bytes(s.as_bytes()),"byte_buf"=>v.visit_byte_buf(s.as_bytes().to_owned()),"none"=>v.visit_none(),"some"=>v.visit_some(D::<O>{q}),"unit"=>v.visit_unit(),"newtype"=>v.visit_newtype_struct(D::<O>{q}),"seq"=>v.visit_seq(Access::<O>(PhantomData)),"map"=>v.visit_map(Access::<O>(PhantomData)),"enum"=>v.visit_enum(Access::<O>(PhantomData)),_=>panic!()}}
 serde_core::forward_to_deserialize_any!{bool i8 i16 i32 i64 u8 u16 u32 u64 f32 f64 char string bytes byte_buf option unit unit_struct newtype_struct seq tuple tuple_struct map struct enum identifier ignored_any}
}
struct Access<const O:bool>(PhantomData<()>);impl<'de,const O:bool>de::SeqAccess<'de> for Access<O>{type Error=E<O>;fn next_element_seed<T:de::DeserializeSeed<'de>>(&mut self,_:T)->Result<Option<T::Value>,Self::Error>{panic!()}}impl<'de,const O:bool>de::MapAccess<'de> for Access<O>{type Error=E<O>;fn next_key_seed<T:de::DeserializeSeed<'de>>(&mut self,_:T)->Result<Option<T::Value>,Self::Error>{panic!()}fn next_value_seed<T:de::DeserializeSeed<'de>>(&mut self,_:T)->Result<T::Value,Self::Error>{panic!()}}
impl<'de,const O:bool>de::EnumAccess<'de> for Access<O>{type Error=E<O>;type Variant=Self;fn variant_seed<V:de::DeserializeSeed<'de>>(self,_:V)->Result<(V::Value,Self),Self::Error>{panic!()}}impl<'de,const O:bool>de::VariantAccess<'de> for Access<O>{type Error=E<O>;fn unit_variant(self)->Result<(),Self::Error>{panic!()}fn newtype_variant_seed<T:de::DeserializeSeed<'de>>(self,_:T)->Result<T::Value,Self::Error>{panic!()}fn tuple_variant<V:Visitor<'de>>(self,_:usize,_:V)->Result<V::Value,Self::Error>{panic!()}fn struct_variant<V:Visitor<'de>>(self,_:&'static [&'static str],_:V)->Result<V::Value,Self::Error>{panic!()}}
fn de<const O:bool,T:for<'de>Deserialize<'de>+fmt::Display>(q:&Value)->Value{match T::deserialize(D::<O>{q}){Ok(v)=>json!({"ok":true,"value":v.to_string()}),Err(e)=>json!({"ok":false,"error":e.0})}}
fn ser<T:Serialize>(q:&Value,v:T)->Value{let reject=q["reject"].as_bool().unwrap();let r=if q["collect"].as_bool().unwrap(){v.serialize(Collect{reject})}else{v.serialize(Base{reject})};match r{Ok(v)=>json!({"ok":true,"value":v}),Err(e)=>json!({"ok":false,"error":e.0})}}
fn json_input<T:for<'de>Deserialize<'de>+Serialize+fmt::Display>(q:&Value)->Value {
 let data=if text(q,"input")=="json_bytes"{text(q,"a").as_bytes().chunks(2).map(|b|u8::from_str_radix(std::str::from_utf8(b).unwrap(),16).unwrap()).collect::<Vec<_>>()}else{text(q,"a").as_bytes().to_vec()};match serde_json::from_slice::<T>(&data){Ok(v)=>json!({"ok":true,"value":v.to_string(),"encoded":serde_json::to_string(&v).unwrap()}),Err(e)=>{let message=e.to_string();let short=message.rsplit_once(" at line ").map(|p|p.0).unwrap_or(&message);json!({"ok":false,"error":if e.is_data(){short}else{"invalid JSON"},"syntax":!e.is_data()})}}
}
pub fn run(q:&Value)->Value{TRACE.with(|v|v.borrow_mut().clear());PROBE.with(|v|*v.borrow_mut()=if q["reject"].as_bool().unwrap()&&text(q,"input")!="serialize"{Some(q["fail"].as_u64().unwrap() as usize)}else{None});let mut result=match text(q,"input"){"json"|"json_bytes"=>match text(q,"kind"){"version"=>json_input::<Version>(q),"req"=>json_input::<VersionReq>(q),"comparator"=>json_input::<Comparator>(q),_=>panic!()},"serialize"=>match text(q,"kind"){"version"=>ser(q,Version::parse(text(q,"a")).unwrap()),"req"=>ser(q,VersionReq::parse(text(q,"a")).unwrap()),"comparator"=>ser(q,Comparator::parse(text(q,"a")).unwrap()),_=>panic!()},_=>{macro_rules! call{($ty:ty)=>{if q["override"].as_bool().unwrap(){de::<true,$ty>(q)}else{de::<false,$ty>(q)}}}match text(q,"kind"){"version"=>call!(Version),"req"=>call!(VersionReq),"comparator"=>call!(Comparator),_=>panic!()}}};result["trace"]=json!(TRACE.with(|v|v.borrow().clone()));result}
