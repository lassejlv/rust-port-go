mod nested { pub fn value() -> i32 { 7 } }
fn main() { println!("{}", nested::value()); }
