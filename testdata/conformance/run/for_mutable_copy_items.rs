#[derive(Copy, Clone)]
struct Record { value: i32 }
fn main() {
    let records = [Record { value: 1 }, Record { value: 2 }];
    let mut sum = 0;
    for mut record in records { record.value += 10; sum += record.value; }
    println!("{} {} {}", sum, records[0usize].value, records[1usize].value);
    let rows = [[1, 2], [3, 4]];
    let mut total = 0;
    for mut row in rows { row[0usize] += 100; total += row[0usize]; }
    println!("{} {} {}", total, rows[0usize][0usize], rows[1usize][0usize]);
    let mut range_total = 0;
    for mut value in 0u8..3u8 { value += 10; range_total += value as i32; }
    println!("{}", range_total);
}
