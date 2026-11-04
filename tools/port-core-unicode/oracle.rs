use std::io::{self, Write};

pub mod ops {
    pub use std::ops::RangeInclusive;
}

pub mod cmp {
    pub use std::cmp::Ordering;
}

pub mod intrinsics {
    // The original intrinsic only supplies an optimization assumption. Checking
    // it leaves all defined executions unchanged and catches an invalid shim.
    pub unsafe fn assume(condition: bool) {
        assert!(condition, "core Unicode table invariant failed");
    }
}

#[path = "unicode_data.rs"]
mod unicode_data;

fn main() -> io::Result<()> {
    let predicates: [fn(char) -> bool; 13] = [
        unicode_data::alphabetic::lookup,
        unicode_data::case_ignorable::lookup,
        unicode_data::cf::lookup,
        unicode_data::cn_planes_0_3::lookup,
        unicode_data::default_ignorable_code_point::lookup,
        unicode_data::grapheme_extend::lookup,
        unicode_data::lowercase::lookup,
        unicode_data::lt::lookup,
        unicode_data::n::lookup,
        unicode_data::uppercase::lookup,
        unicode_data::white_space::lookup,
        unicode_data::xid_continue::lookup,
        unicode_data::xid_start::lookup,
    ];
    let conversions: [fn(char) -> [char; 3]; 4] = [
        unicode_data::conversions::to_lower,
        unicode_data::conversions::to_upper,
        unicode_data::conversions::to_title,
        unicode_data::conversions::to_casefold,
    ];
    let version = unicode_data::UNICODE_VERSION;
    let mut output = Vec::with_capacity(4_000_000);
    output.extend_from_slice(b"FERRUGOC");
    output.extend_from_slice(&[version.0, version.1, version.2, 0]);
    output.extend_from_slice(&1_112_064u32.to_le_bytes());
    for code_point in 0..=0x10ffff {
        let Some(c) = char::from_u32(code_point) else { continue };
        let mut flags = 0u16;
        if !c.is_ascii() {
            for (i, predicate) in predicates.iter().enumerate() {
                flags |= u16::from(predicate(c)) << i;
            }
        }
        output.extend_from_slice(&flags.to_le_bytes());
        let converted = conversions.map(|convert| convert(c));
        let mut changes = 0u8;
        for (i, value) in converted.iter().enumerate() {
            if *value != [c, '\0', '\0'] {
                changes |= 1 << i;
            }
        }
        output.push(changes);
        for (i, value) in converted.iter().enumerate() {
            if changes & (1 << i) != 0 {
                for converted_char in value {
                    output.extend_from_slice(&(*converted_char as u32).to_le_bytes());
                }
            }
        }
    }
    io::stdout().lock().write_all(&output)
}
