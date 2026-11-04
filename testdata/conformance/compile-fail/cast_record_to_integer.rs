#[derive(Copy, Clone)]
struct Record { value: i32 }
fn main() { println!("{}", Record { value: 1 } as i64); }
