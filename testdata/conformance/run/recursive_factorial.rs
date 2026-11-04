fn factorial(n: i64) -> i64 {
    if n < 2 { return 1; }
    n * factorial(n - 1)
}
fn main() { println!("{} {}", factorial(0), factorial(10)); }
