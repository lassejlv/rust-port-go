fn rhs() -> i32 { print!("R"); 5 }
fn index() -> usize { print!("I"); 4 }
fn main() {
    let mut values = [1, 2];
    values[index()] += rhs();
}
