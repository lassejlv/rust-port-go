//! External client of the complete unchanged rustc_data_structures crate.
use std::cell::RefCell;
use std::fmt::{self, Write};
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead};
use std::rc::Rc;
use rustc_data_structures::fingerprint::Fingerprint;
use rustc_data_structures::stable_hash::{DummyStableHashCtxt, StableHash, StableHasher};
use rustc_data_structures::svh::Svh;
use rustc_data_structures::unhash::Unhasher;
use rustc_serialize::{Encodable, Decodable};
use rustc_serialize::opaque::{MemDecoder, MAGIC_END_BYTES, mem_encoder::MemEncoder};

fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
#[derive(Default)]
struct State { calls: usize, fail: Option<usize>, events: Vec<String>, text: String, after: Vec<String> }
impl State {
    fn write(&mut self, kind: &str, text: &str) -> fmt::Result {
        self.events.push(format!("{kind}:{}", hex(text.as_bytes())));
        let index = self.calls; self.calls += 1;
        if self.fail == Some(index) { return Err(fmt::Error); }
        self.text.push_str(text); Ok(())
    }
}
struct Strings(Rc<RefCell<State>>);
impl Write for Strings { fn write_str(&mut self, s: &str) -> fmt::Result { self.0.borrow_mut().write("S", s) } }
struct Chars(Rc<RefCell<State>>);
impl Write for Chars {
    fn write_str(&mut self, s: &str) -> fmt::Result { self.0.borrow_mut().write("S", s) }
    fn write_char(&mut self, c: char) -> fmt::Result { self.0.borrow_mut().write("C", &c.to_string()) }
}
struct Observed<T> { value: T, state: Rc<RefCell<State>>, reuse: bool }
fn align(value: Option<fmt::Alignment>) -> u8 { match value { None => 0, Some(fmt::Alignment::Left) => 1, Some(fmt::Alignment::Right) => 2, Some(fmt::Alignment::Center) => 3 } }
impl<T: fmt::Debug> fmt::Debug for Observed<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let first = self.value.fmt(f);
        self.state.borrow_mut().after.push(format!("{}:{}:{}", first.is_ok(), f.fill() as u32, align(f.align())));
        if self.reuse {
            let second = self.value.fmt(f);
            self.state.borrow_mut().after.push(format!("{}:{}:{}", second.is_ok(), f.fill() as u32, align(f.align())));
            first.and(second)
        } else { first }
    }
}
impl<T: fmt::Display> fmt::Display for Observed<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let first = self.value.fmt(f);
        self.state.borrow_mut().after.push(format!("{}:{}:{}", first.is_ok(), f.fill() as u32, align(f.align())));
        if self.reuse {
            let second = self.value.fmt(f);
            self.state.borrow_mut().after.push(format!("{}:{}:{}", second.is_ok(), f.fill() as u32, align(f.align())));
            first.and(second)
        } else { first }
    }
}
fn formatted<T: fmt::Debug + fmt::Display>(value: T, index: usize, fail: Option<usize>, chars: bool, reuse: bool) -> String {
    let state = Rc::new(RefCell::new(State { fail, ..Default::default() }));
    let observed = Observed { value, state: state.clone(), reuse };
    let mut strings = Strings(state.clone()); let mut characters = Chars(state.clone());
    let sink: &mut dyn Write = if chars { &mut characters } else { &mut strings };
    let result = match index {
        // FORMAT_ARMS
        _ => panic!("format index"),
    };
    let state = state.borrow();
    format!("{}\t{}\t{}\t{}", result.is_ok(), hex(state.text.as_bytes()), state.events.join("|"), state.after.join("|"))
}
struct Raw { writes: Vec<String>, value: u64 }
impl Hasher for Raw {
    fn finish(&self) -> u64 { self.value }
    fn write(&mut self, bytes: &[u8]) { self.writes.push(format!("B:{}", hex(bytes))); for b in bytes { self.value = self.value.rotate_left(5) ^ u64::from(*b); } }
}
struct Typed(Raw);
impl Hasher for Typed {
    fn finish(&self) -> u64 { self.0.finish() }
    fn write(&mut self, bytes: &[u8]) { self.0.write(bytes); }
    fn write_u64(&mut self, n: u64) { self.0.writes.push(format!("U:{n:016x}")); self.0.value = self.0.value.rotate_left(7) ^ n; }
}
struct Wrapped(Unhasher);
impl Hasher for Wrapped {
    fn finish(&self) -> u64 { self.0.finish() }
    fn write(&mut self, bytes: &[u8]) { self.0.write(bytes); }
    fn write_u64(&mut self, n: u64) { self.0.write_u64(n); }
}
fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    for line in io::stdin().lock().lines() {
        let line = line.unwrap(); let p: Vec<_> = line.split('\t').collect();
        let lo = u64::from_str_radix(p[1], 16).unwrap(); let hi = u64::from_str_radix(p[2], 16).unwrap();
        let fp = Fingerprint::new(lo, hi); let value = Svh::new(fp);
        let output = match p[0] {
            "V" => {
                let other = Svh::new(Fingerprint::new(hi, lo)); let mut copied = value.clone(); copied.clone_from(&other);
                let mut e = MemEncoder::new(); value.encode(&mut e); let encoded = e.finish();
                let mut frame = encoded.clone(); frame.extend_from_slice(MAGIC_END_BYTES);
                let mut d = MemDecoder::new(&frame, 0).unwrap(); let decoded = Svh::decode(&mut d);
                let mut h = StableHasher::new(); h.write(b"seeded-svh"); value.stable_hash(&mut DummyStableHashCtxt, &mut h);
                let stable: Fingerprint = h.finish();
                let (first, second) = value.as_fingerprint().split();
                let projected = u128::from(first.as_u64()) | (u128::from(second.as_u64()) << 64);
                format!("{:032x}\t{}\t{projected:032x}\t{}\t{}\t{}\t{}\t{}\t{}", value.as_u128(), value.to_hex(), value == other, value != other, copied == other, decoded == value, hex(&encoded), stable.to_hex())
            },
            "H" => {
                let values = [value, Svh::new(Fingerprint::new(hi, lo))]; let n: usize = p[4].parse().unwrap();
                if p[3] == "raw" { let mut h = Raw { writes: vec![], value: 7 }; Svh::hash_slice(&values[..n], &mut h); format!("{}\t{:016x}", h.writes.join("|"), h.finish()) }
                else { let mut h = Typed(Raw { writes: vec![], value: 7 }); Svh::hash_slice(&values[..n], &mut h); format!("{}\t{:016x}", h.0.writes.join("|"), h.finish()) }
            },
            "U" => {
                let values = [value, Svh::new(Fingerprint::new(hi, lo))];
                let seed: u64 = p[4].parse().unwrap(); let n: usize = p[5].parse().unwrap();
                assert_eq!(p[6] == "1", cfg!(debug_assertions));
                let mut h = Unhasher::default(); h.write_u64(seed);
                if p[3] == "direct" {
                    let ok = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| Svh::hash_slice(&values[..n], &mut h))).is_ok();
                    format!("{ok}\t{:016x}", h.finish())
                } else if p[3] == "reference" {
                    let ok = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| { let mut reference = &mut h; Svh::hash_slice(&values[..n], &mut reference); })).is_ok();
                    format!("{ok}\t{:016x}", h.finish())
                } else {
                    let mut wrapped = Wrapped(h);
                    let ok = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| Svh::hash_slice(&values[..n], &mut wrapped))).is_ok();
                    format!("{ok}\t{:016x}", wrapped.finish())
                }
            },
            "F" => { let index = p[4].parse().unwrap(); let fail = p[5].parse::<usize>().ok(); let chars = p[6] == "1"; let reuse = p[7] == "1";
                if p[3] == "S" { formatted(value, index, fail, chars, reuse) } else { formatted(fp, index, fail, chars, reuse) }
            },
            _ => panic!("unknown operation"),
        };
        println!("{output}");
    }
}
