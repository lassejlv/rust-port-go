// SPDX-License-Identifier: MIT OR Apache-2.0
// Full derived Debug contracts from the pinned rustc_lexer/src/lib.rs.
package rustclexer

import (
	"fmt"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// DebugFormatter is the primary mutable source formatter. String-returning
// RustDebug methods are ordinary-default conveniences over the same dispatch.
type DebugFormatter = rustfmt.Formatter
type DebugOptions = rustfmt.Options
type DebugContext = rustfmt.Context

func debugField(f *DebugFormatter, name string, format func(*DebugFormatter) error) rustfmt.Field {
	return rustfmt.Field{Name: name, Value: f.Value(f.Options.Alternate, format)}
}
func debugU32(value *uint32, f *DebugFormatter) error {
	return rustfmt.FormatDebugUint(uint64(*value), 32, f)
}
func debugBool(value *bool, f *DebugFormatter) error { return rustfmt.FormatDebugBool(*value, f) }
func debugOptionU32(value *uint32, present bool, f *DebugFormatter) error {
	if !present {
		return f.WriteString("None")
	}
	return rustfmt.WriteTuple(f.Writer, "Some", f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, func(child *DebugFormatter) error { return debugU32(value, child) })})
}
func debugOptionU8(value *uint8, present bool, f *DebugFormatter) error {
	if !present {
		return f.WriteString("None")
	}
	return rustfmt.WriteTuple(f.Writer, "Some", f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, func(child *DebugFormatter) error { return rustfmt.FormatDebugUint(uint64(*value), 8, child) })})
}
func debugDocOption(value *DocStyle, f *DebugFormatter) error {
	if *value == NoDocStyle {
		return f.WriteString("None")
	}
	return rustfmt.WriteTuple(f.Writer, "Some", f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, value.FormatDebug)})
}

