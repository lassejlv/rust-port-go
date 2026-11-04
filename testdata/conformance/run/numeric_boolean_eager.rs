fn mark(n: i32, value: bool) -> bool { print!("{}", n); value }
fn main() {
    let a = mark(1, false) & mark(2, true);
    let b = mark(3, true) | mark(4, false);
    let c = mark(5, true) ^ mark(6, true);
    println!(" {} {} {}", a, b, c);
}
