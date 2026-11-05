// Development-only client of the complete unchanged rustc_ast crate.
// The source algorithms and derived implementations execute in that crate.
use rustc_ast::token::CommentKind;
use rustc_ast::util::{case::Case, comments::{self, Comment, CommentStyle}, unicode};
use rustc_data_structures::stable_hash::{DummyStableHashCtxt, StableHash, StableHasher};
use rustc_serialize::{Decodable, Decoder, Encodable, opaque::{MemDecoder, mem_encoder::MemEncoder, MAGIC_END_BYTES}};
use rustc_span::{BytePos, Symbol};
use std::{fmt, hash::{Hash, Hasher}, io::{self, BufRead, Write}, sync::{Arc, Mutex}};

fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
fn unhex(text: &str) -> String {
    let bytes = text.as_bytes().chunks_exact(2).map(|p| u8::from_str_radix(std::str::from_utf8(p).unwrap(), 16).unwrap()).collect();
    String::from_utf8(bytes).unwrap()
}
fn bit(value: bool) -> u8 { u8::from(value) }
fn kind(index: usize) -> CommentKind { [CommentKind::Line, CommentKind::Block][index] }
fn style(index: usize) -> CommentStyle { [CommentStyle::Isolated, CommentStyle::Trailing, CommentStyle::Mixed, CommentStyle::BlankLine][index] }

// Each scalar is checked both alone and after/before valid non-control E2
// sequences. Invalid scalar positions are marked 255, never constructed.
fn flow() {
    let mut out = io::BufWriter::new(io::stdout().lock());
    for n in 0..=0x10ffff {
        let value = match char::from_u32(n) {
            None => 255,
            Some(c) => {
                let one = c.to_string();
                let texts = [one.clone(), format!("x{one}"), format!("{one}\u{2000}"), format!("\u{2000}{one}\0"), format!("\u{2000}\u{2603}{one}\u{202f}")];
                texts.iter().enumerate().fold(0, |bits, (i, text)| bits | (bit(unicode::contains_text_flow_control_chars(text)) << i))
            }
        };
        out.write_all(&[value]).unwrap();
    }
}

