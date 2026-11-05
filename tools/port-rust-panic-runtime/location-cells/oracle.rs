// SPDX-License-Identifier: MIT OR Apache-2.0
// Safe public Location pointer-to-integer comparisons within one process.
use std::{io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe},sync::{Arc,Mutex}};
#[inline(never)]fn site_a(){panic!("same")}
#[inline(never)]fn site_b(){panic!("same")}
fn main(){let cells=Arc::new(Mutex::new(Vec::new()));let observed=Arc::clone(&cells);std::panic::set_hook(Box::new(move|info|{let location=info.location().unwrap();observed.lock().unwrap().push((location as *const _ as usize,format!("{}:{}:{}",location.file(),location.line(),location.column())))}));for line in io::stdin().lock().lines(){let line=line.unwrap();let(id,seq)=line.split_once('\t').unwrap();cells.lock().unwrap().clear();for site in seq.chars(){let _=catch_unwind(AssertUnwindSafe(||match site{'a'=>site_a(),'b'=>site_b(),_=>unreachable!()}));}let records=cells.lock().unwrap();let mut matrix=String::new();for a in &*records{for b in &*records{matrix.push(if a.0==b.0{'1'}else{'0'})}}println!("{id}\t{}\t{matrix}",records.iter().map(|r|r.1.clone()).collect::<Vec<_>>().join("|"));}}
