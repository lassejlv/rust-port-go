#[derive(Copy, Clone)]
struct Pair { left: i32, right: i32 }
fn choose(first: bool) -> Pair {
    if first { Pair { left: 1, right: 2 } } else { Pair { left: 3, right: 4 } }
}
fn main() {
    let chosen = choose(false);
    println!("{} {}", chosen.left, chosen.right);
}
