fn main() {
    let mut count = 0;
    for _ in 3i32..3i32 { count += 1; }
    for _ in 4u8..2u8 { count += 1; }
    for _ in 4u8..=2u8 { count += 1; }
    println!("{}", count);
}
