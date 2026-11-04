fn index() -> usize { 0 }
fn main() {
    let values: [i32; 0] = [];
    println!("before empty bounds");
    println!("{}", values[index()]);
}
