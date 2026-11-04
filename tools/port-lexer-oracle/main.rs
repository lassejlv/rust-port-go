// SPDX-License-Identifier: MIT OR Apache-2.0
// The Go test prepends #[path = "<verified upstream lib.rs>"] mod lexer;
// No upstream source is copied, edited, or compiled as part of its workspace.
use std::io::{self, BufRead, Write};

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for &byte in bytes {
        out.push(DIGITS[(byte >> 4) as usize] as char);
        out.push(DIGITS[(byte & 15) as usize] as char);
    }
    out
}

fn unhex(input: &str) -> Vec<u8> {
    assert_eq!(input.len() % 2, 0, "invalid request hex length");
    input.as_bytes().chunks_exact(2).map(|pair| {
        let digit = |c: u8| (c as char).to_digit(16).expect("invalid request hex") as u8;
        digit(pair[0]) * 16 + digit(pair[1])
    }).collect()
}

fn frontmatter(argument: u32) -> lexer::FrontmatterAllowed {
    match argument {
        0 => lexer::FrontmatterAllowed::No,
        1 => lexer::FrontmatterAllowed::Yes,
        _ => panic!("invalid frontmatter argument"),
    }
}

fn state(cursor: &lexer::Cursor<'_>) -> String {
    format!("{},{},{},{},{},{},{}", cursor.first() as u32, cursor.second() as u32,
        cursor.third() as u32, cursor.is_eof(), cursor.pos_within_token(),
        cursor.prev() as u32, hex(cursor.as_str().as_bytes()))
}

fn cursor(input: &str, argument: u32) -> String {
    let mut cursor = lexer::Cursor::new(input, lexer::FrontmatterAllowed::No);
    let mut out = vec![format!("initial:{}", state(&cursor))];
    let actions = match argument {
        0 => "b".repeat(input.chars().count() + 2),
        1 => "briwnqlabriwnqlabe".to_string(),
        2 => "nrbbiwqalnrbbiwqale".to_string(),
        _ => panic!("invalid cursor script"),
    };
    for action in actions.chars() {
        let value = match action {
            'b' => cursor.bump().map_or("none".to_string(), |c| (c as u32).to_string()),
            'r' => { cursor.reset_pos_within_token(); "reset".to_string() }
            'n' => {
                let bytes = cursor.as_str().chars().next().map_or(0, char::len_utf8);
                cursor.bump_bytes(bytes);
                bytes.to_string()
            }
            'i' => { cursor.eat_while(lexer::is_id_continue); "ident".to_string() }
            'w' => { cursor.eat_while(lexer::is_whitespace); "whitespace".to_string() }
            'q' => { cursor.eat_until(b'"'); "quote".to_string() }
            'l' => { cursor.eat_until(b'\n'); "newline".to_string() }
            'a' => format!("{:?}", cursor.advance_token()),
            'e' => { cursor.eat_while(|_| true); "end".to_string() }
            _ => unreachable!(),
        };
        out.push(format!("{action}:{value}:{}", state(&cursor)));
    }
    out.join("\n")
}

fn evaluate(operation: &str, argument: u32, input: &str) -> String {
    match operation {
        "tokens" => lexer::tokenize(input, frontmatter(argument))
            .map(|token| format!("{token:?}")).collect::<Vec<_>>().join("\n"),
        "shebang" => lexer::strip_shebang(input).map_or("none".to_string(), |n| n.to_string()),
        "raw" => match lexer::validate_raw_str(input, argument) {
            Ok(()) => "ok".to_string(),
            Err(lexer::RawStrError::InvalidStarter { bad_char }) => format!("invalid_starter:{}", bad_char as u32),
            Err(lexer::RawStrError::NoTerminator { expected, found, possible_terminator_offset }) =>
                format!("no_terminator:{expected}:{found}:{}",
                    possible_terminator_offset.map_or("none".to_string(), |n| n.to_string())),
            Err(lexer::RawStrError::TooManyDelimiters { found }) => format!("too_many:{found}"),
        },
        "guarded" => {
            let mut cursor = lexer::Cursor::new(input, lexer::FrontmatterAllowed::No);
            let result = cursor.guarded_double_quoted_string();
            let value = result.map_or("none".to_string(), |g|
                format!("some:{}:{}:{}", g.n_hashes, g.terminated, g.token_len));
            format!("{value}:{}", state(&cursor))
        }
        "properties" => {
            let mut out = vec![format!("ident:{}", lexer::is_ident(input))];
            for c in input.chars() {
                out.push(format!("{}:{}:{}:{}:{}", c as u32, lexer::is_whitespace(c),
                    lexer::is_horizontal_whitespace(c), lexer::is_id_start(c), lexer::is_id_continue(c)));
            }
            out.join("\n")
        }
        "cursor" => cursor(input, argument),
        _ => panic!("unknown oracle operation"),
    }
}

fn main() -> io::Result<()> {
    let stdin = io::stdin();
    let mut stdout = io::BufWriter::new(io::stdout().lock());
    for line in stdin.lock().lines() {
        let line = line?;
        let mut fields = line.splitn(3, '\t');
        let operation = fields.next().expect("missing operation");
        let argument = fields.next().expect("missing argument").parse().expect("invalid argument");
        let bytes = unhex(fields.next().expect("missing input"));
        let input = std::str::from_utf8(&bytes).expect("input must be valid UTF-8");
        writeln!(stdout, "{}", hex(evaluate(operation, argument, input).as_bytes()))?;
        stdout.flush()?;
    }
    Ok(())
}
