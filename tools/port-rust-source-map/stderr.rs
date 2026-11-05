#![feature(rustc_private,internal_output_capture)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Whole SourceMap fatal registration path and actual SDK stderr panic hooks.
use std::{io,path::{Path,PathBuf},sync::Arc,panic::{catch_unwind,AssertUnwindSafe}};
use rustc_span::{FileName,SourceFile,SourceFileHashAlgorithm};
use rustc_span::def_id::CrateNum;
use rustc_span::source_map::{SourceMap,SourceMapInputs,FileLoader,FilePathMapping};
unsafe extern "C"{fn close(fd:i32)->i32;}
fn outer_panic(){panic!("outer")}
struct Loader;
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{Ok("/foo".into())}}
fn construct(){let sm=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader),path_mapping:FilePathMapping::empty(),hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});let f=SourceFile::new(FileName::Custom("max".into()),"".into(),SourceFileHashAlgorithm::Md5,None).unwrap();sm.new_imported_source_file(f.name,f.src_hash,f.checksum_hash,f.stable_id,u32::MAX,f.unnormalized_source_len,CrateNum::from_u32(1),f.lines,f.multibyte_chars,f.normalized_pos,0);let variant=std::env::args().nth(2).unwrap();let _capture=(variant=="capture").then(CaptureGuard::new);let _held=(variant=="held").then(||Held(std::io::stderr().lock()));sm.new_source_file(FileName::Custom("fatal".into()),"a".into());}
fn report(f:impl FnOnce()){match catch_unwind(AssertUnwindSafe(f)){Ok(())=>println!("caught:false:false:{}",std::thread::panicking()),Err(e)=>println!("caught:true:{}:{}",e.is::<rustc_span::fatal_error::FatalErrorMarker>(),std::thread::panicking())}}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer_drop:{}",std::thread::panicking());if self.caught{report(construct)}else{construct()}}}
struct Held(std::io::StderrLock<'static>);
impl Drop for Held{fn drop(&mut self){println!("held_drop:{}",std::thread::panicking())}}
struct CaptureGuard(Arc<std::sync::Mutex<Vec<u8>>>);
impl CaptureGuard{fn new()->Self{let buffer=Arc::new(std::sync::Mutex::new(Vec::new()));std::io::set_output_capture(Some(Arc::clone(&buffer)));Self(buffer)}}
impl Drop for CaptureGuard{fn drop(&mut self){std::io::set_output_capture(None);let bytes=self.0.lock().unwrap();let hex:String=bytes.iter().map(|b|format!("{b:02x}")).collect();println!("capture:{hex}:{}",std::thread::panicking())}}
fn main(){let mode=std::env::args().nth(1).unwrap();if std::env::args().nth(2).as_deref()==Some("closed"){unsafe{close(2);}}std::panic::set_hook(Box::new(move|p|{let l=p.location().unwrap();let file=if l.file().ends_with("stderr.rs"){"stderr.rs"}else{l.file()};let kind=if p.payload().is::<String>(){"owned_string"}else if p.payload().is::<&str>(){"static_str"}else{"opaque"};println!("hook:{}:{}:{}:{}:{}",std::thread::panicking(),kind,file,l.line(),l.column());println!("payload:{}",p.payload_as_str().unwrap_or("opaque"));if mode=="processing"&&p.payload_as_str()==Some("outer"){construct();}}));rustc_data_structures::sync::set_dyn_thread_safe_mode(true);match std::env::args().nth(1).unwrap().as_str(){"normal"=>report(construct),"inherited_caught"|"inherited_escape"=>report(||{let _outer=Outer{caught:std::env::args().nth(1).unwrap()=="inherited_caught"};outer_panic()}),"processing"=>report(outer_panic),_=>panic!("invalid mode")}}
