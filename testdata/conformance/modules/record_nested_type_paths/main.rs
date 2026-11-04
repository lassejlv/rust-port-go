mod geometry;
use crate::geometry::coords::Point;
use crate::geometry::Rect;
fn main() {
    let point = Point { x: 2, y: 3 };
    let mut rect = Rect { origin: point, width: 5 };
    rect.origin.x += 10;
    println!("{} {} {} {}", rect.origin.x, rect.origin.y, rect.width, point.x);
}
