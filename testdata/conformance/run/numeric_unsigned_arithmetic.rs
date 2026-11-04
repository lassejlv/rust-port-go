fn main() {
    let small: u8 = 10 + 20;
    let medium: u16 = 300 * 4;
    let large: u32 = 4_000_000_000 + 1;
    let wide: u64 = 18_000_000_000_000_000_000 + 1;
    println!("{} {} {} {} {} {}", small, medium, large, wide, 17u8 / 5, 17u8 % 5);
}
