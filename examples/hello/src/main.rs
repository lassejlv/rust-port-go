// A bootstrap program compiled by Ferrugo without rustc or upstream Cargo.
mod math;
use math::fib;

fn main() {
    println!("Hello, Ferrugo!");
    let mut n: i64 = 0;
    while n < 10 {
        println!("fib({}) = {}", n, fib(n));
        n += 1;
    }
    assert!(fib(10) == 55);
}
