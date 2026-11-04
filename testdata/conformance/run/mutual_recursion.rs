fn main() { println!("{} {}", even(20), odd(20)); }
fn even(n: i32) -> bool { if n == 0 { return true; } odd(n - 1) }
fn odd(n: i32) -> bool { if n == 0 { return false; } even(n - 1) }
