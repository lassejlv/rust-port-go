#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Calls the unchanged whole-crate fatal_error module and actual std unwind.
use std::panic::{catch_unwind,AssertUnwindSafe};
use rustc_span::fatal_error::{FatalError,catch_fatal_errors};
#[derive(Clone)]struct Dropper(&'static str,&'static str);
impl Drop for Dropper{fn drop(&mut self){println!("drop:{}:{}",self.0,std::thread::panicking());match self.1{"ordinary"=>panic!("drop"),"fatal"=>FatalError.raise(),_=>{}}}}
fn invoke(case:&str){
 let (action,env_action,body_action)=match case{"ok"=>("ok","none","none"),"fatal"=>("fatal","none","none"),"ordinary"=>("ordinary","none","none"),"env_ordinary"=>("ok","ordinary","none"),"env_fatal"=>("ok","fatal","none"),"body_ordinary"=>("ok","none","ordinary"),"fatal_env_ordinary"=>("fatal","ordinary","none"),"ordinary_env_ordinary"=>("ordinary","ordinary","none"),"fatal_body_ordinary"=>("fatal","none","ordinary"),_=>panic!("unknown case")};
 let env=Dropper("env",env_action);
 let result=catch_fatal_errors(move||{let _=&env;let _body=Dropper("body",body_action);println!("call:{}",std::thread::panicking());match action{"fatal"=>FatalError.raise(),"ordinary"=>panic!("ordinary"),_=>{}};Dropper("return","none")});
 match result{Ok(value)=>{println!("result:ok:{}",std::thread::panicking());drop(value)},Err(_)=>println!("result:fatal:{}",std::thread::panicking())}
}
fn tag(p:&(dyn std::any::Any+Send))->&'static str{if p.is::<rustc_data_structures::FatalErrorMarker>(){"fatal"}else if let Some(s)=p.downcast_ref::<&str>(){if *s=="outer"{"outer"}else if *s=="ordinary"{"ordinary"}else if *s=="panic in a destructor during cleanup"{"cleanup"}else{"drop"}}else{"other"}}
fn report(f:impl FnOnce()){
 match catch_unwind(AssertUnwindSafe(f)){Ok(())=>println!("caught:none:{}",std::thread::panicking()),Err(p)=>println!("caught:{}:{}",tag(p.as_ref()),std::thread::panicking())}
}
struct Outer{mode:String,case:String}
impl Drop for Outer{fn drop(&mut self){println!("outer_drop:{}",std::thread::panicking());if self.mode=="inherited_caught"{report(||invoke(&self.case))}else{invoke(&self.case)}}}
fn main(){let a:Vec<_>=std::env::args().collect();let mode=a[1].clone();let case=a[2].clone();
 if mode=="processing"{std::panic::set_hook(Box::new(move|p|{println!("hook:{}:{}",tag(p.payload()),std::thread::panicking());invoke(&case);println!("hook_return:{}",std::thread::panicking())}));report(||panic!("outer"));return}
 std::panic::set_hook(Box::new(|p|println!("hook:{}:{}",tag(p.payload()),std::thread::panicking())));
 if mode=="normal"{report(||invoke(&case))}else{report(||{let _outer=Outer{mode,case};panic!("outer")})}
}
