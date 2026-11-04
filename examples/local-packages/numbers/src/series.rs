pub fn fib(n: i64) -> i64 {
    if base_numbers::is_base(n) {
        return n;
    }
    fib(n - 1) + fib(n - 2)
}
