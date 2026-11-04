// SPDX-License-Identifier: MIT OR Apache-2.0
// Formatting adapters required by fingerprint.rs. The scoped DebugTuple and
// PadAdapter operations follow pinned core/src/fmt/builders.rs; no whole-file
// standard-library port credit is claimed for this supporting implementation.
package datastructures

import (
	"fmt"
	"io"
	"strings"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func fingerprintWrite(writer io.Writer, text string) error { return rustfmt.WriteString(writer, text) }

func fingerprintDebugTuple(formatter *rustchashes.NumericFormatter, name string, fields ...func(*rustchashes.NumericFormatter) error) error {
	pretty := formatter.Options.Alternate
	values := make([]rustfmt.Value, len(fields))
	for index, field := range fields {
		values[index] = func(writer io.Writer) error {
			if pretty {
				child := *formatter
				child.Writer = writer
				return field(&child)
			}
			return field(formatter)
		}
	}
	return rustfmt.WriteTuple(formatter.Writer, name, pretty, values)
}

// WriteDebug retains the source's tuple structure, inherited integer options,
// per-write errors, and optional independent write_char overrides.
func (value Fingerprint) WriteDebug(formatter *rustchashes.NumericFormatter) error {
	return fingerprintDebugTuple(formatter, "Fingerprint",
		func(child *rustchashes.NumericFormatter) error { return child.DebugU64(value.first) },
		func(child *rustchashes.NumericFormatter) error { return child.DebugU64(value.second) })
}
func (value PackedFingerprint) WriteDebug(formatter *rustchashes.NumericFormatter) error {
	return fingerprintDebugTuple(formatter, "PackedFingerprint", value.Fingerprint().WriteDebug)
}

// Display's inner format string supplies default options for each hex field;
// the outer formatter's width, precision, fill and flags are not consulted.
func (value Fingerprint) WriteDisplay(formatter *rustchashes.NumericFormatter) error {
	child := rustchashes.NumericFormatter{Writer: formatter.Writer}
	if err := child.LowerHexU64(value.first); err != nil {
		return err
	}
	if err := fingerprintWrite(formatter.Writer, "-"); err != nil {
		return err
	}
	return child.LowerHexU64(value.second)
}
func (value PackedFingerprint) WriteDisplay(formatter *rustchashes.NumericFormatter) error {
	return value.Fingerprint().WriteDisplay(formatter)
}
func (value Fingerprint) ToHex() string {
	var output strings.Builder
	f := rustchashes.NumericFormatter{Writer: &output}
	_ = f.LowerHexU64(value.first)
	_ = f.LowerHexU64(value.second)
	return output.String()
}
func (value Fingerprint) String() string {
	var output strings.Builder
	_ = value.WriteDisplay(&rustchashes.NumericFormatter{Writer: &output})
	return output.String()
}
func (value PackedFingerprint) String() string { return value.Fingerprint().String() }
func (value Fingerprint) DebugString() string {
	var output strings.Builder
	_ = value.WriteDebug(&rustchashes.NumericFormatter{Writer: &output})
	return output.String()
}
func (value PackedFingerprint) DebugString() string {
	var output strings.Builder
	_ = value.WriteDebug(&rustchashes.NumericFormatter{Writer: &output})
	return output.String()
}

type fingerprintFormatValue interface {
	WriteDisplay(*rustchashes.NumericFormatter) error
	WriteDebug(*rustchashes.NumericFormatter) error
}

func formatFingerprint(state fmt.State, verb rune, value fingerprintFormatValue) {
	options := rustchashes.FormatOptions{Plus: state.Flag('+'), Alternate: state.Flag('#'), ZeroPad: state.Flag('0')}
	if width, ok := state.Width(); ok {
		options.Width = uint(width)
	}
	if state.Flag('-') {
		options.Alignment = rustchashes.AlignLeft
	}
	f := rustchashes.NumericFormatter{Writer: state, Options: options}
	switch verb {
	case 's':
		_ = value.WriteDisplay(&f)
	case 'v':
		if state.Flag('#') {
			_ = value.WriteDebug(&f)
		} else {
			_ = value.WriteDisplay(&f)
		}
	default:
		_, _ = fmt.Fprintf(state, "%%!%c(datastructures.Fingerprint)", verb)
	}
}
func (value Fingerprint) Format(state fmt.State, verb rune) { formatFingerprint(state, verb, value) }
func (value PackedFingerprint) Format(state fmt.State, verb rune) {
	formatFingerprint(state, verb, value)
}
