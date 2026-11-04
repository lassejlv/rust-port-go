fn announce() { print!("!"); }
fn finish() { return announce(); }
fn classify(n: i32) -> i32 {
    if n < 0 { return -1; } else if n == 0 { return 0; } else { return 1; }
}
fn main() { finish(); println!("{} {} {}", classify(-3), classify(0), classify(7)); }
