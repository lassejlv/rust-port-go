// Actual unchanged default UnordItems filter/filter_map/flat_map/chain.
#![feature(fn_traits,unboxed_closures,panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::unord::UnordItems;
use std::{cell::Cell,io::{self,Write},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{family:usize,length:usize,inner:usize,keep:u64,next_fail:isize,call_fail:isize,item_fail:isize,env_fail:bool,token_fail:bool,input_fail:bool,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{family:0,length:0,inner:2,keep:0,next_fail:-1,call_fail:-1,item_fail:-1,env_fail:false,token_fail:false,input_fail:false,resume:false})};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Value(u64);impl Drop for Value{fn drop(&mut self){event(format!("item:{}:{}",self.0,std::thread::panicking()));if self.0 as isize==CONFIG.get().item_fail{raise("item_drop")}}}
struct Token;impl Drop for Token{fn drop(&mut self){event(format!("token:{}",std::thread::panicking()));if CONFIG.get().token_fail{raise("token_drop")}}}
struct Function{calls:Cell<usize>,_token:Token}impl Drop for Function{fn drop(&mut self){event(format!("function:{}:{}",self.calls.get(),std::thread::panicking()));if CONFIG.get().env_fail{raise("function_drop")}}}
impl Function{fn observe(&self,value:&Value,which:&str)->bool{let n=self.calls.get();self.calls.set(n+1);event(format!("{which}:{n}:{}:{}",value.0,std::thread::panicking()));if n as isize==CONFIG.get().call_fail{raise("call")};value.0%2==CONFIG.get().keep}}
struct Predicate(Function);
impl<'a>FnOnce<(&'a Value,)>for Predicate{type Output=bool;extern "rust-call" fn call_once(self,(value,):(&'a Value,))->bool{self.0.observe(value,"once")}}
impl<'a>FnMut<(&'a Value,)>for Predicate{extern "rust-call" fn call_mut(&mut self,(value,):(&'a Value,))->bool{self.0.observe(value,"mut")}}
impl<'a>Fn<(&'a Value,)>for Predicate{extern "rust-call" fn call(&self,(value,):(&'a Value,))->bool{self.0.observe(value,"shared")}}
struct Optional(Function);
impl FnOnce<(Value,)>for Optional{type Output=Option<Value>;extern "rust-call" fn call_once(self,(value,):(Value,))->Self::Output{self.0.observe(&value,"once").then(||Value(value.0+1000))}}
impl FnMut<(Value,)>for Optional{extern "rust-call" fn call_mut(&mut self,(value,):(Value,))->Self::Output{self.0.observe(&value,"mut").then(||Value(value.0+1000))}}
impl Fn<(Value,)>for Optional{extern "rust-call" fn call(&self,(value,):(Value,))->Self::Output{self.0.observe(&value,"shared").then(||Value(value.0+1000))}}
struct Input{next:usize,length:usize,kind:u64}
impl Drop for Input{fn drop(&mut self){event(format!("input:{}:{}:{}",self.kind,self.next,std::thread::panicking()));if CONFIG.get().input_fail{raise("input_drop")}}}
impl Iterator for Input{type Item=Value;
 fn next(&mut self)->Option<Value>{let n=self.next;self.next+=1;event(format!("next:{}:{n}:{}",self.kind,std::thread::panicking()));if n as isize==CONFIG.get().next_fail{raise("next")};if n<self.length{Some(Value(self.kind*1000+n as u64))}else{None}}
 fn size_hint(&self)->(usize,Option<usize>){let n=self.length.saturating_sub(self.next);(n,Some(n))}
 fn count(self)->usize{event(format!("count:{}:{}",self.kind,std::thread::panicking()));self.length.saturating_sub(self.next)}
 fn fold<B,F:FnMut(B,Value)->B>(mut self,mut init:B,mut f:F)->B{event(format!("fold:{}:{}",self.kind,std::thread::panicking()));while let Some(value)=self.next(){init=f(init,value)};init}
}
struct Flat(Function);
impl FnOnce<(Value,)>for Flat{type Output=UnordItems<Value,Input>;extern "rust-call" fn call_once(self,(value,):(Value,))->Self::Output{self.0.observe(&value,"once");UnordItems::new(Input{next:0,length:CONFIG.get().inner,kind:100+value.0})}}
impl FnMut<(Value,)>for Flat{extern "rust-call" fn call_mut(&mut self,(value,):(Value,))->Self::Output{self.0.observe(&value,"mut");UnordItems::new(Input{next:0,length:CONFIG.get().inner,kind:100+value.0})}}
impl Fn<(Value,)>for Flat{extern "rust-call" fn call(&self,(value,):(Value,))->Self::Output{self.0.observe(&value,"shared");UnordItems::new(Input{next:0,length:CONFIG.get().inner,kind:100+value.0})}}
struct Override(Input);impl Iterator for Override{type Item=Value;fn next(&mut self)->Option<Value>{self.0.next()}
 fn filter<P>(self,p:P)->std::iter::Filter<Self,P>where Self:Sized,P:FnMut(&Value)->bool{event("filter_override");let value=Value(99);let output=FnOnce::call_once(p,(&value,));event(format!("constructed:{output}"));drop(value);raise("filter_constructor")}
 fn filter_map<B,F>(self,f:F)->std::iter::FilterMap<Self,F>where Self:Sized,F:FnMut(Value)->Option<B>{event("filter_map_override");let output=FnOnce::call_once(f,(Value(99),));event("constructed");drop(output);raise("filter_map_constructor")}
 fn chain<U>(self,other:U)->std::iter::Chain<Self,U::IntoIter>where Self:Sized,U:IntoIterator<Item=Value>{event("chain_override");drop(other.into_iter());raise("chain_constructor")}
}
fn display<I:Iterator<Item=Value>>(items:UnordItems<Value,I>,count:bool){if count{event(format!("returned:{}",items.count()))}else{let value=items.get_only();event(format!("returned:{}",value.as_ref().map(|v|v.0.to_string()).unwrap_or_else(||"none".into())));drop(value)}}
fn execute(){let c=CONFIG.get();let input=if c.family<8{Some(UnordItems::new(Input{next:0,length:c.length,kind:0}))}else{None};let count=c.family%2==1;match c.family{
 0|1=>display(input.unwrap().filter(Predicate(Function{calls:Cell::new(0),_token:Token})),count),
 2|3=>display(input.unwrap().filter_map(Optional(Function{calls:Cell::new(0),_token:Token})),count),
 4|5=>display(input.unwrap().flat_map(Flat(Function{calls:Cell::new(0),_token:Token})),count),
 6|7=>display(input.unwrap().chain(UnordItems::new(Input{next:0,length:c.inner,kind:1})),count),
 8=>{let input=UnordItems::new(Override(Input{next:0,length:c.length,kind:0}));drop(input.filter(Predicate(Function{calls:Cell::new(0),_token:Token})))},
 9=>{let input=UnordItems::new(Override(Input{next:0,length:c.length,kind:0}));drop(input.filter_map(Optional(Function{calls:Cell::new(0),_token:Token})))},
 10=>{let input=UnordItems::new(Override(Input{next:0,length:c.length,kind:0}));drop(input.chain(UnordItems::new(Input{next:0,length:c.inner,kind:1})))},_=>unreachable!()}}
fn body(){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(execute));let message=result.err().map(|v|v.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer;impl Drop for Outer{fn drop(&mut self){body()}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let n:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{family:n[0]as usize,length:n[1]as usize,inner:n[2]as usize,keep:n[3]as u64,next_fail:n[4]as isize,call_fail:n[5]as isize,item_fail:n[6]as isize,env_fail:n[7]!=0,token_fail:n[8]!=0,input_fail:n[9]!=0,resume:n[10]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body()}}));match mode.as_str(){"normal"=>body(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
