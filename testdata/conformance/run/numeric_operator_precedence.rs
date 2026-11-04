fn main() {
    let a = 1u8 << 2 + 1;
    let b = 1u8 | 6u8 & 3u8 ^ 2u8;
    let c = 16u16 >> 1 + 1;
    let d = 1u8 + 2u8 << 1;
    println!("{} {} {} {} {}", a, b, c, d, 1u8 | 2u8 == 3u8);
}
