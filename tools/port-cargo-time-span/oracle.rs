#![feature(panic_can_unwind,panic_internals)]
#![allow(internal_features)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Development-only: the entire original file and exact locked anyhow crate.
use std::{fmt, io::{self, BufRead}, panic::{catch_unwind, AssertUnwindSafe}};
type CargoResult<T> = anyhow::Result<T>;
mod original { include!(env!("FERRUGO_TIME_SPAN_SOURCE")); }
fn hex(text: &str) -> String { text.as_bytes().iter().map(|c| format!("{c:02x}")).collect() }
fn unhex(text: &str) -> String {
    let bytes = (0..text.len()).step_by(2).map(|i| u8::from_str_radix(&text[i..i+2],16).unwrap()).collect();
    String::from_utf8(bytes).unwrap()
}
struct Sink { calls: Vec<String>, accepted: String, fail: usize, chars: bool }
impl fmt::Write for Sink {
    fn write_str(&mut self, s: &str) -> fmt::Result {
        self.calls.push(format!("S:{}",hex(s)));
        if self.calls.len() == self.fail { return Err(fmt::Error); }
        self.accepted.push_str(s); Ok(())
    }
    fn write_char(&mut self, c: char) -> fmt::Result {
        if !self.chars { return self.write_str(c.encode_utf8(&mut [0;4])); }
        self.calls.push(format!("C:{}",hex(&c.to_string())));
        if self.calls.len() == self.fail { return Err(fmt::Error); }
        self.accepted.push(c); Ok(())
    }
}
fn formatted(value: &anyhow::Error, sink: &mut Sink, pattern: usize) -> fmt::Result {
    use fmt::Write;
    match pattern {
        0 => write!(sink,"{value}"), 1 => write!(sink,"{value:#}"),
        2 => write!(sink,"{value:>30}"), 3 => write!(sink,"{value:.2}"),
        4 => write!(sink,"{value:?}"), 5 => write!(sink,"{value:#?}"),
        6 => write!(sink,"{value:08?}"), 7 => write!(sink,"{value:#x?}"),
        8 => write!(sink,"{value:\0>30.2?}"), 9 => write!(sink,"{value:🦀^30.2?}"),
        10 => write!(sink,"{value:+#08X?}"), 11 => write!(sink,"{value:\n<60?}"),
        _ => unreachable!(),
    }
}
fn event(text: &str) { println!("EVENT:{text}"); }
struct BodyDrop;
impl Drop for BodyDrop { fn drop(&mut self) { event(&format!("scope_drop:{}",std::thread::panicking())); } }
fn invoke(op: &str, span: &str) {
    let _drop = BodyDrop;
    event(&format!("body:{}",std::thread::panicking()));
    if op == "R" {
        let mut error=original::parse_time_span(span).unwrap_err();
        error.downcast_mut::<String>().unwrap().reserve(usize::MAX);
    } else if op == "M" {
        match original::maybe_parse_time_span(span) { Some(d)=>event(&format!("some:{}:{}",d.as_secs(),d.subsec_nanos())),None=>event("none") }
    } else {
        match original::parse_time_span(span) { Ok(d)=>event(&format!("some:{}:{}",d.as_secs(),d.subsec_nanos())),Err(e)=>event(&format!("error:{}",hex(&e.to_string()))) }
    }
}
fn ambient(mode:String,op:String,span:String) {
    let (hook_mode,hook_op,hook_span)=(mode.clone(),op.clone(),span.clone());
    std::panic::set_hook(Box::new(move|p|{
        let l=p.location().unwrap();
        let kind=if p.payload().is::<&str>(){"static_str"}else if p.payload().is::<String>(){"owned_string"}else{"opaque"};
        event(&format!("hook:{kind}:{}:{}:{}:{}:{}:{}",hex(p.payload_as_str().unwrap_or("opaque")),hex(l.file()),l.line(),l.column(),p.can_unwind(),p.force_no_backtrace()));
        if hook_mode=="processing"&&p.payload_as_str()==Some("outer"){invoke(&hook_op,&hook_span);}
    }));
    if mode=="normal" {
        let caught=catch_unwind(AssertUnwindSafe(||invoke(&op,&span))).is_err();event(&format!("caught:{caught}"));return;
    }
    struct Outer{op:String,span:String}
    impl Drop for Outer{fn drop(&mut self){let caught=catch_unwind(AssertUnwindSafe(||invoke(&self.op,&self.span))).is_err();event(&format!("inner_caught:{caught}"));}}
    let caught=catch_unwind(AssertUnwindSafe(||{if mode=="unwinding"{let _outer=Outer{op,span};panic!("outer");}else{panic!("outer");}})).is_err();
    event(&format!("outer_caught:{caught}"));
}
fn main() {
    let args:Vec<_>=std::env::args().collect();
    if args.len()>1 {ambient(args[1].clone(),args[2].clone(),unhex(&args[3]));return;}
    std::panic::set_hook(Box::new(|_| {}));
    for line in io::stdin().lock().lines() {
        let line=line.unwrap(); let p:Vec<_>=line.split('\t').collect();
        let span=unhex(p[1]);
        if p[0]=="F" {
            let error=original::parse_time_span(&span).unwrap_err();
            let bt=error.backtrace().to_string();
            let mut sink=Sink{calls:vec![],accepted:String::new(),fail:p[3].parse().unwrap(),chars:p[4]=="true"};
            let ok=formatted(&error,&mut sink,p[2].parse().unwrap()).is_ok();
            println!("F\t{}\t{}\t{}\t{}\t{}",ok,sink.calls.join(","),hex(&sink.accepted),hex(&bt),error.backtrace().status()==std::backtrace::BacktraceStatus::Captured);
            continue;
        }
        let result=catch_unwind(AssertUnwindSafe(|| if p[0]=="M" {
            match original::maybe_parse_time_span(&span) { Some(d)=>format!("some\t{}\t{}",d.as_secs(),d.subsec_nanos()),None=>"none".into() }
        } else {
            match original::parse_time_span(&span) {
                Ok(d)=>format!("some\t{}\t{}",d.as_secs(),d.subsec_nanos()),
                Err(error)=>{
                    let display=error.to_string();let debug=format!("{error:?}");let alternate=format!("{error:#?}");
                    let string_type=error.is::<String>();let static_type=error.is::<&'static str>();let source_none=error.source().is_none();let chain=error.chain().count();
                    let cell=error.downcast_ref::<String>().unwrap() as *const String;
                    let trace=error.backtrace() as *const std::backtrace::Backtrace;
                    let moved=error;
                    let stable=cell==moved.downcast_ref::<String>().unwrap()as*const String;
                    let trace_stable=trace==moved.backtrace()as*const std::backtrace::Backtrace;
                    let mut moved=moved;
                    let mut snapshots=vec![];
                    {
                        let text=moved.downcast_mut::<String>().unwrap();
                        snapshots.push(format!("initial:{}:{}",text.len(),text.capacity()));
                        let old=text.as_ptr();text.reserve(0);snapshots.push(format!("reserve0:{}:{}:{}",text.len(),text.capacity(),old==text.as_ptr()));
                        let old=text.as_ptr();let stable=text.len()<text.capacity();text.push('!');snapshots.push(format!("push1:{}:{}:{}",text.len(),text.capacity(),if stable {(old==text.as_ptr()).to_string()}else{"-".into()}));
                        let old=text.as_ptr();let stable=text.capacity()-text.len()>=13;text.reserve(13);snapshots.push(format!("reserve13:{}:{}:{}",text.len(),text.capacity(),if stable {(old==text.as_ptr()).to_string()}else{"-".into()}));
                        let old=text.as_ptr();let stable=text.capacity()-text.len()>=4;text.push('🦀');snapshots.push(format!("push4:{}:{}:{}",text.len(),text.capacity(),if stable {(old==text.as_ptr()).to_string()}else{"-".into()}));
                        let old=text.as_ptr();let stable=text.capacity()-text.len()>=4;text.push_str("tail");snapshots.push(format!("append:{}:{}:{}",text.len(),text.capacity(),if stable {(old==text.as_ptr()).to_string()}else{"-".into()}));
                    }
                    let buffer=moved.downcast_ref::<String>().unwrap().as_ptr();
                    let result=moved.downcast::<String>().unwrap();let buffer_stable=buffer==result.as_ptr();
                    snapshots.push(format!("downcast:{}:{}",result.len(),result.capacity()));
                    format!("error\t{}\t{}\t{}\t{string_type}\t{static_type}\t{source_none}\t{chain}\t{stable}\t{trace_stable}\t{buffer_stable}\t{}\t{}",hex(&display),hex(&debug),hex(&alternate),hex(&result),snapshots.join("|"))
                }
            }
        }));
        match result { Ok(output)=>println!("{}\t{}",p[0],output),Err(payload)=>{
            let message=payload.downcast_ref::<&str>().copied().or_else(||payload.downcast_ref::<String>().map(String::as_str)).unwrap();
            println!("{}\tpanic\t{}",p[0],hex(message));
        }}
    }
}
