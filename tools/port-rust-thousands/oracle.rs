// Development-only original-source oracle. The Go driver inserts the verified
// complete mod.rs bytes verbatim at the marker and copies tests.rs unchanged.
// MIT OR Apache-2.0, as the wrapped source. Never linked into the Go port.
mod thousands {
// PINNED_THOUSANDS_MODULE

    // Test-only visibility adapter; calls the original private function.
    pub fn raw(text: String) -> String { format_with_underscores(text) }
}

use std::io::{self, BufRead, Write};

fn decode(text: &str) -> String {
    assert_eq!(text.len() % 2, 0);
    let bytes = text.as_bytes().chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).unwrap(), 16).unwrap())
        .collect();
    String::from_utf8(bytes).unwrap()
}

fn main() {
    let mut out = io::BufWriter::new(io::stdout().lock());
    for line in io::stdin().lock().lines() {
        let line = line.unwrap();
        let (operation, value) = line.split_once('\t').unwrap();
        let result = match operation {
            "u" => thousands::usize_with_underscores(value.parse().unwrap()),
            "i" => thousands::isize_with_underscores(value.parse().unwrap()),
            "f" => thousands::f64p1_with_underscores(f64::from_bits(u64::from_str_radix(value, 16).unwrap())),
            "s" => thousands::raw(decode(value)),
            _ => panic!("unknown operation"),
        };
        write!(out, "{line}\t").unwrap();
        for byte in result.as_bytes() { write!(out, "{byte:02x}").unwrap(); }
        writeln!(out).unwrap();
    }
}
