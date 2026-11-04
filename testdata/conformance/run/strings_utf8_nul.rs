fn identity(value: &str) -> &str { value }
fn main() {
    let text = identity("h\u{e9}🦀\0tail");
    println!("{}", text);
    println!("{} {}", "a\0b" == "a\0c", text == "hé🦀\0tail");
}
