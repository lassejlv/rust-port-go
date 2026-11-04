fn mark(n: i32, value: bool) -> bool { print!("{}", n); value }
fn main() {
    let first = mark(1, true) as u8;
    let second = mark(2, false) as i64;
    println!(" {} {} {} {}", first, second, true as isize, false as usize);
}
