#[derive(Copy, Clone)]
struct First { value: i32 }
#[derive(Copy, Clone)]
struct Second { value: i32 }
#[derive(Copy, Clone)]
struct Outer { inner: First }
fn main() { let record = Outer { inner: Second { value: 1 } }; }
