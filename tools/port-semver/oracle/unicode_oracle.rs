// Rust character Debug compatibility data consumed by semver's error.rs.
// Private storage of this table has no dependency on the host Go Unicode data.
fn main(){let mut start=None;let mut last=0u32;for n in 0..=0x10ffff{let escaped=char::from_u32(n).is_some_and(|ch|!matches!(ch,'\0'|'\t'|'\n'|'\r'|'\''|'\\')&&format!("{ch:?}").starts_with("'\\"));if escaped{if start.is_none(){start=Some(n)}last=n;}else if let Some(start)=start.take(){println!("{start:x}\t{last:x}")}}if let Some(start)=start{println!("{start:x}\t{last:x}")}}
