fn main() {
    let values = [4, 5, 6];
    let mut total = 0;
    for index in 0..values.len() { total += values[index]; }
    println!("{} {}", total, values.len());
}
