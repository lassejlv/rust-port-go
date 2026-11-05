// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual stable1.99 public std io::Error, without copied SDK algorithms.
use std::{fmt::{self, Write}, io::{self, BufRead}};
fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b|format!("{b:02x}")).collect() }
struct Sink { out:String, calls:Vec<String>, fail:usize }
impl fmt::Write for Sink {
    fn write_str(&mut self, text:&str)->fmt::Result {
        self.calls.push(format!("S:{}",hex(text.as_bytes())));
        if self.calls.len()==self.fail {return Err(fmt::Error)}
        self.out.push_str(text); Ok(())
    }
    fn write_char(&mut self, value:char)->fmt::Result {
        self.calls.push(format!("C:{:x}",value as u32));
        if self.calls.len()==self.fail {return Err(fmt::Error)}
        self.out.push(value); Ok(())
    }
}
fn main() {
    for line in io::stdin().lock().lines() {
        let line=line.unwrap();let p:Vec<_>=line.split('\t').collect();
        assert_eq!(p[1],"errno");
        let mut sink=Sink{out:String::new(),calls:vec![],fail:p[3].parse().unwrap()};
        let result=write!(&mut sink,"{}",io::Error::from_raw_os_error(p[2].parse().unwrap()));
        println!("{}\t{}\t{}\t{}",p[0],result.is_ok(),hex(sink.out.as_bytes()),sink.calls.join(","));
    }
}
