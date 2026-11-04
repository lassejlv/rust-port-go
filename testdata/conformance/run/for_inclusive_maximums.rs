fn maximum() -> u64 { 18446744073709551615u64 }
fn pointer_maximum() -> usize { -1isize as usize }
fn main() {
    let mut count = 0;
    for value in 254u8..=255u8 { print!("{} ", value); count += 1; if value == 255u8 { continue; } }
    for value in (maximum() - 1u64)..=maximum() { print!("{} ", value); count += 1; }
    for _ in (pointer_maximum() - 1usize)..=pointer_maximum() { count += 1; continue; }
    println!("={}", count);
}
