#[derive(Copy, Clone)]
struct Record { count: i32, wide: i64, enabled: bool }
fn main() {
    let count = 7;
    let enabled = true;
    let record = Record { wide: 9_000_000_000, count, enabled };
    println!("{} {} {}", record.count, record.wide, record.enabled);
}
