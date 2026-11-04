// Real std HashSet layouts, including narrow elements, aligned values and ZSTs.
use rustc_hash::FxHashSet;
use std::{hash::Hash,io::{self,Write}};
#[derive(Clone,Copy,Hash,Eq,PartialEq)]#[repr(align(32))]struct Aligned(u8);
#[derive(Clone,Copy,Hash,Eq,PartialEq)]#[repr(align(32))]struct AlignedUnit;
fn run<T:Hash+Eq+Copy>(out:&mut impl Write,kind:usize,make:impl Fn(u64)->T,identity:impl Fn(&T)->u64)->io::Result<()>{let mut set=FxHashSet::<T>::default();for step in 0..512u64{let key=make((step*31)%256);let result=match step%7{0..=2=>set.insert(key).to_string(),3=>set.remove(&key).to_string(),4=>set.contains(&key).to_string(),5=>{set=set.clone();"clone".into()},6=>{if step%63==0{set.clear();}"clear".into()},_=>unreachable!()};write!(out,"{kind}\t{step}\t{}\t{}\t{result}\t",set.len(),set.capacity())?;for key in &set{write!(out,"{},",identity(key))?;}writeln!(out)?;}Ok(())}
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());run(&mut out,0,|v|v as u8,|v|*v as u64)?;run(&mut out,1,|v|v as u16,|v|*v as u64)?;run(&mut out,2,|v|v as u32,|v|*v as u64)?;run(&mut out,3,|v|v,|v|*v)?;run(&mut out,4,|v|Aligned(v as u8),|v|v.0 as u64)?;run(&mut out,5,|_|(),|_|0)?;run(&mut out,6,|_|AlignedUnit,|_|0)?;run(&mut out,7,|v|v%2!=0,|v|u64::from(*v))?;Ok(())}
