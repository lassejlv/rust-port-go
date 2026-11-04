fn values(n: i32) -> [i32; 2] { print!("{}", n); [n, n + 1] }
fn main() {
    println!(" {} {}", values(1).len(), ({ let result = values(2); result }).len());
    println!(" {}", (if true { values(3) } else { values(9) }).len());
}
