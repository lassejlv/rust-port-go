fn index() -> usize { 3 }
fn main() {
    let values = [1, 2, 3];
    println!("before dynamic bounds");
    println!("{}", values[index()]);
}
