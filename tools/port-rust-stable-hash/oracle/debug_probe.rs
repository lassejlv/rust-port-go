use rustc_stable_hash::hashers::SipHasher128;
use std::{fmt::{self},hash::Hasher};
struct Sink;impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{println!("S:{s:?}");Ok(())}fn write_char(&mut self,c:char)->fmt::Result{println!("C:{c:?}");Ok(())}}
fn main(){let mut h=SipHasher128::new_with_keys(1,2);h.write(b"abc");println!("{h:?}");println!("{h:#?}");fmt::write(&mut Sink,format_args!("{h:?}")).unwrap();}
