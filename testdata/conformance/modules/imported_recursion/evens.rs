use crate::odds::odd;
pub fn even(n: i32) -> bool { if n == 0 { return true; } odd(n - 1) }
