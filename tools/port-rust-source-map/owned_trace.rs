#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual full SourceMap + owned failure callbacks under a genuine subscriber.
use std::{fmt,io,path::{Path,PathBuf},sync::{Arc,atomic::{AtomicUsize,Ordering}},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::SourceFileHashAlgorithm;
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
use tracing::{Subscriber,Metadata,Event,span::{Attributes,Id,Record},field::{Field,Visit}};
fn trace(phase:&str){tracing::info!(phase);}
fn debug_panic(){panic!("error Debug panic")}
fn error_panic(){panic!("error Drop panic")}
fn loader_panic(){panic!("loader Drop panic")}
fn outer_panic(){panic!("outer")}
static LOADER:AtomicUsize=AtomicUsize::new(0);
static ERROR:AtomicUsize=AtomicUsize::new(0);
fn has(value:&str)->bool{std::env::args().nth(2).unwrap().contains(value)}
struct Visitor;
impl Visit for Visitor{fn record_debug(&mut self,field:&Field,value:&dyn fmt::Debug){println!("field:{}:{:?}:{}",field.name(),value,std::thread::panicking())}fn record_str(&mut self,field:&Field,value:&str){println!("str:{}:{}:{}",field.name(),value,std::thread::panicking())}}
struct Sub;
impl Drop for Sub{fn drop(&mut self){println!("subscriber_drop:{}",std::thread::panicking())}}
impl Subscriber for Sub{
 fn enabled(&self,m:&Metadata<'_>)->bool{m.is_event()}
 fn new_span(&self,_:&Attributes<'_>)->Id{Id::from_u64(1)}
 fn record(&self,_:&Id,_:&Record<'_>){}
 fn record_follows_from(&self,_:&Id,_:&Id){}
 fn event(&self,event:&Event<'_>){let m=event.metadata();println!("event:{}:{}:{}:{}",m.name(),m.file().unwrap_or(""),m.line().unwrap_or(0),std::thread::panicking());event.record(&mut Visitor)}
 fn enter(&self,_:&Id){}
 fn exit(&self,_:&Id){}
}
struct Local;
impl Drop for Local{fn drop(&mut self){println!("debug_local_drop:{}",std::thread::panicking());trace("debug_local_drop")}}
struct E{tag:Vec<u8>}
impl fmt::Debug for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{assert_eq!(self.tag,[7,8]);ERROR.store(self as *const _ as usize,Ordering::SeqCst);println!("error_debug:{}",std::thread::panicking());trace("error_debug");let _local=Local;if has("debug"){debug_panic()};f.write_str("E")}}
impl fmt::Display for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{f.write_str("E")}}
impl std::error::Error for E{}
impl Drop for E{fn drop(&mut self){println!("error_drop:{}:{}",std::thread::panicking(),ERROR.load(Ordering::SeqCst)==self as *const _ as usize);trace("error_drop");if has("error"){error_panic()}}}
struct Loader{tag:Vec<u8>}
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{assert_eq!(self.tag,[1,2]);LOADER.store(self as *const _ as usize,Ordering::SeqCst);println!("cwd:{}",std::thread::panicking());trace("cwd");Err(io::Error::other(E{tag:vec![7,8]}))}}
impl Drop for Loader{fn drop(&mut self){println!("loader_drop:{}:{}",std::thread::panicking(),LOADER.load(Ordering::SeqCst)==self as *const _ as usize);trace("loader_drop");if has("loader"){loader_panic()}}}
fn construct(){let _=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader{tag:vec![1,2]}),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});}
fn report(f:impl FnOnce()){let r=catch_unwind(AssertUnwindSafe(f));println!("caught:{}:{}",r.is_err(),std::thread::panicking());}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer_drop:{}",std::thread::panicking());trace("outer_drop");if self.caught{report(construct)}else{construct()}}}
fn main(){let mode=std::env::args().nth(1).unwrap();std::panic::set_hook(Box::new(move|p|{let l=p.location().unwrap();let path=if l.file().ends_with("owned_trace.rs"){"owned_trace.rs"}else{l.file()};let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};println!("hook:{}:{}:{}:{}:{}",std::thread::panicking(),kind,path,l.line(),l.column());println!("payload:{}",p.payload_as_str().unwrap_or("not-string"));trace("hook");if mode=="processing"&&p.payload_as_str()==Some("outer"){construct();}}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);tracing::subscriber::with_default(Sub,||match std::env::args().nth(1).unwrap().as_str(){"normal"=>report(construct),"inherited_caught"|"inherited_escape"=>report(||{let _outer=Outer{caught:std::env::args().nth(1).unwrap()=="inherited_caught"};outer_panic()}),"processing"=>report(outer_panic),_=>panic!("invalid mode")})}
