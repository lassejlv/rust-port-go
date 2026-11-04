// Development-only direct std/FxHashMap reference. All expected bucket order,
// capacities, entry results and clone behavior come from the actual Rust types.
#![feature(map_try_insert)]
use rustc_hash::FxHashMap;
use std::{collections::hash_map::Entry,io::{self,Write}};
fn random(state:&mut u64)->u64{*state^=*state<<13;*state^=*state>>7;*state^=*state<<17;*state}
fn main()->io::Result<()>{let mut out=io::BufWriter::new(io::stdout().lock());for seed in 1..=24u64{let mut rng=seed;let mut map=FxHashMap::<u64,u64>::default();for step in 0..4000{let x=random(&mut rng);let key=(x>>8)%257;let value=random(&mut rng);let result=match x%17{
0..=4=>format!("{:?}",map.insert(key,value)),
5..=7=>format!("{:?}",map.remove(&key)),
8=>format!("{:?}",map.get(&key)),
9=>{map=map.clone();String::new()},
10=>{if step%997==0{map.clear()}String::new()},
11=>format!("{}",map.entry(key).or_insert(value)),
12=>format!("{}",map.entry(key).and_modify(|v|*v=v.wrapping_add(value)).or_default()),
13=>match map.try_insert(key,value){Ok(v)=>format!("Ok({v})"),Err(e)=>format!("Err({},{},{})",e.entry.get(),e.key,e.value)},
14=>{let entry=map.entry(key).insert_entry(value);format!("{}:{}",entry.key(),entry.get())},
15=>match map.entry(key){Entry::Occupied(entry)=>format!("{:?}",entry.remove_entry()),Entry::Vacant(entry)=>format!("Key({})",entry.into_key())},
16=>{let mut dest=FxHashMap::with_capacity_and_hasher(500,Default::default());dest.clone_from(&map);map=dest;String::new()},
_=>unreachable!()};write!(out,"{seed} {step} {} {} {result} ",map.len(),map.capacity())?;for(k,v)in &map{write!(out,"{k}:{v},")?;}writeln!(out)?;}}Ok(())}
