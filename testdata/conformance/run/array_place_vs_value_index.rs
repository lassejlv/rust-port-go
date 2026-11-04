fn main() {
    let mut values = [1, 2];
    let place = values[{ values[0usize] = 9; 0usize }];
    values[0usize] = 1;
    let snapshot = ({ values })[{ values[0usize] = 9; 0usize }];
    println!("{} {} {}", place, snapshot, values[0usize]);
}
