// The adjacent base_n.rs and base_n/tests.rs are copied byte-for-byte from the
// pinned upstream Git objects by the Go validation tool. This harness changes
// neither source file.
#![feature(ascii_char, ascii_char_variants)]

mod base_n;

use base_n::ToBaseN;
use std::cell::RefCell;
use std::io::{self, BufRead, Write};

thread_local! {
    static TRACE: RefCell<Vec<&'static str>> = const { RefCell::new(Vec::new()) };
}

fn trace(operation: &'static str) {
    TRACE.with(|trace| trace.borrow_mut().push(operation));
}

#[derive(Clone, Copy)]
struct Custom<const WIDTH: usize>(u128);

impl<const WIDTH: usize> From<Custom<WIDTH>> for u128 {
    fn from(value: Custom<WIDTH>) -> Self {
        trace("into");
        value.0
    }
}

impl<const WIDTH: usize> ToBaseN for Custom<WIDTH> {
    fn encoded_len(_: usize) -> usize {
        trace("len");
        WIDTH
    }
}

#[derive(Clone, Copy)]
struct OverrideBase(u128);

impl From<OverrideBase> for u128 {
    fn from(value: OverrideBase) -> Self {
        trace("into");
        value.0
    }
}

impl ToBaseN for OverrideBase {
    fn encoded_len(_: usize) -> usize {
        trace("len");
        5
    }
    fn to_base(self, base: usize) -> base_n::BaseNString {
        trace("base");
        42u128.to_base(base)
    }
}

#[derive(Clone, Copy)]
struct OverrideFixed(u128);

impl From<OverrideFixed> for u128 {
    fn from(value: OverrideFixed) -> Self {
        trace("into");
        value.0
    }
}

impl ToBaseN for OverrideFixed {
    fn encoded_len(_: usize) -> usize {
        trace("len");
        5
    }
    fn to_base_fixed_len(self, base: usize) -> base_n::BaseNString {
        trace("fixed");
        99u128.to_base(base)
    }
}

#[derive(Clone, Copy)]
struct OverrideBoth(u128);

impl From<OverrideBoth> for u128 {
    fn from(value: OverrideBoth) -> Self {
        trace("into");
        value.0
    }
}

impl ToBaseN for OverrideBoth {
    fn encoded_len(_: usize) -> usize {
        trace("len");
        5
    }
    fn to_base(self, base: usize) -> base_n::BaseNString {
        trace("base");
        42u128.to_base(base)
    }
    fn to_base_fixed_len(self, base: usize) -> base_n::BaseNString {
        trace("fixed");
        99u128.to_base(base)
    }
}

fn operate<T: ToBaseN + Copy>(op: &str, value: T, base: usize) -> String {
    match op {
        "all" => format!(
            "ok\t{}\t{}\t{}",
            T::encoded_len(base),
            value.to_base(base),
            value.to_base_fixed_len(base)
        ),
        "base" => format!("ok\t{}", value.to_base(base)),
        "fixed" => format!("ok\t{}", value.to_base_fixed_len(base)),
        "len" => format!("ok\t{}", T::encoded_len(base)),
        _ => panic!("unknown operation"),
    }
}

fn custom(op: &str, value: u128, base: usize) -> String {
    let (kind, operation) = op.split_once(':').unwrap();
    match kind {
        "custom0" => operate(operation, Custom::<0>(value), base),
        "custom1" => operate(operation, Custom::<1>(value), base),
        "custom5" => operate(operation, Custom::<5>(value), base),
        "custom128" => operate(operation, Custom::<128>(value), base),
        "custom129" => operate(operation, Custom::<129>(value), base),
        "custommax" => operate(operation, Custom::<{ usize::MAX }>(value), base),
        "override-base" => operate(operation, OverrideBase(value), base),
        "override-fixed" => operate(operation, OverrideFixed(value), base),
        "override-both" => operate(operation, OverrideBoth(value), base),
        _ => panic!("unknown custom type"),
    }
}

fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    let stdout = io::stdout();
    let mut out = io::BufWriter::new(stdout.lock());
    for line in io::stdin().lock().lines() {
        let line = line.unwrap();
        let fields: Vec<_> = line.split('\t').collect();
        assert_eq!(fields.len(), 5);
        let op = fields[0];
        let width: u32 = fields[1].parse().unwrap();
        let base: usize = fields[2].parse().unwrap();
        let hi: u64 = fields[3].parse().unwrap();
        let lo: u64 = fields[4].parse().unwrap();
        TRACE.with(|trace| trace.borrow_mut().clear());
        let mut result = std::panic::catch_unwind(|| {
            if op.contains(':') {
                return custom(op, ((hi as u128) << 64) | lo as u128, base);
            }
            match width {
                32 => operate(op, u32::try_from(lo).unwrap(), base),
                64 => operate(op, lo, base),
                128 => operate(op, ((hi as u128) << 64) | lo as u128, base),
                _ => panic!("unknown width"),
            }
        })
        .unwrap_or_else(|_| "panic".to_owned());
        if op.contains(':') {
            result.push_str("\ttrace:");
            TRACE.with(|trace| result.push_str(&trace.borrow().join(",")));
        }
        writeln!(out, "{line}\t{result}").unwrap();
    }
}
