#![feature(rustc_private,panic_can_unwind,panic_internals)]
// The generator prepends the verified full original lexer as a path module.
// Other algorithms are called through the unchanged whole compiler crates and
// exact registry literal-escaper dependency. This file is only an authored user.
use rustc_data_structures::{fingerprint::{Fingerprint,FingerprintComponent},binary_search_util::binary_search_slice};
use rustc_index::interval::IntervalSet;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher,RawSpan,RawDefId,StableHashControls};
use rustc_literal_escaper as lit;
use std::{any::Any,cell::{Cell,RefCell},cmp::Ordering,ops::{Bound,RangeBounds},panic::{self,catch_unwind,AssertUnwindSafe},rc::Rc};
thread_local!{
 static RUNTIME:Cell<u8>=const{Cell::new(1)};
 static ACTION:RefCell<Option<Box<dyn FnOnce()>>>=const{RefCell::new(None)};
}
fn runtime()->u8{RUNTIME.with(Cell::get)}
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Default)]struct Config{call:&'static str,drop:&'static str,field:&'static str,acc:bool}
fn config(component:&str,case:&str)->Config{
 let call=match component{"fingerprint"=>"convert_a","binary"=>"extract","interval"=>"start",_=>"call"};
 let drop=match component{"fingerprint"=>"b","interval"=>"range1",_=>"env"};
 match case{
 "ok"=>Config::default(),"call"=>Config{call,..Config::default()},"call_b"=>Config{call:"convert_b",..Config::default()},
 "drop"=>Config{drop,..Config::default()},"call_drop"=>Config{call,drop,..Config::default()},"drop_field"=>Config{drop,field:drop,..Config::default()},
 "cmp"=>Config{call:"lt",..Config::default()},"key"=>Config{drop:"key",..Config::default()},"cmp_key"=>Config{call:"lt",drop:"key",..Config::default()},
 "clone"=>Config{call:"clone",..Config::default()},"start"=>Config{call:"start",..Config::default()},"end"=>Config{call:"end",..Config::default()},"original_drop"=>Config{drop:"range0",..Config::default()},
 "acc"=>Config{call,acc:true,..Config::default()},_=>panic!("invalid case")
 }
}
struct Core{cfg:Config,calls:Cell<usize>}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){println!("local:{}:{}:{}",self.0,std::thread::panicking(),runtime())}}
impl Core{
 fn call(&self,stage:&'static str){let _local=Local("call");self.calls.set(self.calls.get()+1);println!("call:{stage}:{}:{}:{}",self.calls.get(),std::thread::panicking(),runtime());if self.cfg.call==stage{panic!("callback")}}
 fn drop_value(&self,label:&'static str){let _local=Local("drop");println!("drop:{label}:{}:{}",std::thread::panicking(),runtime());if self.cfg.drop==label{panic!("drop")}}
}
struct Field{core:Rc<Core>,label:&'static str}
impl Drop for Field{fn drop(&mut self){println!("field:{}:{}:{}",self.label,std::thread::panicking(),runtime());if self.core.cfg.field==self.label{panic!("field")}}}
struct Env{core:Rc<Core>,label:&'static str,_field:Field,metadata:Vec<u8>}
impl Env{fn new(core:&Rc<Core>,label:&'static str)->Self{Self{core:core.clone(),label,_field:Field{core:core.clone(),label},metadata:vec![1,2,3]}}fn call(&self,stage:&'static str){assert_eq!(self.metadata.len(),3);self.core.call(stage)}}
impl Drop for Env{fn drop(&mut self){self.core.drop_value(self.label)}}
impl FingerprintComponent for Env{fn as_u64(&self)->u64{self.call(if self.label=="a"{"convert_a"}else{"convert_b"});if self.label=="a"{10}else{20}}}
struct Acc{n:usize,core:Rc<Core>}
impl Drop for Acc{fn drop(&mut self){println!("acc:{}:{}:{}",self.n,std::thread::panicking(),runtime());if self.core.cfg.acc&&std::thread::panicking(){panic!("acc")}}}
struct Key{n:i32,core:Rc<Core>,query:bool,_field:Option<Field>}
impl Key{fn op(&self,b:&Self,stage:&'static str)->bool{self.core.call(stage);match stage{"lt"=>self.n<b.n,"ne"=>self.n!=b.n,_=>self.n<=b.n}}}
impl PartialEq for Key{fn eq(&self,b:&Self)->bool{self.n==b.n}fn ne(&self,b:&Self)->bool{self.op(b,"ne")}}
impl Eq for Key{}
impl PartialOrd for Key{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(self.cmp(b))}fn lt(&self,b:&Self)->bool{self.op(b,"lt")}fn le(&self,b:&Self)->bool{self.op(b,"le")}}
impl Ord for Key{fn cmp(&self,b:&Self)->Ordering{self.n.cmp(&b.n)}}
impl Drop for Key{fn drop(&mut self){if !self.query{self.core.drop_value("key")}}}
struct Bounds{core:Rc<Core>,generation:u8,start:usize,end:usize,_field:Field}
impl Bounds{fn new(core:&Rc<Core>,generation:u8)->Self{Self{core:core.clone(),generation,start:1,end:3,_field:Field{core:core.clone(),label:if generation==0{"range0"}else{"range1"}}}}}
impl Clone for Bounds{fn clone(&self)->Self{self.core.call("clone");Self::new(&self.core,self.generation+1)}}
impl RangeBounds<usize> for Bounds{fn start_bound(&self)->Bound<&usize>{self.core.call("start");Bound::Included(&self.start)}fn end_bound(&self)->Bound<&usize>{self.core.call("end");Bound::Included(&self.end)}}
impl Drop for Bounds{fn drop(&mut self){self.core.drop_value(if self.generation==0{"range0"}else{"range1"})}}
struct Hcx{core:Rc<Core>}
impl StableHashCtxt for Hcx{fn stable_hash_span(&mut self,_:RawSpan,_:&mut StableHasher){self.core.call("call")}fn def_path_hash(&self,_:RawDefId)->Fingerprint{self.core.call("call");Fingerprint::ZERO}fn stable_hash_controls(&self)->StableHashControls{self.core.call("call");StableHashControls{hash_spans:true}}fn assert_default_stable_hash_controls(&self,_:&str){self.core.call("call")}}
fn build(component:&str,core:Rc<Core>)->Box<dyn FnOnce()>{match component{
 "predicate"=>{let mut cursor=lexer::Cursor::new("ab!c",lexer::FrontmatterAllowed::No);let env=Env::new(&core,"env");Box::new(move||{cursor.eat_while(move|c|{env.call("call");c!='!'});println!("returned:cursor:{}",hex(cursor.as_str()))})},
 "foreach"=>{let it=lexer::tokenize("a ! b",lexer::FrontmatterAllowed::No);let env=Env::new(&core,"env");Box::new(move||{it.for_each(move|_|env.call("call"));println!("returned:foreach")})},
 "fold"=>{let it=lexer::tokenize("a ! b",lexer::FrontmatterAllowed::No);let env=Env::new(&core,"env");Box::new(move||{let result=it.fold(Acc{n:0,core:core.clone()},move|acc,_|{env.call("call");let next=acc.n+1;drop(acc);Acc{n:next,core:core.clone()}});println!("returned:fold:{}",result.n);drop(result)})},
 "literal"=>{let env=Env::new(&core,"env");Box::new(move||{lit::unescape_str("a\\nb",move|_,_|env.call("call"));println!("returned:literal")})},
 "fingerprint"=>{let a=Env::new(&core,"a");let b=Env::new(&core,"b");Box::new(move||{let value=Fingerprint::new(a,b);let(a,b)=value.split();println!("returned:fingerprint:{}:{}",a.as_u64(),b.as_u64())})},
 "binary"=>{let env=Env::new(&core,"env");Box::new(move||{let data=[1,2,2,3];let query=Key{n:2,core:core.clone(),query:true,_field:None};let result=binary_search_slice(&data,move|n|{env.call("extract");Key{n:*n,core:env.core.clone(),query:false,_field:Some(Field{core:env.core.clone(),label:"key"})}},&query);println!("returned:binary:{}",result.len())})},
 "interval"=>{let bounds=Bounds::new(&core,0);Box::new(move||{let mut set=IntervalSet::<usize>::new(16);let result=set.insert_range(bounds);println!("returned:interval:{result}:{:?}",set.iter().collect::<Vec<_>>())})},
 "stable_hash_context"=>{let hcx=Hcx{core};Box::new(move||{hcx.assert_default_stable_hash_controls("probe");println!("returned:stable_hash_context")})},
 "internal"=>Box::new(||{let _local=Local("internal");let mut cursor=lexer::Cursor::new("λ",lexer::FrontmatterAllowed::No);cursor.eat_until(0xbb);println!("returned:internal")}),
 _=>panic!("invalid component")}}
fn invoke(){let action=ACTION.with(|s|s.borrow_mut().take().unwrap());action()}
fn payload(value:&(dyn Any+Send))->(&'static str,String){if let Some(v)=value.downcast_ref::<&str>(){("static_str",v.to_string())}else if let Some(v)=value.downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())}}
fn report(f:impl FnOnce()){let result=catch_unwind(AssertUnwindSafe(f));match result{Ok(())=>println!("caught:false::{}:{}",std::thread::panicking(),runtime()),Err(p)=>println!("caught:true:{}:{}:{}",hex(payload(p.as_ref()).1),std::thread::panicking(),runtime())}}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer:{}:{}",std::thread::panicking(),runtime());if self.caught{report(invoke)}else{invoke()}}}
fn main(){let a:Vec<_>=std::env::args().collect();let(component,case,mode)=(&a[1],&a[2],a[3].clone());let core=Rc::new(Core{cfg:config(component,case),calls:Cell::new(0)});
 RUNTIME.with(|r|r.set(1));panic::set_hook(Box::new(|_|println!("stale_hook")));println!("construct:{}",runtime());let action=build(component,core);ACTION.with(|s|*s.borrow_mut()=Some(action));RUNTIME.with(|r|r.set(if mode=="unobserved"{0}else{2}));println!("invoke:{}",runtime());
 let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let(kind,text)=payload(info.payload());let location=info.location().map(|l|format!("{}:{}:{}",l.file(),l.line(),l.column())).unwrap_or_default();println!("hook:{kind}:{}:{}:{}:{}:{}:{}",hex(text),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),runtime(),location);if processing{invoke();println!("hook_return:{}:{}",std::thread::panicking(),runtime())}}));
 if mode=="unobserved"{panic::set_hook(Box::new(|_|{}));}
 match mode.as_str(){"normal"|"unobserved"=>report(invoke),"processing"=>report(||panic!("outer")),_=>report(||{let _outer=Outer{caught:mode=="inherited_caught"};panic!("outer")})};println!("complete:{}",runtime())
}
