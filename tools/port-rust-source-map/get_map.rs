#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Public actual scoped get_source_map and returned Arc/container behavior.
use std::{io,path::{Path,PathBuf},sync::Arc};
use rustc_span::{FileName,SourceFileHashAlgorithm,edition::Edition};
use rustc_span::source_map::{FileLoader,SourceMapInputs,FilePathMapping,get_source_map};
struct Loader;
impl Drop for Loader{fn drop(&mut self){println!("loader_drop:{}",std::thread::panicking())}}
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{Ok("/foo".into())}}
fn main(){rustc_data_structures::sync::set_dyn_thread_safe_mode(true);for present in [false,true]{let inputs=present.then(||SourceMapInputs{file_loader:Box::new(Loader),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});rustc_span::create_session_globals_then(Edition::Edition2024,&[],inputs,||{if let Some(a)=get_source_map(){let b=get_source_map().unwrap();println!("some:{}:{}:{}",Arc::strong_count(&a),Arc::strong_count(&b),Arc::ptr_eq(&a,&b));let file=a.new_source_file(FileName::Custom("getter.rs".into()),"abc".into());drop(file);println!("shared_files:{}",b.files().len());drop(a);println!("after_drop:{}",Arc::strong_count(&b));drop(b);}else{println!("none:{}",get_source_map().is_none())}});println!("after:{present}")}}
