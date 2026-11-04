fn equal(left: &str, right: &str) -> bool { left == right }
fn main() {
    let a = equal("same", "same");
    let b = "left" != "right";
    println!("{} {} {} {}", a, b, !a, a == b);
    assert!(a && b);
}
