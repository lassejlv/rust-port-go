// Actual Rust assignment and Clone::clone_from provided-default observer.
#![feature(panic_can_unwind,panic_internals)]
use serde_json::json;
use std::{cell::{Cell,RefCell},panic::{self,AssertUnwindSafe}};
thread_local!{static OBS:RefCell<String>=RefCell::new("none".into());static ANCHOR:Cell<usize>=const{Cell::new(0)};static FAILURE:RefCell<String>=RefCell::new(String::new());}
fn event(s:String){println!("E:{s}")}
fn observer()->String{OBS.with(|o|o.borrow().clone())}
struct Local(u32);impl Drop for Local{fn drop(&mut self){event(format!("local:{}:{}:{}",self.0,observer(),std::thread::panicking()))}}
struct Field{id:u32,data:Vec<u8>}
impl Drop for Field{fn drop(&mut self){event(format!("field:{}:{}:{}",self.id,observer(),std::thread::panicking()));if self.id==1&&FAILURE.with(|f|{let f=f.borrow();*f=="field"||*f=="both"}){panic!("field")}}}
struct V{id:u32,field:Field}
impl Clone for V{fn clone(&self)->Self{event(format!("clone:{}:{}:{}",self.id,observer(),std::thread::panicking()));Self{id:self.id+10,field:Field{id:self.id+10,data:self.field.data.clone()}}}}
impl Drop for V{fn drop(&mut self){let original=self.id;let same=ANCHOR.with(|p|p.get()==self as*const Self as usize);event(format!("drop:{original}:{}:{}:{same}",observer(),std::thread::panicking()));let _g=Local(original);self.id+=1000;event(format!("mut:{}",self.id));if original==1&&FAILURE.with(|f|{let f=f.borrow();*f=="body"||*f=="both"}){panic!("body")}}}
fn action(op:&str){event(format!("enter:{}:{}",observer(),std::thread::panicking()));let mut target=V{id:1,field:Field{id:1,data:vec![1,2,3]}};let mut replacement=Some(V{id:2,field:Field{id:2,data:vec![4,5]}});ANCHOR.with(|p|p.set(&target as*const V as usize));
 let result=panic::catch_unwind(AssertUnwindSafe(||if op=="assign"{target=replacement.take().unwrap()}else{target.clone_from(replacement.as_ref().unwrap())}));
 event(format!("caught:{}",result.is_err()));event(format!("state:{}:{}:{}",target.id,target.field.id,ANCHOR.with(|p|p.get()==&target as*const V as usize)));ANCHOR.with(|p|p.set(0));
}
struct Outer(String);impl Drop for Outer{fn drop(&mut self){action(&self.0)}}
fn hook(info:&panic::PanicHookInfo<'_>){if observer()=="none"{return}let(payload,kind)=if let Some(v)=info.payload().downcast_ref::<&str>(){((*v).to_owned(),"static_str")}else if let Some(v)=info.payload().downcast_ref::<String>(){(v.clone(),"owned_string")}else{("opaque".into(),"opaque")};println!("H:{}",json!({"Payload":payload,"Kind":kind,"Location":info.location().map(|l|json!({"File":l.file(),"Line":l.line(),"Column":l.column()})),"CanUnwind":info.can_unwind(),"ForceNoBacktrace":info.force_no_backtrace()}))}
fn main(){let args:Vec<_>=std::env::args().collect();let(op,context,scope,failure)=(args[1].clone(),&args[2],&args[3],&args[4]);OBS.with(|o|*o.borrow_mut()=scope.clone());FAILURE.with(|f|*f.borrow_mut()=failure.clone());panic::set_hook(Box::new(hook));
 match context.as_str(){"inherited"=>{let _=panic::catch_unwind(AssertUnwindSafe(||{let _g=Outer(op);panic!("outer")}));event("outer-caught:true".into())},"processing"=>{panic::set_hook(Box::new(move|info|{hook(info);action(&op)}));let _=panic::catch_unwind(||panic!("outer"));event("outer-caught:true".into())},_=>action(&op)}
}
