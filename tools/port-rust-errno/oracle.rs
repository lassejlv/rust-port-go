// SPDX-License-Identifier: MIT OR Apache-2.0
// Public actual SDK std Error/CStr operations; no copied error implementation.
use std::{ffi::CStr, fmt::{self, Write}, io::{self, BufRead}};
fn hex(s: impl AsRef<[u8]>) -> String { s.as_ref().iter().map(|b|format!("{b:02x}")).collect() }
struct Sink { out: String, calls: Vec<String>, fail: usize }
impl fmt::Write for Sink {
    fn write_str(&mut self,s:&str)->fmt::Result { self.calls.push(format!("S:{}",hex(s))); if self.calls.len()==self.fail {return Err(fmt::Error)} self.out.push_str(s); Ok(()) }
    fn write_char(&mut self,c:char)->fmt::Result { self.calls.push(format!("C:{:x}",c as u32)); if self.calls.len()==self.fail {return Err(fmt::Error)} self.out.push(c); Ok(()) }
}
fn main() {
    for line in io::stdin().lock().lines() {
        let line=line.unwrap(); let p:Vec<_>=line.split('\t').collect();
        let mut sink=Sink{out:String::new(),calls:vec![],fail:p[3].parse().unwrap()};
        let result=if p[1]=="errno" {write!(&mut sink,"{}",io::Error::from_raw_os_error(p[2].parse().unwrap()))}
        else {let mut bytes:Vec<u8>=p[2].as_bytes().chunks(2).map(|p|u8::from_str_radix(std::str::from_utf8(p).unwrap(),16).unwrap()).collect();bytes.push(0);let value=CStr::from_bytes_with_nul(&bytes).unwrap();write!(&mut sink,"{}",value.display())};
        println!("{}\t{}\t{}\t{}",p[0],result.is_ok(),hex(sink.out),sink.calls.join(","));
    }
}
