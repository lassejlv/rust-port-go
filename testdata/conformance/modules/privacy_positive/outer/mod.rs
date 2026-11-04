fn secret(n: i32) -> i32 { n + 1 }
mod inner;
pub fn visible(n: i32) -> i32 { inner::reveal(n) }
