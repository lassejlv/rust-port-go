// Original-source development oracle. SPDX-License-Identifier: MIT OR Apache-2.0
#[path = "restricted_names.rs"]
mod restricted_names;

use std::error::Error;
use std::io::{self, Read, Write};
use restricted_names::*;

fn u32_in(input: &mut &[u8]) -> u32 {
    let (value, rest) = input.split_at(4);
    *input = rest;
    u32::from_le_bytes(value.try_into().unwrap())
}
fn text_in(input: &mut &[u8]) -> String {
    let size = u32_in(input) as usize;
    let (value, rest) = input.split_at(size);
    *input = rest;
    String::from_utf8(value.to_vec()).unwrap()
}
fn text_out(output: &mut impl Write, value: &str) {
    output.write_all(&(value.len() as u32).to_le_bytes()).unwrap();
    output.write_all(value.as_bytes()).unwrap();
}
fn unicode(output: &mut impl Write) {
    output.write_all(b"RNU1").unwrap();
    for value in 0..0x110000 {
        let mut flags = 0;
        if let Some(ch) = char::from_u32(value) {
            if unicode_ident::is_xid_start(ch) { flags |= 1; }
            if unicode_ident::is_xid_continue(ch) { flags |= 2; }
            if ch.is_alphanumeric() { flags |= 4; }
            if ch.is_digit(10) { flags |= 8; }
            let unicode_escape = format!("\\u{{{value:x}}}");
            let escaped = ch.escape_debug().to_string();
            if escaped == unicode_escape { flags |= 16; }
            else if escaped != ch.to_string() {
                assert_eq!(escaped, match ch { '\0'=>"\\0", '\t'=>"\\t", '\r'=>"\\r", '\n'=>"\\n", '\\'=>"\\\\", '\''=>"\\'", '"'=>"\\\"", _=>panic!("unexpected escape {value:x}") });
            }
            let char_debug = format!("{ch:?}");
            assert_eq!(&char_debug[1..char_debug.len()-1], if ch=='"' { "\"" } else { &escaped });
            let first_debug = format!("{:?}", ch.to_string());
            let first_body = &first_debug[1..first_debug.len()-1];
            assert_eq!(first_body == unicode_escape, flags & 16 != 0);
            if flags & 16 == 0 { assert_eq!(first_body, if ch=='\'' { "'" } else { &escaped }); }
            let string_debug = format!("{:?}", format!("A{ch}"));
            let following_body=&string_debug[2..string_debug.len()-1];
            if following_body == unicode_escape { flags |= 32; }
            else if following_body != ch.to_string() {
                assert_eq!(following_body, match ch { '\0'=>"\\0", '\t'=>"\\t", '\r'=>"\\r", '\n'=>"\\n", '\\'=>"\\\\", '"'=>"\\\"", _=>panic!("unexpected following string escape {value:x}") });
            }
            output.write_all(&[flags]).unwrap();
            let lower = ch.to_string().to_lowercase();
            output.write_all(&[lower.len().try_into().unwrap()]).unwrap();
            output.write_all(lower.as_bytes()).unwrap();
        } else {
            output.write_all(&[0, 0]).unwrap();
        }
    }
}
fn main() {
    assert_eq!(std::char::UNICODE_VERSION, (17, 0, 0));
    let mut output = io::BufWriter::new(io::stdout().lock());
    if std::env::args().nth(1).as_deref() == Some("unicode") {
        unicode(&mut output);
        return;
    }
    let mut data = Vec::new();
    io::stdin().read_to_end(&mut data).unwrap();
    let mut input = data.as_slice();
    assert_eq!(&input[..4], b"RNR1");
    input = &input[4..];
    let count = u32_in(&mut input);
    output.write_all(b"RNO1").unwrap();
    output.write_all(&count.to_le_bytes()).unwrap();
    for _ in 0..count {
        let op = input[0];
        input = &input[1..];
        let name = text_in(&mut input);
        let what = text_in(&mut input);
        let placeholder = char::from_u32(u32_in(&mut input)).unwrap();
        if op == 6 || op == 7 {
            let result = if op == 6 { sanitize_name(&name, placeholder) } else { sanitize_package_name(&name, placeholder) };
            output.write_all(&[2]).unwrap();
            text_out(&mut output, &result);
            text_out(&mut output, "");
            output.write_all(&[0]).unwrap();
            continue;
        }
        let result = match op {
            0 => validate_package_name(&name),
            1 => validate_registry_name(&name),
            2 => validate_name(&name, Box::leak(what.into_boxed_str())),
            3 => validate_profile_name(&name),
            4 => validate_feature_name(&name),
            5 => validate_path_base_name(&name),
            _ => panic!("unknown operation"),
        };
        match result {
            Ok(()) => {
                output.write_all(&[0]).unwrap();
                text_out(&mut output, "");
                text_out(&mut output, "");
                output.write_all(&[0]).unwrap();
            }
            Err(error) => {
                output.write_all(&[1]).unwrap();
                text_out(&mut output, &error.to_string());
                text_out(&mut output, &format!("{error:?}"));
                output.write_all(&[u8::from(error.source().is_some())]).unwrap();
            }
        }
    }
    assert!(input.is_empty());
}
