fn main() {
    let empty: [u64; 0] = [];
    let nested = [[5i16; 2]; 3];
    let bytes = [257u16 as u8, -1i8 as u8];
    println!("{} {} {} {} {}", empty.len(), nested[2usize][1usize], bytes[0usize], bytes[1usize], nested[0usize].len());
}
