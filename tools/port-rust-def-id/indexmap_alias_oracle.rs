// Exact public alias from the unchanged pinned whole def_id.rs.
use def_id_reference::{*,fingerprint::Fingerprint};
use rustc_hashes::Hash64;
use serde_json::{json,Value};
use std::{hash::BuildHasher,io::{self,BufRead}};
fn key(n:u64)->StableCrateId{DefPathHash(Fingerprint::new(Hash64::new(n),Hash64::new(0))).stable_crate_id()}
fn num(v:&Value,k:&str)->u64{v[k].as_u64().unwrap_or(0)}
fn pairs(map:&StableCrateIdMap)->Vec<(u64,u32)>{map.iter().map(|(k,v)|(k.as_u64(),v.as_u32())).collect()}
fn run(q:&Value)->Value{
 let mut map=StableCrateIdMap::with_capacity_and_hasher(num(q,"capacity")as usize,Default::default());let mut rows=Vec::new();
 for op in q["ops"].as_array().unwrap(){let k=key(num(op,"key"));let v=CrateNum::from_u32(num(op,"value")as u32);let n=num(op,"n")as usize;
 let result=match op["op"].as_str().unwrap(){
 "insert"=>{let(i,old)=map.insert_full(k,v);json!([i,old.map(CrateNum::as_u32)])},
 "get"=>json!(map.get_full(&k).map(|(i,k,v)|(i,k.as_u64(),v.as_u32()))),
 "entry"=>{let e=map.entry(k);json!([e.index(),e.key().as_u64(),matches!(e,indexmap::map::Entry::Occupied(_))])},
 "or_insert"=>json!(map.entry(k).or_insert(v).as_u32()),
 "with"=>json!(map.entry(k).or_insert_with(||v).as_u32()),
 "modify"=>{let e=map.entry(k).and_modify(|old|*old=v);json!([e.index(),e.key().as_u64()])},
 "insert_entry"=>{let e=map.entry(k).insert_entry(v);json!([e.index(),e.key().as_u64(),e.get().as_u32()])},
 "swap_remove"=>json!(map.swap_remove(&k).map(CrateNum::as_u32)),
 "shift_remove"=>json!(map.shift_remove(&k).map(CrateNum::as_u32)),
 "swap_index"=>json!(map.swap_remove_index(n).map(|(k,v)|(k.as_u64(),v.as_u32()))),
 "shift_index"=>json!(map.shift_remove_index(n).map(|(k,v)|(k.as_u64(),v.as_u32()))),
 "reverse"=>{map.reverse();Value::Null},"clear"=>{map.clear();Value::Null},
 "pop"=>json!(map.pop().map(|(k,v)|(k.as_u64(),v.as_u32()))),
 "reserve"=>{map.reserve(n);Value::Null},"reserve_exact"=>{map.reserve_exact(n);Value::Null},
 "clone"=>{let other=map.clone();json!({"capacity":other.capacity(),"entries":pairs(&other),"equal":map==other})},
 "clone_from"=>{let mut other=StableCrateIdMap::with_capacity_and_hasher(n,Default::default());for i in 0..n{other.insert(key(i as u64*17+1),CrateNum::from_u32(i as u32+1));}map.clone_from(&other);Value::Null},
 "truncate"=>{map.truncate(n);Value::Null},
 "drain"|"drain_count"|"get_range"=>{let start=n.min(map.len());let end=start+(num(op,"value")as usize%(map.len()-start+1));if op["op"]=="get_range"{json!(map.get_range(start..end).unwrap().iter().map(|(k,v)|(k.as_u64(),v.as_u32())).collect::<Vec<_>>())}else if op["op"]=="drain_count"{json!(map.drain(start..end).count())}else{let mut drain=map.drain(start..end);let mut result=Vec::new();while let Some((k,v))=drain.next(){result.push((k.as_u64(),v.as_u32()))}json!(result)}},
 "hash_one"=>json!(map.hasher().hash_one(k)),"debug"=>json!(format!("{map:?}")),_=>panic!("unknown op")};
 rows.push(json!({"value":result,"entries":pairs(&map),"capacity":map.capacity(),"len":map.len()}));
 }json!(rows)
}
fn main(){for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
