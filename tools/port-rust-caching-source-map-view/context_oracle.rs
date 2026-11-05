//! Complete external operation/cleanup witness; library bodies are unchanged.
#[allow(dead_code)]
#[path = "oracle.rs"]
mod ordinary;
use std::cell::{Cell, RefCell};
use std::panic::{AssertUnwindSafe, catch_unwind};
use rustc_span::{BytePos, CachingSourceMapView, SpanData, SyntaxContext};
use rustc_span::source_map::{FilePathMapping, SourceMap};

fn event(label: &str, map: &SourceMap) {
    println!("E:{label}:{}:{}", std::thread::panicking(), ordinary::counts(map));
}
struct Local<'a>(&'a SourceMap);
impl Drop for Local<'_> { fn drop(&mut self) { event("local", self.0); } }

fn state(mode: &str, header: &str, operation: &str) -> String {
    let map = SourceMap::new(FilePathMapping::empty());
    if header.starts_with("I,") { ordinary::import(&map, header); }
    else { for i in 0..header.parse().unwrap() { ordinary::add(&map, i); } }
    let mut view = if operation == "N" { None } else { Some(CachingSourceMapView::new(&map)) };
    let mut execute = || {
        let _local = Local(&map);
        event("before", &map);
        let parts: Vec<_> = operation.split(',').collect();
        match parts[0] {
            "N" => { let _new = CachingSourceMapView::new(&map); event("new", &map); },
            "b" => {
                match view.as_mut().unwrap().byte_pos_to_line_and_col(BytePos(parts[1].parse().unwrap())) {
                    Some((file, line, col)) => event(&format!("b:{}:{line}:{}", file.start_pos.0, col.0), &map),
                    None => event("none", &map),
                }
            },
            "s" => {
                let span = SpanData { lo: BytePos(parts[1].parse().unwrap()), hi: BytePos(parts[2].parse().unwrap()), ctxt: SyntaxContext::root(), parent: None };
                match view.as_mut().unwrap().span_data_to_lines_and_cols(&span) {
                    Some((file, lo, lc, hi, hc)) => event(&format!("s:{}:{lo}:{}:{hi}:{}", file.start_pos.0, lc.0, hc.0), &map),
                    None => event("none", &map),
                }
            },
            _ => panic!("unknown operation"),
        }
    };
    let success = match mode {
        "normal" => catch_unwind(AssertUnwindSafe(&mut execute)).is_ok(),
        "processing" => { execute(); true },
        "caught" | "escaping" => {
            struct Outer<'a> { execute: &'a mut dyn FnMut(), caught: bool, result: &'a Cell<bool> }
            impl Drop for Outer<'_> {
                fn drop(&mut self) {
                    let ok = if self.caught { catch_unwind(AssertUnwindSafe(&mut self.execute)).is_ok() }
                             else { (self.execute)(); true };
                    self.result.set(ok);
                }
            }
            let result = Cell::new(false);
            let _ = catch_unwind(AssertUnwindSafe(|| { let _outer = Outer { execute: &mut execute, caught: mode == "caught", result: &result }; panic!("outer"); }));
            result.get()
        },
        _ => panic!("unknown mode"),
    };
    let before_drop = format!("{success}:{}:{}:{}", ordinary::counts(&map), ordinary::metadata(&map), std::thread::panicking());
    drop(view);
    format!("{before_drop}:drop:{}", ordinary::counts(&map))
}

thread_local! {
    static REQUEST: RefCell<(String, String)> = RefCell::new((String::new(), String::new()));
    static RESULT: RefCell<Option<String>> = const { RefCell::new(None) };
}
fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    let args: Vec<_> = std::env::args().collect();
    rustc_span::create_session_globals_then(rustc_span::edition::Edition::Edition2024, &[], None, || {
        let result = if args[1] == "processing" {
            REQUEST.with(|slot| *slot.borrow_mut() = (args[2].clone(), args[3].clone()));
            std::panic::set_hook(Box::new(|_| {
                let (header, operation) = REQUEST.with(|slot| slot.borrow().clone());
                let result = state("processing", &header, &operation);
                RESULT.with(|slot| *slot.borrow_mut() = Some(result));
            }));
            let _ = catch_unwind(|| panic!("outer"));
            std::panic::set_hook(Box::new(|_| {}));
            RESULT.with(|slot| slot.borrow_mut().take().unwrap())
        } else { state(&args[1], &args[2], &args[3]) };
        println!("R:{result}");
    });
}
