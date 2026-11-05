// Calls unchanged original UnordItems reductions with independent trait paths.
#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
use rustc_data_structures::unord::UnordItems;
use std::{cell::Cell,cmp::Ordering,io::{self,Write},iter::{Sum,Product},panic::{self,catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{family:usize,length:usize,next_fail:isize,compare_fail:isize,item_fail:isize,total_fail:isize,input_fail:bool,trait_fail:bool,resume:bool}
thread_local!{static CONFIG:Cell<Config>=const{Cell::new(Config{family:0,length:0,next_fail:-1,compare_fail:-1,item_fail:-1,total_fail:-1,input_fail:false,trait_fail:false,resume:false})};static COMPARES:Cell<usize>=const{Cell::new(0)};}
fn event(s:impl AsRef<str>){println!("{}",s.as_ref());io::stdout().flush().unwrap();}fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
#[track_caller]fn raise(s:&'static str)->!{if CONFIG.get().resume{panic::resume_unwind(Box::new(s))}else{panic::panic_any(s)}}
struct Value(u64);impl Drop for Value{fn drop(&mut self){event(format!("item:{}:{}",self.0,std::thread::panicking()));if self.0 as isize==CONFIG.get().item_fail{raise("item_drop")}}}
impl PartialEq for Value{fn eq(&self,b:&Self)->bool{self.0%3==b.0%3}}impl Eq for Value{}
impl PartialOrd for Value{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(self.cmp(b))}}
impl Ord for Value{fn cmp(&self,b:&Self)->Ordering{let n=COMPARES.get();COMPARES.set(n+1);event(format!("cmp:{n}:{}:{}:{}",self.0,b.0,std::thread::panicking()));if n as isize==CONFIG.get().compare_fail{raise("compare")};(self.0%3).cmp(&(b.0%3))}
 fn max(self,other:Self)->Self{event("ord_max");if self.cmp(&other).is_gt(){drop(other);self}else{drop(self);other}}fn min(self,other:Self)->Self{event("ord_min");if self.cmp(&other).is_le(){drop(other);self}else{drop(self);other}}}
struct Input{next:usize}impl Drop for Input{fn drop(&mut self){event(format!("input:{}:{}",self.next,std::thread::panicking()));if CONFIG.get().input_fail{raise("input_drop")}}}
impl Iterator for Input{type Item=Value;fn next(&mut self)->Option<Value>{let n=self.next;self.next+=1;event(format!("next:{n}:{}",std::thread::panicking()));if n as isize==CONFIG.get().next_fail{raise("next")};if n<CONFIG.get().length{Some(Value(n as u64))}else{None}}}
struct ByInput(Input);impl Iterator for ByInput{type Item=Value;fn next(&mut self)->Option<Value>{self.0.next()}
 fn max_by<F:FnMut(&Value,&Value)->Ordering>(self,_f:F)->Option<Value>{raise("UNEXPECTED max_by")}
 fn min_by<F:FnMut(&Value,&Value)->Ordering>(self,_f:F)->Option<Value>{raise("UNEXPECTED min_by")}}
struct ReduceInput(Input);impl Iterator for ReduceInput{type Item=Value;fn next(&mut self)->Option<Value>{raise("UNEXPECTED next")}
 fn reduce<F:FnMut(Value,Value)->Value>(self,mut f:F)->Option<Value>{event("reduce_override");Some(f(Value(98),Value(99)))}}
struct DirectInput(Input);impl Iterator for DirectInput{type Item=Value;fn next(&mut self)->Option<Value>{raise("UNEXPECTED next")}
 fn max(self)->Option<Value>{event("max_override");Some(Value(77))}fn min(self)->Option<Value>{event("min_override");Some(Value(77))}
 fn sum<S:Sum<Value>>(self)->S{event("sum_override");S::sum(std::iter::once(Value(77)))}fn product<P:Product<Value>>(self)->P{event("product_override");P::product(std::iter::once(Value(77)))}}
struct FoldInput(Input);impl Iterator for FoldInput{type Item=Value;fn next(&mut self)->Option<Value>{self.0.next()}
 fn fold<B,F:FnMut(B,Value)->B>(self,mut init:B,mut f:F)->B{event("fold_override");init=f(init,Value(98));f(init,Value(99))}}
struct Total(u64);impl Drop for Total{fn drop(&mut self){event(format!("total:{}:{}",self.0,std::thread::panicking()));if self.0 as isize==CONFIG.get().total_fail{raise("total_drop")}}}
impl Sum<Value> for Total{fn sum<I:Iterator<Item=Value>>(input:I)->Self{event("sum_trait");if CONFIG.get().trait_fail{raise("sum_trait")};input.fold(Total(0),|acc,value|{event(format!("sum_step:{}:{}:{}",acc.0,value.0,std::thread::panicking()));let result=Total(acc.0+value.0);drop(value);result})}}
impl Product<Value> for Total{fn product<I:Iterator<Item=Value>>(input:I)->Self{event("product_trait");if CONFIG.get().trait_fail{raise("product_trait")};input.fold(Total(1),|acc,value|{event(format!("product_step:{}:{}:{}",acc.0,value.0,std::thread::panicking()));let result=Total(acc.0*(value.0+1));drop(value);result})}}
fn extreme<I:Iterator<Item=Value>>(input:I,max:bool){let result=if max{UnordItems::new(input).max()}else{UnordItems::new(input).min()};event(format!("returned:{}",result.as_ref().map(|v|v.0.to_string()).unwrap_or_else(||"none".into())));drop(result)}
fn total<I:Iterator<Item=Value>>(input:I,sum:bool){let result:Total=if sum{UnordItems::new(input).sum()}else{UnordItems::new(input).product()};event(format!("returned:{}",result.0));drop(result)}
fn execute(){let c=CONFIG.get();let input=Input{next:0};match c.family{0=>extreme(input,true),1=>extreme(input,false),2=>extreme(ByInput(input),true),3=>extreme(ByInput(input),false),4=>extreme(ReduceInput(input),true),5=>extreme(ReduceInput(input),false),6=>extreme(DirectInput(input),true),7=>extreme(DirectInput(input),false),8=>total(input,true),9=>total(input,false),10=>total(DirectInput(input),true),11=>total(DirectInput(input),false),12=>total(FoldInput(input),true),13=>total(FoldInput(input),false),14=>extreme(FoldInput(input),true),15=>extreme(FoldInput(input),false),_=>unreachable!()}}
fn body(){event(format!("body:{}",std::thread::panicking()));let result=catch_unwind(AssertUnwindSafe(execute));let message=result.err().map(|v|v.downcast_ref::<&str>().copied().unwrap_or("opaque").to_owned()).unwrap_or_default();event(format!("caught:{}:{}",hex(&message),std::thread::panicking()));}
struct Outer;impl Drop for Outer{fn drop(&mut self){body()}}
fn main(){let args:Vec<_>=std::env::args().collect();let mode=args[1].clone();let n:Vec<i64>=args[2].split(',').map(|v|v.parse().unwrap()).collect();CONFIG.set(Config{family:n[0]as usize,length:n[1]as usize,next_fail:n[2]as isize,compare_fail:n[3]as isize,item_fail:n[4]as isize,total_fail:n[5]as isize,input_fail:n[6]!=0,trait_fail:n[7]!=0,resume:n[8]!=0});let processing=mode=="processing";panic::set_hook(Box::new(move|info|{let payload=info.payload().downcast_ref::<&str>().copied().unwrap_or("");let kind=if info.payload().is::<&str>(){"static_str"}else{"opaque"};let(file,line,column)=info.location().map(|l|(l.file(),l.line(),l.column())).unwrap_or(("",0,0));event(format!("HOOK:{kind}:{}:{}:{}:{}:{}:{line}:{column}",hex(payload),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),hex(file)));if processing{body()}}));match mode.as_str(){"normal"=>body(),"unwind"=>{let _=catch_unwind(||{let _outer=Outer;panic::panic_any("outer")});event("outer_end:false")},"processing"=>{let _=catch_unwind(||panic::panic_any("outer"));event("outer_end:false")},_=>unreachable!()}}
