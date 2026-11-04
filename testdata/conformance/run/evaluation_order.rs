fn mark(n: i32) -> i32 { print!("{}", n); n }
fn pair(a: i32, b: i32) -> i32 { a * 10 + b }
fn main() {
    println!("={}", pair(mark(1), mark(2)));
    println!("={}", mark(3) + mark(4));
    println!("{} {}", mark(5), mark(6));
}
