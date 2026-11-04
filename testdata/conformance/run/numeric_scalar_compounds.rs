fn main() {
    let mut bits = 0b1010u8;
    bits |= 0b0101;
    bits &= 0b1100;
    bits ^= 0b1000;
    bits <<= 1u32;
    bits >>= 2i8;
    let mut flag = true;
    flag &= false;
    flag |= true;
    flag ^= true;
    let mut signed = -16i16;
    signed >>= 2u64;
    signed <<= 1u8;
    println!("{} {} {}", bits, flag, signed);
}
