//! Span wrappers exercised through the complete original hygiene graph.
use std::cell::RefCell;
use std::hash::Hasher;
use std::io::{self, BufRead};
use std::panic::{catch_unwind, AssertUnwindSafe};
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::stable_hash::{RawDefId, RawSpan, StableHashCtxt, StableHashControls, StableHasher};
use rustc_span::{BytePos, Span, Symbol, DUMMY_SP, SPAN_TRACK};
use rustc_span::def_id::{DefIndex, LocalDefId};
use rustc_span::edition::Edition;
use rustc_span::hygiene::*;

thread_local! { static TRACK: RefCell<Vec<u32>> = const { RefCell::new(Vec::new()) }; }
fn track(id: LocalDefId) { TRACK.with(|v|v.borrow_mut().push(id.local_def_index.as_u32())); }
static TRACK_FN: fn(LocalDefId) = track;
struct Hcx;
impl StableHashCtxt for Hcx {
    fn stable_hash_span(&mut self, s: RawSpan, h: &mut StableHasher) { h.write_u64(s.0) }
    fn def_path_hash(&self, d: RawDefId) -> Fingerprint { Fingerprint::new(u64::from(d.0)+100,u64::from(d.1)+1000) }
    fn stable_hash_controls(&self) -> StableHashControls { StableHashControls{hash_spans:true} }
    fn assert_default_stable_hash_controls(&self, _: &str) {}
}
fn edition(n:usize)->Edition { [Edition::Edition2015,Edition::Edition2018,Edition::Edition2021,Edition::Edition2024,Edition::EditionFuture][n%5] }
fn repr(s:Span)->String {
    let d=s.data_untracked();format!("{},{},{},{}",d.lo.0,d.hi.0,d.ctxt.as_u32(),d.parent.map(|p|p.local_def_index.as_u32().to_string()).unwrap_or("-".into()))
}
fn optional(s:Option<Span>)->String {s.map(repr).unwrap_or("_".into())}
fn scope(id:Option<ExpnId>)->String {id.map(|id|format!("{},{}",id.krate.as_u32(),id.local_id.as_u32())).unwrap_or("_".into())}
fn reasons()->[DesugaringKind;14] {use DesugaringKind::*;[QuestionMark,TryBlock,YeetExpr,OpaqueTy,Async,Await,ForLoop,WhileLoop,BoundModifier,Contract,PatTyRange,FormatLiteral{source:false},FormatLiteral{source:true},RangeExpr]}
fn data(d:&ExpnData)->String {
    let refs=d.allow_internal_unstable.as_ref().map(std::sync::Arc::strong_count).unwrap_or(0);
    format!("{};{};{};{:?};{};{};{};{}",d.parent.local_id.as_u32(),repr(d.call_site),repr(d.def_site),d.edition,d.kind.descr(),d.local_inner_macros,d.diagnostic_opaque,refs)
}
fn emit(out:&mut Vec<String>,tag:String,call:impl FnOnce()->String) {
    TRACK.with(|log|log.borrow_mut().clear());
    let result=catch_unwind(AssertUnwindSafe(call)).unwrap_or("panic".into());
    let tracked=TRACK.with(|log|log.borrow().iter().map(u32::to_string).collect::<Vec<_>>().join(","));
    out.push(format!("{tag}={result}@{tracked}"));
}
fn request(n:usize,mode:usize)->String {
    let mut ids=vec![ExpnId::root()];let mut contexts=vec![SyntaxContext::root()];let mut current=SyntaxContext::root();
    for i in 0..n {
        let kind=match(i+mode)%7 {0=>ExpnKind::Macro(MacroKind::Bang,Symbol::intern("span_macro")),1=>ExpnKind::Macro(MacroKind::Attr,Symbol::intern("span_attr")),2=>ExpnKind::Macro(MacroKind::Derive,Symbol::intern("span_derive")),3=>ExpnKind::Desugaring(DesugaringKind::Async),4=>ExpnKind::Desugaring(DesugaringKind::Await),5=>ExpnKind::Desugaring(DesugaringKind::FormatLiteral{source:i%2==0}),_=>ExpnKind::AstPass(AstPass::TestHarness)};
        let lo=if mode&4!=0 {10}else{10+i as u32};
        let call=Span::new(BytePos(lo),BytePos(lo+1),if mode&1!=0 {current}else{SyntaxContext::root()},Some(LocalDefId{local_def_index:DefIndex::from_u32(i as u32+1)}));
        let features=if i%2==0 {Some(std::sync::Arc::from([Symbol::intern("feature")]))}else{None};
        let d=ExpnData::new(kind,if mode&2!=0 {ids[0]}else{ids[i]},call,DUMMY_SP,features,edition(i+mode),None,None,i%2==0,i%3==0,i%2==0,i%3==0);
        let id=LocalExpnId::fresh(d,Hcx).to_expn_id();ids.push(id);
        current=current.apply_mark(id,[Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque][(i+mode)%3]);contexts.push(current);
    }
    let spans:Vec<_>=contexts.iter().enumerate().map(|(i,&c)|{let lo=if mode&8!=0 {10}else{100+i as u32};Span::new(BytePos(lo),BytePos(lo+1),c,Some(LocalDefId{local_def_index:DefIndex::from_u32(i as u32+20)}))}).collect();
    let mut outers=spans.clone();outers.push(Span::new(BytePos(0),BytePos(200),SyntaxContext::root(),None));outers.push(Span::new(BytePos(500),BytePos(600),SyntaxContext::root(),None));
    let mut out=vec![];
    for(i,&s)in spans.iter().enumerate() {
        emit(&mut out,format!("flags{i}"),||format!("{},{},{},{},{},{},{},{},{},{},{}",s.from_expansion(),s.in_derive_expansion(),s.is_from_async_await(),s.can_be_used_for_suggestions(),s.allows_unsafe(),s.allows_unstable(Symbol::intern("feature")),s.allows_unstable(Symbol::intern("missing")),s.is_rust_2015(),s.at_least_rust_2018(),s.at_least_rust_2021(),s.at_least_rust_2024()));
        emit(&mut out,format!("edition{i}"),||format!("{:?}",s.edition()));
        emit(&mut out,format!("parent{i}"),||optional(s.parent_callsite()));
        emit(&mut out,format!("source{i}"),||repr(s.source_callsite()));
        emit(&mut out,format!("notmacro{i}"),||optional(s.find_ancestor_not_from_macro()));
        emit(&mut out,format!("desugar{i}"),||format!("{:?},{},{}",s.desugaring_kind(),s.is_desugaring(DesugaringKind::Async),s.is_desugaring(DesugaringKind::Await)));
        emit(&mut out,format!("callee{i}"),||s.source_callee().as_ref().map(data).unwrap_or("_".into()));
        emit(&mut out,format!("backtrace{i}"),||{let mut iter=s.macro_backtrace();let mut results=vec![];for _ in 0..n+3 {let hint=iter.size_hint();let next=iter.next();results.push(format!("{}:{}:{}",hint.0,hint.1.map(|n|n.to_string()).unwrap_or("_".into()),next.as_ref().map(data).unwrap_or("_".into())));}results.join("/")});
        emit(&mut out,format!("normal20{i}"),||repr(s.normalize_to_macros_2_0()));
        emit(&mut out,format!("normalrules{i}"),||repr(s.normalize_to_macro_rules()));
        emit(&mut out,format!("remove{i}"),||{let mut v=s;let id=v.remove_mark();format!("{};{}",repr(v),scope(Some(id)))});
        for(j,&outer)in outers.iter().enumerate() {
            emit(&mut out,format!("inside{i},{j}"),||optional(s.find_ancestor_inside(outer)));
            emit(&mut out,format!("same{i},{j}"),||optional(s.find_ancestor_in_same_ctxt(outer)));
            emit(&mut out,format!("insames{i},{j}"),||optional(s.find_ancestor_inside_same_ctxt(outer)));
        }
        for(j,&id)in ids.iter().enumerate() {
            emit(&mut out,format!("adjust{i},{j}"),||{let mut v=s;let result=v.adjust(id);format!("{};{}",repr(v),scope(result))});
            emit(&mut out,format!("normaladjust{i},{j}"),||{let mut v=s;let result=v.normalize_to_macros_2_0_and_adjust(id);format!("{};{}",repr(v),scope(result))});
            for(t,transparency)in [Transparency::Transparent,Transparency::SemiOpaque,Transparency::Opaque].into_iter().enumerate() {
                emit(&mut out,format!("apply{i},{j},{t}"),||repr(s.apply_mark(id,transparency)));
            }
            emit(&mut out,format!("sites{i},{j}"),||format!("{};{};{}",repr(s.with_def_site_ctxt(id)),repr(s.with_call_site_ctxt(id)),repr(s.with_mixed_site_ctxt(id))));
        }
    }
    for(i,&id)in ids.iter().enumerate(){emit(&mut out,format!("final{i}"),||data(&id.expn_data()));}
    if n<=2 && mode<3 {
        for(i,&s)in spans.iter().enumerate(){for ed in 0..5 {for(k,reason)in reasons().into_iter().enumerate(){for feature in 0..3 {
            emit(&mut out,format!("reason{i},{ed},{k},{feature}"),||{
                let held:Option<std::sync::Arc<[Symbol]>>=match feature {0=>None,1=>Some(std::sync::Arc::from([])),_=>Some(std::sync::Arc::from([Symbol::intern("feature")]))};
                let marked=s.mark_with_reason(held.clone(),reason,edition(ed),Hcx);
                let d=marked.ctxt().outer_expn_data();let detail=data(&d);drop(d);
                format!("{};{};{}",repr(marked),detail,held.as_ref().map(std::sync::Arc::strong_count).unwrap_or(0))
            });
        }}}}
    }
    out.join("|")
}
fn main() {
    std::panic::set_hook(Box::new(|_|{}));let previous=SPAN_TRACK.swap(&TRACK_FN);
    for line in io::stdin().lock().lines() {let line=line.unwrap();let p:Vec<usize>=line.split('\t').map(|v|v.parse().unwrap()).collect();assert_eq!(p.len(),3);let result=rustc_span::create_session_globals_then(edition(p[2]),&[],None,||request(p[0],p[1]));println!("{result}");}
    SPAN_TRACK.swap(previous);
}
