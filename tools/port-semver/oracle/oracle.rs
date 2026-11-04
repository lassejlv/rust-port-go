// Development-only oracle. Calls the unchanged, checksum-verified official
// semver 1.0.28 crate, including its real std/serde feature profiles.
#![cfg_attr(feature="prefixfree", feature(hasher_prefixfree_extras))]
use semver::{Version, VersionReq, Comparator, Prerelease, BuildMetadata, Op};
use serde_json::{json,Value};
use std::{io::{self,BufRead},hash::{Hash,Hasher},fmt};
fn hex(b:&[u8])->String{b.iter().map(|x|format!("{x:02x}")).collect()}
#[derive(Default)]struct Raw(Vec<String>);impl Hasher for Raw{fn finish(&self)->u64{0}fn write(&mut self,b:&[u8]){self.0.push(hex(b))}}
#[derive(Default)]struct Primitive(Vec<String>);impl Hasher for Primitive{fn finish(&self)->u64{0}fn write(&mut self,b:&[u8]){self.0.push(format!("bytes:{}",hex(b)))}fn write_u8(&mut self,v:u8){self.0.push(format!("u8:{v}"))}fn write_u64(&mut self,v:u64){self.0.push(format!("u64:{v}"))}fn write_usize(&mut self,v:usize){self.0.push(format!("usize:{v}"))}fn write_isize(&mut self,v:isize){self.0.push(format!("isize:{v}"))}}
#[cfg(feature="prefixfree")]
#[derive(Default)]struct Prefix(Vec<String>);#[cfg(feature="prefixfree")]impl Hasher for Prefix{fn finish(&self)->u64{0}fn write(&mut self,b:&[u8]){self.0.push(format!("bytes:{}",hex(b)))}fn write_u8(&mut self,v:u8){self.0.push(format!("u8:{v}"))}fn write_u64(&mut self,v:u64){self.0.push(format!("u64:{v}"))}fn write_usize(&mut self,v:usize){self.0.push(format!("usize:{v}"))}fn write_isize(&mut self,v:isize){self.0.push(format!("isize:{v}"))}fn write_str(&mut self,v:&str){self.0.push(format!("str:{v}"))}fn write_length_prefix(&mut self,v:usize){self.0.push(format!("len:{v}"))}}
fn hash<T:Hash>(v:&T)->Value{let mut r=Raw::default();v.hash(&mut r);let mut p=Primitive::default();v.hash(&mut p);let mut out=json!({"raw":r.0,"primitive":p.0});#[cfg(feature="prefixfree")]{let mut h=Prefix::default();v.hash(&mut h);out["prefix"]=json!(h.0);}out}
fn error(e:semver::Error)->Value{json!({"ok":false,"display":e.to_string(),"debug":format!("{e:?}")})}
fn basic<T:fmt::Display+fmt::Debug+Hash+Clone+Eq>(v:T)->Value{json!({"ok":true,"display":v.to_string(),"debug":format!("{v:?}"),"pretty":format!("{v:#?}"),"hex":format!("{v:x?}"),"pretty_hex":format!("{v:#X?}"),"clone_eq":v==v.clone(),"hash":hash(&v)})}
fn op(i:u64)->Op{match i{0=>Op::Exact,1=>Op::Greater,2=>Op::GreaterEq,3=>Op::Less,4=>Op::LessEq,5=>Op::Tilde,6=>Op::Caret,7=>Op::Wildcard,_=>panic!("invalid op")}}
fn text<'a>(q:&'a Value,k:&str)->&'a str{q[k].as_str().unwrap_or("")}
fn cmp(q:&Value)->Comparator{Comparator{op:op(q["operator"].as_u64().unwrap()),major:q["major"].as_u64().unwrap(),minor:q["minor"].as_u64(),patch:q["patch"].as_u64(),pre:Prerelease::new(text(q,"pre")).unwrap()}}
struct Sink{calls:Vec<String>,accepted:String,fail:usize}impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.calls.push(s.into());if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push_str(s);Ok(())}}}
fn display(v:&dyn fmt::Display,mode:u64,fail:usize)->Value{let mut w=Sink{calls:vec![],accepted:String::new(),fail};let err=match mode{0=>fmt::write(&mut w,format_args!("{v}")),1=>fmt::write(&mut w,format_args!("{v:>32}")),2=>fmt::write(&mut w,format_args!("{v:*^33}")),3=>fmt::write(&mut w,format_args!("{v:🦀<31}")),4=>fmt::write(&mut w,format_args!("{v:+032.2}")),5=>fmt::write(&mut w,format_args!("{v:>2}")),_=>panic!()}.is_err();json!({"error":err,"accepted":w.accepted,"chunks":w.calls})}
fn run(q:&Value)->Value{let kind=text(q,"kind");let a=text(q,"a");let b=text(q,"b");match text(q,"op"){
"parse"=>match kind{"version"=>Version::parse(a).map(basic).unwrap_or_else(error),"req"=>VersionReq::parse(a).map(basic).unwrap_or_else(error),"comparator"=>Comparator::parse(a).map(basic).unwrap_or_else(error),"pre"=>Prerelease::new(a).map(basic).unwrap_or_else(error),"build"=>BuildMetadata::new(a).map(basic).unwrap_or_else(error),_=>panic!()},
"order"=>{macro_rules! order{($ty:ty)=>{{let a=<$ty>::new(a).unwrap();let b=<$ty>::new(b).unwrap();json!({"cmp":a.cmp(&b) as i8,"partial":a.partial_cmp(&b).unwrap() as i8,"eq":a==b})}}}match kind{"pre"=>order!(Prerelease),"build"=>order!(BuildMetadata),"version"=>{let a=Version::parse(a).unwrap();let b=Version::parse(b).unwrap();json!({"cmp":a.cmp(&b) as i8,"partial":a.partial_cmp(&b).unwrap() as i8,"precedence":a.cmp_precedence(&b) as i8,"eq":a==b})},_=>panic!()}},
"matches"=>{let v=Version::parse(b).unwrap();json!(VersionReq::parse(a).unwrap().matches(&v))},
"raw"=>{let c=cmp(q);let mut out=basic(c.clone());let v=Version::parse(b).unwrap();out["matches"]=json!(c.matches(&v));out},
"display"=>{let mode=q["mode"].as_u64().unwrap();let fail=q["fail"].as_u64().unwrap() as usize;match kind{"version"=>display(&Version::parse(a).unwrap(),mode,fail),"req"=>display(&VersionReq::parse(a).unwrap(),mode,fail),"comparator"=>display(&Comparator::parse(a).unwrap(),mode,fail),"pre"=>display(&Prerelease::new(a).unwrap(),mode,fail),"build"=>display(&BuildMetadata::new(a).unwrap(),mode,fail),"error"=>display(&Version::parse(a).unwrap_err(),mode,fail),_=>panic!()}},
#[cfg(feature="serde")]
"serde"=>serde_probe::run(q),
_=>panic!("bad request")}}
#[cfg(feature="serde")]mod serde_probe;
fn main(){for line in io::stdin().lock().lines(){let line=line.unwrap();let q:Value=serde_json::from_str(&line).unwrap();let result=run(&q);println!("{}",json!({"request":q,"result":result}));}}
