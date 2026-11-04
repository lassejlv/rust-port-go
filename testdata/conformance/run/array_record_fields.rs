#[derive(Copy, Clone)]
struct Record { numbers: [u8; 3], enabled: bool }
fn main() {
    let mut record = Record { numbers: [1, 2, 3], enabled: true };
    let before = record;
    record.numbers[1usize] += 10;
    println!("{} {} {} {}", before.numbers[1usize], record.numbers[1usize], record.enabled, record.numbers.len());
}
