// SPDX-License-Identifier: MIT OR Apache-2.0
// Formatting, stable comparison and stable hashing blocks of symbol.rs.
package rustcspan

import (
	"io"
	"strings"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// SymbolUnicodeProfile separates the pinned source's Unicode18 dependency from
// the explicitly recorded Rust1.99.1/Unicode17 comparison-runtime adapter.
type SymbolUnicodeProfile uint8

const (
	SymbolPinnedUnicode18 SymbolUnicodeProfile = iota
	SymbolComparisonUnicode17
)

func (value Symbol) WriteDisplayFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	return f.PadString(value.AsStrWithContext(session, context))
}
func (value Symbol) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	return value.WriteDebugFormatterWithProfile(session, f, SymbolPinnedUnicode18)
}
func (value Symbol) WriteDebugFormatterWithProfile(session *SessionGlobals, f *rustfmt.Formatter, profile SymbolUnicodeProfile) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	text := value.AsStrWithContext(session, context)
	switch profile {
	case SymbolPinnedUnicode18:
		return symbolDebugString18(text, f)
	case SymbolComparisonUnicode17:
		return rustfmt.FormatDebugString(text, f)
	default:
		context.Panic("rustcspan: invalid Symbol Unicode profile")
		return nil
	}
}
func (value ByteSymbol) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	bytes := value.AsByteStrWithContext(session, context)
	pretty := f.Options.Alternate
	builder := rustfmt.BeginList(f.Writer, pretty)
	for index := 0; index < len(bytes); index++ {
		byteValue := bytes[index]
		builder.Entry(f.Value(pretty, func(child *rustfmt.Formatter) error {
			return rustfmt.FormatDebugUint(uint64(byteValue), 8, child)
		}))
	}
	return builder.Finish()
}
func (value Symbol) DisplayTo(session *SessionGlobals, writer io.Writer, options ...rustfmt.Options) error {
	flags := rustfmt.DefaultOptions()
	if len(options) > 1 {
		panic("rustcspan: multiple formatter options")
	}
	if len(options) == 1 {
		flags = options[0]
	}
	return value.WriteDisplayFormatter(session, &rustfmt.Formatter{Writer: writer, Options: flags})
}
func (value Symbol) DebugTo(session *SessionGlobals, writer io.Writer, options ...rustfmt.Options) error {
	flags := rustfmt.DefaultOptions()
	if len(options) > 1 {
		panic("rustcspan: multiple formatter options")
	}
	if len(options) == 1 {
		flags = options[0]
	}
	return value.WriteDebugFormatter(session, &rustfmt.Formatter{Writer: writer, Options: flags})
}
func (value Symbol) DisplayString(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDisplayFormatter(session, f) })
}
func (value Symbol) RustDebug(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDebugFormatter(session, f) })
}
func (value ByteSymbol) RustDebug(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDebugFormatter(session, f) })
}

const SymbolCanUseUnstableSort = true

func (value Symbol) StableCompare(session *SessionGlobals, other Symbol) int {
	return value.StableCompareWithContext(session, other, rustfmt.Context{})
}
func (value Symbol) StableCompareWithContext(session *SessionGlobals, other Symbol, context rustfmt.Context) int {
	return strings.Compare(value.AsStrWithContext(session, context), other.AsStrWithContext(session, context))
}

// StableHash uses the actual concrete source hasher and the complete borrowed
// context trait. str delegates to specialized [u8] StableHash: length then one
// bulk write, distinct from std Hash for str's sentinel convention.
func (value Symbol) StableHash(session *SessionGlobals, hashContext datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.StableHashWithContext(session, hashContext, hasher, rustfmt.Context{})
}
func (value Symbol) StableHashWithContext(session *SessionGlobals, hashContext datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	text := value.AsStrWithContext(session, ambient)
	hasher.WriteUsize(uint(len(text)))
	hasher.Write([]byte(text))
}
func (value ByteSymbol) StableHash(session *SessionGlobals, hashContext datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.StableHashWithContext(session, hashContext, hasher, rustfmt.Context{})
}
func (value ByteSymbol) StableHashWithContext(session *SessionGlobals, hashContext datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	bytes := value.AsByteStrWithContext(session, ambient)
	hasher.WriteUsize(uint(len(bytes)))
	hasher.Write([]byte(bytes))
}
