#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine public to_real_filename(custom Into<Cow<Path>>) consumption.
use std::{borrow::Cow,path::{Path,PathBuf},sync::atomic::{AtomicUsize,Ordering},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{RealFileName,RemapPathScopeComponents};use rustc_span::source_map::FilePathMapping;
static ADDRESS:AtomicUsize=AtomicUsize::new(0);
fn body_panic(){panic!("body")}
fn env_panic(){panic!("env")}
fn outer_panic(){panic!("outer")}
struct Input{path:Cow<'static,Path>,action:String,tag:Vec<u8>}
impl Into<Cow<'static,Path>> for Input{fn into(mut self)->Cow<'static,Path>{assert_eq!(self.tag,[1,2]);ADDRESS.store(&self as*const Self as usize,Ordering::SeqCst);println!("into:{}:{}",matches!(self.path,Cow::Owned(_)),std::thread::panicking());if self.action=="body"||self.action=="both"{body_panic()}let path=std::mem::take(&mut self.path);println!("prepared:{}",matches!(path,Cow::Owned(_)));path}}
impl Drop for Input{fn drop(&mut self){println!("input_drop:{}:{}:{}:{}",ADDRESS.load(Ordering::SeqCst)==self as*const Self as usize,matches!(self.path,Cow::Owned(_)),self.path.as_os_str().is_empty(),std::thread::panicking());if self.action=="env"||self.action=="both"{env_panic()}}}
fn invoke(path:usize,owned:bool,action:&str){let paths=["abc/in.rs","abc","other",""];let input=if owned{Cow::Owned(PathBuf::from(paths[path]))}else{Cow::Borrowed(Path::new(paths[path]))};let mapping=FilePathMapping::new(vec![("abc".into(),"REMAPPED".into())],RemapPathScopeComponents::all());let wd=FilePathMapping::empty().to_real_filename(&RealFileName::empty(),Path::new("/wd"));let out=mapping.to_real_filename(&wd,Input{path:input,action:action.into(),tag:vec![1,2]});println!("result:{out:?}:{}",std::thread::panicking());}
fn report(f:impl FnOnce()){let r=catch_unwind(AssertUnwindSafe(f));let tag=match r{Ok(())=>"none",Err(ref p)=>p.downcast_ref::<&str>().copied().unwrap_or("other")};println!("caught:{tag}:{}",std::thread::panicking());}
struct Outer{path:usize,owned:bool,action:String,caught:bool}impl Drop for Outer{fn drop(&mut self){println!("outer_drop:{}",std::thread::panicking());if self.caught{report(||invoke(self.path,self.owned,&self.action))}else{invoke(self.path,self.owned,&self.action)}}}
fn main(){let args:Vec<String>=std::env::args().collect();let(mode,action,path,owned)=(args[1].clone(),args[2].clone(),args[3].parse::<usize>().unwrap(),args[4]=="1");std::panic::set_hook(Box::new(move|p|{let l=p.location().unwrap();let file=if l.file().ends_with("owned_into.rs"){"owned_into.rs"}else{l.file()};let kind=if p.payload().is::<String>(){"owned_string"}else{"static_str"};println!("hook:{kind}:{file}:{}:{}:{}",l.line(),l.column(),p.payload_as_str().unwrap_or("opaque"));if mode=="processing"&&p.payload_as_str()==Some("outer"){invoke(path,owned,&action);println!("hook_return:{}",std::thread::panicking())}}));match args[1].as_str(){"normal"=>report(||invoke(path,owned,&args[2])),"processing"=>report(outer_panic),"inherited_caught"|"inherited_escape"=>report(||{let _outer=Outer{path,owned,action:args[2].clone(),caught:args[1]=="inherited_caught"};outer_panic()}),_=>unreachable!()}}