#[derive(Default)]
struct Raw(Vec<String>);
impl Hasher for Raw {
    fn finish(&self) -> u64 { 0 }
    fn write(&mut self, bytes: &[u8]) { self.0.push(format!("bytes:{}", hex(bytes))); }
}
#[derive(Default)]
struct Typed(Vec<String>);
impl Hasher for Typed {
    fn finish(&self) -> u64 { 0 }
    fn write(&mut self, bytes: &[u8]) { self.0.push(format!("bytes:{}", hex(bytes))); }
    fn write_isize(&mut self, value: isize) { self.0.push(format!("isize:{value}")); }
}
#[derive(Default)]
struct Usize(Vec<String>);
impl Hasher for Usize {
    fn finish(&self) -> u64 { 0 }
    fn write(&mut self, bytes: &[u8]) { self.0.push(format!("bytes:{}", hex(bytes))); }
    fn write_usize(&mut self, value: usize) { self.0.push(format!("usize:{value}")); }
}
#[derive(Default)]
struct Sink { calls: Vec<String>, fail: bool }
impl fmt::Write for Sink {
    fn write_str(&mut self, value: &str) -> fmt::Result {
        self.calls.push(format!("str:{}", hex(value.as_bytes())));
        if self.fail { Err(fmt::Error) } else { Ok(()) }
    }
    fn write_char(&mut self, value: char) -> fmt::Result {
        self.calls.push(format!("char:{:x}", value as u32));
        if self.fail { Err(fmt::Error) } else { Ok(()) }
    }
}
fn format_unit(value: &dyn fmt::Debug, which: usize, fail: bool) -> (bool, String) {
    let mut sink = Sink { fail, ..Default::default() };
    let result = match which {
        0 => fmt::write(&mut sink, format_args!("{:?}", value)),
        1 => fmt::write(&mut sink, format_args!("{:#?}", value)),
        2 => fmt::write(&mut sink, format_args!("{:*^20.1?}", value)),
        3 => fmt::write(&mut sink, format_args!("{:+#020X?}", value)),
        _ => unreachable!(),
    };
    (result.is_ok(), sink.calls.join(","))
}
fn traits() {
    println!("comment_layout\t{}\t{}", std::mem::size_of::<String>(), std::mem::align_of::<String>());
    for (family, values) in [
        ("case", vec![Box::new(Case::Sensitive) as Box<dyn fmt::Debug>, Box::new(Case::Insensitive)]),
        ("style", (0..4).map(|i| Box::new(style(i)) as Box<dyn fmt::Debug>).collect()),
        ("kind", (0..2).map(|i| Box::new(kind(i)) as Box<dyn fmt::Debug>).collect()),
    ] {
        for (i, value) in values.iter().enumerate() { for options in 0..4 { for fail in [false, true] {
            let (ok, calls) = format_unit(&**value, options, fail);
            println!("format\t{family}\t{i}\t{options}\t{}\t{}\t{calls}", bit(fail), bit(ok));
        } } }
    }
    for i in 0..2 {
        let value = kind(i);
        let mut raw = Raw::default(); value.hash(&mut raw);
        let mut typed = Typed::default(); value.hash(&mut typed);
        let mut usize = Usize::default(); value.hash(&mut usize);
        println!("hash\t{i}\t{}\t{}\t{}", raw.0.join(","), typed.0.join(","), usize.0.join(","));
        let mut raw = Raw::default(); CommentKind::hash_slice(&[value, value, kind(1-i)], &mut raw);
        let mut typed = Typed::default(); CommentKind::hash_slice(&[value, value, kind(1-i)], &mut typed);
        println!("hash_slice\t{i}\t{}\t{}", raw.0.join(","), typed.0.join(","));
        for prefix in [vec![], vec![0, 255, 1], (0..80).collect()] {
            let mut hasher = StableHasher::new(); hasher.write(&prefix);
            value.stable_hash(&mut DummyStableHashCtxt, &mut hasher);
            let hash: rustc_hashes::Hash128 = hasher.finish();
            println!("stable\t{i}\t{}\t{}\t{}", hex(&prefix), hash.as_u128() as u64, (hash.as_u128() >> 64) as u64);
        }
        let mut encoder = MemEncoder::new(); value.encode(&mut encoder);
        println!("encode\t{i}\t{}\t{}", encoder.position(), hex(&encoder.finish()));
        for j in 0..2 { println!("kind_eq\t{i}\t{j}\t{}\t{}", bit(value == kind(j)), bit(value != kind(j))); }
        let mut copy = kind(1-i); copy.clone_from(&value);
        println!("kind_clone\t{i}\t{}\t{}", bit(copy == value), bit(value.clone() == value));
    }
    for i in 0..4 {
        for j in 0..4 { println!("style_eq\t{i}\t{j}\t{}\t{}", bit(style(i) == style(j)), bit(style(i) != style(j))); }
        let mut copy = style((i+1)%4); copy.clone_from(&style(i));
        println!("style_clone\t{i}\t{}\t{}", bit(copy == style(i)), bit(style(i).clone() == style(i)));
    }
    for (i, value) in [Case::Sensitive, Case::Insensitive].into_iter().enumerate() {
        for (j, other) in [Case::Sensitive, Case::Insensitive].into_iter().enumerate() { println!("case_eq\t{i}\t{j}\t{}\t{}", bit(value == other), bit(value != other)); }
        let mut copy = Case::Sensitive; copy.clone_from(&value);
        println!("case_clone\t{i}\t{}\t{}", bit(copy == value), bit(value.clone() == value));
    }
    println!("flow_constants\t{}", unicode::TEXT_FLOW_CONTROL_CHARS.iter().map(|c| format!("{:x}", *c as u32)).collect::<Vec<_>>().join(","));
}

