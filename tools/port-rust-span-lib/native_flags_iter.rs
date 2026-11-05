extern crate rustc_span;
extern crate bitflags;
use rustc_span::RemapPathScopeComponents as F;
use bitflags::Flags;
fn main(){
 for bits in 0..=255u8{let f=F::from_bits_retain(bits);let mut names=f.iter_names();let mut rows=vec![];for _ in 0..9{let next=names.next();rows.push(match next{Some((name,v))=>format!("{name}:{}:{}",v.bits(),names.remaining().bits()),None=>format!("none:{}",names.remaining().bits())})};let mut iter=f.iter();let mut values=vec![];for _ in 0..9{values.push(match iter.next(){Some(v)=>v.bits().to_string(),None=>"none".to_owned()})};println!("iter\t{bits}\t{}\t{}\t{}\t{}",rows.join(","),values.join(","),f.contains_unknown_bits(),F::from_bits(bits).is_some());}
 let names=F::iter_defined_names().map(|(n,b)|format!("{n}:{}",b.bits())).collect::<Vec<_>>().join(",");println!("defined\t{names}");
 for name in["","MACRO","DIAGNOSTICS","DEBUGINFO","COVERAGE","DOCUMENTATION","OBJECT","macro","Macro","ALL"," MACRO","MACRO ","MACRO | DIAGNOSTICS","\0"]{println!("name\t{}\t{}",name.as_bytes().iter().map(|b|format!("{b:02x}")).collect::<String>(),F::from_name(name).map(|v|v.bits().to_string()).unwrap_or("none".to_string()))}
 for a in 0..=255u8{for b in 0..=255u8{let a=F::from_bits_retain(a);let b=F::from_bits_retain(b);let mut truncate=a;truncate.truncate();let mut clear=a;clear.clear();println!("ops\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",a.bits(),b.bits(),(a|b).bits(),(a&b).bits(),(a^b).bits(),(a-b).bits(),(!a).bits(),a.intersects(b),a.contains(b),truncate.bits(),clear.bits());}}
}
