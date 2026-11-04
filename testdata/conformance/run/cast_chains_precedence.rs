fn main() {
    let a = 257u16 as u8 as u16;
    let b = 1u8 + 2u16 as u8 * 3u8;
    let c = (255u16 as u8) as i8 as i16;
    let d = (1u8 as u16) << 8;
    let e = -1i16 as u8;
    println!("{} {} {} {} {}", a, b, c, d, e);
}
