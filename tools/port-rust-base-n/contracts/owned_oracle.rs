#![feature(ascii_char,ascii_char_variants)]
//! Calls the complete unchanged pinned base_n module with non-Copy implementors.
mod base_n;
use base_n::{BaseNString,ToBaseN};
use std::{any::Any,cell::{Cell,RefCell},fmt,io::{self,BufRead},ops::Deref};

#[derive(Clone,Copy)]struct Config{kind:u8,width:usize,fixed:bool,base:usize,n:u128,mask:u8,disposition:u8,mode:u8}
#[derive(Clone,Copy,Default)]struct Snapshot{counter:u32,a:u32,b:u32,self_drops:u32,a_drops:u32,b_drops:u32}
thread_local!{
 static CONFIG:Cell<Config>=const{Cell::new(Config{kind:0,width:0,fixed:false,base:10,n:0,mask:0,disposition:0,mode:0})};
 static TRACE:RefCell<Vec<String>>=const{RefCell::new(Vec::new())};
 static STATE:Cell<Snapshot>=const{Cell::new(Snapshot{counter:0,a:0,b:0,self_drops:0,a_drops:0,b_drops:0})};
 static STORED:RefCell<Option<Box<dyn Any>>>=const{RefCell::new(None)};
 static PHASE:Cell<&'static str>=const{Cell::new("construct")};
 static SCOPE:Cell<&'static str>=const{Cell::new("op")};
 static RESULT:RefCell<Option<Outcome>>=const{RefCell::new(None)};
}
fn config()->Config{CONFIG.with(Cell::get)}
fn log(message:impl AsRef<str>){let message=format!("{}:{}:{}",message.as_ref(),std::thread::panicking(),SCOPE.with(Cell::get));if std::env::var_os("FERRUGO_BASE_N_ABORT_TRACE").is_some(){println!("{message}")}TRACE.with(|trace|trace.borrow_mut().push(message))}
fn state(update:impl FnOnce(&mut Snapshot)){STATE.with(|cell|{let mut snapshot=cell.get();update(&mut snapshot);cell.set(snapshot)})}
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
struct Probe(&'static str);
impl Drop for Probe{fn drop(&mut self){log(self.0)}}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){log(format!("local:{}",self.0));let _probe=Probe("local-probe");if config().mask&2!=0{panic!("local")}}}
struct Field{which:u8,value:u32}
impl Drop for Field{fn drop(&mut self){
 let name=if self.which==0{"a"}else{"b"};log(format!("field:{name}:{}",self.value));
 state(|state|if self.which==0{state.a=self.value;state.a_drops+=1}else{state.b=self.value;state.b_drops+=1});
 let _probe=Probe(if self.which==0{"a-probe"}else{"b-probe"});
 if config().mask&(if self.which==0{8}else{16})!=0{panic!("field")}
}}
struct Input<const KIND:u8,const WIDTH:usize>{counter:u32,n:u128,a:Field,b:Field}
impl<const KIND:u8,const WIDTH:usize> Drop for Input<KIND,WIDTH>{fn drop(&mut self){
 log(format!("self:{}:{}:{}",self.counter,self.a.value,self.b.value));let _probe=Probe("self-probe");
 self.counter+=1;self.a.value+=3;self.b.value+=5;
 state(|state|{state.counter=self.counter;state.a=self.a.value;state.b=self.b.value;state.self_drops+=1});
 if config().mask&4!=0{panic!("self")}
}}
fn enter<const K:u8,const W:usize>(value:&mut Input<K,W>,kind:&str){
 log(format!("body:{kind}:{}:{}:{}",value.counter,value.a.value,value.b.value));
 value.counter+=10;value.a.value+=100;value.b.value+=1000;
 state(|state|{state.counter=value.counter;state.a=value.a.value;state.b=value.b.value});
 log(format!("mutated:{}:{}:{}",value.counter,value.a.value,value.b.value));
 if config().mask&1!=0{panic!("body")}
}
fn dispose<const K:u8,const W:usize>(value:Input<K,W>){match config().disposition{
 0=>{drop(value)},
 1=>{drop(value);log("manual-done")},
 2=>{std::mem::forget(value);log("forgot")},
 3=>{STORED.with(|stored|{assert!(stored.borrow().is_none());*stored.borrow_mut()=Some(Box::new(value))});log("stored")},
 _=>panic!("invalid disposition"),
}}
// For the default disposition, leave the actual argument live until ordinary
// parameter cleanup. Explicit dispositions move it at the chosen source point.
fn into_body<const K:u8,const W:usize>(mut value:Input<K,W>)->u128{
 let _local=Local("into");enter(&mut value,"into");let result=value.n;
 if config().disposition!=0{dispose(value);log("return:into");return result}
 log("return:into");result
}
impl<const K:u8,const W:usize> From<Input<K,W>>for u128{fn from(value:Input<K,W>)->u128{into_body(value)}}
fn override_body<const K:u8,const W:usize>(mut value:Input<K,W>,base:usize,kind:&'static str,n:u128)->BaseNString{
 let _local=Local(kind);enter(&mut value,kind);let result=n.to_base(base);
 if config().disposition!=0{dispose(value);log(format!("return:{kind}"));return result}
 log(format!("return:{kind}"));result
}
fn width<const W:usize>(base:usize)->usize{
 let state=STATE.with(Cell::get);log(format!("len:{base}:{W}:{}:{}:{}",state.self_drops,state.a_drops,state.b_drops));
 if config().mask&32!=0{panic!("len")};W
}
impl<const W:usize>ToBaseN for Input<0,W>{fn encoded_len(base:usize)->usize{width::<W>(base)}}
impl<const W:usize>ToBaseN for Input<1,W>{fn encoded_len(base:usize)->usize{width::<W>(base)}fn to_base(self,base:usize)->BaseNString{override_body(self,base,"base",42)}}
impl<const W:usize>ToBaseN for Input<2,W>{fn encoded_len(base:usize)->usize{width::<W>(base)}fn to_base_fixed_len(self,base:usize)->BaseNString{override_body(self,base,"fixed",99)}}
impl<const W:usize>ToBaseN for Input<3,W>{fn encoded_len(base:usize)->usize{width::<W>(base)}fn to_base(self,base:usize)->BaseNString{override_body(self,base,"base",42)}fn to_base_fixed_len(self,base:usize)->BaseNString{override_body(self,base,"fixed",99)}}

