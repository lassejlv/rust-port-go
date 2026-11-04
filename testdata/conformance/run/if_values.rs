fn absolute(n: i32) -> i32 { if n < 0 { -n } else { n } }
fn main() {
    let chosen: i64 = if true { 9_000_000_000 } else { 1 };
    let label = if chosen > 1 { "large" } else { "small" };
    println!("{} {} {}", absolute(-9), chosen, label);
}
