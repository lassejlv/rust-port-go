fn main() {
    let mut count = 0;
    let mut sum = 0;
    for outer in 0..4 {
        if outer == 1 { continue; }
        for inner in 0..5 {
            if inner == 1 { continue; }
            if inner == 3 { break; }
            count += 1;
            sum += outer * 10 + inner;
        }
    }
    println!("{} {}", count, sum);
}
