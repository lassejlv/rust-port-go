// SPDX-License-Identifier: MIT OR Apache-2.0
// The linked unicode-width crate is compiled unchanged from its exact archive.
use std::io::{self, Write};
use unicode_width::UnicodeWidthChar;

fn main() {
    let mut out = io::BufWriter::new(io::stdout().lock());
    out.write_all(b"UWC1").unwrap();
    let (major, minor, patch) = unicode_width::UNICODE_VERSION;
    out.write_all(&[major, minor, patch]).unwrap();
    for value in 0..0x110000 {
        let code = match char::from_u32(value) {
            None => 255,
            Some(ch) => match ch.width() {
                None => 0,
                Some(width) => u8::try_from(width + 1).unwrap(),
            },
        };
        out.write_all(&[code]).unwrap();
    }
}
