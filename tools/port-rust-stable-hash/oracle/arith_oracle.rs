// Exact original int_overflow.rs is verified/copied before compilation.
#[path="arith_source.rs"]mod arithmetic;
use arithmetic::{DebugStrictAdd,DebugStrictSub};use serde_json::{Value,json};use std::io::{self,BufRead};
fn main(){std::panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();let a=q["a"].as_u64().unwrap()as usize;let b=q["b"].as_u64().unwrap()as usize;let result=std::panic::catch_unwind(||if q["op"]=="add"{a.debug_strict_add(b)}else{a.debug_strict_sub(b)});let result=match result{Ok(v)=>json!({"value":v}),Err(p)=>json!({"panic":if let Some(s)=p.downcast_ref::<&str>(){(*s).to_owned()}else{p.downcast_ref::<String>().unwrap().clone()}})};println!("{}",json!({"request":q,"result":result}))}}
