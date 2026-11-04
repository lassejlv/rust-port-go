mod maths;
use crate::maths::double as twice;
use crate::maths as numbers;
fn main() { println!("{} {}", twice(4), numbers::double(5)); }
