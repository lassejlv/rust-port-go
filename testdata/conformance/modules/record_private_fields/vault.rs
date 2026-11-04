#[derive(Copy, Clone)]
pub struct Key { value: i32 }
pub fn create(value: i32) -> Key { Key { value } }
pub fn read(key: Key) -> i32 { key.value }
