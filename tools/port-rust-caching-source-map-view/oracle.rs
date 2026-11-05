//! Calls the real caching view in the unchanged, fully built rustc_span crate.
use std::io::{self, BufRead};
use std::panic::{AssertUnwindSafe, catch_unwind};
use std::sync::Arc;
use rustc_span::CachingSourceMapView;
use rustc_span::edition::Edition;
use rustc_span::source_map::{FilePathMapping, SourceMap};
use rustc_span::{BytePos, FileName, SpanData, SyntaxContext};
use rustc_span::{RelativeBytePos, SourceFile, SourceFileHashAlgorithm, SourceFileLines};
use rustc_span::def_id::CrateNum;
use rustc_data_structures::sync::FreezeLock;
use rustc_serialize::{Encodable, Decodable};

#[path = "../port-rust-span-lib/native_source_helpers.rs"]
mod native_source_helpers;

const TEXTS: [&str; 9] = ["", "abc", "abc\n", "\n", "x\nz", "λ🙂\r\nΣx\r\n", "\u{feff}a", "\n\n\n", "tab\tend\nlast"];

pub(crate) fn counts(map: &SourceMap) -> String {
    map.files().iter().map(|file| Arc::strong_count(file).to_string()).collect::<Vec<_>>().join("_")
}

pub(crate) fn add(map: &SourceMap, index: usize) {
    let file = map.new_source_file(FileName::Custom(format!("file{index}")), TEXTS[index].into());
    drop(file);
}

pub(crate) fn import(map: &SourceMap, header: &str) {
    let fields: Vec<_> = header.split(',').collect();
    let prefix: usize = fields[1].parse().unwrap();
    for index in 0..prefix { add(map, index); }
    let len = fields[2].parse().unwrap();
    let mut seed = SourceFile::new(FileName::Custom("import".into()), "a\nb\nc".into(), SourceFileHashAlgorithm::Md5, None).unwrap();
    let lines = if fields[3] == "R" {
        let raw: Vec<u8> = (0..fields[6].len()).step_by(2).map(|i| u8::from_str_radix(&fields[6][i..i+2], 16).unwrap()).collect();
        native_source_helpers::source_with_encoded_line_diffs(fields[4].parse().unwrap(), fields[5].parse().unwrap(), &raw).lines
    } else {
        let values = if fields[4] == "-" { vec![] } else { fields[4].split('~').map(|s| RelativeBytePos(s.parse().unwrap())).collect() };
        seed.lines = FreezeLock::new(SourceFileLines::Lines(values));
        match fields[3] {
            "L" => seed.lines.clone(),
            "F" => { let _ = seed.lines(); seed.lines.clone() },
            "D" => {
                let mut e = rustc_serialize::opaque::mem_encoder::MemEncoder::new();
                seed.encode(&mut e);
                let mut data = e.finish();
                data.extend_from_slice(rustc_serialize::opaque::MAGIC_END_BYTES);
                SourceFile::decode(&mut rustc_serialize::opaque::MemDecoder::new(&data, 0).unwrap()).lines
            },
            _ => panic!("unknown line mode"),
        }
    };
    drop(map.new_imported_source_file(seed.name.clone(), seed.src_hash, seed.checksum_hash, seed.stable_id, len,
        seed.unnormalized_source_len, CrateNum::ZERO, lines, seed.multibyte_chars.clone(), seed.normalized_pos.clone(), 0));
}

pub(crate) fn metadata(map: &SourceMap) -> String {
    map.files().iter().map(|file| format!("{}:{}", file.lines.is_frozen() as u8, file.lines.read().is_lines() as u8)).collect::<Vec<_>>().join("_")
}

fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    rustc_span::create_session_globals_then(Edition::Edition2024, &[], None, || {
        for line in io::stdin().lock().lines() {
            let line = line.unwrap();
            let (number, script) = line.split_once('|').unwrap();
            let map = SourceMap::new(FilePathMapping::empty());
            if number.starts_with("I,") {
                import(&map, number);
            } else {
                let initial: usize = number.parse().unwrap();
                assert!(initial <= TEXTS.len());
                for index in 0..initial { add(&map, index); }
            }
            let mut output = vec![format!("base:{}", counts(&map))];
            let mut cache = match catch_unwind(AssertUnwindSafe(|| CachingSourceMapView::new(&map))) {
                Ok(cache) => cache,
                Err(_) => { println!("{}|new:panic:{}", output.join("|"), counts(&map)); continue; }
            };
            output.push(format!("new:ok:{}", counts(&map)));
            for operation in script.split(';').filter(|op| !op.is_empty()) {
                let parts: Vec<_> = operation.split(',').collect();
                let rendered = match parts[0] {
                    "m" => format!("m:{}", metadata(&map)),
                    "a" => {
                        let index = map.files().len();
                        add(&map, index);
                        format!("a:{index}")
                    },
                    "b" => {
                        let pos = BytePos(parts[1].parse().unwrap());
                        match catch_unwind(AssertUnwindSafe(|| cache.byte_pos_to_line_and_col(pos))) {
                            Ok(Some((file, line, col))) => format!("b:ok:{}:{line}:{}:{}", file.start_pos.0, col.0, counts(&map)),
                            Ok(None) => "b:none".into(),
                            Err(_) => "b:panic".into(),
                        }
                    },
                    "s" => {
                        let data = SpanData { lo: BytePos(parts[1].parse().unwrap()), hi: BytePos(parts[2].parse().unwrap()), ctxt: SyntaxContext::root(), parent: None };
                        match catch_unwind(AssertUnwindSafe(|| cache.span_data_to_lines_and_cols(&data).map(|(file, lo_line, lo_col, hi_line, hi_col)| (file.start_pos.0, lo_line, lo_col.0, hi_line, hi_col.0)))) {
                            Ok(Some((file, lo_line, lo_col, hi_line, hi_col))) => format!("s:ok:{file}:{lo_line}:{lo_col}:{hi_line}:{hi_col}:{}", counts(&map)),
                            Ok(None) => "s:none".into(),
                            Err(_) => "s:panic".into(),
                        }
                    },
                    _ => panic!("unknown operation"),
                };
                output.push(format!("{rendered}:after:{}", counts(&map)));
            }
            drop(cache);
            output.push(format!("drop:{}", counts(&map)));
            println!("{}", output.join("|"));
        }
    });
}
