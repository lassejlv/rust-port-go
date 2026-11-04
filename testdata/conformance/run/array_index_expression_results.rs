fn values() -> [u16; 3] { [10, 20, 30] }
fn main() {
    let index: usize = 1;
    println!("{} {} {}", values()[index], (if true { [3, 4] } else { [5, 6] })[0usize], ({ let result = [8u8, 9]; result })[1usize]);
}
