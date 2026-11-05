// Independently invoke actual SDK operations inside real Drop/panic-hook paths.
use std::{io::{self,Write},panic::{catch_unwind,AssertUnwindSafe}};
fn event(value:&str){eprintln!("EVENT:{value}");io::stderr().flush().unwrap();}
struct Scope;
impl Drop for Scope{fn drop(&mut self){event(&format!("scope_drop:{}",std::thread::panicking()))}}
fn invoke(op:&str){
 let _scope=Scope;event(&format!("before:{}",std::thread::panicking()));let text="aé🦀z";
 let _view=match op {
 "valid"=>&text[1..7],
 "boundary"=>&text[2..3],
 "backward"=>&text[7..1],
 "begin_oob"=>&text[9..10],
 "end_oob"=>&text[0..9],
 "inclusive_boundary"=>&text[1..=1],
 "inclusive_len"=>&text[0..=8],
 "from_boundary"=>&text[2..],
 "to_boundary"=>&text[..2],
 _=>panic!("unknown probe")};event("done");
}
fn run(op:&str){let result=catch_unwind(AssertUnwindSafe(||invoke(op)));event(&format!("caught:{}",result.is_err()));}
struct Ambient<'a>(&'a str);
impl Drop for Ambient<'_>{fn drop(&mut self){event("ambient_drop");run(self.0);}}
fn main(){let args:Vec<_>=std::env::args().collect();let ambient=args[1].clone();let op=args[2].clone();
 if ambient=="processing"{std::panic::set_hook(Box::new(move|_|{event("ambient_hook");run(&op);}));let result=catch_unwind(||panic!("ambient"));event(&format!("ambient_caught:{}",result.is_err()));}
 else{std::panic::set_hook(Box::new(|_|{event("hook");}));
 if ambient=="unwinding"{let result=catch_unwind(||{let _ambient=Ambient(&op);panic!("ambient")});event(&format!("ambient_caught:{}",result.is_err()));}else{run(&op);}}
}
