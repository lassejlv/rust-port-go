#[derive(Copy, Clone)]
struct Bits { small: u8, signed: i16, count: usize }
fn main() {
    let mut record = Bits { small: 0b1010, signed: -32, count: 8 };
    record.small |= 5;
    record.small &= 12;
    record.small ^= 8;
    record.small <<= 2u32;
    record.signed >>= 3u8;
    record.count >>= 1i16;
    println!("{} {} {}", record.small, record.signed, record.count);
}
