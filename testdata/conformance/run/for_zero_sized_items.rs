fn values() -> [[u8; 0]; 3] { print!("A"); [[]; 3] }
fn main() {
    let mut count = 0;
    for empty in values() { count += 1; assert!(empty.len() == 0usize); }
    println!("={}", count);
}
