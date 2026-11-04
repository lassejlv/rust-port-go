#[derive(Copy, Clone)]
struct Point { x: i32 }
fn main() {
    let mut points = [Point { x: 1 }, Point { x: 2 }];
    let first = points[0usize];
    points[1usize].x += first.x;
    let copies = [first; 2];
    println!("{} {} {}", first.x, points[1usize].x, copies[1usize].x);
}
