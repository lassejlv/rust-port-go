#![feature(rustc_private)]
// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual RealFileLoader rejects oversized sparse files before reading them.
use rustc_span::source_map::{FileLoader, RealFileLoader};
use std::os::unix::fs::MetadataExt;
fn main() {
    let dir = std::env::temp_dir().join(format!("ferrugo-source-map-sparse-{}", std::process::id()));
    std::fs::create_dir(&dir).unwrap();
    let path = dir.join("input");
    for len in [u32::MAX as u64, u32::MAX as u64 + 1] {
        let file = std::fs::File::create(&path).unwrap();
        file.set_len(len).unwrap();
        assert_eq!(file.metadata().unwrap().blocks(), 0);
        drop(file);
        let error = RealFileLoader.read_file(&path).unwrap_err();
        println!("{len}\t{}\t{}\t{}", RealFileLoader.file_exists(&path), hex(format!("{error:?}")), hex(error.to_string()));
    }
    std::fs::remove_file(path).unwrap();
    std::fs::remove_dir(dir).unwrap();
}
fn hex(value: String) -> String { value.bytes().map(|b| format!("{b:02x}")).collect() }
