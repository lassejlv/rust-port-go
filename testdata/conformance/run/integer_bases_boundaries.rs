fn main() {
    let large: i64 = 9_000_000_000;
    let lo = -2147483648;
    let wide = -9223372036854775808i64;
    println!("{} {} {} {} {} {}", 0xff_i32, 0b1010, 0o77, large + 1, lo, wide);
}