fn call<const K:u8,const W:usize>()->BaseNString where Input<K,W>:ToBaseN{
 let cfg=config();let value=Input::<K,W>{counter:7,n:cfg.n,a:Field{which:0,value:11},b:Field{which:1,value:22}};
 if cfg.fixed{value.to_base_fixed_len(cfg.base)}else{value.to_base(cfg.base)}
}
fn custom_width<const K:u8>()->BaseNString where Input<K,0>:ToBaseN,Input<K,1>:ToBaseN,Input<K,5>:ToBaseN,Input<K,128>:ToBaseN,Input<K,129>:ToBaseN,Input<K,{usize::MAX}>:ToBaseN{
 match config().width{0=>call::<K,0>(),1=>call::<K,1>(),5=>call::<K,5>(),128=>call::<K,128>(),129=>call::<K,129>(),usize::MAX=>call::<K,{usize::MAX}>(),_=>panic!("invalid width")}
}
fn produce()->BaseNString{let cfg=config();match cfg.kind{
 0=>custom_width::<0>(),1=>custom_width::<1>(),2=>custom_width::<2>(),3=>custom_width::<3>(),
 32=>if cfg.fixed{(cfg.n as u32).to_base_fixed_len(cfg.base)}else{(cfg.n as u32).to_base(cfg.base)},
 64=>if cfg.fixed{(cfg.n as u64).to_base_fixed_len(cfg.base)}else{(cfg.n as u64).to_base(cfg.base)},
 128=>if cfg.fixed{cfg.n.to_base_fixed_len(cfg.base)}else{cfg.n.to_base(cfg.base)},_=>panic!("invalid type")
}}
#[derive(Clone)]struct Outcome{phase:String,text:String,alias:bool,display:String}
struct Sink{expected:*const u8,same:bool,text:String}
impl fmt::Write for Sink{fn write_str(&mut self,text:&str)->fmt::Result{self.same&=text.as_ptr()==self.expected;self.text.push_str(text);Ok(())}}
fn execute()->Outcome{
 PHASE.with(|phase|phase.set("construct"));let value=Box::new(produce());log("constructed");PHASE.with(|phase|phase.set("deref"));
 let text=value.deref().deref();let as_ref:&str=value.as_ref().as_ref();
 let begin=(&*value as*const BaseNString)as usize;let end=begin+std::mem::size_of::<BaseNString>();let pointer=text.as_ptr()as usize;
 let alias=pointer>=begin&&pointer+text.len()<=end&&text.as_ptr()==as_ref.as_ptr()&&text.as_ptr()==value.deref().deref().as_ptr();
 let mut sink=Sink{expected:text.as_ptr(),same:true,text:String::new()};PHASE.with(|phase|phase.set("display"));fmt::write(&mut sink,format_args!("{}",value)).unwrap();
 assert!(alias&&sink.same);log("observed");Outcome{phase:"ok".into(),text:hex(text.as_bytes()),alias,display:hex(sink.text.as_bytes())}
}
fn capture()->Outcome{std::panic::catch_unwind(execute).unwrap_or_else(|_|Outcome{phase:format!("{}:panic",PHASE.with(Cell::get)),text:String::new(),alias:false,display:String::new()})}
struct Outer;
impl Drop for Outer{fn drop(&mut self){log("outer-drop");let result=if config().mode==1{capture()}else{execute()};RESULT.with(|slot|*slot.borrow_mut()=Some(result))}}
fn observe()->String{
 let cfg=config();TRACE.with(|trace|trace.borrow_mut().clear());STATE.with(|state|state.set(Snapshot::default()));SCOPE.with(|scope|scope.set("op"));
 let result=match cfg.mode{
 0=>capture(),
 1|2=>{let _=std::panic::catch_unwind(||{let _outer=Outer;panic!("outer")});RESULT.with(|slot|slot.borrow_mut().take().unwrap())},
 3=>{std::panic::set_hook(Box::new(|_|{log("processing");let outcome=execute();RESULT.with(|slot|*slot.borrow_mut()=Some(outcome))}));let _=std::panic::catch_unwind(||panic!("outer"));std::panic::set_hook(Box::new(|_|{}));RESULT.with(|slot|slot.borrow_mut().take().unwrap())},
 _=>panic!("invalid mode"),
 };
 SCOPE.with(|scope|scope.set("after"));let after=std::panic::catch_unwind(||{let stored=STORED.with(|slot|slot.borrow_mut().take());drop(stored)}).is_ok();
 let snapshot=STATE.with(Cell::get);let trace=TRACE.with(|trace|trace.borrow().join("|"));
 format!("{}\t{}\t{}\t{}\t{}\t{}:{}:{}:{}:{}:{}\t{}",result.phase,result.text,result.alias,result.display,after,snapshot.counter,snapshot.a,snapshot.b,snapshot.self_drops,snapshot.a_drops,snapshot.b_drops,trace)
}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));
 for line in io::stdin().lock().lines(){let line=line.unwrap();let p=line.split('\t').collect::<Vec<_>>();assert_eq!(p.len(),10);
  let config=Config{kind:p[0].parse().unwrap(),width:p[1].parse().unwrap(),fixed:p[2]=="1",base:p[3].parse().unwrap(),n:(u128::from(p[4].parse::<u64>().unwrap())<<64)|u128::from(p[5].parse::<u64>().unwrap()),mask:p[6].parse().unwrap(),disposition:p[7].parse().unwrap(),mode:p[8].parse().unwrap()};
  assert_eq!(p[9],"v1");CONFIG.with(|slot|slot.set(config));println!("{}",observe());
 }
}
