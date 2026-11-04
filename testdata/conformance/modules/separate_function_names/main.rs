mod left;
mod right;
fn value() -> i32 { 1 }
fn main() { println!("{} {} {}", value(), left::value(), right::value()); }
