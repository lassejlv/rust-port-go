#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Real whole-crate SourceMap Debug. Only opaque loader-slot digits normalized.
use std::{cell::RefCell,fmt,fmt::Write,io,path::{Path,PathBuf},sync::Arc};
use rustc_span::{FileName,SourceFileHashAlgorithm,RemapPathScopeComponents,RealFileName};
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
use std::os::unix::ffi::OsStrExt;
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
struct Loader;
impl FileLoader for Loader{fn file_exists(&self,_:&Path)->bool{true}fn read_file(&self,_:&Path)->io::Result<String>{Ok("".into())}fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{Ok(Arc::from(&b""[..]))}fn current_directory(&self)->io::Result<PathBuf>{Ok("/foo".into())}}
fn map(scenario:usize,bits:u8)->SourceMap{let mapping=FilePathMapping::new(vec![("/foo".into(),"FOO".into()),("/bar".into(),"BAR".into())],RemapPathScopeComponents::from_bits_retain(bits));let sm=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader),path_mapping:mapping,hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:Some(SourceFileHashAlgorithm::Sha1)});if scenario>0{let name=sm.path_mapping().to_real_filename(sm.working_dir(),Path::new("/foo/input.rs"));sm.new_source_file(FileName::Real(name),"a…b".into());}if scenario>1{sm.new_source_file(FileName::Custom("custom\n\"\\ 🦀\u{200d}".into()),"x".into());let path=Path::new(std::ffi::OsStr::from_bytes(b"/invalid/\xff\xed\xa0\x80\0"));sm.new_source_file(FileName::Real(RealFileName::from_virtual_path(path)),"z".into());}sm}
struct Sink{out:String,calls:Vec<String>,n:usize,fail:usize,address:String}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.n+=1;let normalized=if s==self.address{"<ptr>".to_owned()}else if s==format!("0x{}",self.address){"0x<ptr>".to_owned()}else{s.to_owned()};self.calls.push(format!("S:{}",hex(&normalized)));if self.n==self.fail{return Err(fmt::Error)}self.out.push_str(&normalized);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{self.n+=1;self.calls.push(format!("C:{:x}",c as u32));if self.n==self.fail{return Err(fmt::Error)}self.out.push(c);Ok(())}
}
struct Probe<'a>{map:&'a SourceMap,after:&'a RefCell<String>}
impl fmt::Debug for Probe<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let first=fmt::Debug::fmt(self.map,f);*self.after.borrow_mut()=format!("{}:{:?}:{:?}:{}:{:?}:{}:{}:{}",f.alternate(),f.width(),f.precision(),f.fill() as u32,f.align(),f.sign_plus(),f.sign_minus(),f.sign_aware_zero_pad());let second=fmt::Debug::fmt(&7u16,f);if first.is_ok()&&second.is_ok(){Ok(())}else{Err(fmt::Error)}}}
fn format(sink:&mut Sink,style:usize,value:&Probe<'_>)->fmt::Result{match style{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:*^12.3?}"),3=>write!(sink,"{value:+012?}"),4=>write!(sink,"{value:*<12?}"),5=>write!(sink,"{value:λ^12.3?}"),6=>write!(sink,"{value:x?}"),7=>write!(sink,"{value:#X?}"),8=>write!(sink,"{value:\0>12?}"),_=>unreachable!()}}
fn main(){rustc_data_structures::sync::set_dyn_thread_safe_mode(true);rustc_span::create_default_session_globals_then(||{for scenario in 0..3{for bits in [0,2,56,255]{let sm=map(scenario,bits);let baseline=format!("{sm:?}");let prefix="<file_loader@0x";let start=baseline.find(prefix).unwrap()+prefix.len();let end=start+baseline[start..].find('>').unwrap();let address=&baseline[start..end];for style in 0..9{for fail in (0..=10).chain([20,40,60,80,120,160,240,320,400]){let after=RefCell::new(String::new());let mut sink=Sink{out:String::new(),calls:vec![],n:0,fail,address:address.into()};let result=format(&mut sink,style,&Probe{map:&sm,after:&after});println!("{scenario}\t{bits}\t{style}\t{fail}\t{}\t{}\t{}\t{}",result.is_ok(),hex(sink.out),hex(sink.calls.join("\n")),hex(after.into_inner()));}}}}});}
