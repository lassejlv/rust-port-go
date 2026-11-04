fn mark(n: i32) -> u16 { print!("{}", n); 257u16 }
fn main() {
    let first = mark(1) as u8;
    let second = mark(2) as u8 as u64;
    println!(" {} {}", first, second);
}
