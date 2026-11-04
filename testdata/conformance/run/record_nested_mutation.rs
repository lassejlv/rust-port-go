#[derive(Copy, Clone)]
struct Inner { number: i32, valid: bool }
#[derive(Copy, Clone)]
struct Outer { inner: Inner, wide: i64 }
fn main() {
    let mut record = Outer { inner: Inner { number: 3, valid: true }, wide: 100i64 };
    let before = record;
    record.inner.number *= 4;
    record.inner.valid = false;
    record.wide -= 1;
    println!("{} {} {} {}", record.inner.number, record.inner.valid, record.wide, before.inner.number);
}
