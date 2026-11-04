mod vault;
fn main() { let key = vault::create(7); println!("{} {}", vault::read(key), vault::read(key)); }
