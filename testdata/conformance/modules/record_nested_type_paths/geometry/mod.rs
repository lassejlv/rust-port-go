pub mod coords;
use self::coords::Point;
#[derive(Copy, Clone)]
pub struct Rect { pub origin: Point, pub width: i32 }
