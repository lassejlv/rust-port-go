// SPDX-License-Identifier: MIT OR Apache-2.0
#![feature(rustc_private)]
use rustc_span::{RealFileName,RemapPathScopeComponents};
use rustc_span::source_map::FilePathMapping;
use std::{borrow::Cow,path::{Path,PathBuf},cell::RefCell,sync::{Arc,atomic::{AtomicUsize,Ordering}},io::{self,Write},panic::{self,AssertUnwindSafe,catch_unwind}};
fn event(message:impl AsRef<str>){println!("{}",message.as_ref());io::stdout().flush().unwrap()}
struct Token{value:usize,drops:Arc<AtomicUsize>}
impl Drop for Token{fn drop(&mut self){event(format!("token-drop:{}:{}",self.value,std::thread::panicking()));self.drops.fetch_add(1,Ordering::SeqCst);}}
struct Input{path:Cow<'static,Path>,token:Token,action:String}
impl Drop for Input{fn drop(&mut self){event(format!("self-drop:{}:{}:{}",self.path.as_os_str().is_empty(),self.token.value,std::thread::panicking()));}}
thread_local!{static RETAINED:RefCell<Option<Input>>=const{RefCell::new(None)};}
impl Into<Cow<'static,Path>> for Input{
 fn into(mut self)->Cow<'static,Path>{
  event(format!("into:{}:{}:{}",self.action,self.token.value,std::thread::panicking()));
  self.token.value+=100;
  let result=std::mem::replace(&mut self.path,Cow::Borrowed(Path::new("")));
  event(format!("prepared:{}:{}",matches!(result,Cow::Owned(_)),std::thread::panicking()));
  match self.action.as_str(){
   "take"=>{event("take-self");RETAINED.with(|slot|*slot.borrow_mut()=Some(self));},
   "forget"=>{event("forget-self");std::mem::forget(self);},
   "forget_panic"=>{event("forget-self");std::mem::forget(self);panic::panic_any("convert");},
   "drop"=>{},_=>unreachable!()
  }
  result
 }
}
fn operation(action:&str){
 let drops=Arc::new(AtomicUsize::new(0));
 let mapping=FilePathMapping::new(vec![("src".into(),"V".into())],RemapPathScopeComponents::all());
 let working=FilePathMapping::empty().to_real_filename(&RealFileName::empty(),Path::new("/wd"));
 let result=catch_unwind(AssertUnwindSafe(||{
  let input=Input{path:Cow::Owned(PathBuf::from("src/input.rs")),token:Token{value:7,drops:drops.clone()},action:action.into()};
  let result=mapping.to_real_filename(&working,input);
  event(format!("result:{result:?}:{}",std::thread::panicking()));
 }));
 event(format!("caught:{}:{}",result.is_err(),std::thread::panicking()));
 event(format!("state:{}:{}",drops.load(Ordering::SeqCst),Arc::strong_count(&drops)));
 RETAINED.with(|slot|drop(slot.borrow_mut().take()));
 event(format!("final:{}:{}",drops.load(Ordering::SeqCst),Arc::strong_count(&drops)));
}
struct Outer{action:String,caught:bool}
impl Drop for Outer{fn drop(&mut self){event(format!("outer-drop:{}",std::thread::panicking()));if self.caught{operation(&self.action)}else{
 let drops=Arc::new(AtomicUsize::new(0));let mapping=FilePathMapping::empty();let working=mapping.to_real_filename(&RealFileName::empty(),Path::new("/wd"));
 let input=Input{path:Cow::Owned(PathBuf::from("src/input.rs")),token:Token{value:7,drops},action:self.action.clone()};
 let result=mapping.to_real_filename(&working,input);event(format!("escaped-result:{result:?}"));RETAINED.with(|slot|drop(slot.borrow_mut().take()));
}}}
fn main(){
 let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let action=args[2].clone();let processing=mode=="processing";let hook_action=action.clone();
 panic::set_hook(Box::new(move|info|{let location=info.location().unwrap();event(format!("hook:{}:{}:{}:{}:{}",info.payload().downcast_ref::<&str>().copied().unwrap_or("opaque"),std::thread::panicking(),location.file(),location.line(),location.column()));if processing{operation(&hook_action)}}));
 if mode=="normal"{operation(&action)}else if processing{let _=catch_unwind(||panic::panic_any("outer"));event("outer-end");}else{let caught=mode=="inherited_caught";let result=catch_unwind(AssertUnwindSafe(||{let _outer=Outer{action,caught};panic::panic_any("outer")}));event(format!("outer-caught:{}",result.is_err()));}
}
