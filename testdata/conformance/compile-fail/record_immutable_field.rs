#[derive(Copy, Clone)]
struct Counter { value: i32 }
fn main() { let counter = Counter { value: 1 }; counter.value += 1; }
