#![feature(rustc_private,step_trait)]
// Actual inherited comparison/Step operations on unchanged original types.
use rustc_span::hygiene::{ExpnIndex,MacroKind,Transparency};
use std::iter::Step;
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
fn ord<T:Ord+Copy>(a:T,b:T,c:T,raw:impl Fn(T)->u32)->String{
 let clamped=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||a.clamp(b,c)));
 let clamp=match clamped{Ok(v)=>raw(v).to_string(),Err(p)=>p.downcast_ref::<&str>().map(|s|s.to_string()).or_else(||p.downcast_ref::<String>().cloned()).unwrap_or_default()};
 format!("{:?} {:?} {} {} {} {} {} {} {}\n",a.cmp(&b),a.partial_cmp(&b),a.lt(&b),a.le(&b),a.gt(&b),a.ge(&b),raw(a.min(b)),raw(a.max(b)),clamp)
}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();
 if args[1]=="--count"{println!("209");return}
 let n:usize=args[1].parse().unwrap();
 let output=if n<125{
  let values=[0,1,17,0xffff_feff,0xffff_ff00];
  ord(ExpnIndex::from_u32(values[n/25]),ExpnIndex::from_u32(values[n/5%5]),ExpnIndex::from_u32(values[n%5]),|v|v.as_u32())
 }else if n<152{
  let n=n-125;let values=[MacroKind::Bang,MacroKind::Attr,MacroKind::Derive];
  ord(values[n/9],values[n/3%3],values[n%3],|v|v as u32)
 }else if n<179{
  let n=n-152;let values=[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque];let a=values[n/9];let b=values[n/3%3];
  format!("{:?} {} {} {} {}\n",a.partial_cmp(&b),a.lt(&b),a.le(&b),a.gt(&b),a.ge(&b))
 }else if n<204{
  let n=n-179;let values=[0,1,17,0xffff_feff,0xffff_ff00];
  format!("{:?}\n",ExpnIndex::steps_between(&ExpnIndex::from_u32(values[n/5]),&ExpnIndex::from_u32(values[n%5])))
 }else{
  let(a,count)=[(17,0),(17,1),(17,17),(0xffff_ff00,0),(0xffff_feff,1)][n-204];let a=ExpnIndex::from_u32(a);
  let forward=ExpnIndex::forward(a,count);let backward=ExpnIndex::backward(a,count);
  format!("{} {} {} {} {} {:?} {:?} {:?} {:?}\n",std::any::type_name::<ExpnIndex>(),forward.as_u32(),backward.as_u32(),unsafe{ExpnIndex::forward_unchecked(a,count)}.as_u32(),unsafe{ExpnIndex::backward_unchecked(a,count)}.as_u32(),ExpnIndex::forward_checked(a,count).map(ExpnIndex::as_u32),ExpnIndex::backward_checked(a,count).map(ExpnIndex::as_u32),{let(v,o)=ExpnIndex::forward_overflowing(a,count);(v.as_u32(),o)},{let(v,o)=ExpnIndex::backward_overflowing(a,count);(v.as_u32(),o)})
 };
 println!("{n}\t0\t{}",hex(&output));
}
