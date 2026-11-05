// Original function bodies are included byte-for-byte by the verified builder.
use std::{cmp, mem};
use std::io::{self, BufRead};
include!("edit_distance_original.rs");

fn unhex(value: &str) -> String {
    assert!(value.len() % 2 == 0);
    let bytes: Vec<u8> = (0..value.len()).step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i+2],16).unwrap()).collect();
    String::from_utf8(bytes).unwrap()
}
fn main() {
    assert_eq!(std::char::UNICODE_VERSION,(18,0,0),"Symbol source Unicode profile mismatch");
    std::panic::set_hook(Box::new(|_| {}));
    for line in io::stdin().lock().lines() {
        let line = line.unwrap();
        assert!(line.len() <= 16384);
        let parts: Vec<&str> = line.split('\t').collect();
        assert_eq!(parts.len(),4);
        let a=unhex(parts[1]); let b=unhex(parts[2]); let limit:usize=parts[3].parse().unwrap();
        let result=std::panic::catch_unwind(|| match parts[0] {
            "D" => edit_distance(&a,&b,limit),
            "S" => edit_distance_with_substrings(&a,&b,limit),
            _ => panic!("unknown request"),
        });
        match result { Ok(Some(value))=>println!("{value}"),Ok(None)=>println!("none"),Err(_)=>println!("panic") }
    }
}
