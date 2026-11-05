// SPDX-License-Identifier: MIT OR Apache-2.0
// Matching pinned std native error-kind/message client; scoped used-contract proof.
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn main(){for code in (-256..=4096).chain([i32::MIN,i32::MAX]){let e=std::io::Error::from_raw_os_error(code);println!("{code}\t{:?}\t{}\t{}",e.kind(),hex(e.to_string()),hex(format!("{e:?}")));}}
