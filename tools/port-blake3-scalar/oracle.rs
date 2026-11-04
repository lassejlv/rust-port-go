use std::io::{self, BufRead, Write};

fn main() {
    let stdout = io::stdout();
    let mut out = io::BufWriter::new(stdout.lock());
    for line in io::stdin().lock().lines() {
        let line = line.unwrap();
        let args: Vec<usize> = line.split_whitespace().map(|n| n.parse().unwrap()).collect();
        assert_eq!(args.len(), 3);
        let (length, seed, stride) = (args[0], args[1], args[2]);
        assert!(length <= 2 * 1024 * 1024);
        let data: Vec<u8> = (0..length).map(|i| ((i % 251) as u8) ^ seed as u8).collect();
        let halfway = length / 2;
        let mut hasher = blake3::Hasher::new();
        let empty = hasher.finalize();
        let mut position = 0;
        for end in [halfway, length] {
            while position < end {
                let take = if stride == 0 { end - position } else { stride.min(end - position) };
                hasher.update(&data[position..position + take]);
                // Mutating a separate source buffer after Update is checked by
                // Go replay; original Rust shared borrowing expires here.
                position += take;
            }
            hasher.update(&[]);
            let digest = hasher.finalize();
            assert_eq!(digest, hasher.finalize());
            write!(out, "{} ", digest.to_hex()).unwrap();
        }
        writeln!(out, "{} {}", blake3::hash(&data).to_hex(), empty.to_hex()).unwrap();
    }
}
