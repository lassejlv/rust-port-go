#![feature(panic_can_unwind, panic_internals)]
//! Safe callers of the unchanged complete-crate Span::macro_backtrace iterator.
use std::{cell::{Cell, RefCell}, panic::{self, AssertUnwindSafe, catch_unwind}, sync::Arc};
use rustc_data_structures::{fingerprint::Fingerprint, stable_hash::{RawDefId, RawSpan, StableHashCtxt, StableHashControls, StableHasher}};
use rustc_span::{BytePos, DUMMY_SP, Span, Symbol, SPAN_TRACK};
use rustc_span::{def_id::{DefIndex, LocalDefId}, edition::Edition, hygiene::*};
thread_local! {
    static RUNTIME: Cell<u8> = const { Cell::new(1) };
    static FEATURES: RefCell<Vec<Arc<[Symbol]>>> = const { RefCell::new(Vec::new()) };
    static ITER: RefCell<Option<Box<dyn Iterator<Item=ExpnData>>>> = const { RefCell::new(None) };
    static SELECTED: RefCell<String> = const { RefCell::new(String::new()) };
    static TRACKS: Cell<u32> = const { Cell::new(0) };
    static FAIL: Cell<u32> = const { Cell::new(0) };
    static PROCESSING: Cell<bool> = const { Cell::new(false) };
}
struct Hcx;
impl StableHashCtxt for Hcx {
    fn stable_hash_span(&mut self, s:RawSpan,h:&mut StableHasher){std::hash::Hasher::write_u64(h,s.0)}
    fn def_path_hash(&self,d:RawDefId)->Fingerprint{Fingerprint::new(u64::from(d.0)+100,u64::from(d.1)+1000)}
    fn stable_hash_controls(&self)->StableHashControls{StableHashControls{hash_spans:true}}
    fn assert_default_stable_hash_controls(&self,_:&str){}
}
fn refs()->String{FEATURES.with(|v|v.borrow().iter().map(|a|Arc::strong_count(a).to_string()).collect::<Vec<_>>().join(","))}
fn event(tag:&str){println!("{tag}:{}:{}:{}",std::thread::panicking(),RUNTIME.with(Cell::get),refs())}
struct Local;
impl Drop for Local {fn drop(&mut self){event("track_local_drop")}}
fn track(id:LocalDefId){
    let _local=Local;
    let nth=TRACKS.with(|v|{v.set(v.get()+1);v.get()});
    event(&format!("track:{nth}:{}",id.local_def_index.as_u32()));
    if FAIL.with(Cell::get)==nth {
        if SELECTED.with(|v|v.borrow().starts_with("resume_")){panic::resume_unwind(Box::new("resumed"))}
        panic!("track");
    }
}
static TRACK_FN:fn(LocalDefId)=track;
fn outer(){panic!("outer")}
fn payload(p:&(dyn std::any::Any+Send))->(&'static str,String){
    if let Some(s)=p.downcast_ref::<&str>(){("static_str",s.to_string())}
    else if let Some(s)=p.downcast_ref::<String>(){("owned_string",s.clone())}
    else{("opaque",String::new())}
}
fn next(label:&str){
    event(&format!("next:{label}"));
    let result=ITER.with(|i|i.borrow_mut().as_mut().unwrap().next());
    if let Some(data)=result {
        event(&format!("yield:{label}:{}:{}",data.parent.local_id.as_u32(),data.kind.descr()));
        event(&format!("return_drop:{label}"));drop(data);event(&format!("return_dropped:{label}"));
    }else{event(&format!("none:{label}"))}
}
fn report(label:&str,call:impl FnOnce()){
    match catch_unwind(AssertUnwindSafe(call)) {
        Ok(())=>event(&format!("caught:{label}:false::")),
        Err(p)=>{let(kind,text)=payload(p.as_ref());event(&format!("caught:{label}:true:{kind}:{}",hex(&text)))}
    }
}
fn hex(text:&str)->String{text.bytes().map(|b|format!("{b:02x}")).collect()}
struct Ambient{caught:bool}
impl Drop for Ambient {fn drop(&mut self){event("ambient_drop");if self.caught{report("target",||next("target"))}else{next("target")}}}
fn setup(selected:&str){
    FEATURES.with(|v|*v.borrow_mut()=vec![Arc::from([Symbol::intern("backtrace_a")]),Arc::from([Symbol::intern("backtrace_b")])]);
    let a=LocalExpnId::fresh(ExpnData::new(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("macro_a")),ExpnId::root(),Span::new(BytePos(10),BytePos(11),SyntaxContext::root(),Some(LocalDefId{local_def_index:DefIndex::from_u32(11)})),DUMMY_SP,Some(FEATURES.with(|v|v.borrow()[0].clone())),Edition::Edition2024,None,None,false,false,false,false),Hcx).to_expn_id();
    let ctxt_a=SyntaxContext::root().apply_mark(a,Transparency::Opaque);
    let recursive=selected.contains("recursive");
    let b=LocalExpnId::fresh(ExpnData::new(ExpnKind::Macro(MacroKind::Bang,Symbol::intern("macro_b")),a,Span::new(BytePos(if recursive{0}else{20}),BytePos(if recursive{0}else{21}),ctxt_a,Some(LocalDefId{local_def_index:DefIndex::from_u32(22)})),DUMMY_SP,Some(FEATURES.with(|v|v.borrow()[1].clone())),Edition::Edition2024,None,None,false,false,false,false),Hcx).to_expn_id();
    let ctxt_b=SyntaxContext::root().apply_mark(b,Transparency::Opaque);
    let span=Span::new(BytePos(100),BytePos(101),ctxt_b,Some(LocalDefId{local_def_index:DefIndex::from_u32(33)}));
    ITER.with(|i|*i.borrow_mut()=Some(Box::new(span.macro_backtrace())));event("construct");
}
fn execute(selected:&str,mode:&str){
    SELECTED.with(|v|*v.borrow_mut()=selected.into());setup(selected);
    if selected.contains("second"){next("warmup")}
    TRACKS.with(|v|v.set(0));
    let fail=if selected=="ok"{0}else if selected.ends_with('3'){3}else if selected.ends_with('2'){2}else{1};FAIL.with(|v|v.set(fail));
    RUNTIME.with(|v|v.set(if mode=="normal"{1}else if mode=="nil"{0}else{2}));
    PROCESSING.with(|v|v.set(mode=="processing"));
    panic::set_hook(Box::new(|info|{
        let(kind,text)=payload(info.payload());let loc=info.location().map(|p|format!("{}:{}:{}",p.file(),p.line(),p.column())).unwrap_or_default();
        event(&format!("hook:{kind}:{}:{}:{}:{loc}",hex(&text),info.can_unwind(),info.force_no_backtrace()));
        if PROCESSING.with(Cell::get){report("target",||next("target"));event("hook_return")}
    }));
    if mode=="nil"{panic::set_hook(Box::new(|_|{}));}
    event("invoke");
    match mode {
        "normal"|"currentB"|"nil"=>report("target",||next("target")),
        "processing"=>report("ambient",outer),
        _=>report("ambient",||{let _guard=Ambient{caught:mode=="inherited_caught"};outer()}),
    }
    PROCESSING.with(|v|v.set(false));FAIL.with(|v|v.set(0));event("after_target");
    for n in 0..4{report("drain",||next(&format!("resume{n}")))}
    ITER.with(|i|i.borrow_mut().take());event("iterator_dropped");
}
fn main(){
    let args:Vec<_>=std::env::args().collect();let old=SPAN_TRACK.swap(&TRACK_FN);
    rustc_span::create_session_globals_then(Edition::Edition2024,&[],None,||execute(&args[1],&args[2]));
    event("session_dropped");FEATURES.with(|v|v.borrow_mut().clear());SPAN_TRACK.swap(old);
}
