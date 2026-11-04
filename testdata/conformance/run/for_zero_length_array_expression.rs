fn values() -> [i32; 0] { print!("Z"); [] }
fn main() {
    let mut count = 0;
    for _ in values() { count += 1; }
    println!("={}", count);
}
