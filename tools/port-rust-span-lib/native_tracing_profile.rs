// SPDX-License-Identifier: MIT OR Apache-2.0
#![feature(rustc_private)]
use std::{fmt,sync::{Arc,Mutex}};
use tracing::{Subscriber,Metadata,Event,span::{Attributes,Id,Record},subscriber::Interest,metadata::LevelFilter,field::{Field,Visit}};
use rustc_span::{SourceFile,FileName,SourceFileHashAlgorithm,BytePos};
fn hex(bytes:&[u8])->String{bytes.iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Clone)]struct Log(Arc<Mutex<Vec<String>>>);
impl Log{fn add(&self,text:String){self.0.lock().unwrap().push(text)}}
fn meta(m:&Metadata<'_>)->String{format!("{}|{}|{}|{}|{}|{}|{}|{}",m.name(),m.level(),m.target(),m.module_path().unwrap_or(""),m.file().unwrap_or(""),m.line().unwrap_or(0),if m.is_span(){"span"}else{"event"},m.fields().iter().map(|f|f.name()).collect::<Vec<_>>().join(","))}
struct Visitor(Log);
impl Visit for Visitor{fn record_debug(&mut self,field:&Field,value:&dyn fmt::Debug){self.0.add(format!("debug|{}|{}|{}",field.name(),std::thread::panicking(),hex(format!("{value:?}").as_bytes())));}}
struct State{log:Log,interest:String,enabled:bool,max:LevelFilter,next:Mutex<u64>}
impl Drop for State{fn drop(&mut self){self.log.add(format!("drop|{}",std::thread::panicking()))}}
impl Subscriber for State{
 fn register_callsite(&self,m:&'static Metadata<'static>)->Interest{self.log.add(format!("register|{}|{}",meta(m),std::thread::panicking()));match self.interest.as_str(){"always"=>Interest::always(),"never"=>Interest::never(),_=>Interest::sometimes()}}
 fn enabled(&self,m:&Metadata<'_>)->bool{self.log.add(format!("enabled|{}|{}",meta(m),std::thread::panicking()));self.enabled}
 fn max_level_hint(&self)->Option<LevelFilter>{self.log.add(format!("max|{}|{}",self.max,std::thread::panicking()));Some(self.max)}
 fn new_span(&self,a:&Attributes<'_>)->Id{let mut next=self.next.lock().unwrap();let id=*next;*next+=1;self.log.add(format!("new|{}|{}",id,meta(a.metadata())));a.record(&mut Visitor(self.log.clone()));Id::from_u64(id)}
 fn record(&self,id:&Id,fields:&Record<'_>){self.log.add(format!("record|{}",id.into_u64()));fields.record(&mut Visitor(self.log.clone()))}
 fn record_follows_from(&self,id:&Id,from:&Id){self.log.add(format!("follows|{}|{}",id.into_u64(),from.into_u64()))}
 fn event_enabled(&self,e:&Event<'_>)->bool{self.log.add(format!("event_enabled|{}|{}",meta(e.metadata()),std::thread::panicking()));true}
 fn event(&self,e:&Event<'_>){self.log.add(format!("event|{}|{}",meta(e.metadata()),std::thread::panicking()));e.record(&mut Visitor(self.log.clone()))}
 fn enter(&self,id:&Id){self.log.add(format!("enter|{}",id.into_u64()))}
 fn exit(&self,id:&Id){self.log.add(format!("exit|{}",id.into_u64()))}
 fn clone_span(&self,id:&Id)->Id{self.log.add(format!("clone|{}",id.into_u64()));id.clone()}
 fn try_close(&self,id:Id)->bool{self.log.add(format!("close|{}",id.into_u64()));true}
}
fn main(){
 let args:Vec<_>=std::env::args().collect();let max=match args[2].as_str(){"off"=>LevelFilter::OFF,"info"=>LevelFilter::INFO,"debug"=>LevelFilter::DEBUG,_=>LevelFilter::TRACE};
 let log=Log(Arc::new(Mutex::new(vec![])));let state=State{log:log.clone(),interest:args[1].clone(),max,enabled:args[3]=="true",next:Mutex::new(1)};
 rustc_data_structures::sync::set_dyn_thread_safe_mode(true);
 rustc_span::create_default_session_globals_then(||tracing::subscriber::with_default(state,||{
  let mut file=SourceFile::new(FileName::Custom("trace.rs".to_owned()),"éβ\nx".to_owned(),SourceFileHashAlgorithm::Md5,None).unwrap();
  if args[4]=="missing"{file.src=None;}
  for _ in 0..2{let _=file.lookup_file_pos_with_col_display(BytePos(2));}
 }));
 for entry in log.0.lock().unwrap().iter(){println!("{entry}")}
}
