fn grow(value: usize) -> usize { value + 1 }
fn shrink(value: isize) -> isize { value - 1 }
fn main() {
    let positive: usize = 40;
    let negative: isize = -40;
    let contextual: u16 = 500 + 20;
    let shifted: u8 = 1 << 3;
    println!("{} {} {} {}", grow(positive), shrink(negative), contextual, shifted);
}
