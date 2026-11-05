// Authored client of actual std HashMap/Vec/slice APIs. No std source body is
// rewritten, replaced or reconstructed by this reference observer.
#![feature(panic_can_unwind,panic_internals)]
use std::{cell::RefCell,collections::HashMap,hash::{Hash,Hasher,BuildHasher},io::{self,Write},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone)]struct Config{action:String,resume:bool,hook:String}
thread_local!{static CONFIG:RefCell<Config>=RefCell::new(Config{action:String::new(),resume:false,hook:String::new()});}
fn config()->Config{CONFIG.with(|c|c.borrow().clone())}
fn event(value:impl AsRef<str>){println!("{}",value.as_ref());io::stdout().flush().unwrap();}
fn p()->bool{std::thread::panicking()}
#[track_caller]fn raise(message:&'static str)->!{if config().resume{panic::resume_unwind(Box::new(message))}else{panic::panic_any(message)}}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){event(format!("drop:{}:{}",self.0,p()));}}
struct HookLocal;
impl Drop for HookLocal{fn drop(&mut self){event(format!("drop:hook_local:{}",p()));if config().hook=="drop_panic"{raise("hook_drop")}}}
fn hook(info:&panic::PanicHookInfo<'_>){let(kind,payload)=if let Some(v)=info.payload().downcast_ref::<&str>(){("static_str",(*v).to_owned())}else if let Some(v)=info.payload().downcast_ref::<String>(){("owned_string",v.clone())}else{("opaque",String::new())};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(&payload),p(),info.can_unwind(),info.force_no_backtrace(),hex(file)));let _local=HookLocal;match config().hook.as_str(){"panic"=>raise("hook_panic"),"resume"=>panic::resume_unwind(Box::new("hook_resume")),_=>()}}
struct H{key:usize}
impl Hasher for H{fn finish(&self)->u64{0}fn write(&mut self,value:&[u8]){if value.len()==size_of::<usize>(){self.key=usize::from_le_bytes(value.try_into().unwrap());}}}
impl Drop for H{fn drop(&mut self){event(format!("drop:hasher:{}",p()));if self.key==1&&config().action=="map_hasher_double"{raise("hasher_drop")}}}
#[derive(Clone,Copy)]struct B;
impl BuildHasher for B{type Hasher=H;fn build_hasher(&self)->H{H{key:0}}}
struct K{key:usize,id:usize}
impl Hash for K{fn hash<S:Hasher>(&self,h:&mut S){event(format!("hash:{}:{}",self.id,p()));self.key.hash(h);if self.id==1&&config().action.starts_with("map_hash"){let _local=Local("hash_local");raise("hash")}}}
impl PartialEq for K{fn eq(&self,b:&Self)->bool{event(format!("eq:{}:{}:{}",self.id,b.id,p()));if self.id==1&&config().action=="map_eq"{raise("eq")};self.key==b.key}}
impl Eq for K{}
impl Drop for K{fn drop(&mut self){event(format!("drop:key{}:{}",self.id,p()));if self.id==1&&config().action=="map_hash_key_double"{raise("key_drop")}}}
struct V(usize);
impl Drop for V{fn drop(&mut self){event(format!("drop:value{}:{}",self.0,p()));if self.0==1&&config().action=="map_hash_value_double"{raise("value_drop")}}}
struct I{next:usize}
impl Iterator for I{type Item=V;fn next(&mut self)->Option<V>{event(format!("next:{}:{}",self.next,p()));if self.next==3{return None}let index=self.next;self.next+=1;if index==1&&config().action=="vec_next"{let _local=Local("next_local");raise("next")};Some(V(index))}fn size_hint(&self)->(usize,Option<usize>){event(format!("hint:{}:{}",self.next,p()));if config().action=="vec_capacity"{(usize::MAX,None)}else{let n=3-self.next;(n,Some(n))}}}
impl Drop for I{fn drop(&mut self){event(format!("drop:iterator:{}:{}",self.next,p()));if config().action.starts_with("vec_iter_drop"){raise("iter_drop")}}}
struct CV(usize);
impl Drop for CV{fn drop(&mut self){event(format!("drop:collected{}:{}",self.0,p()));if self.0==0&&config().action=="vec_iter_drop_double"{raise("collected_drop")}}}
struct CI(I);
impl Iterator for CI{type Item=CV;fn next(&mut self)->Option<CV>{self.0.next().map(|v|{let n=v.0;std::mem::forget(v);CV(n)})}fn size_hint(&self)->(usize,Option<usize>){self.0.size_hint()}}
fn result(out:Result<(),Box<dyn std::any::Any+Send>>){match out{Ok(())=>event(format!("caught:none:{}",p())),Err(value)=>{let text=value.downcast_ref::<&str>().map(|s|*s).unwrap_or("other");event(format!("caught:{}:{}",hex(text),p()));}}}
fn action(){let action=config().action;event(format!("call:{action}:{}",p()));
if action=="map_capacity"{result(catch_unwind(||{let _=HashMap::<K,V,B>::with_capacity_and_hasher(usize::MAX,B);}));}
else if action.starts_with("map_"){let mut map=HashMap::with_hasher(B);map.insert(K{key:0,id:0},V(0));result(catch_unwind(AssertUnwindSafe(||{if action=="map_entry"{let env=Local("entry_env");map.entry(K{key:1,id:1}).or_insert_with(move||{let _=&env;let _local=Local("entry_local");raise("entry")});}else{map.insert(K{key:1,id:1},V(1));}})));drop(map);}
else if action.starts_with("vec_"){let mut output:Option<Vec<CV>>=None;result(catch_unwind(AssertUnwindSafe(||{output=Some(CI(I{next:0}).collect());})));drop(output);}
else if action=="sort_compare"{let mut v=vec![V(2),V(1),V(0)];result(catch_unwind(AssertUnwindSafe(||v.sort_by(|_,_|{let _local=Local("compare_local");raise("compare")}))));drop(v);}
else if action=="sort_invalid"{let mut seed=2u64;let mut v:Vec<_>=(0..21).map(|id|{seed^=seed<<13;seed^=seed>>7;seed^=seed<<17;((seed%13)as i64,id)}).collect();result(catch_unwind(AssertUnwindSafe(||v.sort_by(|a,b|{event(format!("compare:{}:{}:{}",a.1,b.1,p()));let a=a.0%3;let b=b.0%3;if a==b{std::cmp::Ordering::Equal}else if(a+1)%3==b{std::cmp::Ordering::Less}else{std::cmp::Ordering::Greater}}))));}
else{panic!("unknown action")};event(format!("return:{}",p()));}
struct Outer;
impl Drop for Outer{fn drop(&mut self){event(format!("outer_drop:{}",p()));action();}}
fn main(){let args:Vec<_>=std::env::args().collect();assert_eq!(args.len(),5);let mode=args[1].clone();CONFIG.with(|c|*c.borrow_mut()=Config{action:args[2].clone(),resume:args[3]=="true",hook:args[4].clone()});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{hook(info);if processing{action();}}));match mode.as_str(){"normal"=>action(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event(format!("outer_end:{}",p()));},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event(format!("outer_end:{}",p()));},_=>panic!("unknown mode")}}
