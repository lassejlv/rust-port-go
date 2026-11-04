fn mark(n: i32, value: bool) -> bool { print!("{}", n); value }
fn main() {
    let a = mark(1, false) && mark(9, true);
    let b = mark(2, true) || mark(8, false);
    let c = mark(3, true) && mark(4, true);
    println!(" {} {} {}", a, b, c);
}
