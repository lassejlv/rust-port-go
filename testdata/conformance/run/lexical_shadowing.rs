fn main() {
    let value = 3;
    let value = value + 1;
    { let mut value = value * 2; value += 1; println!("{}", value); }
    { let value = true; println!("{}", value); }
    println!("{}", value);
}
