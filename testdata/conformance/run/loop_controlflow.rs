fn main() {
    let mut n = 0;
    let mut total = 0;
    while n < 10 {
        n += 1;
        if n == 3 { continue; }
        if n == 8 { break; }
        total += n;
    }
    println!("{} {}", n, total);
}
