fn calculate(a: u64, b: u64) -> u64 { a * b }
fn main() {
    println!("before unsigned multiply");
    println!("{}", calculate(18446744073709551615, 2));
}
