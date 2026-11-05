// Cursor observer of unchanged indexmap2.14.0 borrowed iterator types.
use indexmap::IndexMap;
use serde_json::{json,Value};
use std::{fmt::Debug,io::{self,BufRead}};
fn n(v:&Value,k:&str)->usize{v[k].as_u64().unwrap_or(0)as usize}
fn debug(v:&impl Debug,mode:usize)->String{match mode{1=>format!("{v:#?}"),2=>format!("{v:x?}"),3=>format!("{v:+08X?}"),_=>format!("{v:?}")}}
fn shared<I,F>(mut iter:I,steps:&[Value],convert:F,snapshot:Option<fn(&I)->Value>)->Value where I:Iterator+DoubleEndedIterator+ExactSizeIterator+Clone+Debug,F:Fn(I::Item)->Value{
 let mut rows=Vec::new();for op in steps{let value=match op["op"].as_str().unwrap(){
 "next"=>iter.next().map(&convert),"back"=>iter.next_back().map(&convert),"nth"=>iter.nth(n(op,"n")).map(&convert),"nth_back"=>iter.nth_back(n(op,"n")).map(&convert),
 "count"=>Some(json!(iter.clone().count())),"last"=>iter.clone().last().map(&convert),
 "clone_from"=>{let mut other=iter.clone();let _=other.nth(n(op,"n"));iter.clone_from(&other);None},
 "slice"=>Some(snapshot.unwrap()(&iter)),"debug"=>Some(json!(debug(&iter,n(op,"mode")))),_=>None};rows.push(json!({"value":value,"len":iter.len(),"hint":iter.size_hint(),"debug":debug(&iter,0)}))}json!(rows)
}
fn mutable<I,F>(mut iter:I,steps:&[Value],convert:F,snapshot:Option<fn(&I)->Value>,into_slice:Option<fn(I,i64)->Value>)->Value where I:Iterator+DoubleEndedIterator+ExactSizeIterator+Debug,F:Fn(I::Item,i64)->Value{
 let mut rows=Vec::new();for op in steps{let delta=op["delta"].as_i64().unwrap_or(0);let value=match op["op"].as_str().unwrap(){
 "next"=>iter.next().map(|v|convert(v,delta)),"back"=>iter.next_back().map(|v|convert(v,delta)),"nth"=>iter.nth(n(op,"n")).map(|v|convert(v,delta)),"nth_back"=>iter.nth_back(n(op,"n")).map(|v|convert(v,delta)),
 "count"=>{rows.push(json!({"value":iter.count(),"len":null,"hint":null,"debug":null}));return json!(rows)},
 "last"=>{let value=iter.last().map(|v|convert(v,delta));rows.push(json!({"value":value,"len":null,"hint":null,"debug":null}));return json!(rows)},
 "slice"=>Some(snapshot.unwrap()(&iter)),
 "into_slice"=>{let value=into_slice.unwrap()(iter,delta);rows.push(json!({"value":value,"len":null,"hint":null,"debug":null}));return json!(rows)},
 "debug"=>Some(json!(debug(&iter,n(op,"mode")))),_=>None};rows.push(json!({"value":value,"len":iter.len(),"hint":iter.size_hint(),"debug":debug(&iter,0)}))}json!(rows)
}
fn run(q:&Value)->Value{
 let mut map=IndexMap::<u64,i64>::new();for i in 0..n(q,"n"){map.insert(i as u64*17+1,-(i as i64)*9-1);}let steps=q["steps"].as_array().unwrap();
 let empty=q["default"].as_bool().unwrap_or(false);let rows=match q["kind"].as_str().unwrap(){
 "iter"=>shared(if empty{Default::default()}else{map.iter()},steps,|(k,v)|json!([k,v]),Some(|it|json!(it.as_slice().iter().map(|(k,v)|(*k,*v)).collect::<Vec<_>>()))),
 "keys"=>shared(if empty{Default::default()}else{map.keys()},steps,|k|json!(k),None),
 "values"=>shared(if empty{Default::default()}else{map.values()},steps,|v|json!(v),None),
 "iter_mut"=>mutable(if empty{Default::default()}else{map.iter_mut()},steps,|(k,v),delta|{*v+=delta;json!([k,*v])},Some(|it|json!(it.as_slice().iter().map(|(k,v)|(*k,*v)).collect::<Vec<_>>())),Some(|it,delta|json!(it.into_slice().iter_mut().map(|(k,v)|{*v+=delta;(*k,*v)}).collect::<Vec<_>>()))),
 "values_mut"=>mutable(if empty{Default::default()}else{map.values_mut()},steps,|v,delta|{*v+=delta;json!(*v)},None,None),_=>panic!("bad kind")};
 json!({"rows":rows,"entries":map.into_iter().collect::<Vec<_>>()})
}
fn main(){for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();println!("{}",json!({"request":q,"result":run(&q)}))}}
