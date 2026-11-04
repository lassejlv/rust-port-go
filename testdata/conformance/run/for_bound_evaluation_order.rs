fn start() -> i32 { print!("L"); 1 }
fn end() -> i32 { print!("R"); 4 }
fn main() {
    let mut count = 0;
    for value in start()..end() { print!("{}", value); count += 1; }
    println!("={}", count);
    for _ in end()..=start() { print!("unreachable"); }
    println!("done");
}