func (t *Token) FormatDebug(f *DebugFormatter) error {
	return rustfmt.WriteStruct(f.Writer, "Token", f.Options.Alternate, []rustfmt.Field{
		debugField(f, "kind", t.Kind.FormatDebug),
		debugField(f, "len", func(child *DebugFormatter) error { return debugU32(&t.Len, child) }),
	})
}
func (k *TokenKind) FormatDebug(f *DebugFormatter) error {
	var fields []rustfmt.Field
	switch k.Tag {
	case LineComment:
		fields = []rustfmt.Field{debugField(f, "doc_style", func(child *DebugFormatter) error { return debugDocOption(&k.DocStyle, child) })}
	case BlockComment:
		fields = []rustfmt.Field{debugField(f, "doc_style", func(child *DebugFormatter) error { return debugDocOption(&k.DocStyle, child) }), debugField(f, "terminated", func(child *DebugFormatter) error { return debugBool(&k.Terminated, child) })}
	case Frontmatter:
		fields = []rustfmt.Field{debugField(f, "has_invalid_preceding_whitespace", func(child *DebugFormatter) error { return debugBool(&k.HasInvalidPrecedingWhitespace, child) }), debugField(f, "invalid_infostring", func(child *DebugFormatter) error { return debugBool(&k.InvalidInfostring, child) })}
	case Literal:
		fields = []rustfmt.Field{debugField(f, "kind", k.Literal.FormatDebug), debugField(f, "suffix_start", func(child *DebugFormatter) error { return debugU32(&k.SuffixStart, child) })}
	case Lifetime:
		fields = []rustfmt.Field{debugField(f, "starts_with_number", func(child *DebugFormatter) error { return debugBool(&k.StartsWithNumber, child) })}
	default:
		return f.WriteString(k.Tag.String())
	}
	return rustfmt.WriteStruct(f.Writer, k.Tag.String(), f.Options.Alternate, fields)
}
func (d *DocStyle) FormatDebug(f *DebugFormatter) error {
	switch *d {
	case Outer:
		return f.WriteString("Outer")
	case Inner:
		return f.WriteString("Inner")
	case NoDocStyle:
		return f.WriteString("None")
	default:
		return f.WriteString(fmt.Sprintf("DocStyle(%d)", *d))
	}
}
func (d DocStyle) RustDebug() string                { return rustfmt.String(d.FormatDebug) }
func (b *Base) FormatDebug(f *DebugFormatter) error { return f.WriteString(b.String()) }
func (b Base) RustDebug() string                    { return rustfmt.String(b.FormatDebug) }
func (k *LiteralKind) FormatDebug(f *DebugFormatter) error {
	var name string
	var fields []rustfmt.Field
	switch k.Tag {
	case Int:
		name = "Int"
		fields = []rustfmt.Field{debugField(f, "base", k.Base.FormatDebug), debugField(f, "empty_int", func(child *DebugFormatter) error { return debugBool(&k.EmptyInt, child) })}
	case Float:
		name = "Float"
		fields = []rustfmt.Field{debugField(f, "base", k.Base.FormatDebug), debugField(f, "empty_exponent", func(child *DebugFormatter) error { return debugBool(&k.EmptyExponent, child) })}
	case Char, Byte, Str, ByteStr, CStr:
		name = [...]string{"Char", "Byte", "Str", "ByteStr", "CStr"}[k.Tag-Char]
		fields = []rustfmt.Field{debugField(f, "terminated", func(child *DebugFormatter) error { return debugBool(&k.Terminated, child) })}
	case RawStr, RawByteStr, RawCStr:
		name = [...]string{"RawStr", "RawByteStr", "RawCStr"}[k.Tag-RawStr]
		fields = []rustfmt.Field{debugField(f, "n_hashes", func(child *DebugFormatter) error { return debugOptionU8(&k.NHashes, k.HasNHashes, child) })}
	default:
		return f.WriteString(fmt.Sprintf("LiteralTag(%d)", k.Tag))
	}
	return rustfmt.WriteStruct(f.Writer, name, f.Options.Alternate, fields)
}
func (g *GuardedStr) FormatDebug(f *DebugFormatter) error {
	return rustfmt.WriteStruct(f.Writer, "GuardedStr", f.Options.Alternate, []rustfmt.Field{debugField(f, "n_hashes", func(child *DebugFormatter) error { return debugU32(&g.NHashes, child) }), debugField(f, "terminated", func(child *DebugFormatter) error { return debugBool(&g.Terminated, child) }), debugField(f, "token_len", func(child *DebugFormatter) error { return debugU32(&g.TokenLen, child) })})
}
func (g GuardedStr) RustDebug() string { return rustfmt.String(g.FormatDebug) }
func (e *RawStrError) FormatDebug(f *DebugFormatter) error {
	var name string
	var fields []rustfmt.Field
	switch e.Tag {
	case InvalidStarter:
		name = "InvalidStarter"
		fields = []rustfmt.Field{debugField(f, "bad_char", func(child *DebugFormatter) error { return rustfmt.FormatDebugChar(e.BadChar, child) })}
	case NoTerminator:
		name = "NoTerminator"
		fields = []rustfmt.Field{debugField(f, "expected", func(child *DebugFormatter) error { return debugU32(&e.Expected, child) }), debugField(f, "found", func(child *DebugFormatter) error { return debugU32(&e.Found, child) }), debugField(f, "possible_terminator_offset", func(child *DebugFormatter) error {
			return debugOptionU32(&e.PossibleTerminatorOffset, e.HasPossibleTerminatorOffset, child)
		})}
	case TooManyDelimiters:
		name = "TooManyDelimiters"
		fields = []rustfmt.Field{debugField(f, "found", func(child *DebugFormatter) error { return debugU32(&e.Found, child) })}
	default:
		return f.WriteString(fmt.Sprintf("RawStrErrorTag(%d)", e.Tag))
	}
	return rustfmt.WriteStruct(f.Writer, name, f.Options.Alternate, fields)
}
