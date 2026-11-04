#[derive(Copy, Clone)]
struct Converted { small: u8, wide: i64, flag: u16 }
fn convert(value: i16, flag: bool) -> Converted {
    Converted { small: value as u8, wide: value as i64, flag: flag as u16 }
}
fn main() {
    let record = convert(-1, true);
    println!("{} {} {}", record.small, record.wide, record.flag);
}
