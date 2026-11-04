fn main() {
    let left = 0b1010u8;
    let right = 0b1100u8;
    println!("{} {} {} {} {} {}", left & right, left | right, left ^ right, !left, !-1i16, !0i16);
}
