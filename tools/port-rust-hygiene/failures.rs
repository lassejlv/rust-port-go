// Authored safe callers of the unchanged original public hygiene APIs.
use super::*;
use rustc_data_structures::stable_hash::StableHash;
#[path="index_failures.rs"]mod index_failures;
#[path="pipeline.rs"]mod pipeline;
#[path="debug_panic.rs"]mod debug_panic;
struct EraseQuiet;
impl StableHashCtxt for EraseQuiet{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){*h=StableHasher::new();h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{Quiet.def_path_hash(id)}
 fn stable_hash_controls(&self)->StableHashControls{Quiet.stable_hash_controls()}
 fn assert_default_stable_hash_controls(&self,s:&str){Quiet.assert_default_stable_hash_controls(s)}
}
struct Erasing(Env);
impl StableHashCtxt for Erasing{
 fn stable_hash_span(&mut self,s:RawSpan,h:&mut StableHasher){self.0.call("span",&s.0.to_string());*h=StableHasher::new();h.write_u64(s.0)}
 fn def_path_hash(&self,id:RawDefId)->Fingerprint{self.0.def_path_hash(id)}
 fn stable_hash_controls(&self)->StableHashControls{self.0.stable_hash_controls()}
 fn assert_default_stable_hash_controls(&self,s:&str){self.0.assert_default_stable_hash_controls(s)}
}

