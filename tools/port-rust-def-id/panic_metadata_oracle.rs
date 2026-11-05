// Panic metadata observer; the whole pinned def_id.rs and macros are unchanged.
#![feature(step_trait,panic_can_unwind,panic_internals)]
use def_id_reference::*;
use serde_json::{json,Value};
use std::{fmt,io::{self,BufRead},iter::Step,panic::{self,AssertUnwindSafe}};
fn info(p:&panic::PanicHookInfo<'_>)->Value{
 let(payload,kind)=if let Some(v)=p.payload().downcast_ref::<&str>(){((*v).to_owned(),"static_str")}else if let Some(v)=p.payload().downcast_ref::<String>(){(v.clone(),"owned_string")}else{("opaque".into(),"opaque")};
 json!({"Payload":payload,"Kind":kind,"Location":p.location().map(|l|json!({"File":l.file(),"Line":l.line(),"Column":l.column()})),"CanUnwind":p.can_unwind(),"ForceNoBacktrace":p.force_no_backtrace()})
}
fn partial(_:DefId,f:&mut fmt::Formatter<'_>)->fmt::Result{f.write_str("piece")?;Err(fmt::Error)}
static PARTIAL:fn(DefId,&mut fmt::Formatter<'_>)->fmt::Result=partial;
fn run(op:&str){
 match op{
 "crate_u32"=>{CrateNum::from_u32(0xffff_ff01);},
 "def_u32"=>{DefIndex::from_u32(0xffff_ff01);},
 "crate_usize"=>{CrateNum::from_usize(usize::MAX);},
 "def_usize"=>{DefIndex::from_usize(usize::MAX);},
 "local_idx_limit"=>{let _=LocalDefId::new(0xffff_ff01);},
 "local_idx_overflow"=>{let _=LocalDefId::new(1).plus(usize::MAX);},
 "crate_idx_overflow"=>{let _=CrateNum::from_u32(1).plus(usize::MAX);},
 "def_idx_overflow"=>{let _=DefIndex::from_u32(1).plus(usize::MAX);},
 "crate_idx_limit"=>{let _=CrateNum::from_u32(0xffff_ff00).plus(1);},
 "def_idx_limit"=>{let _=DefIndex::from_u32(0xffff_ff00).plus(1);},
 "crate_add_overflow"=>{let _=CrateNum::from_u32(1)+usize::MAX;},
 "def_add_overflow"=>{let _=DefIndex::from_u32(1)+usize::MAX;},
 "crate_add_limit"=>{let _=CrateNum::from_u32(0xffff_ff00)+1;},
 "def_add_limit"=>{let _=DefIndex::from_u32(0xffff_ff00)+1;},
 "crate_forward"=>{let _=<CrateNum as Step>::forward(CrateNum::from_u32(1),usize::MAX);},
 "def_forward"=>{let _=<DefIndex as Step>::forward(DefIndex::from_u32(1),usize::MAX);},
 "crate_backward"=>{let _=<CrateNum as Step>::backward(CrateNum::from_u32(0),1);},
 "def_backward"=>{let _=<DefIndex as Step>::backward(DefIndex::from_u32(0),1);},
 "expect_local"=>{let _=DefId{krate:CrateNum::from_u32(1),index:DefIndex::from_u32(2)}.expect_local();},
 "mod_expect_local"=>{let _=ModId::new_unchecked(DefId{krate:CrateNum::from_u32(1),index:DefIndex::from_u32(2)}).expect_local();},
 "expect_partial"=>{DEF_ID_DEBUG.swap(&PARTIAL);let _=DefId{krate:CrateNum::from_u32(1),index:DefIndex::from_u32(2)}.expect_local();},
 "format_error"=>{DEF_ID_DEBUG.swap(&PARTIAL);let _=format!("{:?}",DefId{krate:CrateNum::from_u32(1),index:DefIndex::from_u32(2)});},
 _=>panic!("bad operation")}
}
fn main(){for line in io::stdin().lock().lines(){let q:Value=serde_json::from_str(&line.unwrap()).unwrap();let original=&*DEF_ID_DEBUG;let found=std::sync::Arc::new(std::sync::Mutex::new(Vec::new()));let copy=found.clone();panic::set_hook(Box::new(move|p|copy.lock().unwrap().push(info(p))));let failed=panic::catch_unwind(AssertUnwindSafe(||run(q["op"].as_str().unwrap()))).is_err();DEF_ID_DEBUG.swap(original);println!("{}",json!({"request":q,"failed":failed,"hooks":*found.lock().unwrap()}))}}
