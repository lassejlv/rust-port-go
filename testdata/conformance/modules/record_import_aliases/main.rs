mod shape;
use crate::shape::Point as Position;
use crate::shape as geometry;
fn main() {
    let first: Position = Position { x: 6, y: 8 };
    let second = geometry::Point { x: first.y, y: first.x };
    println!("{} {} {} {}", first.x, first.y, second.x, second.y);
}
