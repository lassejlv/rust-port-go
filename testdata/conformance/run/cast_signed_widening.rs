fn main() {
    let small = -128i8;
    let middle = small as i16;
    let wide = middle as i64;
    println!("{} {} {}", small, middle, wide);
}
