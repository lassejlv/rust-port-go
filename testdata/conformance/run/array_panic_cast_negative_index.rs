fn index() -> usize { -1i8 as usize }
fn main() {
    let values = [1, 2];
    println!("before negative index");
    println!("{}", values[index()]);
}
