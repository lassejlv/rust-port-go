#[derive(Copy, Clone)]
struct Ordered { first: i32, second: i32, third: i32 }
fn mark(n: i32) -> i32 { print!("{}", n); n }
fn main() {
    let record = Ordered { third: mark(3), first: mark(1), second: mark(2) };
    println!("={}:{}:{}", record.first, record.second, record.third);
}
