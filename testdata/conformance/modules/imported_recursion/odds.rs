use crate::evens::even;
pub fn odd(n: i32) -> bool { if n == 0 { return false; } even(n - 1) }
