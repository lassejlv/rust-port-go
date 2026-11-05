// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped public String/Vec used methods from each real selected SDK.
use std::{io::{self,BufRead},panic::{catch_unwind,AssertUnwindSafe},sync::Mutex};
static HOOK:Mutex<Vec<String>>=Mutex::new(Vec::new());
fn hex(s:&str)->String{s.as_bytes().iter().map(|c|format!("{c:02x}")).collect()}
fn text(s:&str)->String{String::from_utf8((0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect()).unwrap()}
fn snapshot(slots:&[Option<String>])->String{slots.iter().map(|s|match s{Some(s)=>format!("{}:{}:{}",s.len(),s.capacity(),hex(s)),None=>"-".into()}).collect::<Vec<_>>().join(";")}
fn main(){
 std::panic::set_hook(Box::new(|p|{let loc=p.location().unwrap();let kind=if p.payload().is::<&str>(){"static_str"}else{"owned_string"};let payload=p.payload().downcast_ref::<&str>().copied().or_else(||p.payload().downcast_ref::<String>().map(String::as_str)).unwrap();HOOK.lock().unwrap().push(format!("{kind}:{}:{}:{}:{}",hex(payload),hex(loc.file()),loc.line(),loc.column()));}));
 for line in io::stdin().lock().lines(){
  let line=line.unwrap();let p:Vec<_>=line.split('\t').collect();let initial=text(p[2]);
  let s=if p[0]=="from"{String::from(initial.as_str())}else{let mut s=String::with_capacity(p[1].parse().unwrap());s.push_str(&initial);s};
  let mut slots=vec![Some(s),None,None];let mut output=vec![snapshot(&slots)];
  for op in p[3].split(';').filter(|s|!s.is_empty()){
   let f:Vec<_>=op.split(':').collect();let slot:usize=f[1].parse().unwrap();HOOK.lock().unwrap().clear();let mut identity="-".to_string();
   let caught=catch_unwind(AssertUnwindSafe(||match f[0]{
    "push"=>{let value=text(f[2]);let s=slots[slot].as_mut().unwrap();let ptr=s.as_ptr();let stable=s.capacity()-s.len()>=value.len();s.push_str(&value);if stable{identity=(ptr==s.as_ptr()).to_string()}},
    "char"=>{let c=char::from_u32(f[2].parse().unwrap()).unwrap();let s=slots[slot].as_mut().unwrap();let ptr=s.as_ptr();let stable=s.capacity()-s.len()>=c.len_utf8();s.push(c);if stable{identity=(ptr==s.as_ptr()).to_string()}},
    "reserve"|"exact"=>{let n=f[2].parse().unwrap();let s=slots[slot].as_mut().unwrap();let ptr=s.as_ptr();let stable=s.capacity()-s.len()>=n;if f[0]=="reserve"{s.reserve(n)}else{s.reserve_exact(n)};if stable{identity=(ptr==s.as_ptr()).to_string()}},
    "clone"=>{let src:usize=f[2].parse().unwrap();let value=slots[src].as_ref().unwrap().clone();identity=(value.as_ptr()==slots[src].as_ref().unwrap().as_ptr()).to_string();slots[slot]=Some(value)},
    "clonefrom"=>{let src:usize=f[2].parse().unwrap();let(a,b)=if slot<src{let(a,b)=slots.split_at_mut(src);(a[slot].as_mut().unwrap(),b[0].as_ref().unwrap())}else{let(a,b)=slots.split_at_mut(slot);(b[0].as_mut().unwrap(),a[src].as_ref().unwrap())};let ptr=a.as_ptr();let stable=a.capacity()>=b.len();a.clone_from(b);if stable{identity=(ptr==a.as_ptr()).to_string()}},
    "move"=>{let src:usize=f[2].parse().unwrap();let value=slots[src].take().unwrap();let ptr=value.as_ptr();slots[slot]=Some(value);identity=(ptr==slots[slot].as_ref().unwrap().as_ptr()).to_string()},
    "drop"=>{slots[slot]=None},_=>panic!("invalid source operation"),
   })).is_err();
   output.push(format!("{caught},{identity},{},{}",HOOK.lock().unwrap().join("|"),snapshot(&slots)));
  }
  println!("{}",hex(&output.join("\n")));
 }
}
