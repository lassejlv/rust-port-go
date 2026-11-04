fn main() {
    let result = numbers::sample(10);
    println!("fib({}) = {} through two local library crates", result.input, result.result);
    assert!(result.result == 55);
}
