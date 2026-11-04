fn main() {
    let mut signed = 0;
    for value in -3i16..3i16 { signed += value as i32; }
    let mut inclusive = 0u32;
    for value in 1u8..=3u8 { inclusive += value as u32; }
    println!("{} {}", signed, inclusive);
}
