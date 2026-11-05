extern crate tracing;
use tracing::{Subscriber,Event,Metadata,Level};
use tracing::subscriber::Interest;
use tracing::span::{Attributes,Id,Record};
use tracing::field::{Visit,Field};
use std::fmt;
use std::sync::{Arc,Mutex,atomic::{AtomicU64,Ordering}};
type Log=Arc<Mutex<Vec<String>>>;
fn push(log:&Log,text:String){log.lock().unwrap().push(text)}
struct Borrowed(Arc<AtomicU64>);
impl fmt::Debug for Borrowed{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{write!(f,"borrow({})",self.0.load(Ordering::Relaxed))}}
struct Visitor<'a>{log:&'a Log,label:&'a str}
impl Visit for Visitor<'_>{
 fn record_debug(&mut self,f:&Field,v:&dyn fmt::Debug){push(self.log,format!("debug:{}:{}:{v:?}",self.label,f.name()))}
 fn record_u64(&mut self,f:&Field,v:u64){push(self.log,format!("u64:{}:{}:{v}",self.label,f.name()))}
 fn record_bool(&mut self,f:&Field,v:bool){push(self.log,format!("bool:{}:{}:{v}",self.label,f.name()))}
 fn record_str(&mut self,f:&Field,v:&str){push(self.log,format!("str:{}:{}:{v:?}",self.label,f.name()))}
}
struct S{log:Log,label:&'static str,interest:u8,enabled:bool,event:bool,max:u8,cell:Arc<AtomicU64>,next:AtomicU64,nested:bool}
impl Drop for S{fn drop(&mut self){push(&self.log,format!("drop:{}:{}",self.label,std::thread::panicking()))}}
impl Subscriber for S{
 fn on_register_dispatch(&self,_:&tracing::Dispatch){push(&self.log,format!("dispatch:{}",self.label))}
 fn register_callsite(&self,m:&'static Metadata<'static>)->Interest{push(&self.log,format!("register:{}:{}",self.label,m.name()));match self.interest{0=>Interest::never(),1=>Interest::sometimes(),_=>Interest::always()}}
 fn enabled(&self,m:&Metadata<'_>)->bool{push(&self.log,format!("enabled:{}:{}",self.label,m.name()));self.enabled}
 fn max_level_hint(&self)->Option<tracing::level_filters::LevelFilter>{push(&self.log,format!("hint:{}",self.label));match self.max{0=>Some(tracing::level_filters::LevelFilter::OFF),1=>Some(tracing::level_filters::LevelFilter::INFO),2=>Some(tracing::level_filters::LevelFilter::DEBUG),_=>None}}
 fn new_span(&self,a:&Attributes<'_>)->Id{let id=self.next.fetch_add(1,Ordering::Relaxed);push(&self.log,format!("new:{}:{}:{id}",self.label,a.metadata().name()));a.record(&mut Visitor{log:&self.log,label:self.label});Id::from_u64(id)}
 fn record(&self,id:&Id,r:&Record<'_>){push(&self.log,format!("record:{}:{}",self.label,id.into_u64()));r.record(&mut Visitor{log:&self.log,label:self.label})}
 fn record_follows_from(&self,a:&Id,b:&Id){push(&self.log,format!("follows:{}:{}:{}",self.label,a.into_u64(),b.into_u64()))}
 fn event_enabled(&self,e:&Event<'_>)->bool{push(&self.log,format!("event-enabled:{}:{}",self.label,e.metadata().name()));self.cell.fetch_add(1,Ordering::Relaxed);self.event}
 fn event(&self,e:&Event<'_>){push(&self.log,format!("event:{}:{}",self.label,e.metadata().name()));e.record(&mut Visitor{log:&self.log,label:self.label})}
 fn enter(&self,id:&Id){push(&self.log,format!("enter:{}:{}",self.label,id.into_u64()));if self.nested{emit_nested(&self.log)}}
 fn exit(&self,id:&Id){push(&self.log,format!("exit:{}:{}",self.label,id.into_u64()))}
 fn clone_span(&self,id:&Id)->Id{push(&self.log,format!("clone:{}:{}",self.label,id.into_u64()));Id::from_u64(id.into_u64()+100)}
 fn try_close(&self,id:Id)->bool{push(&self.log,format!("close:{}:{}",self.label,id.into_u64()));true}
}
fn emit_event(log:&Log,cell:&Arc<AtomicU64>){let value=Borrowed(cell.clone());tracing::event!(name:"event",target:"ferrugo",Level::DEBUG,n={push(log,"eval:n".into());cell.load(Ordering::Relaxed)},ok=true,text="a\0🦀",value=?value,message=format_args!("message {}",cell.load(Ordering::Relaxed)));}
fn emit_nested(log:&Log){tracing::event!(name:"nested",target:"ferrugo",Level::DEBUG,n={push(log,"eval:nested".into());99u64});}
fn make_span(log:&Log,cell:&Arc<AtomicU64>)->tracing::Span{let value=Borrowed(cell.clone());tracing::span!(target:"ferrugo",Level::DEBUG,"span",value=?value,ret=tracing::field::Empty,n={push(log,"eval:span".into());3u64})}
fn main(){let a:Vec<_>=std::env::args().skip(1).map(|s|s.parse::<i32>().unwrap()).collect();let log=Arc::new(Mutex::new(Vec::new()));let cell=Arc::new(AtomicU64::new(7));
 let sub=|label,interest|S{log:log.clone(),label,interest,enabled:a[2]!=0,event:a[3]!=0,max:a[4]as u8,cell:cell.clone(),next:AtomicU64::new(1),nested:a[5]!=0};
 let da=tracing::Dispatch::new(sub("a",a[0]as u8));let db=if a[1]>=0{Some(tracing::Dispatch::new(sub("b",a[1]as u8)))}else{None};
 tracing::dispatcher::with_default(&da,||{
  emit_event(&log,&cell);emit_event(&log,&cell);let span=make_span(&log,&cell);push(&log,format!("disabled:{}",span.is_disabled()));
  {let _entered=span.enter();emit_event(&log,&cell);span.record("ret",tracing::field::debug(Borrowed(cell.clone())));let clone=span.clone();if let Some(id)=span.id(){clone.follows_from(id);}drop(clone);}
  drop(span);tracing::callsite::rebuild_interest_cache();emit_event(&log,&cell);
  if let Some(ref b)=db{tracing::dispatcher::with_default(b,||emit_event(&log,&cell));}
 });
 emit_event(&log,&cell);let span=make_span(&log,&cell);push(&log,format!("outside-disabled:{}",span.is_disabled()));drop(span);drop(db);drop(da);
 for text in log.lock().unwrap().iter(){println!("{text}")}
}
