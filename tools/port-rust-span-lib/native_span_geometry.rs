//! Public geometry/combine calls in the complete unchanged rustc_span crate.
use std::cell::RefCell;
use std::io::{self, BufRead};
use std::panic::{AssertUnwindSafe, catch_unwind};
use rustc_span::{BytePos, InnerSpan, Span, SPAN_TRACK, with_metavar_spans};
use rustc_span::def_id::{DefIndex, LocalDefId};
use rustc_span::hygiene::SyntaxContext;

thread_local! { static TRACK: RefCell<Vec<u32>> = const { RefCell::new(Vec::new()) }; }
fn track(parent: LocalDefId) { TRACK.with(|log| log.borrow_mut().push(parent.local_def_index.as_u32())); }
static TRACK_FN: fn(LocalDefId) = track;

fn parent(text: &str) -> Option<LocalDefId> {
    (text != "-").then(|| LocalDefId { local_def_index: DefIndex::from_u32(text.parse().unwrap()) })
}
fn span(fields: &[&str]) -> Span {
    Span::new(BytePos(fields[0].parse().unwrap()), BytePos(fields[1].parse().unwrap()),
              SyntaxContext::from_u32(fields[2].parse().unwrap()), parent(fields[3]))
}
fn repr(value: Span) -> String {
    let data = value.data_untracked();
    format!("{},{},{},{}", data.lo.0, data.hi.0, data.ctxt.as_u32(),
        data.parent.map(|id| id.local_def_index.as_u32().to_string()).unwrap_or_else(|| "-".into()))
}
fn optional(value: Option<Span>) -> String { value.map(repr).unwrap_or_else(|| "_".into()) }

fn request(line: &str) -> String {
    let p: Vec<_> = line.split('\t').collect(); assert_eq!(p.len(), 12);
    let a=span(&p[1..5]); let b=span(&p[5..9]); let x:u64=p[9].parse().unwrap(); let y:u64=p[10].parse().unwrap(); let mode:u8=p[11].parse().unwrap();
    let mut inserted=Vec::new();
    with_metavar_spans(|map| {
        if mode&1!=0 { let ctxt=if mode&4!=0 {SyntaxContext::from_u32(2)} else {b.ctxt()}; inserted.push(map.insert(a,Span::new(BytePos(x as u32),BytePos(y as u32),ctxt,parent(p[4])))); }
        if mode&2!=0 { let ctxt=if mode&4!=0 {SyntaxContext::from_u32(2)} else {a.ctxt()}; inserted.push(map.insert(b,Span::new(BytePos(y as u32),BytePos(x as u32),ctxt,parent(p[8])))); }
        if mode&16!=0 { let _=map.get(a); }
        if mode&8!=0 { drop(map.freeze_and_get_read_spans()); }
    });
    TRACK.with(|log| log.borrow_mut().clear());
    let result=catch_unwind(AssertUnwindSafe(|| match p[0] {
        "lo"=>a.lo().0.to_string(), "hi"=>a.hi().0.to_string(),
        "lohi"=>{let(lo,hi)=a.lo_hi();format!("{},{}",lo.0,hi.0)},
        "withlo"=>repr(a.with_lo(BytePos(x as u32))), "withhi"=>repr(a.with_hi(BytePos(x as u32))),
        "withctxt"=>repr(a.with_ctxt(SyntaxContext::from_u32(x as u32))),
        "root"=>repr(Span::with_root_ctxt(BytePos(x as u32),BytePos(y as u32))),
        "shlo"=>repr(a.shrink_to_lo()), "shhi"=>repr(a.shrink_to_hi()), "empty"=>a.is_empty().to_string(),
        "subdummy"=>repr(a.substitute_dummy(b)), "contains"=>a.contains(b).to_string(),
        "overlap"=>a.overlaps(b).to_string(), "adjacent"=>a.overlaps_or_adjacent(b).to_string(),
        "trimstart"=>optional(a.trim_start(b)), "trimend"=>optional(a.trim_end(b)),
        "neighbor"=>repr(a.with_neighbor(b)), "to"=>repr(a.to(b)), "between"=>repr(a.between(b)), "until"=>repr(a.until(b)),
        "split"=>{let(lo,hi)=a.split_at(x as u32);format!("{};{}",repr(lo),repr(hi))},
        "inner"=>repr(a.from_inner(InnerSpan::new(x as usize,y as usize))),
        _=>panic!("unknown geometry operation"),
    }));
    let result=result.map(|text|format!("ok:{text}")).unwrap_or_else(|_| "panic".into());
    let tracked=TRACK.with(|log| log.borrow().iter().map(u32::to_string).collect::<Vec<_>>().join(","));
    let (len,read_a,read_b)=with_metavar_spans(|map| {let read=map.freeze_and_get_read_spans();(read.len(),read.get(&a).is_some(),read.get(&b).is_some())});
    format!("{result}\t{tracked}\t{}\t{len}:{read_a}:{read_b}",inserted.iter().map(bool::to_string).collect::<Vec<_>>().join(","))
}
fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    let previous=SPAN_TRACK.swap(&TRACK_FN);
    for line in io::stdin().lock().lines() {
        let line=line.unwrap();
        let result=rustc_span::create_session_globals_then(rustc_span::edition::Edition::Edition2024,&[],None,||request(&line));
        println!("{result}");
    }
    SPAN_TRACK.swap(previous);
}
