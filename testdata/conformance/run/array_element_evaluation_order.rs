fn mark(n: i32) -> i32 { print!("{}", n); n }
fn main() {
    let values = [mark(3), mark(1), mark(2)];
    println!(" {} {} {}", values[0usize], values[1usize], values[2usize]);
}
