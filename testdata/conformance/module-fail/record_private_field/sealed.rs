#[derive(Copy, Clone)]
pub struct Record { value: i32 }
pub fn read(record: Record) -> i32 { record.value }
