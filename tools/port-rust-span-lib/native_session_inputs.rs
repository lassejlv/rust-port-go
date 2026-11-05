#![feature(fn_traits,unboxed_closures)]
extern crate rustc_span;
use rustc_span::{SessionGlobals,SourceFileHashAlgorithm,edition::Edition};
use rustc_span::source_map::{FileLoader,SourceMapInputs,FilePathMapping};
use std::{fmt,io,path::{Path,PathBuf},sync::{Arc,Mutex}};
type Log=Arc<Mutex<Vec<String>>>;
fn event(log:&Log,text:String){if std::env::var_os("FERRUGO_TRACE_EVENTS").is_some(){eprintln!("EVENT:{text}")};log.lock().unwrap().push(text)}
struct IoError{log:Log,mode:u8}
impl fmt::Display for IoError{fn fmt(&self,_:&mut fmt::Formatter<'_>)->fmt::Result{event(&self.log,"unexpected_display".into());panic!("unexpected display")}}
impl fmt::Debug for IoError{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{event(&self.log,format!("error_debug:{}",std::thread::panicking()));if self.mode==3{panic!("debug")};f.write_str("IoError")}}
impl std::error::Error for IoError{}
impl Drop for IoError{fn drop(&mut self){event(&self.log,format!("error_drop:{}",std::thread::panicking()));if self.mode==4{panic!("error drop")}}}
struct Loader{log:Log,mode:u8}
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{panic!("unexpected exists")}fn read_file(&self,_:&Path)->io::Result<String>{panic!("unexpected read")}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{panic!("unexpected binary")}fn current_directory(&self)->io::Result<PathBuf>{event(&self.log,format!("cwd:{}",std::thread::panicking()));match self.mode{1=>panic!("cwd"),2|3|4=>Err(io::Error::other(IoError{log:self.log.clone(),mode:self.mode})),_=>Ok("/cwd".into())}}}
impl Drop for Loader{fn drop(&mut self){event(&self.log,format!("loader_drop:{}",std::thread::panicking()));if self.mode==5||self.mode==6{panic!("loader drop")}}}
struct F{log:Log,calls:u32}
impl FnOnce<()>for F{type Output=u32;extern "rust-call" fn call_once(mut self,_:())->u32{event(&self.log,format!("f_call:{}:{}",self.calls,std::thread::panicking()));self.calls+=1;42}}
impl Drop for F{fn drop(&mut self){event(&self.log,format!("f_drop:{}:{}",self.calls,std::thread::panicking()))}}
fn execute(op:&str,mode:u8)->String{let log:Log=Default::default();let inputs=SourceMapInputs{file_loader:Box::new(Loader{log:log.clone(),mode}),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None};let f=F{log:log.clone(),calls:0};let extras:&[&str]=if mode>=6{&["port_duplicate","port_duplicate"]}else{&[]};let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||match op{
 "construct"=>{let s=SessionGlobals::new(Edition::Edition2024,extras,Some(inputs));drop(s);drop(f)},
 "create"=>{let n=rustc_span::create_session_globals_then(Edition::Edition2024,extras,Some(inputs),f);event(&log,format!("result:{n}"))},
 "present"=>rustc_span::create_default_session_globals_then(||{let _=rustc_span::create_session_globals_then(Edition::Edition2018,extras,Some(inputs),f);}),
 _=>unreachable!()
 }));event(&log,format!("panic:{}",result.is_err()));let out=log.lock().unwrap().join(",");out}
fn main(){std::panic::set_hook(Box::new(|_|{}));if let Some(op)=std::env::args().nth(1){let mode=std::env::args().nth(2).unwrap().parse().unwrap();let out=execute(&op,mode);println!("{out}");return};for op in["construct","create","present"]{for mode in[0,1,2,5,7]{if op=="present"&&mode==5{continue};println!("{op}\t{mode}\t{}",execute(op,mode))}}}
