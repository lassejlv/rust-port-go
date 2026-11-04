// Development-only oracle using the unchanged edition source and actual derive
// macros, memory codec, and stable hasher prepared by reference-build.
// SPDX-License-Identifier: MIT OR Apache-2.0
use edition_reference::{Edition, ALL_EDITIONS, DEFAULT_EDITION, LATEST_STABLE_EDITION,
    EDITION_NAME_LIST, EDITION_NAME_LIST_STABLE, Encodable, Decodable, Decoder,
    AuditEncoder, AuditDecoder, StableHash, StableHasher, DummyStableHashCtxt, Hash128};
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead, Write};

fn hex(bytes: &[u8]) -> String { bytes.iter().map(|byte| format!("{byte:02x}")).collect() }
fn unhex(text: &str) -> Vec<u8> {
    text.as_bytes().chunks_exact(2).map(|pair| u8::from_str_radix(std::str::from_utf8(pair).unwrap(), 16).unwrap()).collect()
}
fn index(value: Edition) -> usize { ALL_EDITIONS.iter().position(|entry| *entry == value).unwrap() }

#[derive(Default)]
struct Trace(Vec<String>);
macro_rules! trace_integer {
    ($name:ident, $ty:ty) => { fn $name(&mut self, value: $ty) { self.0.push(format!("{}:{value}", stringify!($ty))); } };
}
impl Hasher for Trace {
    fn finish(&self) -> u64 { 0 }
    fn write(&mut self, bytes: &[u8]) { self.0.push(format!("bytes:{}", hex(bytes))); }
    trace_integer!(write_u8, u8); trace_integer!(write_u16, u16); trace_integer!(write_u32, u32);
    trace_integer!(write_u64, u64); trace_integer!(write_u128, u128); trace_integer!(write_usize, usize);
    trace_integer!(write_i8, i8); trace_integer!(write_i16, i16); trace_integer!(write_i32, i32);
    trace_integer!(write_i64, i64); trace_integer!(write_i128, i128); trace_integer!(write_isize, isize);
}

struct Reject;
impl std::fmt::Write for Reject { fn write_str(&mut self, _: &str) -> std::fmt::Result { Err(std::fmt::Error) } }

fn panic_text(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(text) = payload.downcast_ref::<String>() { return text.clone(); }
    if let Some(text) = payload.downcast_ref::<&str>() { return text.to_string(); }
    "non-string panic".to_string()
}

fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    let mut out = io::BufWriter::new(io::stdout().lock());
    writeln!(out, "constants\t{}\t{}\t{}\t{}", EDITION_NAME_LIST, EDITION_NAME_LIST_STABLE, index(DEFAULT_EDITION), index(LATEST_STABLE_EDITION)).unwrap();
    for (i, e) in ALL_EDITIONS.iter().copied().enumerate() {
        let mut trace = Trace::default(); e.hash(&mut trace);
        let mut encoder = AuditEncoder::new(); e.encode(&mut encoder);
        let operations = format!("{:?}", encoder.operations);
        let bytes = encoder.finish();
        writeln!(out, "variant\t{i}\t{}\t{}\t{:?}\t{}{}{}{}{}{}\t{}\t{}\t{}",
            e.as_str(), e.lint_name(), e, u8::from(e.is_stable()), u8::from(e.is_rust_2015()),
            u8::from(e.at_least_rust_2018()),u8::from(e.at_least_rust_2021()),
            u8::from(e.at_least_rust_2024()),u8::from(e.at_least_edition_future()),
            trace.0.join(","), hex(&bytes), operations).unwrap();
        for (j, other) in ALL_EDITIONS.iter().copied().enumerate() {
            let mut cloned = e.clone(); cloned.clone_from(&other);
            writeln!(out, "compare\t{i}\t{j}\t{:?}\t{}\t{}", e.partial_cmp(&other).unwrap(),e==other,index(cloned)).unwrap();
        }
        let formatted = [format!("{e}"),format!("{e:20}"),format!("{e:<20}"),format!("{e:>20}"),
            format!("{e:^20}"),format!("{e:.0}"),format!("{e:.2}"),format!("{e:+020.2}"),
            format!("{e:#}"),format!("{e:x^20}"),format!("{e:?}"),format!("{e:#?}"),
            format!("{e:20?}"),format!("{e:.2?}"),format!("{e:+020.2?}")];
        for (format, text) in formatted.iter().enumerate() { writeln!(out,"format\t{i}\t{format}\t{}",hex(text.as_bytes())).unwrap(); }
        let display_error = std::fmt::write(&mut Reject, format_args!("{e}")).is_err();
        let debug_error = std::fmt::write(&mut Reject, format_args!("{e:?}")).is_err();
        writeln!(out,"writer\t{i}\t{display_error}\t{debug_error}").unwrap();
    }
    for line in io::stdin().lock().lines() {
        let line = line.unwrap(); let fields:Vec<_> = line.split('\t').collect();
        match fields[0] {
            "parse" => {
                let bytes=unhex(fields[1]); let text=std::str::from_utf8(&bytes).unwrap();
                let result=text.parse::<Edition>().map(index).map(|n|n.to_string()).unwrap_or_else(|()|"error".to_string());
                writeln!(out,"{line}\t{result}").unwrap();
            }
            "hash" => {
                let i:usize=fields[1].parse().unwrap(); let prefix=unhex(fields[2]); let e=ALL_EDITIONS[i];
                // These operations come from executing the actual Go port into
                // typed recording adapters, not from a second Rust enum table.
                let go_hash_isize:isize=fields[3].parse().unwrap();
                let go_stable_u8:u8=fields[4].parse().unwrap();
                let mut standard=std::collections::hash_map::DefaultHasher::new(); standard.write(&prefix); e.hash(&mut standard);
                let mut standard_primitive=std::collections::hash_map::DefaultHasher::new(); standard_primitive.write(&prefix); standard_primitive.write_isize(go_hash_isize);
                let mut stable=StableHasher::new(); stable.write(&prefix); e.stable_hash(&mut DummyStableHashCtxt,&mut stable); let stable:Hash128=stable.finish();
                let mut stable_primitive=StableHasher::new(); stable_primitive.write(&prefix); stable_primitive.write_u8(go_stable_u8); let stable_primitive:Hash128=stable_primitive.finish();
                assert_eq!(standard.finish(),standard_primitive.finish()); assert_eq!(stable,stable_primitive);
                writeln!(out,"{line}\t{}\t{}",standard.finish(),stable.as_u128()).unwrap();
            }
            "decode" => {
                let bytes=unhex(fields[1]); let position:usize=fields[2].parse().unwrap();
                let result=std::panic::catch_unwind(|| match AuditDecoder::new(&bytes,position) {
                    Err(())=>"error".to_string(),
                    Ok(mut decoder)=>{
                        let value=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||Edition::decode(&mut decoder)));
                        let remaining=decoder.inner.remaining(); let operations=format!("{:?}",decoder.operations);
                        match value {Ok(e)=>format!("ok:{}:{remaining}:{operations}",index(e)),Err(p)=>format!("panic:{}:{remaining}:{operations}",hex(panic_text(p).as_bytes()))}
                    }
                }).unwrap_or_else(|p|format!("constructor-panic:{}",hex(panic_text(p).as_bytes())));
                writeln!(out,"{line}\t{result}").unwrap();
            }
            _=>panic!("unknown request"),
        }
    }
    assert_eq!(edition_reference::unused_interface_calls(),0);
    writeln!(out,"unused-interface-calls\t0").unwrap();
}
