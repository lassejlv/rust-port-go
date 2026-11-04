fn main() {
    let values: [i16; 3] = [10, 20, 30];
    let flags = [true, false, true];
    println!("{} {} {} {}", values[0usize], values[1usize], flags[2usize], values.len());
}
