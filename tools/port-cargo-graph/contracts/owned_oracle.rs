#![allow(deprecated)]
#![feature(panic_can_unwind,panic_internals)]
//! The complete graph source is included unchanged. Everything below is an
//! external non-Copy trait implementation and an observation client.
include!(env!("CARGO_GRAPH_SOURCE"));
use std::cell::{Cell as ProbeCell,RefCell as ProbeRefCell};
use std::rc::{Rc as ProbeRc,Weak as ProbeWeak};
use std::io::BufRead;
use std::fmt::Write as _;

#[derive(Clone,Copy,Default)]struct Request{scenario:u32,size:u32,op:u32,mode:u32,fail1:usize,fail2:usize,format:u32,write_fail:usize,chars:bool,reuse:bool}
thread_local!{
 static REQUEST:ProbeCell<Request>=const{ProbeCell::new(Request{scenario:0,size:0,op:0,mode:0,fail1:usize::MAX,fail2:usize::MAX,format:0,write_fail:usize::MAX,chars:false,reuse:false})};
 static TRACE:ProbeRefCell<Vec<String>>=const{ProbeRefCell::new(Vec::new())};
 static REGISTRY:ProbeRefCell<Vec<ProbeWeak<Info>>>=const{ProbeRefCell::new(Vec::new())};
 static SERIAL:ProbeCell<u32>=const{ProbeCell::new(0)};
 static EVENTS:ProbeCell<usize>=const{ProbeCell::new(0)};
 static SCOPE:ProbeCell<&'static str>=const{ProbeCell::new("setup")};
 static OUTPUT:ProbeRefCell<String>=const{ProbeRefCell::new(String::new())};
 static STORED:ProbeRefCell<Option<Graph<Key,Edge>>>=const{ProbeRefCell::new(None)};
}
fn request()->Request{REQUEST.with(ProbeCell::get)}
fn hooks()->bool{std::env::var_os("FERRUGO_GRAPH_HOOKS").is_some()}
fn hook(info:&std::panic::PanicHookInfo<'_>){
 if !hooks(){return}
 let(kind,text)=if let Some(v)=info.payload().downcast_ref::<&str>(){("static_str",*v)}else if let Some(v)=info.payload().downcast_ref::<String>(){("owned_string",v.as_str())}else{("opaque","")};
 let(file,line,column)=info.location().map_or(("",0,0),|x|(x.file(),x.line(),x.column()));
 event(format!("hook:{kind}:{}:{}:{}:{}:{line}:{column}",hex(text),info.can_unwind(),info.force_no_backtrace(),hex(file)));
}
struct HookGuard;
impl Drop for HookGuard{fn drop(&mut self){if hooks(){event("guard:hook");if std::env::var("FERRUGO_GRAPH_HOOK_BEHAVIOR").as_deref()==Ok("drop_panic"){panic!("hook destructor panic")}}}}
fn hook_action(){match std::env::var("FERRUGO_GRAPH_HOOK_BEHAVIOR").as_deref(){Ok("panic")=>panic!("hook panic"),Ok("resume")=>std::panic::resume_unwind(Box::new("hook resume")),_=>()}}
fn hex(s:&str)->String{s.as_bytes().iter().map(|x|format!("{x:02x}")).collect()}
fn event(message:impl AsRef<str>)->usize{
 let line=format!("{}:{}:{}",message.as_ref(),std::thread::panicking(),SCOPE.with(ProbeCell::get));
 if SCOPE.with(ProbeCell::get)!="setup"&&std::env::var_os("FERRUGO_GRAPH_ABORT_TRACE").is_some(){println!("{line}")}
 TRACE.with(|v|v.borrow_mut().push(line));EVENTS.with(|v|{let i=v.get();v.set(i+1);i})
}
fn trip(i:usize){let cfg=request();if SCOPE.with(ProbeCell::get)=="op"&&(i==cfg.fail1||i==cfg.fail2){panic!("selected external trait failure")}}
struct Guard(&'static str);
impl Drop for Guard{fn drop(&mut self){event(format!("guard:{}",self.0));}}
struct Info{serial:u32,name:String}
impl Drop for Info{fn drop(&mut self){let _guard=Guard("info");let i=event(format!("info-drop:{}",self.serial));trip(i)}}
fn info(kind:char,id:i32)->ProbeRc<Info>{
 let serial=SERIAL.with(|v|{let n=v.get();v.set(n+1);n});
 let result=ProbeRc::new(Info{serial,name:format!("{kind}{id}@{serial}")});REGISTRY.with(|v|v.borrow_mut().push(ProbeRc::downgrade(&result)));result
}
struct Key{id:i32,stamp:ProbeCell<u32>,info:ProbeRc<Info>}
fn key(id:i32)->Key{Key{id,stamp:ProbeCell::new(100),info:info('K',id)}}
impl Key{fn label(&self)->String{format!("{}:{}:{}",self.info.serial,self.id,self.stamp.get())}}
impl Clone for Key{fn clone(&self)->Self{
 let _guard=Guard("key-clone");let i=event(format!("key-clone:{}",self.label()));let old=self.stamp.get();self.stamp.set(old+10);trip(i);
 Key{id:self.id,stamp:ProbeCell::new(old+1),info:info('K',self.id)}
}}
impl Drop for Key{fn drop(&mut self){let _guard=Guard("key-drop");let i=event(format!("key-drop:{}",self.label()));self.stamp.set(self.stamp.get()+10000);trip(i)}}
impl PartialEq for Key{
 fn eq(&self,other:&Self)->bool{let _guard=Guard("key-eq");let i=event(format!("key-eq:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+100);other.stamp.set(other.stamp.get()+100);trip(i);self.id==other.id}
 fn ne(&self,other:&Self)->bool{if request().scenario==5{return !self.eq(other)};let _guard=Guard("key-ne");let i=event(format!("key-ne:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+200);other.stamp.set(other.stamp.get()+200);trip(i);self.id!=other.id}
}
impl Eq for Key{}
impl Ord for Key{fn cmp(&self,other:&Self)->std::cmp::Ordering{
 let _guard=Guard("key-cmp");let i=event(format!("key-cmp:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+1);other.stamp.set(other.stamp.get()+1);trip(i);self.id.cmp(&other.id)
}}
impl PartialOrd for Key{fn partial_cmp(&self,other:&Self)->Option<std::cmp::Ordering>{Some(self.cmp(other))}fn lt(&self,other:&Self)->bool{
 if request().scenario!=4{return self.partial_cmp(other)==Some(std::cmp::Ordering::Less)}
 let _guard=Guard("key-lt");let i=event(format!("key-lt:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+300);other.stamp.set(other.stamp.get()+300);trip(i);self.id<other.id
}}
impl Borrow<i32> for Key{fn borrow(&self)->&i32{let _guard=Guard("borrow");let i=event(format!("borrow:{}",self.label()));self.stamp.set(self.stamp.get()+1000);trip(i);&self.id}}
impl fmt::Display for Key{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let _guard=Guard("display");let i=event(format!("display:{}:options:{}",self.label(),formatter_state(f)));self.stamp.set(self.stamp.get()+500);trip(i);
 if request().format==7{return Err(fmt::Error)};f.write_str(&self.info.name)?;if request().format==6{return Err(fmt::Error)};f.write_char('!')
}}
struct Edge{id:i32,stamp:ProbeCell<u32>,info:ProbeRc<Info>}
fn edge(id:i32)->Edge{Edge{id,stamp:ProbeCell::new(300),info:info('E',id)}}
impl Edge{fn label(&self)->String{format!("{}:{}:{}",self.info.serial,self.id,self.stamp.get())}}
impl Clone for Edge{fn clone(&self)->Self{
 let _guard=Guard("edge-clone");let i=event(format!("edge-clone:{}",self.label()));let old=self.stamp.get();self.stamp.set(old+10);trip(i);Edge{id:self.id,stamp:ProbeCell::new(old+1),info:info('E',self.id)}
}}
impl Default for Edge{fn default()->Self{let _guard=Guard("edge-default");let i=event("edge-default");trip(i);edge(-1)}}
impl Drop for Edge{fn drop(&mut self){let _guard=Guard("edge-drop");let i=event(format!("edge-drop:{}",self.label()));self.stamp.set(self.stamp.get()+10000);trip(i)}}
impl PartialEq for Edge{
 fn eq(&self,other:&Self)->bool{let _guard=Guard("edge-eq");let i=event(format!("edge-eq:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+100);other.stamp.set(other.stamp.get()+100);trip(i);self.id==other.id}
 fn ne(&self,other:&Self)->bool{if request().scenario==5{return !self.eq(other)};let _guard=Guard("edge-ne");let i=event(format!("edge-ne:{}:{}",self.label(),other.label()));self.stamp.set(self.stamp.get()+200);other.stamp.set(other.stamp.get()+200);trip(i);self.id!=other.id}
}
impl Eq for Edge{}
fn formatter_state(f:&fmt::Formatter<'_>)->String{
 let align=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
 format!("{}:{align}:{}:{}:{}",f.fill()as u32,f.width().map_or("-".into(),|x|x.to_string()),f.precision().map_or("-".into(),|x|x.to_string()),f.flags())
}
struct Sink{calls:Vec<String>,accepted:String,fail:usize,chars:bool}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{
  let aliases=REGISTRY.with(|v|v.borrow().iter().filter_map(|p|p.upgrade()).filter(|p|p.name.as_ptr()==s.as_ptr()&&p.name.len()==s.len()).map(|p|p.serial.to_string()).collect::<Vec<_>>().join(","));
  self.calls.push(format!("S:{}:{aliases}",hex(s)));if self.calls.len()-1==self.fail{Err(fmt::Error)}else{self.accepted.push_str(s);Ok(())}
 }
 fn write_char(&mut self,c:char)->fmt::Result{if !self.chars{return self.write_str(c.encode_utf8(&mut[0;4]))};self.calls.push(format!("C:{}",c as u32));if self.calls.len()-1==self.fail{Err(fmt::Error)}else{self.accepted.push(c);Ok(())}}
}
struct FormatProbe<'a>(&'a Graph<Key,Edge>);
impl fmt::Debug for FormatProbe<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 event(format!("format-before:{}",formatter_state(f)));let result=fmt::Debug::fmt(self.0,f);event(format!("format-after:{}",formatter_state(f)));result
}}
fn format_graph(g:&Graph<Key,Edge>)->String{
 let cfg=request();let mut sink=Sink{calls:Vec::new(),accepted:String::new(),fail:cfg.write_fail,chars:cfg.chars};
 let mut call=||match cfg.format{
  0|6|7=>fmt::write(&mut sink,format_args!("{:?}",FormatProbe(g))),1=>fmt::write(&mut sink,format_args!("{:#?}",FormatProbe(g))),
  2=>fmt::write(&mut sink,format_args!("{:08x?}",FormatProbe(g))),3=>fmt::write(&mut sink,format_args!("{:*^13.2?}",FormatProbe(g))),
  4=>fmt::write(&mut sink,format_args!("{:🦀>12?}",FormatProbe(g))),5=>fmt::write(&mut sink,format_args!("{:+#08X?}",FormatProbe(g))),_=>panic!("format pattern")
 };
 let a=call().is_ok();let b=if cfg.reuse{call().is_ok()}else{true};format!("{a}:{b}:{}:{}",hex(&sink.accepted),hex(&sink.calls.join("|")))
}
fn build()->Graph<Key,Edge>{
 let cfg=request();let mut g=Graph::new();
 for i in 0..cfg.size{let i=if cfg.scenario==3{cfg.size-i-1}else{i};g.add(key(i as i32));}
 if cfg.scenario==1||cfg.scenario==2{for i in 0..cfg.size.saturating_sub(1){*g.link(key(i as i32),key(i as i32+1))=edge(i as i32+10)}}
 if cfg.scenario==2&&cfg.size>0{*g.link(key(0),key(0))=edge(88)}
 if cfg.scenario==6{for i in 1..cfg.size{*g.link(key(0),key(i as i32))=edge(i as i32+10)}}
 if cfg.scenario==7&&cfg.size>0{*g.link(key(0),key(cfg.size as i32))=edge(77)}
 if cfg.scenario==8&&cfg.size>=3{*g.link(key(0),key(1))=edge(10);*g.link(key(0),key(cfg.size as i32))=edge(77);*g.link(key(1),key(2))=edge(11)}
 if (100..612).contains(&cfg.scenario){let bits=cfg.scenario-100;for a in 0..3{for b in 0..3{if bits&(1<<(a*3+b))!=0{*g.link(key(a),key(b))=edge(a*3+b+10)}}}}
 g
}
fn ids<'a>(it:impl Iterator<Item=&'a Key>)->String{it.map(|n|n.id.to_string()).collect::<Vec<_>>().join(",")}
fn path_text(path:Vec<(&Key,Option<&Edge>)>,q:[&Key;4])->String{path.iter().map(|(n,e)|format!("{}:{}:{}:{}",n.id,n.info.serial,q.iter().position(|p|std::ptr::eq(*p,*n)).map_or("-".into(),|i|i.to_string()),e.map_or("-".into(),|e|e.info.serial.to_string()))).collect::<Vec<_>>().join(",")}
fn operation(g:&mut Graph<Key,Edge>,q:[&Key;4])->String{
 match request().op{
  0=>{g.add(key(1));format!("len:{}",g.len())},
  1=>{let e=g.link(key(0),key(1));format!("edge:{}",e.id)},
  2=>{*g.link(key(0),key(1))=edge(99);"assigned".into()},
  3=>{let h=g.clone();event("cloned");let equal=Graph::eq(&*g,&h);event("drop-clone");drop(h);format!("equal:{equal}")},
  4=>{let mut h=g.clone();event("cloned");*h.link(key(0),key(1))=edge(99);let equal=Graph::eq(&*g,&h);event("drop-clone");drop(h);format!("equal:{equal}")},
  5=>{let h=g.reversed();let result=ids(h.iter());event("drop-reversed");drop(h);result},
  6=>format!("{}",Graph::eq(&*g,&*g)),
  7=>ids(g.iter()),
  8=>{let mut i=g.iter();let a=i.size_hint();let first=i.next().map_or(-1,|k|k.id);let b=i.size_hint();let nth=i.nth(1).map_or(-1,|k|k.id);format!("{a:?}:{first}:{b:?}:{nth}:{}",i.count())},
  9=>{let mut i=g.edges(q[0]);event("created-edges");let a=i.size_hint();let first=i.next().map_or("none".into(),|(n,e)|format!("{}:{}",n.id,e.id));let b=i.size_hint();let rest=i.count();format!("{a:?}:{first}:{b:?}:{rest}")},
  10=>format!("{}:{}:{}",g.contains(q[0]),g.contains(q[1]),g.contains(&999i32)),
  11=>g.edge(q[0],q[1]).map_or("none".into(),|e|e.id.to_string()),
  12=>format_graph(g),
  13=>{STORED.with(|v|*v.borrow_mut()=Some(g.clone()));"stored".into()},
  14=>{let h=build();let equal=Graph::eq(&*g,&h);drop(h);format!("independent:{equal}")},
  15=>{let old=std::mem::replace(g,Graph::new());drop(old);"closed".into()},
  16=>{let mut h=Graph::new();h.add(key(-10));h.clone_from(g);event("clone-from");let equal=Graph::eq(&*g,&h);drop(h);format!("equal:{equal}")},
  17=>{#[cfg(debug_assertions)]{g.print_for_test();"printed".into()}#[cfg(not(debug_assertions))]{"not-compiled".into()}},
  18=>{let values=g.sort();let result=ids(values.iter());event("drop-sorted");drop(values);result},
  19=>path_text(g.path_to_bottom(q[0]),q),20=>path_text(g.path_to_top(q[3]),q),21=>path_text(g.path_to_bottom(q[2]),q),
  22=>format!("{}:{}:{}:{}",g.is_path_from_to(q[0],q[1]),g.is_path_from_to(q[1],q[0]),g.is_path_from_to(q[0],q[3]),g.is_path_from_to(q[0],q[0])),
  23=>{let mut h=g.clone();let before=g.edge(q[0],q[1]).zip(h.edge(q[0],q[1])).is_some_and(|(a,b)|std::ptr::eq(a,b));*h.link(key(0),key(1))=edge(99);let after=g.edge(q[0],q[1]).zip(h.edge(q[0],q[1])).is_some_and(|(a,b)|std::ptr::eq(a,b));let other=g.edge(q[0],q[3]).zip(h.edge(q[0],q[3])).is_some_and(|(a,b)|std::ptr::eq(a,b));drop(h);format!("{before}:{after}:{other}")},
  24=>{let path=g.path_to_bottom(q[0]);if let Some((n,Some(e)))=path.last(){e.stamp.set(e.stamp.get()+7000);let found=if path.len()>1{let p=path[path.len()-2].0;g.edge(p,n).or(g.edge(n,p))}else{None};format!("{}:{}",found.is_some_and(|f|std::ptr::eq(f,*e)),e.stamp.get())}else{"none".into()}},
  26=>{g.add(key(request().size as i32+1));format!("len:{}",g.len())},
  27=>{let e=g.link(key(0),key(request().size as i32+1));format!("edge:{}",e.id)},
  28=>{let mut h=g.clone();h.add(key(request().size as i32+1));let count=h.iter().count();drop(h);format!("count:{count}")},
  29=>{let first=capture(||{g.add(key(request().size as i32+1));"ok".into()});let count=g.iter().count();format!("{first}:{count}:{}",g.len())},
  _=>panic!("unsupported operation"),
 }
}
struct Outer<'a>{g:&'a mut Graph<Key,Edge>,q:[&'a Key;4]}
impl Drop for Outer<'_>{fn drop(&mut self){event("outer-drop");let result=if request().mode==1{capture(||operation(self.g,self.q))}else{operation(self.g,self.q)};OUTPUT.with(|v|*v.borrow_mut()=result)}}
fn capture(call:impl FnOnce()->String)->String{std::panic::catch_unwind(std::panic::AssertUnwindSafe(call)).unwrap_or_else(|_|"panic".into())}
fn main(){
 std::panic::set_hook(Box::new(|info|{hook(info);let _guard=HookGuard;hook_action();}));
 for line in std::io::stdin().lock().lines(){
  let line=line.unwrap();let p=line.split('\t').collect::<Vec<_>>();assert_eq!(p.len(),10);
  REQUEST.with(|v|v.set(Request{scenario:p[0].parse().unwrap(),size:p[1].parse().unwrap(),op:p[2].parse().unwrap(),mode:p[3].parse().unwrap(),fail1:p[4].parse().unwrap(),fail2:p[5].parse().unwrap(),format:p[6].parse().unwrap(),write_fail:p[7].parse().unwrap(),chars:p[8]=="1",reuse:p[9]=="1"}));
  SCOPE.with(|v|v.set("setup"));SERIAL.with(|v|v.set(0));REGISTRY.with(|v|v.borrow_mut().clear());TRACE.with(|v|v.borrow_mut().clear());
  let mut g=build();let q=[&*Box::leak(Box::new(key(0))),&*Box::leak(Box::new(key(1))),&*Box::leak(Box::new(key(999))),&*Box::leak(Box::new(key(request().size as i32-1)))];
  if request().op==17&&request().write_fail==0{use std::os::fd::FromRawFd;unsafe{drop(std::os::fd::OwnedFd::from_raw_fd(2))}}
  if request().op==17&&request().write_fail==1{use std::os::fd::AsRawFd;unsafe extern "C"{fn dup2(old:i32,new:i32)->i32;}let(reader,writer)=std::io::pipe().unwrap();drop(reader);assert_eq!(unsafe{dup2(writer.as_raw_fd(),2)},2);drop(writer);}
  TRACE.with(|v|v.borrow_mut().clear());EVENTS.with(|v|v.set(0));SCOPE.with(|v|v.set("op"));
  let cfg=request();let result=match cfg.mode{
   0=>capture(||operation(&mut g,q)),
   1|2=>{let _=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||{let _outer=Outer{g:&mut g,q};panic!("outer")}));OUTPUT.with(|v|std::mem::take(&mut*v.borrow_mut()))},
   3=>{let gptr=&mut g as*mut Graph<Key,Edge>;let qptr=q.map(|p|p as*const Key as usize);
    // Raw pointers only bridge std's 'static Send+Sync hook interface. The
    // hook executes synchronously before unwind, with this unique graph owner.
    let gp=gptr as usize;std::panic::set_hook(Box::new(move |info|{hook(info);let _guard=HookGuard;hook_action();event("processing");let text=unsafe{operation(&mut*(gp as*mut Graph<Key,Edge>),qptr.map(|p|&*(p as*const Key)))};OUTPUT.with(|v|*v.borrow_mut()=text)}));
    let _=std::panic::catch_unwind(||panic!("outer"));std::panic::set_hook(Box::new(|info|{hook(info);let _guard=HookGuard;hook_action();}));OUTPUT.with(|v|std::mem::take(&mut*v.borrow_mut()))},
   _=>panic!("mode"),
  };
  SCOPE.with(|v|v.set("after"));let len=g.len();let after=capture(||{drop(g);"ok".into()});let stored=capture(||{STORED.with(|v|drop(v.borrow_mut().take()));"ok".into()});
  let alive=REGISTRY.with(|v|v.borrow().iter().filter_map(|p|p.upgrade()).map(|p|p.serial.to_string()).collect::<Vec<_>>().join(","));
  let trace=TRACE.with(|v|v.borrow().join("|"));println!("{}\t{len}\t{after}\t{stored}\t{alive}\t{}",hex(&result),hex(&trace));
 }
}
