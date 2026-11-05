//! Observable allocation/borrow relations in the complete unchanged source.
use rustc_span::edition::Edition;
use rustc_span::symbol::STDLIB_STABLE_CRATES;
use rustc_span::{ByteSymbol, Symbol};

const EXTRA: &str = "ferrugo_native_static_extra_borrow_20261104";

fn main() {
    rustc_span::create_session_globals_then(Edition::Edition2024, &[EXTRA], None, || {
        let input = format!("ferrugo_native_fresh_borrow_{}", 73);
        let first = Symbol::intern(&input);
        let stored = first.as_str();
        println!("fresh_distinct={}", input.as_ptr() != stored.as_ptr());
        let equal_input = input.clone();
        let second = Symbol::intern(&equal_input);
        println!("occupied_retained={}", stored.as_ptr() == second.as_str().as_ptr());
        println!("occupied_distinct={}", equal_input.as_ptr() != second.as_str().as_ptr());
        let bytes = input.as_bytes().to_vec();
        let byte_symbol = ByteSymbol::intern(&bytes);
        println!("cross_kind_shared={}", stored.as_ptr() == byte_symbol.as_byte_str().as_ptr());
        println!("prefill_static={}", EXTRA.as_ptr() == Symbol::intern(EXTRA).as_str().as_ptr());
        println!("stable_crates_static={}", STDLIB_STABLE_CRATES.as_ptr() == std::hint::black_box(STDLIB_STABLE_CRATES).as_ptr());
        println!("stable_crates_indices={}", STDLIB_STABLE_CRATES.iter().map(|s| s.as_u32().to_string()).collect::<Vec<_>>().join(","));
    });
}