pub fn build(operation:&str,core:Rc<Core>)->Box<dyn FnOnce()>{match operation{
 "fresh_hash_duplicate"|"set_hash_duplicate"=>{let d=data();let mut hasher=StableHasher::new();d.stable_hash(&mut Quiet,&mut hasher);let local:rustc_hashes::Hash64=hasher.finish();register_expn_id(CrateNum::from_u32(3),ExpnIndex::from_u32(17),d.clone(),hash(100,local.as_u64()));let id=if operation=="set_hash_duplicate"{Some(LocalExpnId::fresh_empty())}else{None};let env=Env::new(core);Box::new(move||{if let Some(id)=id{id.set_expn_data(d,env);println!("returned:set_hash_duplicate")}else{let id=LocalExpnId::fresh(d,env);println!("returned:fresh_hash_duplicate:{}",id.as_u32())}})},
 "clamp_index"=>Box::new(||{let _=ExpnIndex::from_u32(17).clamp(ExpnIndex::from_u32(99),ExpnIndex::from_u32(1));}),
 "clamp_macro"=>Box::new(||{let _=MacroKind::Attr.clamp(MacroKind::Derive,MacroKind::Bang);}),
 "scope_missing"=>{let env=Env::new(core);Box::new(move||{let _env=env;let _=LocalExpnId::fresh_empty();println!("returned:scope_missing")})},
 "collision_forced"=>{let d=data();LocalExpnId::fresh(d.clone(),EraseQuiet);let env=Erasing(Env::new(core));Box::new(move||{let id=LocalExpnId::fresh(d,env);println!("returned:collision_forced:{}",id.as_u32())})},
 "fresh_parent_missing"|"set_parent_missing"=>{let mut d=data();d.parent=ExpnId{krate:CrateNum::from_u32(0),local_id:ExpnIndex::from_u32(17)};let env=Env::new(core);let id=if operation=="set_parent_missing"{Some(LocalExpnId::fresh_empty())}else{None};Box::new(move||{if let Some(id)=id{id.set_expn_data(d,env)}else{let _=LocalExpnId::fresh(d,env);}println!("returned:parent_missing")})},
 "context_apply_parent"|"context_alloc_self"=>{let id=LocalExpnId::fresh_empty();let c=SyntaxContext::from_u32(if operation=="context_alloc_self"{1}else{17});Box::new(move||{let _=c.apply_mark(id.to_expn_id(),Transparency::Opaque);println!("returned:context")})},
 "context_dollar"=>Box::new(||{let printer=rustc_span::symbol::IdentPrinter::new(rustc_span::symbol::kw::DollarCrate,rustc_span::symbol::IdentPrintMode::Normal,Some(DUMMY_SP.with_ctxt(SyntaxContext::from_u32(17))));let _=format!("{printer}");println!("returned:context")}),
 "register_local_duplicate"=>{let h=hash(9,9);register_local_expn_id(data(),h);let d=data();Box::new(move||{let id=register_local_expn_id(d,h);println!("returned:register:{}:{}",id.krate.as_u32(),id.local_id.as_u32())})},
 "register_foreign_parent"=>{let mut d=data();d.parent=ExpnId{krate:CrateNum::from_u32(2),local_id:ExpnIndex::from_u32(17)};Box::new(move||{let id=register_expn_id(CrateNum::from_u32(3),ExpnIndex::from_u32(17),d,hash(9,9));println!("returned:register:{}:{}",id.krate.as_u32(),id.local_id.as_u32())})},
 "register_foreign_hash"|"register_foreign_id"=>{let change_hash=operation=="register_foreign_hash";register_expn_id(CrateNum::from_u32(3),ExpnIndex::from_u32(17),data(),hash(9,9));let d=data();Box::new(move||{let id=register_expn_id(CrateNum::from_u32(3),ExpnIndex::from_u32(if change_hash{17}else{18}),d,hash(9,if change_hash{10}else{9}));println!("returned:register:{}:{}",id.krate.as_u32(),id.local_id.as_u32())})},
 "decode_root_owned"|"decode_cached_owned"|"decode_new_owned"|"decode_local_owned"|"decode_parent_owned"=>{let raw=if operation=="decode_root_owned"{0}else{17};let krate=CrateNum::from_u32(if operation=="decode_local_owned"{0}else{3});if operation=="decode_cached_owned"{register_expn_id(krate,ExpnIndex::from_u32(raw),data(),hash(9,9));}let mut d=data();if operation=="decode_parent_owned"{d.parent=ExpnId{krate:CrateNum::from_u32(2),local_id:ExpnIndex::from_u32(17)};}let env=Env::new(core);Box::new(move||{let id=decode_expn_id(krate,raw,move|id|{env.call("decode",&format!("{}/{}",id.krate.as_u32(),id.local_id.as_u32()));(d,hash(9,10))});println!("returned:decode:{}:{}",id.krate.as_u32(),id.local_id.as_u32())})},
 "lookup_local_empty"|"lookup_expn_empty"|"lookup_outer_empty"|"lookup_debug_empty"=>{let id=LocalExpnId::fresh_empty();let ctxt=if operation=="lookup_outer_empty"{SyntaxContext::root().apply_mark(id.to_expn_id(),Transparency::Opaque)}else{SyntaxContext::root()};let op=operation.to_owned();Box::new(move||{match op.as_str(){"lookup_local_empty"=>{let _=id.expn_data();},"lookup_expn_empty"=>{let _=id.to_expn_id().expn_data();},"lookup_outer_empty"=>{let _=ctxt.outer_expn_data();},_=>{let _=debug_hygiene_data(false);}}println!("returned:lookup")})},
 "lookup_local_missing"|"lookup_expn_missing"|"lookup_hash_local_missing"|"lookup_foreign_data_missing"|"lookup_foreign_hash_missing"|"lookup_expect_foreign"=>{let op=operation.to_owned();Box::new(move||{let local=LocalExpnId::from_u32(17);let foreign=ExpnId{krate:CrateNum::from_u32(3),local_id:ExpnIndex::from_u32(17)};match op.as_str(){"lookup_local_missing"=>{let _=local.expn_data();},"lookup_expn_missing"=>{let _=local.to_expn_id().expn_data();},"lookup_hash_local_missing"=>{let _=local.to_expn_id().expn_hash();},"lookup_foreign_data_missing"=>{let _=foreign.expn_data();},"lookup_foreign_hash_missing"=>{let _=foreign.expn_hash();},_=>{let _=foreign.expect_local();}}println!("returned:lookup")})},
 "context_outer"|"context_outer_data"|"context_edition"|"context_marks"|"context_remove"|"context_normalize"|"context_normalize_rules"|"context_adjust"|"context_normalize_adjust"|"context_glob"|"context_reverse"|"context_equal"|"context_apply"=>{let op=operation.to_owned();Box::new(move||{let mut c=SyntaxContext::from_u32(17);let mut root=SyntaxContext::root();let expn=ExpnId::root();match op.as_str(){"context_outer"=>{let _=c.outer_expn();},"context_outer_data"=>{let _=c.outer_expn_data();},"context_edition"=>{let _=c.edition();},"context_marks"=>{let _=c.marks();},"context_remove"=>{let _=c.remove_mark();},"context_normalize"=>{let _=c.normalize_to_macros_2_0();},"context_normalize_rules"=>{let _=c.normalize_to_macro_rules();},"context_adjust"=>{let _=c.adjust(expn);},"context_normalize_adjust"=>{let _=c.normalize_to_macros_2_0_and_adjust(expn);},"context_glob"=>{let _=root.glob_adjust(expn,DUMMY_SP.with_ctxt(c));},"context_reverse"=>{let _=c.reverse_glob_adjust(expn,DUMMY_SP);},"context_equal"=>{let _=c.hygienic_eq(root,expn);},_=>{let _=c.apply_mark(expn,Transparency::Opaque);}}println!("returned:context")})},
 "set_missing"=>{let d=data();let env=Env::new(core);Box::new(move||{LocalExpnId::from_u32(17).set_expn_data(d,env);println!("returned:set_missing")})},
 _=>if operation.starts_with("index_"){index_failures::build(operation)}else if operation.starts_with("debug_")||operation.starts_with("already_set_debug"){debug_panic::build(operation,core)}else{pipeline::build(operation,core)}}}
