// A bootstrap program compiled by Ferrugo without rustc or upstream Cargo.
mod math;
mod report;
use math::fib;
use report::sample;

fn main() {
    println!("Hello, Ferrugo!");
    let mut n: i64 = 0;
    while n < 10 {
        let record = sample(n);
        println!("fib({}) = {}", record.input, record.result);
        n += 1;
    }
    assert!(fib(10) == 55);
}
