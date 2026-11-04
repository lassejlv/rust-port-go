fn change(values: [i32; 3]) -> [i32; 3] {
    let mut changed = values;
    changed[1usize] = 99;
    changed
}
fn main() {
    let original = [1, 2, 3];
    let changed = change(original);
    println!("{} {} {}", original[1usize], changed[1usize], changed.len());
}