fn clone_comment(fields: &[&str]) {
    let index: usize = fields[1].parse().unwrap();
    let pos: u32 = fields[2].parse().unwrap();
    let mut lines = Vec::with_capacity(17);
    for field in &fields[3..] { let text = unhex(field); let mut line = String::with_capacity(text.len()+13); line.push_str(&text); lines.push(line); }
    let source = Comment { style: style(index), lines, pos: BytePos(pos) };
    let clone = source.clone();
    let mut replaced = Comment { style: CommentStyle::Mixed, lines: Vec::with_capacity(37), pos: BytePos(42) };
    replaced.clone_from(&source);
    let separate = source.lines.iter().zip(&clone.lines).all(|(a,b)| a.is_empty() || a.as_ptr() != b.as_ptr());
    let replaced_separate = source.lines.iter().zip(&replaced.lines).all(|(a,b)| a.is_empty() || a.as_ptr() != b.as_ptr());
    println!("C\t{index}\t{pos}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}", clone.lines.len(), clone.lines.capacity(), replaced.lines.capacity(), bit(source.lines.as_ptr()!=clone.lines.as_ptr()), bit(separate), bit(replaced_separate), clone.lines.iter().map(|s| s.capacity().to_string()).collect::<Vec<_>>().join(","), clone.lines.iter().map(|s| hex(s.as_bytes())).collect::<Vec<_>>().join(","), bit(clone.style==source.style), bit(replaced.style==source.style), clone.pos.0, replaced.pos.0);
}

fn tag(tag: u8, ambient: &str) {
    let observed = Arc::new(Mutex::new(Vec::<String>::new()));
    let captured = observed.clone();
    std::panic::set_hook(Box::new(move |info| {
        let (kind, text) = if let Some(s) = info.payload().downcast_ref::<String>() { ("String", s.as_str()) } else if let Some(s) = info.payload().downcast_ref::<&str>() { ("str", *s) } else { ("opaque", "") };
        let location = info.location().unwrap();
        let event = format!("hook:{kind}:{}:{}:{}:{}:{}", hex(text.as_bytes()), hex(location.file().as_bytes()), location.line(), location.column(), bit(std::thread::panicking()));
        if std::env::var_os("FERRUGO_AST_EVENTS").is_some() { eprintln!("EVENT:{event}"); }
        captured.lock().unwrap().push(event);
    }));
    let action_observed = observed.clone();
    let action = move || {
        let mut body = vec![tag]; body.extend_from_slice(MAGIC_END_BYTES);
        let mut decoder = MemDecoder::new(&body, 0).unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| CommentKind::decode(&mut decoder)));
        let outcome = match result { Ok(value) => format!("ok:{value:?}"), Err(_) => "panic".to_string() };
        let event = format!("result:{outcome}:position:{}:panicking:{}", decoder.position(), bit(std::thread::panicking()));
        if std::env::var_os("FERRUGO_AST_EVENTS").is_some() { eprintln!("EVENT:{event}"); }
        action_observed.lock().unwrap().push(event);
    };
    match ambient {
        "normal" => action(),
        "inherited" => {
            struct During<F: Fn()>(F); impl<F: Fn()> Drop for During<F> { fn drop(&mut self) { (self.0)(); } }
            let _ = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| { let _during = During(action); panic!("outer"); }));
        }
        "processing" => {
            let old = std::panic::take_hook();
            std::panic::set_hook(Box::new(move |info| { old(info); action(); }));
            let _ = std::panic::catch_unwind(|| panic!("outer"));
        }
        _ => unreachable!(),
    }
    println!("{}", observed.lock().unwrap().join("|"));
}

fn main() {
    let arguments: Vec<_> = std::env::args().collect();
    match arguments.get(1).map(String::as_str) {
        Some("flow") => return flow(),
        Some("traits") => return traits(),
        Some("tag") => return tag(arguments[2].parse().unwrap(), &arguments[3]),
        _ => (),
    }
    rustc_span::create_default_session_globals_then(|| {
        for (index, line) in io::stdin().lock().lines().enumerate() {
            let line = line.unwrap(); let fields: Vec<_> = line.split('\t').collect();
            match fields[0] {
                "B" => {
                    let k: usize = fields[1].parse().unwrap(); let text = unhex(fields[2]);
                    let value = Symbol::intern(&text);
                    let before = Symbol::intern(&format!("__ferrugo_ast_before_{index}"));
                    let result = comments::beautify_doc_string(value, kind(k));
                    let after = Symbol::intern(&format!("__ferrugo_ast_after_{index}"));
                    println!("B\t{k}\t{}\t{}\t{}\t{}\t{}\t{}", fields[2], hex(result.as_str().as_bytes()), bit(result==value), after.as_u32()-before.as_u32(), bit(comments::may_have_doc_links(&text)), bit(unicode::contains_text_flow_control_chars(&text)));
                }
                "C" => clone_comment(&fields),
                _ => unreachable!(),
            }
        }
    });
}
