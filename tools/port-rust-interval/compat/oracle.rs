// SPDX-License-Identifier: MIT OR Apache-2.0
// Trait client only. Whole original interval/idx/slice/vec/test files and the
// exact actual SmallVec crate are compiled without changing source bodies.
#![feature(step_trait,formatting_options)]
#![allow(dead_code)]
mod idx {pub use original_index::Idx;}
mod vec {pub use original_index::IndexVec;}
mod interval;
use interval::{IntervalSet,SparseIntervalMatrix};
use std::{cell::RefCell,collections::HashMap,fmt,fmt::Write,hash::{Hash,Hasher},io::{self,BufRead,Write as IoWrite},iter::Step,ops::{Bound,RangeBounds},panic::{catch_unwind,AssertUnwindSafe},rc::Rc};
use idx::Idx;

#[derive(Clone)]struct Request{kind:String,profile:usize,style:usize,ty:usize,shape:usize,char_writer:bool,fail_at:usize,panic_stage:String,panic_at:usize,cleanup_stage:String,context:usize,action:String,start_kind:usize,end_kind:usize,start:usize,end:usize,domain:usize,delta:usize,row:usize,repeat:usize}
impl Request{fn parse(line:&str)->Self{let p=line.split_whitespace().collect::<Vec<_>>();assert_eq!(p.len(),20);let n=|i:usize|p[i].parse::<usize>().unwrap();Self{kind:p[0].into(),profile:n(1),style:n(2),ty:n(3),shape:n(4),char_writer:n(5)!=0,fail_at:n(6),panic_stage:p[7].into(),panic_at:n(8),cleanup_stage:p[9].into(),context:n(10),action:p[11].into(),start_kind:n(12),end_kind:n(13),start:n(14),end:n(15),domain:n(16),delta:n(17),row:n(18),repeat:n(19)}}}
#[derive(Default)]struct Hooks{enabled:bool,stream:bool,panic_stage:String,panic_at:usize,cleanup_stage:String,counts:HashMap<String,usize>,trace:Vec<String>,track_writer:bool}
thread_local!{
 static HOOKS:RefCell<Hooks>=RefCell::new(Hooks::default());
 static ACTION:RefCell<Option<Box<dyn FnOnce()->String>>>=RefCell::new(None);
 static RESULT:RefCell<Option<Result<String,()>>>=RefCell::new(None);
}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn trace(text:String){HOOKS.with(|h|{let mut h=h.borrow_mut();if h.stream{println!("event:{}",hex(&text));io::stdout().flush().unwrap();}h.trace.push(text)})}
fn event(stage:&str,value:usize){
 let enabled=HOOKS.with(|h|h.borrow().enabled);if !enabled{return}
 trace(format!("{stage}:{value}:{}",std::thread::panicking()as u8));
 let fail=HOOKS.with(|h|{let mut h=h.borrow_mut();let n=h.counts.entry(stage.into()).or_default();*n+=1;let n=*n;(stage==h.panic_stage&&n==h.panic_at)||stage==h.cleanup_stage});if fail{panic!("injected callback panic")}
}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){if HOOKS.with(|h|h.borrow().enabled){trace(format!("local:{}:{}",self.0,std::thread::panicking()as u8))}}}
#[derive(Clone,Copy)]struct Key<const ROLE:u8>{n:usize,metadata:&'static[u8]}
impl<const R:u8> Key<R>{fn raw(n:usize)->Self{Self{n,metadata:&[1,2]}}}
impl<const R:u8> PartialEq for Key<R>{fn eq(&self,b:&Self)->bool{self.n==b.n}}impl<const R:u8> Eq for Key<R>{}
impl<const R:u8> PartialOrd for Key<R>{fn partial_cmp(&self,b:&Self)->Option<std::cmp::Ordering>{Some(self.cmp(b))}}
impl<const R:u8> Ord for Key<R>{fn cmp(&self,b:&Self)->std::cmp::Ordering{self.n.cmp(&b.n)}}
impl<const R:u8> Hash for Key<R>{fn hash<H:Hasher>(&self,h:&mut H){self.n.hash(h)}}
impl<const R:u8> fmt::Debug for Key<R>{fn fmt(&self,_:&mut fmt::Formatter<'_>)->fmt::Result{panic!("PhantomData must not invoke Key Debug")}}
impl<const R:u8> Idx for Key<R>{
 fn new(n:usize)->Self{let stage=if R==0{"new_r"}else{"new_c"};let _local=Local(stage);event(stage,n);Self::raw(n)}
 fn index(self)->usize{let stage=if R==0{"index_r"}else{"index_c"};let _local=Local(stage);event(stage,self.n);self.n}
}
impl<const R:u8> Step for Key<R>{
 fn steps_between(a:&Self,b:&Self)->(usize,Option<usize>){if a.n>b.n{(0,None)}else{(b.n-a.n,Some(b.n-a.n))}}
 fn forward_checked(a:Self,n:usize)->Option<Self>{a.n.checked_add(n).map(Self::raw)}
 fn backward_checked(a:Self,n:usize)->Option<Self>{a.n.checked_sub(n).map(Self::raw)}
 fn forward_overflowing(a:Self,n:usize)->(Self,bool){let(v,o)=a.n.overflowing_add(n);(Self::raw(v),o)}
 fn backward_overflowing(a:Self,n:usize)->(Self,bool){let(v,o)=a.n.overflowing_sub(n);(Self::raw(v),o)}
}
type Column=Key<1>;type Row=Key<0>;
struct Bounds{generation:usize,start_kind:usize,end_kind:usize,start:Column,end:Column,delta:usize}
impl Clone for Bounds{fn clone(&self)->Self{let _local=Local("clone");event("clone",self.generation);Self{generation:self.generation+1,start_kind:self.start_kind,end_kind:self.end_kind,start:Column::raw(self.start.n.wrapping_add(self.delta)),end:self.end,delta:self.delta}}}
impl RangeBounds<Column> for Bounds{
 fn start_bound(&self)->Bound<&Column>{let _local=Local("start");event("start",self.generation);match self.start_kind{0=>Bound::Unbounded,1=>Bound::Included(&self.start),2=>Bound::Excluded(&self.start),_=>panic!()}}
 fn end_bound(&self)->Bound<&Column>{let _local=Local("end");event("end",self.generation);match self.end_kind{0=>Bound::Unbounded,1=>Bound::Included(&self.end),2=>Bound::Excluded(&self.end),_=>panic!()}}
}
impl Drop for Bounds{fn drop(&mut self){let stage=if self.generation==0{"drop0"}else{"drop1"};let _local=Local(stage);event(stage,self.generation)}}
fn execute_action(){let action=ACTION.with(|f|f.borrow_mut().take().unwrap());let result=catch_unwind(AssertUnwindSafe(action)).map_err(|_|());RESULT.with(|r|*r.borrow_mut()=Some(result));}
struct Outer;
impl Drop for Outer{fn drop(&mut self){execute_action()}}
fn scoped(context:usize,action:impl FnOnce()->String+'static)->Result<String,()>{
 ACTION.with(|f|*f.borrow_mut()=Some(Box::new(action)));RESULT.with(|r|*r.borrow_mut()=None);
 match context{
  0=>execute_action(),
  1=>{let _=catch_unwind(||{let _outer=Outer;panic!("ambient outer panic")});},
  2=>{std::panic::set_hook(Box::new(|_|execute_action()));let _=catch_unwind(||panic!("outer panic message"));std::panic::set_hook(Box::new(|_|{}));},
  _=>panic!()
 }
 RESULT.with(|r|r.borrow_mut().take().expect("action was not executed"))
}
fn flags(f:&fmt::Formatter<'_>)->String{
 let width=f.width().map(|n|n.to_string()).unwrap_or("-".into());let precision=f.precision().map(|n|n.to_string()).unwrap_or("-".into());let align=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
 format!("{}/{}/{}/{}/{}/{}/{}/{}",f.alternate()as u8,width,precision,f.fill()as u32,align,f.sign_plus()as u8,f.sign_minus()as u8,f.sign_aware_zero_pad()as u8)
}
fn options(style:usize)->fmt::FormattingOptions{
 let mut o=fmt::FormattingOptions::new();match style{
  1=>{o.alternate(true);},2=>{o.debug_as_hex(Some(fmt::DebugAsHex::Lower));},3=>{o.debug_as_hex(Some(fmt::DebugAsHex::Upper));},4=>{o.debug_as_hex(Some(fmt::DebugAsHex::Lower)).alternate(true);},5=>{o.debug_as_hex(Some(fmt::DebugAsHex::Upper)).alternate(true);},
  6=>{o.width(Some(10));},7=>{o.width(Some(10)).fill('*').align(Some(fmt::Alignment::Left));},8=>{o.width(Some(10)).fill('λ').align(Some(fmt::Alignment::Center));},9=>{o.width(Some(10)).sign(Some(fmt::Sign::Plus)).sign_aware_zero_pad(true);},10=>{o.sign(Some(fmt::Sign::Minus));},11=>{o.precision(Some(3));},
  12|13=>{o.width(Some(10)).precision(Some(3)).fill('*').align(Some(fmt::Alignment::Right)).alternate(style==13);},14=>{o.width(Some(10)).fill('\0').align(Some(fmt::Alignment::Center));},_=>{}
 };o
}
#[derive(Default)]struct SinkState{calls:usize,text:String}
struct Sink<const CHAR:bool>{state:Rc<RefCell<SinkState>>,fail:usize}
impl<const C:bool> fmt::Write for Sink<C>{
 fn write_str(&mut self,text:&str)->fmt::Result{
  let _local=if HOOKS.with(|h|h.borrow().track_writer){Some(Local("write_s"))}else{None};let call={let mut s=self.state.borrow_mut();s.calls+=1;s.calls};trace(format!("write:{call}:write_s:{}:{}",hex(text),std::thread::panicking()as u8));
  let fail=HOOKS.with(|h|{let mut h=h.borrow_mut();let n=h.counts.entry("write_s".into()).or_default();*n+=1;let n=*n;h.panic_stage=="write_s"&&n==h.panic_at});if fail{panic!("writer")};if call==self.fail{return Err(fmt::Error)}self.state.borrow_mut().text.push_str(text);Ok(())
 }
 fn write_char(&mut self,c:char)->fmt::Result{
  if !C{return self.write_str(c.encode_utf8(&mut [0;4]))}
  let _local=if HOOKS.with(|h|h.borrow().track_writer){Some(Local("write_c"))}else{None};let call={let mut s=self.state.borrow_mut();s.calls+=1;s.calls};trace(format!("write:{call}:write_c:{}:{}",c as u32,std::thread::panicking()as u8));
  let fail=HOOKS.with(|h|{let mut h=h.borrow_mut();let n=h.counts.entry("write_c".into()).or_default();*n+=1;let n=*n;h.panic_stage=="write_c"&&n==h.panic_at});if fail{panic!("writer")};if call==self.fail{return Err(fmt::Error)}self.state.borrow_mut().text.push(c);Ok(())
 }
}
struct Response{value:String,text:String,calls:usize,state:String}
fn format_case<T:Idx+fmt::Debug+'static,const C:bool>(r:Request,set:IntervalSet<T>)->Response{
 let data=Rc::new(RefCell::new(set));let work=data.clone();let sink_state=Rc::new(RefCell::new(SinkState::default()));let sink_work=sink_state.clone();let copy=r.clone();
 HOOKS.with(|h|h.borrow_mut().enabled=true);
 let value=scoped(r.context,move||{
  let mut sink=Sink::<C>{state:sink_work,fail:copy.fail_at};let mut f=fmt::Formatter::new(&mut sink,options(copy.style));let mut results=Vec::new();
  for _ in 0..copy.repeat{let status=match catch_unwind(AssertUnwindSafe(||fmt::Debug::fmt(&*work.borrow(),&mut f))){Err(_)=>"panic",Ok(Err(_))=>"error",Ok(Ok(_))=>"ok"};results.push(status);trace(format!("result:{status}:{}",flags(&f)));}
  results.join(",")
 }).unwrap_or_else(|_|"panic".into());
 HOOKS.with(|h|h.borrow_mut().enabled=false);let state=format!("{:?}",data.borrow());let s=sink_state.borrow();Response{value,text:s.text.clone(),calls:s.calls,state}
}
fn replay(r:Request,stream:bool)->String{
 HOOKS.with(|h|*h.borrow_mut()=Hooks{stream,panic_stage:r.panic_stage.clone(),panic_at:r.panic_at,cleanup_stage:r.cleanup_stage.clone(),track_writer:r.panic_stage!="-",..Hooks::default()});
 let response=if r.kind=="f"{
  let domain=if r.shape==2{usize::MAX}else{512};
  macro_rules! render{($t:ty)=>{{let mut set=IntervalSet::<$t>::new(domain);if r.shape==1{set.insert_range(<$t as Idx>::new(15)..=<$t as Idx>::new(18));set.insert_range(<$t as Idx>::new(255)..=<$t as Idx>::new(260));}if r.char_writer{format_case::<$t,true>(r,set)}else{format_case::<$t,false>(r,set)}}}}
  match r.ty{0=>render!(u32),1=>render!(usize),_=>render!(Column)}
 }else{
  let mut set=IntervalSet::<Column>::new(r.domain);if r.domain>5&&r.action!="append"{set.insert_range(Column::raw(3)..=Column::raw(5));}
  let data=Rc::new(RefCell::new((set,SparseIntervalMatrix::<Row,Column>::new(r.domain))));let work=data.clone();let copy=r.clone();HOOKS.with(|h|h.borrow_mut().enabled=true);
  let value=scoped(r.context,move||{
   let bounds=Bounds{generation:0,start_kind:copy.start_kind,end_kind:copy.end_kind,start:Column::raw(copy.start),end:Column::raw(copy.end),delta:copy.delta};let mut data=work.borrow_mut();
   match copy.action.as_str(){
    "insert"=>data.0.insert_range(bounds).to_string(),"append"=>{data.0.append_range(bounds);"unit".into()},"intersects"=>data.0.intersects_range(bounds).to_string(),
    "first"=>data.0.first_unset_in(bounds).map(|k|format!("some:{}",k.n)).unwrap_or("none".into()),"last"=>data.0.last_set_in(bounds).map(|k|format!("some:{}",k.n)).unwrap_or("none".into()),
    "matrix_insert"=>{data.1.insert_range(Row::raw(copy.row),bounds);"unit".into()},"matrix_append"=>{data.1.append_range(Row::raw(copy.row),bounds);"unit".into()},_=>panic!()
   }
  }).unwrap_or_else(|_|"panic".into());
  HOOKS.with(|h|h.borrow_mut().enabled=false);let data=data.borrow();let mut rows=Vec::new();for n in 0..128{if let Some(row)=data.1.row(Row::raw(n)){rows.push(format!("{row:?}"))}else{break}}
  Response{value,text:String::new(),calls:0,state:format!("{:?}~[{}]",data.0,rows.join(";"))}
 };
 let events=HOOKS.with(|h|h.borrow().trace.join("\n"));format!("{}\t{}\t{}\t{}\t{}",hex(&response.value),hex(&events),hex(&response.text),response.calls,hex(&response.state))
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let stream=std::env::args().any(|s|s=="--abort");for line in io::stdin().lock().lines(){let output=replay(Request::parse(&line.unwrap()),stream);if stream{println!("return:{output}")}else{println!("{output}")}}}
