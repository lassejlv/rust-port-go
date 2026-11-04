fn main() {
    println!("{} {} {} {}",
        (if true { 256 } else { 0 }) as u8,
        (256 + 0) as u8,
        (256 << 0) as u8,
        ({ 18446744073709551615 }) as u64);
}
