#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete unchanged SourceMap constructor, owned custom io::Error and loader.
use std::{fmt,io,path::{Path,PathBuf},sync::{Arc,atomic::{AtomicUsize,Ordering}},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{SourceFileHashAlgorithm};
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
static LOADER:AtomicUsize=AtomicUsize::new(0);
static ERROR:AtomicUsize=AtomicUsize::new(0);
#[derive(Clone,Copy)]struct Settings{debug:bool,error:bool,loader:bool}
fn settings()->Settings{let c=std::env::args().nth(2).unwrap();Settings{debug:c.contains("debug"),error:c.contains("error"),loader:c.contains("loader")}}
struct Local;
impl Drop for Local{fn drop(&mut self){println!("debug_local_drop:{}",std::thread::panicking())}}
struct E{tag:Vec<u8>}
impl fmt::Debug for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{assert_eq!(self.tag,[7,8]);ERROR.store(self as *const _ as usize,Ordering::SeqCst);println!("error_debug:{}",std::thread::panicking());let _local=Local;if settings().debug{panic!("error Debug panic")};if std::env::args().nth(2).as_deref()==Some("format"){f.write_str("partial")?;return Err(fmt::Error)};f.write_str("E")}}
impl fmt::Display for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{f.write_str("E")}}
impl std::error::Error for E{}
impl Drop for E{fn drop(&mut self){println!("error_drop:{}:{}",std::thread::panicking(),ERROR.load(Ordering::SeqCst)==self as *const _ as usize);if settings().error{panic!("error Drop panic")}}}
struct Loader{tag:Vec<u8>}
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{assert_eq!(self.tag,[1,2]);LOADER.store(self as *const _ as usize,Ordering::SeqCst);println!("cwd:{}",std::thread::panicking());Err(io::Error::other(E{tag:vec![7,8]}))}}
impl Drop for Loader{fn drop(&mut self){println!("loader_drop:{}:{}",std::thread::panicking(),LOADER.load(Ordering::SeqCst)==self as *const _ as usize);if settings().loader{panic!("loader Drop panic")}}}
fn construct(){let _=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader{tag:vec![1,2]}),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});}
fn report(f:impl FnOnce()){let r=catch_unwind(AssertUnwindSafe(f));println!("caught:{}:{}",r.is_err(),std::thread::panicking());}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer_drop:{}",std::thread::panicking());if self.caught{report(construct)}else{construct()}}}
fn main(){let mode=std::env::args().nth(1).unwrap();std::panic::set_hook(Box::new(move|p|{let l=p.location().unwrap();let path=l.file();let path=if path.ends_with("owned_runtime.rs"){"owned_runtime.rs"}else{path};let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};println!("hook:{}:{}:{}:{}:{}",std::thread::panicking(),kind,path,l.line(),l.column());println!("payload:{}",p.payload_as_str().unwrap_or("not-string"));if mode=="processing"&&p.payload_as_str()==Some("outer"){construct();}}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);match std::env::args().nth(1).unwrap().as_str(){"normal"=>report(construct),"inherited_caught"|"inherited_escape"=>report(||{let _outer=Outer{caught:std::env::args().nth(1).unwrap()=="inherited_caught"};panic!("outer")}),"processing"=>report(||panic!("outer")),_=>panic!("invalid mode")}}
