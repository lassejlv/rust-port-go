fn main() {
    let mut outer = 0;
    let mut total = 0;
    while outer < 3 {
        outer += 1;
        let mut inner = 0;
        while inner < 4 {
            inner += 1;
            if inner == 2 { continue; }
            total += outer * inner;
        }
    }
    println!("{}", total);
}
