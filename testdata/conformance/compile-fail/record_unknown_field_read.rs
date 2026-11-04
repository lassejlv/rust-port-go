#[derive(Copy, Clone)]
struct Counter { value: i32 }
fn main() { let counter = Counter { value: 1 }; println!("{}", counter.unknown); }
