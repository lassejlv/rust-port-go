#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Full original SourceMap + real tracing subscriber, never a source substitute.
use std::{fmt,fmt::Write,io,path::{Path,PathBuf},sync::{Arc,Mutex},panic::{catch_unwind,AssertUnwindSafe}};
use tracing::{Subscriber,Metadata,Event,span::{Attributes,Id,Record},subscriber::Interest,metadata::LevelFilter,field::{Field,Visit}};
use rustc_span::{BytePos,Span,FileName,SourceFileHashAlgorithm};
use rustc_span::source_map::{SourceMap,SourceMapInputs,FilePathMapping,FileLoader};
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Clone)]struct Log(Arc<Mutex<Vec<String>>>);
impl Log{fn push(&self,s:String){self.0.lock().unwrap_or_else(|p|p.into_inner()).push(s)}}
static VERBATIM:std::sync::atomic::AtomicBool=std::sync::atomic::AtomicBool::new(false);
fn meta(m:&Metadata<'_>)->String{let verbatim=VERBATIM.load(std::sync::atomic::Ordering::Relaxed);let name=if verbatim{m.name().to_owned()}else{m.name().replace("original/","")};let file=if verbatim{m.file().unwrap_or("")}else{m.file().unwrap_or("").trim_start_matches("original/")};format!("{}|{}|{}|{}|{}|{}|{}|{}",name,m.level(),m.target(),m.module_path().unwrap_or(""),file,m.line().unwrap_or(0),if m.is_span(){"span"}else{"event"},m.fields().iter().map(|f|f.name()).collect::<Vec<_>>().join(","))}

struct Sink{out:String,calls:Vec<String>,n:usize,fail:usize}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.n+=1;self.calls.push(format!("S:{}",hex(s)));if self.n==self.fail{return Err(fmt::Error)}self.out.push_str(s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{self.n+=1;self.calls.push(format!("C:{:x}",c as u32));if self.n==self.fail{return Err(fmt::Error)}self.out.push(c);Ok(())}
}
fn debug(sink:&mut Sink,style:u32,value:&dyn fmt::Debug)->fmt::Result{match style{0=>write!(sink,"{value:?}"),1=>write!(sink,"{value:#?}"),2=>write!(sink,"{value:*^12.3?}"),3=>write!(sink,"{value:#x?}"),_=>unreachable!()}}
struct Visitor{log:Log,style:u32,fail:usize}
impl Visit for Visitor{
 fn record_debug(&mut self,f:&Field,v:&dyn fmt::Debug){let mut sink=Sink{out:String::new(),calls:vec![],n:0,fail:self.fail};let result=debug(&mut sink,self.style,v);self.log.push(format!("debug|{}|{}|{}|{}|{}",f.name(),std::thread::panicking(),result.is_ok(),hex(sink.out),sink.calls.join(",")))}
 fn record_bool(&mut self,f:&Field,v:bool){self.log.push(format!("bool|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_i64(&mut self,f:&Field,v:i64){self.log.push(format!("i64|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_u64(&mut self,f:&Field,v:u64){self.log.push(format!("u64|{}|{}|{}",f.name(),v,std::thread::panicking()))}
 fn record_str(&mut self,f:&Field,v:&str){self.log.push(format!("str|{}|{}|{}",f.name(),hex(v),std::thread::panicking()))}
}
struct State{log:Log,interest:String,enabled:bool,max:LevelFilter,next:Mutex<u64>,style:u32,fail:usize}
impl State{fn visitor(&self)->Visitor{Visitor{log:self.log.clone(),style:self.style,fail:self.fail}}}
impl Drop for State{fn drop(&mut self){self.log.push(format!("subscriber_drop|{}",std::thread::panicking()))}}
impl Subscriber for State{
 fn register_callsite(&self,m:&'static Metadata<'static>)->Interest{self.log.push(format!("register|{}|{}",meta(m),std::thread::panicking()));match self.interest.as_str(){"always"=>Interest::always(),"never"=>Interest::never(),_=>Interest::sometimes()}}
 fn enabled(&self,m:&Metadata<'_>)->bool{self.log.push(format!("enabled|{}|{}",meta(m),std::thread::panicking()));self.enabled}
 fn max_level_hint(&self)->Option<LevelFilter>{self.log.push(format!("max|{}|{}",self.max,std::thread::panicking()));Some(self.max)}
 fn new_span(&self,a:&Attributes<'_>)->Id{let mut next=self.next.lock().unwrap();let id=*next;*next+=1;drop(next);self.log.push(format!("new|{}|{}|{}",id,meta(a.metadata()),std::thread::panicking()));a.record(&mut self.visitor());Id::from_u64(id)}
 fn record(&self,id:&Id,values:&Record<'_>){self.log.push(format!("record|{}|{}",id.into_u64(),std::thread::panicking()));values.record(&mut self.visitor())}
 fn record_follows_from(&self,id:&Id,f:&Id){self.log.push(format!("follows|{}|{}|{}",id.into_u64(),f.into_u64(),std::thread::panicking()))}
 fn event_enabled(&self,e:&Event<'_>)->bool{self.log.push(format!("event_enabled|{}|{}",meta(e.metadata()),std::thread::panicking()));true}
 fn event(&self,e:&Event<'_>){self.log.push(format!("event|{}|{}",meta(e.metadata()),std::thread::panicking()));e.record(&mut self.visitor())}
 fn enter(&self,id:&Id){self.log.push(format!("enter|{}|{}",id.into_u64(),std::thread::panicking()))}
 fn exit(&self,id:&Id){self.log.push(format!("exit|{}|{}",id.into_u64(),std::thread::panicking()))}
 fn clone_span(&self,id:&Id)->Id{self.log.push(format!("clone|{}|{}",id.into_u64(),std::thread::panicking()));id.clone()}
 fn try_close(&self,id:Id)->bool{self.log.push(format!("close|{}|{}",id.into_u64(),std::thread::panicking()));true}
}
struct Loader{log:Log}
impl Drop for Loader{fn drop(&mut self){self.log.push(format!("loader_drop|{}",std::thread::panicking()))}}
impl FileLoader for Loader{
 fn file_exists(&self,_:&Path)->bool{self.log.push(format!("exists|{}",std::thread::panicking()));true}
 fn read_file(&self,_:&Path)->io::Result<String>{self.log.push(format!("read|{}",std::thread::panicking()));Ok("a…b".into())}
 fn read_binary_file(&self,_:&Path)->io::Result<Arc<[u8]>>{self.log.push(format!("binary|{}",std::thread::panicking()));Ok(Arc::from(&b"a"[..]))}
 fn current_directory(&self)->io::Result<PathBuf>{self.log.push(format!("cwd|{}",std::thread::panicking()));Ok(PathBuf::from("/foo"))}
}
fn run(op:&str,log:Log){
 let mapping=FilePathMapping::new(vec![(PathBuf::from("/foo"),PathBuf::from("FOO")),(PathBuf::from("/foo/bar"),PathBuf::from("BAR"))],rustc_span::RemapPathScopeComponents::all());
 let sm=SourceMap::with_inputs(SourceMapInputs{file_loader:Box::new(Loader{log:log.clone()}),path_mapping:mapping,hash_kind:SourceFileHashAlgorithm::Md5,checksum_hash_kind:None});
 if op=="construct"{return}
 let source=if op=="malformed"{""}else{"a…b"};let file=sm.new_source_file(FileName::Custom("input.rs".into()),source.into());
 let s=Span::with_root_ctxt(BytePos(1),BytePos(4));
 let result=catch_unwind(AssertUnwindSafe(||{match op{
  "valid"=>{let _=sm.is_valid_span(s);},"lines"=>{let _=sm.span_to_lines(s);},"point"=>{let _=sm.next_point(Span::with_root_ctxt(BytePos(0),BytePos(1)));},"empty"=>{let _=sm.end_point(Span::with_root_ctxt(BytePos(1),BytePos(1)));},"malformed"=>{let _=sm.next_point(Span::with_root_ctxt(BytePos(0),BytePos(1)));},"different"=>{let other=sm.new_source_file(FileName::Custom("other.rs".into()),"z".into());let _=sm.next_point(Span::with_root_ctxt(BytePos(1),BytePos(other.start_pos.0+1)));},"remap"=>{let _=sm.path_mapping().to_real_filename(sm.working_dir(),PathBuf::from("/foo/bar/input.rs"));},"remap_miss"=>{let _=sm.path_mapping().to_real_filename(sm.working_dir(),PathBuf::from("/unknown/input.rs"));},"reverse"=>{let f=rustc_span::SourceFile::new(FileName::Custom("external-template".into()),"a…b".into(),SourceFileHashAlgorithm::Md5,None).unwrap();let foreign=sm.new_imported_source_file(FileName::Real(rustc_span::RealFileName::from_virtual_path(Path::new("FOO/input.rs"))),f.src_hash,f.checksum_hash,f.stable_id,f.normalized_source_len.0,f.unnormalized_source_len,rustc_span::def_id::CrateNum::from_u32(1),f.lines,f.multibyte_chars,f.normalized_pos,0);let _=sm.ensure_source_file_source_present(&foreign);},_=>panic!("bad operation")};}));
 log.push(format!("outcome|{}|{}|{}",result.is_ok(),Arc::strong_count(&file),std::thread::panicking()));
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let a:Vec<_>=std::env::args().collect();VERBATIM.store(a.get(7).map(String::as_str)==Some("verbatim"),std::sync::atomic::Ordering::Relaxed);let log=Log(Arc::new(Mutex::new(vec![])));let max=match a[2].as_str(){"off"=>LevelFilter::OFF,"info"=>LevelFilter::INFO,"debug"=>LevelFilter::DEBUG,_=>LevelFilter::TRACE};let state=State{log:log.clone(),interest:a[1].clone(),enabled:a[3]=="true",max,next:Mutex::new(1),style:a[5].parse().unwrap(),fail:a[6].parse().unwrap()};rustc_data_structures::sync::set_dyn_thread_safe_mode(true);rustc_span::create_default_session_globals_then(||tracing::subscriber::with_default(state,||run(&a[4],log.clone())));let events=log.0.lock().unwrap();for event in &*events{println!("{event}")}}
