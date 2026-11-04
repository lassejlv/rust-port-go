fn main() {
    let inputs: [i64; 4] = [0, 1, 5, 10];
    let mut results = [numbers::sample(0); 4];
    for index in 0..inputs.len() {
        results[index] = numbers::sample(inputs[index]);
    }
    let result = results[3];
    println!("fib({}) = {} through two local library crates", result.input, result.result);
    assert!(result.result == 55);
}
