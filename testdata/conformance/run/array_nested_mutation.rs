fn main() {
    let mut matrix: [[i32; 2]; 2] = [[1, 2], [3, 4]];
    let row: usize = 1;
    matrix[row][0usize] *= 10;
    matrix[0usize][1usize] = matrix[1usize][0usize] + 1;
    println!("{} {} {} {}", matrix[0usize][0usize], matrix[0usize][1usize], matrix[1usize][0usize], matrix.len());
}
