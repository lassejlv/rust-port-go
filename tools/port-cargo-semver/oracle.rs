// Original-source oracle only. SPDX-License-Identifier: MIT OR Apache-2.0
mod util;
use semver::{Comparator, Op, Prerelease, Version, VersionReq};
use util::semver_ext::{OptVersionReq, VersionExt, VersionReqExt, VersionReqMatchMode};
use std::cell::RefCell;
use std::hash::{Hash, Hasher};
use std::io::{self, Read, Write};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::rc::Rc;

fn u8_in(input:&mut &[u8])->u8 {let value=input[0];*input=&input[1..];value}
fn u32_in(input:&mut &[u8])->u32 {let(value,rest)=input.split_at(4);*input=rest;u32::from_le_bytes(value.try_into().unwrap())}
fn u64_in(input:&mut &[u8])->u64 {let(value,rest)=input.split_at(8);*input=rest;u64::from_le_bytes(value.try_into().unwrap())}
fn text_in(input:&mut &[u8])->String {let size=u32_in(input)as usize;let(value,rest)=input.split_at(size);*input=rest;String::from_utf8(value.to_vec()).unwrap()}
fn u32_out(output:&mut impl Write,value:u32){output.write_all(&value.to_le_bytes()).unwrap()}
fn text_out(output:&mut impl Write,value:&str){u32_out(output,value.len()as u32);output.write_all(value.as_bytes()).unwrap()}
fn strings_out(output:&mut impl Write,values:&[String]){u32_out(output,values.len()as u32);for value in values{text_out(output,value)}}
fn op(value:u8)->Op {match value{0=>Op::Exact,1=>Op::Greater,2=>Op::GreaterEq,3=>Op::Less,4=>Op::LessEq,5=>Op::Tilde,6=>Op::Caret,7=>Op::Wildcard,_=>panic!("bad oracle op")}}
fn panic_text(value:Box<dyn std::any::Any+Send>)->String {let text=if let Some(s)=value.downcast_ref::<&str>(){s.to_string()}else if let Some(s)=value.downcast_ref::<String>(){s.clone()}else{"non-string panic".into()};if text.contains("attempt to add with overflow"){ "overflow".into() }else{text}}
#[derive(Default)]
struct Trace {events:Vec<String>,panic_at:usize}
impl Trace {fn event(&mut self,value:String){self.events.push(value);if self.panic_at!=0&&self.events.len()==self.panic_at{panic!("hasher panic")}}}
macro_rules! trace_number {($name:ident,$kind:ty)=>{fn $name(&mut self,value:$kind){self.event(format!("{}:{value}",stringify!($name)))}}}
impl Hasher for Trace {
 fn finish(&self)->u64{0}
 fn write(&mut self,value:&[u8]){self.event(format!("write:{}",value.iter().map(|b|format!("{b:02x}")).collect::<String>()))}
 trace_number!(write_u8,u8);trace_number!(write_u16,u16);trace_number!(write_u32,u32);trace_number!(write_u64,u64);trace_number!(write_u128,u128);trace_number!(write_usize,usize);
 trace_number!(write_i8,i8);trace_number!(write_i16,i16);trace_number!(write_i32,i32);trace_number!(write_i64,i64);trace_number!(write_i128,i128);trace_number!(write_isize,isize);
}
fn state(output:&mut impl Write,value:&OptVersionReq){
 text_out(output,&value.to_string());text_out(output,&format!("{value:?}"));
 output.write_all(&[value.is_exact()as u8,value.is_locked()as u8,value.is_precise()as u8]).unwrap();
 for version in [value.locked_version(),value.precise_version()]{match version{Some(v)=>{output.write_all(&[1]).unwrap();text_out(output,&v.to_string())},None=>output.write_all(&[0]).unwrap()}}
 let clone=value.clone();output.write_all(&[u8::from(value==&clone)]).unwrap();let mut hasher=Trace::default();value.hash(&mut hasher);strings_out(output,&hasher.events);
}
struct Base {version:Version,log:Rc<RefCell<Vec<String>>>,panic_mask:u8}
impl Base {
 fn call(&self,label:String,mask:u8){self.log.borrow_mut().push(label);if self.panic_mask&mask!=0{panic!("custom trait panic")}}
 fn is_pre(&self)->bool{self.call("is_prerelease".into(),1);self.version.is_prerelease()}
 fn req(&self,op:Op)->VersionReq{self.call(format!("to_req:{op:?}"),2);self.version.to_req(op)}
 fn exact(&self)->VersionReq{self.call("to_exact_req".into(),4);self.version.to_req(Op::Tilde)}
 fn caret(&self)->VersionReq{self.call("to_caret_req".into(),8);self.version.to_req(Op::Greater)}
}
macro_rules! implement {
 ($name:ident,$($extra:tt)*)=>{struct $name(Base);impl VersionExt for $name{fn is_prerelease(&self)->bool{self.0.is_pre()}fn to_req(&self,op:Op)->VersionReq{self.0.req(op)}$($extra)*}}
}
implement!(Required,);
implement!(ExactOverride,fn to_exact_req(&self)->VersionReq{self.0.exact()});
implement!(CaretOverride,fn to_caret_req(&self)->VersionReq{self.0.caret()});
implement!(BothOverride,fn to_exact_req(&self)->VersionReq{self.0.exact()}fn to_caret_req(&self)->VersionReq{self.0.caret()});
struct CustomReq<'a>(&'a Base);
impl VersionReqExt for CustomReq<'_>{fn matches_prerelease(&self,version:&Version)->bool{self.0.call(format!("req_ext:{version}"),16);version.patch%2==0}}
fn traits(output:&mut impl Write,version:Version,variant:u8,panic_mask:u8,actions:&[(u8,String)]){
 let log=Rc::new(RefCell::new(Vec::new()));let base=Base{version,log:log.clone(),panic_mask};let custom:Box<dyn VersionExt>=match variant{0=>Box::new(Required(base)),1=>Box::new(ExactOverride(base)),2=>Box::new(CaretOverride(base)),3=>Box::new(BothOverride(base)),_=>panic!("bad trait kind")};
 // The separate VersionReqExt implementer has no required VersionExt methods.
 let req_base=Base{version:Version::new(0,0,0),log:log.clone(),panic_mask};
 u32_out(output,actions.len()as u32);
 for (action,text) in actions{
  let result=catch_unwind(AssertUnwindSafe(||{match action{
   0=>custom.is_prerelease().to_string(),
   1=>{let r=custom.to_req(op(text.parse().unwrap()));format!("{r}\n{r:?}")},
   2=>{let r=custom.to_exact_req();format!("{r}\n{r:?}")},
   3=>{let r=custom.to_caret_req();format!("{r}\n{r:?}")},
   4=>CustomReq(&req_base).matches_prerelease(&Version::parse(text).unwrap()).to_string(),
   _=>panic!("bad trait action"),
  }}));match result{Ok(value)=>{text_out(output,"");text_out(output,&value)},Err(e)=>{text_out(output,&panic_text(e));text_out(output,"")}};strings_out(output,&log.borrow());
 }
}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));let mut data=Vec::new();io::stdin().read_to_end(&mut data).unwrap();let mut input=data.as_slice();assert_eq!(&input[..4],b"CGSR");input=&input[4..];let count=u32_in(&mut input);let mut output=io::BufWriter::new(io::stdout().lock());output.write_all(b"CGSO").unwrap();u32_out(&mut output,count);
 for _ in 0..count{
  let kind=u8_in(&mut input);let checked=u8_in(&mut input);assert_eq!(checked!=0, cfg!(debug_assertions)&&std::env::var("FERRUGO_EXPECT_CHECKED").unwrap()=="true");
  let constructor=u8_in(&mut input);let requirement=text_in(&mut input);let initial_version=text_in(&mut input);let raw=u8_in(&mut input);let comparator_count=u32_in(&mut input);let mut comparators=Vec::new();
  for _ in 0..comparator_count{let operation=op(u8_in(&mut input));let major=u64_in(&mut input);let minor=if u8_in(&mut input)!=0{Some(u64_in(&mut input))}else{None};let patch=if u8_in(&mut input)!=0{Some(u64_in(&mut input))}else{None};let pre=Prerelease::new(&text_in(&mut input)).unwrap();comparators.push(Comparator{op:operation,major,minor,patch,pre})}
  let action_count=u32_in(&mut input);let mut actions=Vec::new();for _ in 0..action_count{actions.push((u8_in(&mut input),text_in(&mut input)))}
  output.write_all(&[kind]).unwrap();
  if kind==1{let version=Version::parse(&initial_version).unwrap();text_out(&mut output,"");traits(&mut output,version,constructor,requirement.parse().unwrap(),&actions);continue}
  let req=if raw!=0{Ok(VersionReq{comparators})}else{VersionReq::parse(&requirement)};
  let req=match req{Ok(r)=>r,Err(e)=>{text_out(&mut output,&format!("requirement: {e}"));continue}};
  let initial_version=match Version::parse(&initial_version){Ok(v)=>v,Err(e)=>{text_out(&mut output,&format!("version: {e}"));continue}};
  if kind==2{
   text_out(&mut output,"");let optional=OptVersionReq::Req(req.clone());
   for action in 0..4{let result=catch_unwind(AssertUnwindSafe(||match action{0=>req.matches(&initial_version),1=>util::semver_eval_ext::matches_prerelease(&req,&initial_version),2=>optional.matches(&initial_version,VersionReqMatchMode::Default),_=>optional.matches(&initial_version,VersionReqMatchMode::Prerelease)}));match result{Ok(value)=>{text_out(&mut output,"");output.write_all(&[if value{2}else{1}]).unwrap()},Err(e)=>{text_out(&mut output,&panic_text(e));output.write_all(&[0]).unwrap()}}};continue;
  }
  let mut value=match constructor{0=>OptVersionReq::Any,1=>OptVersionReq::Req(req.clone()),2=>OptVersionReq::Locked(initial_version.clone(),req.clone()),3=>OptVersionReq::Precise(initial_version.clone(),req.clone()),4=>OptVersionReq::exact(&initial_version),5=>OptVersionReq::lock_to_exact(&initial_version),6=>OptVersionReq::from(req.clone()),_=>panic!("bad constructor")};
  text_out(&mut output,"");state(&mut output,&value);u32_out(&mut output,actions.len()as u32);
  for (action,text) in actions{let mut trace=Trace::default();let result=catch_unwind(AssertUnwindSafe(||->Option<bool>{
   if action==0{return None}if action==7{value=OptVersionReq::Any;return None}if action==8{trace.panic_at=text.parse().unwrap();value.hash(&mut trace);return None}
   if action==11{value=OptVersionReq::from(req.clone());return None}
   if action==12{let(kind,text)=text.split_once('|').unwrap();let version=Version::parse(text).unwrap();let other=match kind{ "0"=>OptVersionReq::Any,"1"=>OptVersionReq::Req(req.clone()),"2"=>OptVersionReq::Locked(version,req.clone()),"3"=>OptVersionReq::Precise(version,req.clone()),_=>panic!("bad equality kind")};return Some(value==other)}
   let version=Version::parse(&text).unwrap();match action{
    1=>Some(value.matches(&version,VersionReqMatchMode::Default)),2=>Some(value.matches(&version,VersionReqMatchMode::Prerelease)),3=>Some(util::semver_eval_ext::matches_prerelease(&req,&version)),
    4=>{value.lock_to(&version,VersionReqMatchMode::Default);None},5=>{value.lock_to(&version,VersionReqMatchMode::Prerelease);None},6=>{value.precise_to(&version);None},9=>{value=OptVersionReq::exact(&version);None},10=>{value=OptVersionReq::lock_to_exact(&version);None},_=>panic!("bad action")}
  }));match result{Ok(value)=>{text_out(&mut output,"");output.write_all(&[match value{None=>0,Some(false)=>1,Some(true)=>2}]).unwrap()},Err(e)=>{text_out(&mut output,&panic_text(e));output.write_all(&[0]).unwrap()}};strings_out(&mut output,&trace.events);state(&mut output,&value)}
 }
 assert!(input.is_empty());
}
