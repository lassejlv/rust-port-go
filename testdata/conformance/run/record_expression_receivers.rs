#[derive(Copy, Clone)]
struct Point { x: i32 }
fn point(x: i32) -> Point { Point { x } }
fn main() {
    println!("{} {} {}", point(7).x, (if true { point(8) } else { point(0) }).x, ({ let result = point(9); result }).x);
}
