#![feature(rustc_private)]
// Malformed primitive values remain lawful Decoder returns. The entire actual
// original source validates its enum/Option/index/Vec domains and partial state.
#[path="codec.rs"]mod client;
use client::{Atom,D};
use rustc_span::hygiene::{ExpnId,ExpnIndex};
use rustc_span::def_id::CrateNum;
use std::panic::{catch_unwind,AssertUnwindSafe};
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn input<const B:bool>(n:usize)->(usize,Vec<Atom>)where D<B>:rustc_serialize::Decoder{
 if n<10{let ty=[0,3,6,9,23][n/2];let tag=if n%2==0{[3,3,3,13,4][n/2]}else{255};return(ty,vec![Atom::U8(tag)])}
 if n<12{return(47,vec![Atom::U32([0x2000_0000,u32::MAX][n-10])])}
 if n<14{return(50,vec![Atom::Expn(ExpnId{krate:CrateNum::from_u32(if n==12{1}else{0xffff_ff00}),local_id:ExpnIndex::from_u32(17)})])}
 if n<28{
  let mut atoms=if B{client::encoded::<true>(55)}else{client::encoded::<false>(55)};
  match n{
   14..=15=>atoms[7]=Atom::U8(if n==14{2}else{255}),
   16..=17=>atoms[11]=Atom::U8(if n==16{5}else{255}),
   18..=19=>atoms[12]=Atom::U8(if n==18{2}else{255}),
   20..=21=>atoms[13]=Atom::U8(if n==20{2}else{255}),
   22..=23=>atoms[8]=Atom::Usize(if n==22{usize::MAX}else{isize::MAX as usize/4+1}),
   24..=25=>atoms[5]=Atom::U32(if n==24{1}else{u32::MAX}),
   26..=27=>atoms[15]=if B{Atom::Bool(true)}else{Atom::U8(if n==26{2}else{255})},
   _=>unreachable!()
  };return(55,atoms)
 }
 if n<30{return(20,vec![Atom::U8(11),if B{Atom::Bool(true)}else{Atom::U8(if n==28{2}else{255})}])}
 if n==30{return(47,vec![Atom::U32(0x1fff_ffff)])}
 (9,vec![Atom::U8(12)])
}
fn run<const B:bool>(n:usize)where D<B>:rustc_serialize::Decoder{
 let(ty,atoms)=input::<B>(n);let mut d=D::<B>{atoms,pos:0,events:vec![],fail:usize::MAX};
 let result=catch_unwind(AssertUnwindSafe(||client::decode(ty,&mut d)));
 let(value,panic)=match result{Ok(v)=>(v,String::new()),Err(p)=>{let text=if let Some(s)=p.downcast_ref::<&str>(){s.to_string()}else if let Some(s)=p.downcast_ref::<String>(){s.clone()}else{"other".into()};(String::new(),text)}};
 let output=format!("decode {} {} {}\nvalue {}\npanic {}\n",!panic.is_empty(),d.pos,d.events.join(","),value,hex(panic));println!("{n}\t{}\t{}",usize::from(B),hex(output));
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let a:Vec<_>=std::env::args().collect();if a[1]=="--count"{println!("32");return}let n=a[1].parse().unwrap();rustc_span::create_default_session_globals_then(||{run::<false>(n);run::<true>(n)});}

// Uncaught operation entry for the shared full-source panic lifecycle client.
pub(crate) fn observe(n:usize,overridden:bool){if overridden{observe_typed::<true>(n)}else{observe_typed::<false>(n)}}
fn observe_typed<const B:bool>(n:usize)where D<B>:rustc_serialize::Decoder{
 struct Observed<const B:bool>(D<B>);
 impl<const B:bool>Drop for Observed<B>{fn drop(&mut self){println!("codec_state:{}:{}:{}",self.0.pos,self.0.events.join(","),std::thread::panicking())}}
 let(ty,atoms)=input::<B>(n);let mut d=Observed(D::<B>{atoms,pos:0,events:vec![],fail:usize::MAX});let _=client::decode(ty,&mut d.0);println!("returned:codec");
}
