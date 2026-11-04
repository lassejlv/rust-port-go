use rustc_literal_escaper::*;
use std::io::{self, BufReader, BufWriter, Read, Write};
use std::ops::Range;
use std::num::NonZero;

fn error_code(error: EscapeError) -> u32 {
    match error {
        EscapeError::ZeroChars => 1,
        EscapeError::MoreThanOneChar => 2,
        EscapeError::LoneSlash => 3,
        EscapeError::InvalidEscape => 4,
        EscapeError::BareCarriageReturn => 5,
        EscapeError::BareCarriageReturnInRawString => 6,
        EscapeError::EscapeOnlyChar => 7,
        EscapeError::TooShortHexEscape => 8,
        EscapeError::InvalidCharInHexEscape => 9,
        EscapeError::OutOfRangeHexEscape => 10,
        EscapeError::NoBraceInUnicodeEscape => 11,
        EscapeError::InvalidCharInUnicodeEscape => 12,
        EscapeError::EmptyUnicodeEscape => 13,
        EscapeError::UnclosedUnicodeEscape => 14,
        EscapeError::LeadingUnderscoreUnicodeEscape => 15,
        EscapeError::OverlongUnicodeEscape => 16,
        EscapeError::LoneSurrogateUnicodeEscape => 17,
        EscapeError::OutOfRangeUnicodeEscape => 18,
        EscapeError::UnicodeEscapeInByte => 19,
        EscapeError::NonAsciiCharInByte => 20,
        EscapeError::NulInCStr => 21,
        EscapeError::UnskippedWhitespaceWarning => 22,
        EscapeError::MultipleSkippedLinesWarning => 23,
    }
}

fn event(out: &mut Vec<u8>, range: Range<usize>, kind: u8, value: u32) {
    out.extend_from_slice(&(range.start as u32).to_le_bytes());
    out.extend_from_slice(&(range.end as u32).to_le_bytes());
    out.push(kind);
    out.extend_from_slice(&value.to_le_bytes());
}

fn result<T>(out: &mut Vec<u8>, range: Range<usize>, value: Result<T, EscapeError>, convert: impl FnOnce(T) -> (u8, u32)) {
    match value {
        Ok(value) => { let (kind, value) = convert(value); event(out, range, kind, value); }
        Err(error) => event(out, range, 0, error_code(error)),
    }
}

fn evaluate(op: u8, source: &str, out: &mut Vec<u8>) {
    out.clear();
    out.extend_from_slice(&0u32.to_le_bytes());
    match op {
        0 => result(out, 0..source.len(), unescape_char(source), |c| (1, c as u32)),
        1 => result(out, 0..source.len(), unescape_byte(source), |b| (2, b as u32)),
        2 => unescape_str(source, |range, value| result(out, range, value, |c| (1, c as u32))),
        3 => unescape_byte_str(source, |range, value| result(out, range, value, |b| (2, b as u32))),
        4 => unescape_c_str(source, |range, value| result(out, range, value, |unit| match unit {
            MixedUnit::Char(c) => (3, c.get() as u32), MixedUnit::HighByte(b) => (4, b.get() as u32),
        })),
        5 => check_raw_str(source, |range, value| result(out, range, value, |c| (1, c as u32))),
        6 => check_raw_byte_str(source, |range, value| result(out, range, value, |b| (2, b as u32))),
        7 => check_raw_c_str(source, |range, value| result(out, range, value, |c| (5, c.get() as u32))),
        8..=15 => {
            let mode = [Mode::Char, Mode::Byte, Mode::Str, Mode::RawStr, Mode::ByteStr, Mode::RawByteStr, Mode::CStr, Mode::RawCStr][(op - 8) as usize];
            check_for_errors(source, mode, |range, error| event(out, range, 0, error_code(error)));
        }
        _ => panic!("invalid oracle operation"),
    }
    let count = ((out.len() - 4) / 13) as u32;
    out[..4].copy_from_slice(&count.to_le_bytes());
}

