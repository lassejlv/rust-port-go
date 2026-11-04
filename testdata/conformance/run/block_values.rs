fn main() {
    let outer = 5;
    let result = { let inner = outer * 2; { let extra = 3; inner + extra } };
    let truth = { let a = result > 10; a && true };
    println!("{} {} {}", outer, result, truth);
}
