use std::io::{self, Write};
use unicode_properties::UnicodeEmoji;

fn main() -> io::Result<()> {
    let ident = unicode_ident::UNICODE_VERSION;
    let properties = unicode_properties::UNICODE_VERSION;
    assert_eq!(
        (ident.0 as u64, ident.1 as u64, ident.2 as u64),
        properties,
        "rustc_lexer requires matching Unicode versions"
    );
    let mut output = Vec::with_capacity(12 + 0x110000);
    output.extend_from_slice(b"FERRUGOU");
    output.extend_from_slice(&[ident.0, ident.1, ident.2, 0]);
    for code_point in 0..=0x10ffff {
        let flags = char::from_u32(code_point).map_or(0, |c| {
            u8::from(unicode_ident::is_xid_start(c))
                | (u8::from(unicode_ident::is_xid_continue(c)) << 1)
                | (u8::from(c.is_emoji_char()) << 2)
        });
        output.push(flags);
    }
    io::stdout().lock().write_all(&output)
}
