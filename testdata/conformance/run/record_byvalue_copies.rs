#[derive(Copy, Clone)]
struct Counter { value: i32 }
fn advance(counter: Counter) -> Counter {
    let mut changed = counter;
    changed.value += 1;
    changed
}
fn main() {
    let original = Counter { value: 4 };
    let next = advance(original);
    let another = advance(next);
    println!("{} {} {}", original.value, next.value, another.value);
}
