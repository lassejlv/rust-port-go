#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete unchanged SourceMap expect/hook/cell-ownership observation.
use std::{fmt,io,path::{Path,PathBuf},sync::{Arc,atomic::{AtomicUsize,AtomicBool,Ordering}},panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{SourceFileHashAlgorithm};
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
static PANIC_DEBUG:AtomicBool=AtomicBool::new(false);
static LOADER:AtomicUsize=AtomicUsize::new(0);static ERROR:AtomicUsize=AtomicUsize::new(0);
struct Local;
impl Drop for Local{fn drop(&mut self){println!("debug_local_drop:{}",std::thread::panicking())}}
struct E { tag: Vec<u8> }
impl fmt::Debug for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{assert_eq!(self.tag,[7,8]);ERROR.store(self as *const _ as usize,Ordering::SeqCst);println!("error_debug:{}",std::thread::panicking());let _local=Local;if PANIC_DEBUG.load(Ordering::SeqCst){panic!("error Debug panic")};f.write_str("E")}}
impl fmt::Display for E{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{f.write_str("E")}}
impl std::error::Error for E{}
impl Drop for E{fn drop(&mut self){println!("error_drop:{}:{}",std::thread::panicking(),ERROR.load(Ordering::SeqCst)==self as *const _ as usize)}}
struct Loader { tag: Vec<u8> }
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{assert_eq!(self.tag,[1,2]);LOADER.store(self as *const _ as usize,Ordering::SeqCst);println!("cwd:{}",std::thread::panicking());Err(io::Error::other(E{tag:vec![7,8]}))}}
impl Drop for Loader{fn drop(&mut self){println!("loader_drop:{}:{}",std::thread::panicking(),LOADER.load(Ordering::SeqCst)==self as *const _ as usize)}}
fn main(){PANIC_DEBUG.store(std::env::args().nth(1).as_deref()==Some("debug_panic"),Ordering::SeqCst);std::panic::set_hook(Box::new(|p|{let l=p.location().unwrap();println!("hook:{}:{}:{}:{}",std::thread::panicking(),l.file(),l.line(),l.column());println!("payload:{}",p.payload_as_str().unwrap_or("not-string"));}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);let r=catch_unwind(AssertUnwindSafe(||{let _=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader{tag:vec![1,2]}),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});}));println!("caught:{}:{}",r.is_err(),std::thread::panicking());}
