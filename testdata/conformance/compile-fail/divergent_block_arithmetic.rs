fn f() -> i32 { ({ return 1; }) + 2 }
fn main() { println!("{}", f()); }