fn scalar_oracle(mut output: impl Write) -> io::Result<()> {
    let mut result = Vec::new();
    for value in 0..=0x10ffff {
        let Some(c) = char::from_u32(value) else { continue };
        let direct = c.to_string();
        for op in 0..8 {
            evaluate(op, &direct, &mut result);
            output.write_all(&result)?;
        }
        let escaped = format!("\\u{{{value:x}}}");
        for op in 0..5 {
            evaluate(op, &escaped, &mut result);
            output.write_all(&result)?;
        }
        let continuation = format!("\\\n{c}");
        for op in 2..5 {
            evaluate(op, &continuation, &mut result);
            output.write_all(&result)?;
        }
    }
    Ok(())
}

fn metadata(mut output: impl Write) -> io::Result<()> {
    let errors = [
        EscapeError::ZeroChars, EscapeError::MoreThanOneChar, EscapeError::LoneSlash,
        EscapeError::InvalidEscape, EscapeError::BareCarriageReturn, EscapeError::BareCarriageReturnInRawString,
        EscapeError::EscapeOnlyChar, EscapeError::TooShortHexEscape, EscapeError::InvalidCharInHexEscape,
        EscapeError::OutOfRangeHexEscape, EscapeError::NoBraceInUnicodeEscape, EscapeError::InvalidCharInUnicodeEscape,
        EscapeError::EmptyUnicodeEscape, EscapeError::UnclosedUnicodeEscape, EscapeError::LeadingUnderscoreUnicodeEscape,
        EscapeError::OverlongUnicodeEscape, EscapeError::LoneSurrogateUnicodeEscape, EscapeError::OutOfRangeUnicodeEscape,
        EscapeError::UnicodeEscapeInByte, EscapeError::NonAsciiCharInByte, EscapeError::NulInCStr,
        EscapeError::UnskippedWhitespaceWarning, EscapeError::MultipleSkippedLinesWarning,
    ];
    for error in errors { output.write_all(&[u8::from(error.is_fatal())])?; }
    for mode in [Mode::Char,Mode::Byte,Mode::Str,Mode::RawStr,Mode::ByteStr,Mode::RawByteStr,Mode::CStr,Mode::RawCStr] {
        let prefix = mode.prefix_noraw().as_bytes();
        output.write_all(&[u8::from(mode.in_double_quotes()), prefix.len() as u8])?;
        output.write_all(prefix)?;
    }
    Ok(())
}

fn main() -> io::Result<()> {
    let mut output = BufWriter::new(io::stdout().lock());
    match std::env::args().nth(1).as_deref() {
        Some("scalars") => return scalar_oracle(output),
        Some("metadata") => return metadata(output),
        Some("constructors") => return constructors(output),
        _ => (),
    }
    let mut input = BufReader::new(io::stdin().lock());
    let mut result = Vec::new();
    loop {
        let mut operation = [0u8; 1];
        match input.read_exact(&mut operation) {
            Err(error) if error.kind() == io::ErrorKind::UnexpectedEof => break,
            value => value?,
        }
        let mut length = [0u8; 4];
        input.read_exact(&mut length)?;
        let mut source = vec![0; u32::from_le_bytes(length) as usize];
        input.read_exact(&mut source)?;
        evaluate(operation[0], std::str::from_utf8(&source).unwrap(), &mut result);
        output.write_all(&result)?;
    }
    Ok(())
}

fn constructors(mut output: impl Write) -> io::Result<()> {
    for value in [1u8, 65, 127, 128, 255] {
        let nonzero = NonZero::new(value).unwrap();
        let direct = MixedUnit::HighByte(nonzero);
        let normalized = MixedUnit::from(nonzero);
        let character = MixedUnit::Char(NonZero::new(value as char).unwrap());
        for unit in [direct, normalized, character] {
            let (kind, value) = match unit {
                MixedUnit::Char(c) => (3, c.get() as u32),
                MixedUnit::HighByte(b) => (4, b.get() as u32),
            };
            output.write_all(&[kind])?;
            output.write_all(&value.to_le_bytes())?;
        }
        output.write_all(&[
            u8::from(direct == normalized),
            u8::from(direct == character),
            u8::from(normalized == character),
            u8::from(direct == direct),
        ])?;
    }
    Ok(())
}
