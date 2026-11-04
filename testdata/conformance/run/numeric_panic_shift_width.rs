fn calculate(value: i16, count: u32) -> i16 { value >> count }
fn main() {
    println!("before shift width");
    println!("{}", calculate(1, 16));
}
