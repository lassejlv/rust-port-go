fn mark(n: i32) -> i32 { print!("{}", n); n }
fn main() {
    let values = [mark(7); 3];
    println!(" {} {}", values[0usize], values[2usize]);
    let empty: [i32; 0] = [mark(8); 0];
    println!(" {}", empty.len());
}
