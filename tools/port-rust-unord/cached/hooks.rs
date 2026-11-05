// Actual std cached-key owned FnMut operations with observable ordinary hooks.
#![feature(fn_traits,unboxed_closures,partial_ord_chaining_methods,panic_can_unwind,panic_internals)]
#![allow(unused_attributes,internal_features,dead_code)]
#[path="oracle.rs"]mod cached;
use std::{io::{self,Write},panic::{self,catch_unwind}};
fn event(value:impl AsRef<str>){println!("{}",value.as_ref());io::stdout().flush().unwrap();}
fn hex(value:&str)->String{value.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct HookLocal;
impl Drop for HookLocal{fn drop(&mut self){event(format!("hook_local:{}",std::thread::panicking()));}}
fn hook(info:&panic::PanicHookInfo<'_>){let(kind,payload)=if let Some(v)=info.payload().downcast_ref::<&str>(){("static_str",(*v).to_owned())}else if let Some(v)=info.payload().downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(&payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));let _local=HookLocal;}
struct Scope;
impl Drop for Scope{fn drop(&mut self){event(format!("scope_drop:{}",std::thread::panicking()));}}
fn body(c:cached::Config){event(format!("scope:{}",std::thread::panicking()));let _scope=Scope;let(message,state,_,compares,drops)=cached::run(c);event(format!("RESULT:{}:{compares}:{drops}:{}",hex(&message),hex(&state)));}
struct Outer(cached::Config);
impl Drop for Outer{fn drop(&mut self){body(self.0)}}
fn main(){let args:Vec<_>=std::env::args().collect();assert_eq!(args.len(),4);let mode=args[1].clone();let v:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();assert_eq!(v.len(),10);let c=cached::Config{kind:v[0]as usize,chain:v[1]!=0,length:v[2]as usize,seed:v[3]as u64,key_fail:v[4]as isize,cmp_fail:v[5]as isize,drop_fail:v[6]as isize,fn_fail:v[7]!=0,token_fail:v[8]!=0,moved:v[9]!=0,stream:true};cached::set_resume(args[3]=="true");let processing=mode=="processing";panic::set_hook(Box::new(move|info|{hook(info);if processing{body(c)}}));match mode.as_str(){"normal"=>body(c),"unwind"=>{let _=catch_unwind(||{let _outer=Outer(c);panic::panic_any("outer")});event("outer_end:false");},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false");},_=>panic!("invalid mode")}}
