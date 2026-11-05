// Original complete graph.rs composes with the actual SDK stdio/capture runtime.
#![feature(internal_output_capture,panic_can_unwind,panic_internals)]
use std::{io::{self,Write},panic::{self,AssertUnwindSafe},sync::{Arc,Mutex,mpsc},time::Duration};
mod original {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/graph.rs"));
    pub fn print(graph:&Graph<usize,()>){graph.print_for_test()}
}
fn hex(bytes:&[u8])->String{bytes.iter().map(|v|format!("{v:02x}")).collect()}
fn record(text:String){let mut out=io::stdout().lock();out.write_all(text.as_bytes()).unwrap();out.write_all(b"\n").unwrap();}
fn payload(value:&(dyn std::any::Any+Send))->(&str,&str){if let Some(v)=value.downcast_ref::<&str>(){(v,"static_str")}else if let Some(v)=value.downcast_ref::<String>(){(v,"owned_string")}else{("opaque","opaque")}}
fn snapshot(stage:&str,capture:&Arc<Mutex<Vec<u8>>>){let value=capture.lock().unwrap();record(format!("S\t{stage}\t{}\t{}\t{}\t{}",value.len(),value.capacity(),Arc::strong_count(capture),hex(&value)));}
fn main(){
 let args:Vec<_>=std::env::args().collect();assert_eq!(args.len(),7);let nodes=args[1].parse::<usize>().unwrap();let enabled=args[2]=="1";let preheld=args[3]=="1";let repeats=args[4].parse::<usize>().unwrap();let capacity=args[5].parse::<usize>().unwrap();let error=args[6]=="eio";
 panic::set_hook(Box::new(move|info|{let(message,kind)=payload(info.payload());let location=info.location().unwrap();record(format!("H\t{kind}\t{}\t{}\t{}\t{}\t{}\t{}",hex(message.as_bytes()),hex(location.file().as_bytes()),location.line(),location.column(),info.can_unwind(),info.force_no_backtrace()));if error&&!preheld{let(tx,rx)=mpsc::channel();let handle=std::thread::spawn(move||{let _guard=io::stderr().lock();tx.send(()).unwrap();});let acquired=rx.recv_timeout(Duration::from_secs(2)).is_ok();record(format!("L\t{acquired}"));if acquired{handle.join().unwrap();}}}));
 let mut graph=original::Graph::<usize,()>::new();for n in 0..nodes{graph.add(n);}for n in 1..nodes{*graph.link(n-1,n)=();}
 let capture=Arc::new(Mutex::new(Vec::with_capacity(capacity)));snapshot("before",&capture);
 let old=io::set_output_capture(if enabled{Some(capture.clone())}else{None});assert!(old.is_none());snapshot("set",&capture);
 let held=if preheld{Some(io::stderr().lock())}else{None};
 for n in 1..=repeats{let result=panic::catch_unwind(AssertUnwindSafe(||original::print(&graph)));let message=result.as_ref().err().map(|value|payload(&**value).0).unwrap_or("");record(format!("P\t{n}\t{}\t{}",result.is_err(),hex(message.as_bytes())));snapshot(&format!("print{n}"),&capture);}
 drop(held);let slot=io::set_output_capture(None);record(format!("R\t{}",slot.as_ref().is_some_and(|value|Arc::ptr_eq(value,&capture))));snapshot("popped",&capture);drop(slot);snapshot("dropped",&capture);
}
