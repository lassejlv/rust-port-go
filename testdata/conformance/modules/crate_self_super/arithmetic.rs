fn local(n: i32) -> i32 { n + 1 }
pub fn value() -> i32 { self::local(crate::seed()) + nested::value() }
mod nested { pub fn value() -> i32 { super::local(super::super::seed()) } }
