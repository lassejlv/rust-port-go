fn main() {
    let mut values = [1, 2, 3];
    let mut total = 0;
    for value in values { values[2usize] = 9; total += value; }
    println!("{} {}", total, values[2usize]);
}
