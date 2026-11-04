// Calls the complete unchanged rustc_data_structures crate through the shared
// full rustc_span reference graph. This probes only the retained map foundation.
#![feature(map_try_insert)]
use rustc_data_structures::unord::{UnordMap,UnordBag};
use std::io::{self,Write};
fn random(s:&mut u64)->u64{*s^=*s<<13;*s^=*s>>7;*s^=*s<<17;*s}
fn snapshot(m:&UnordMap<u64,(u64,bool)>)->String{format!("{:?}",m.items().map(|(k,(v,b))|(*k,(*v,*b))).collect::<UnordBag<_>>())}
fn read_snapshot(m:&UnordMap<u64,u64>)->String{format!("{:?}",m.items().map(|(k,v)|(*k,*v)).collect::<UnordBag<_>>())}
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());for seed in 1..=4u64{let mut state=seed;let mut map=UnordMap::<u64,(u64,bool)>::default();for step in 0..500{let x=random(&mut state);let key=(x>>8)%41;let value=random(&mut state);let mut frozen="-".to_owned();let result=match x%7{
0=>format!("{:?}",map.insert(key,(value,false))),
1=>match map.try_insert(key,(value,false)){Ok(_)=>"true".into(),Err(e)=>(e.entry.get().0==value).to_string()},
2=>{let value=map.get_mut(&key).map(|(v,b)|{*b=true;*v});format!("{value:?}")},
3=>{let read:UnordMap<u64,u64>=map.items().filter(|(_,(_,b))|*b).map(|(k,(v,_))|(*k,*v)).collect();frozen=read_snapshot(&read);"freeze".into()},
4=>{let clone=map.clone();map.clone_from(&clone);"clone".into()},
5=>{if step%97==0{map.clear()}"clear".into()},
6=>format!("{:?}",map.remove(&key)),_=>unreachable!()};writeln!(out,"{seed}\t{step}\t{}\t{result}\t{}\t{frozen}",map.len(),snapshot(&map))?;}}Ok(())}
