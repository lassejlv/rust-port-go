// Canonical type_name and derived IntervalSet Debug from the whole original crates.
use rustc_index::{interval::IntervalSet,Idx};
use rustc_span::def_id::{CrateNum,DefIndex};
use rustc_span::AttrId;
fn emit<I:Idx>(kind:&str,domain:usize){let mut set=IntervalSet::<I>::new(domain);for n in [0,2,3,7,20,21,35]{if n<domain{set.insert(I::new(n));}}println!("{kind}\t{domain}\t{}\t{}\t{}",std::any::type_name::<I>(),hex(format!("{set:?}").as_bytes()),hex(format!("{set:#?}").as_bytes()));}
fn hex(value:&[u8])->String{value.iter().map(|v|format!("{v:02x}")).collect()}
fn main(){for domain in [0,1,40]{emit::<DefIndex>("DefIndex",domain);emit::<CrateNum>("CrateNum",domain);emit::<AttrId>("AttrId",domain)}}
