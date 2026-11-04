fn calculate(value: u8, count: i8) -> u8 { value << count }
fn main() {
    println!("before negative shift");
    println!("{}", calculate(1, -1));
}
