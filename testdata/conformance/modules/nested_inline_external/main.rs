mod outer {
    pub mod leaf;
    pub fn combined(n: i32) -> i32 { leaf::double(n) + 1 }
}
fn main() { println!("{} {}", outer::combined(20), outer::leaf::double(3)); }
