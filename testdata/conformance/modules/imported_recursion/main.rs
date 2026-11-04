mod evens;
mod odds;
use crate::evens::even as check;
fn main() { println!("{} {}", check(20), check(21)); }
