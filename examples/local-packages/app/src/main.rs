fn main() {
    let inputs: [i64; 4] = [0, 1, 5, 10];
    let mut results = [numbers::sample(0); 4];
    let mut index: usize = 0;
    while index < inputs.len() {
        results[index] = numbers::sample(inputs[index]);
        index += 1;
    }
    let result = results[3];
    println!("fib({}) = {} through two local library crates", result.input, result.result);
    assert!(result.result == 55);
}
