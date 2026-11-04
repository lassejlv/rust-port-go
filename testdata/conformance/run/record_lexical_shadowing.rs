#[derive(Copy, Clone)]
struct Slot { number: i32 }
fn main() {
    let slot = Slot { number: 5 };
    {
        let slot = Slot { number: slot.number + 1 };
        println!("{}", slot.number);
    }
    println!("{}", slot.number);
}
