// Development-only wrapper: the complete pinned implementation is inserted
// verbatim; formatting arms select actual Rust std::fmt operations.
mod original {
// PINNED_HASHES
}
use original::{Hash64, Hash128};
use rustc_stable_hash::{FromStableHash, SipHasher128Hash};
use std::fmt::{self, Debug, LowerHex};
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead, Write};

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}
struct Sink { events: Vec<String>, fail: usize, chars: bool }
impl Sink {
    fn record(&mut self, kind: char, text: &str) -> fmt::Result {
        let n = self.events.len();
        self.events.push(format!("{kind}:{}:{}", text.len(), hex(text.as_bytes())));
        if n == self.fail { Err(fmt::Error) } else { Ok(()) }
    }
}
impl fmt::Write for Sink {
    fn write_str(&mut self, text: &str) -> fmt::Result {
        self.record('S',text)
    }
    fn write_char(&mut self, value: char) -> fmt::Result {
        let mut buffer = [0u8;4];
        let text = value.encode_utf8(&mut buffer);
        if self.chars { self.record('C',text) } else { self.write_str(text) }
    }
}
static FORMAT_STATE: std::sync::Mutex<Option<(u32,u8)>>=std::sync::Mutex::new(None);
fn capture(f: &fmt::Formatter<'_>) {
    let alignment=match f.align() {None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
    *FORMAT_STATE.lock().unwrap()=Some((f.fill() as u32,alignment));
}
fn state() -> String {
    let (fill,align)=FORMAT_STATE.lock().unwrap().unwrap();
    format!("{fill}:{align}")
}
struct Observed<'a,T>(&'a T);
impl<T: Debug> Debug for Observed<'_,T> {
    fn fmt(&self,f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let result=Debug::fmt(self.0,f); capture(f); result
    }
}
impl<T: LowerHex> LowerHex for Observed<'_,T> {
    fn fmt(&self,f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let result=LowerHex::fmt(self.0,f); capture(f); result
    }
}
fn render<T: Debug + LowerHex>(out: &mut Sink, value: &T, pattern: usize) -> fmt::Result {
    let value=Observed(value);
    match pattern {
        // FORMAT_ARMS
        _ => panic!("invalid formatter selection"),
    }
}
#[derive(Default)]
struct TypedHash(Vec<u64>);
impl Hasher for TypedHash {
    fn finish(&self) -> u64 { 0 }
    fn write(&mut self, _: &[u8]) { panic!("unexpected byte hash") }
    fn write_u64(&mut self, value: u64) { self.0.push(value) }
}
fn main() {
    let mut output = io::BufWriter::new(io::stdout().lock());
    for line in io::stdin().lock().lines() {
        let line = line.unwrap();
        let fields: Vec<_> = line.split('\t').collect();
        let lo: u64 = fields[1].parse().unwrap();
        let hi: u64 = fields[2].parse().unwrap();
        let wide = u128::from(lo) | (u128::from(hi) << 64);
        match fields[0] {
            "V" => {
                let other_lo: u64 = fields[3].parse().unwrap();
                let other_hi: u64 = fields[4].parse().unwrap();
                let a = Hash64::new(lo);
                let b = Hash64::new(other_lo);
                let a128 = Hash128::new(wide);
                let b128 = Hash128::new(u128::from(other_lo) | (u128::from(other_hi) << 64));
                let mut clone = Hash64::ZERO;
                clone.clone_from(&a);
                let mut clone128 = Hash128::default();
                clone128.clone_from(&a128);
                assert!(clone == a.clone() && clone128 == a128.clone());
                assert!(Hash64::default() == Hash64::ZERO);
                assert!(<Hash64 as FromStableHash>::from(SipHasher128Hash([lo,hi])) == a);
                assert!(<Hash128 as FromStableHash>::from(SipHasher128Hash([lo,hi])) == a128);
                let mut xor = a;
                xor ^= other_lo;
                let mut hasher = TypedHash::default();
                a.hash(&mut hasher);
                a128.hash(&mut hasher);
                assert_eq!(hasher.0, [lo,lo]);
                writeln!(output,"{line}\t{}\t{}\t{}\t{:?}\t{}\t{}\t{:?}\t{}\t{}",a.as_u64(),a.wrapping_add(b).as_u64(),xor.as_u64(),a.cmp(&b),a128.as_u128(),a128.wrapping_add(b128).as_u128(),a128.cmp(&b128),a128.truncate().as_u64(),hasher.0.iter().map(|n|n.to_string()).collect::<Vec<_>>().join(",")).unwrap();
            }
            "F" => {
                let pattern: usize = fields[3].parse().unwrap();
                let bits = fields[4];
                for chars in [false,true] {
                let mut sink = Sink{events:vec![],fail:usize::MAX,chars};
                let status = if bits=="64" { render(&mut sink,&Hash64::new(lo),pattern) } else { render(&mut sink,&Hash128::new(wide),pattern) };
                assert!(status.is_ok());
                writeln!(output,"{line}\t{chars}\t-1\ttrue\t{}\t{}",sink.events.join("|"),state()).unwrap();
                for fail in 0..=sink.events.len() {
                    let mut sink=Sink{events:vec![],fail,chars};
                    let result = if bits=="64" { render(&mut sink,&Hash64::new(lo),pattern) } else { render(&mut sink,&Hash128::new(wide),pattern) };
                    writeln!(output,"{line}\t{chars}\t{fail}\t{}\t{}\t{}",result.is_ok(),sink.events.join("|"),state()).unwrap();
                }
                }
            }
            _ => panic!("unknown operation"),
        }
    }
}
