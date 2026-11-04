// A bootstrap program compiled by Ferrugo without rustc or upstream Cargo.
fn fib(n: i64) -> i64 {
    if n < 2 {
        return n;
    }
    fib(n - 1) + fib(n - 2)
}

fn main() {
    println!("Hello, Ferrugo!");
    let mut n: i64 = 0;
    while n < 10 {
        println!("fib({}) = {}", n, fib(n));
        n += 1;
    }
    assert!(fib(10) == 55);
}
