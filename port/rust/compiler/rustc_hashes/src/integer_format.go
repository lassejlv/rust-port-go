// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped standard-integer fmt adapter used by the source-ported hash wrappers.
// This supporting file is not completion credit for any standard-library file.
package rustchashes

import (
	"fmt"
	"io"
	"math/bits"
	"strings"
	"unicode/utf8"
)

type Alignment uint8

const (
	AlignDefault Alignment = iota
	AlignLeft
	AlignRight
	AlignCenter
)

type DebugRadix uint8

const (
	DebugDecimal DebugRadix = iota
	DebugLowerHex
	DebugUpperHex
)

// FormatOptions models the options consulted by Rust's unsigned integer
// Debug/LowerHex implementations. Integer precision and sign-minus are ignored.
type FormatOptions struct {
	Width                    uint
	Alignment                Alignment
	Fill                     rune // zero selects the default space unless FillSet
	FillSet                  bool // distinguishes an explicit NUL fill
	Plus, Alternate, ZeroPad bool
	DebugRadix               DebugRadix
}

type NumericFormatter struct {
	Writer  io.Writer
	Options FormatOptions
}

func (f *NumericFormatter) DebugU64(v uint64) error { return f.DebugU128(Uint128{Lo: v}) }
func (f *NumericFormatter) DebugU128(v Uint128) error {
	switch f.Options.DebugRadix {
	case DebugDecimal:
		return f.number(v, 10, false)
	case DebugLowerHex:
		return f.number(v, 16, false)
	case DebugUpperHex:
		return f.number(v, 16, true)
	default:
		panic("rustchashes: invalid Debug radix")
	}
}
func (f *NumericFormatter) LowerHexU64(v uint64) error   { return f.LowerHexU128(Uint128{Lo: v}) }
func (f *NumericFormatter) LowerHexU128(v Uint128) error { return f.number(v, 16, false) }

func unsignedDigits(v Uint128, radix uint64, upper bool) string {
	const lower = "0123456789abcdef"
	const caps = "0123456789ABCDEF"
	alphabet := lower
	if upper {
		alphabet = caps
	}
	var buffer [128]byte
	position := len(buffer)
	for {
		quotientHi, rem := v.Hi/radix, v.Hi%radix
		quotientLo, rem := bits.Div64(rem, v.Lo, radix)
		position--
		buffer[position] = alphabet[rem]
		v = Uint128{Lo: quotientLo, Hi: quotientHi}
		if v.Lo|v.Hi == 0 {
			return string(buffer[position:])
		}
	}
}

func writeText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

// CharWriter adapts an independent fmt::Write::write_char override. Writers
// without this method use UTF-8 write_str, as Rust's default method does.
type CharWriter interface{ WriteChar(rune) error }

func writeChar(w io.Writer, ch rune) error {
	if chars, ok := w.(CharWriter); ok {
		return chars.WriteChar(ch)
	}
	return writeText(w, string(ch))
}

func (f *NumericFormatter) number(v Uint128, radix uint64, upper bool) error {
	digits := unsignedDigits(v, radix, upper)
	prefix := ""
	if radix == 16 && f.Options.Alternate {
		prefix = "0x"
	}
	sign := ""
	if f.Options.Plus {
		sign = "+"
	}
	width := uint(len(sign) + len(prefix) + len(digits))
	padding := uint(0)
	if f.Options.Width > width {
		padding = f.Options.Width - width
	}
	fill := f.Options.Fill
	if fill == 0 && !f.Options.FillSet {
		fill = ' '
	}
	if !utf8.ValidRune(fill) {
		panic("rustchashes: fill must be a Unicode scalar")
	}
	alignment := f.Options.Alignment
	if alignment > AlignCenter {
		panic("rustchashes: invalid alignment")
	}
	writePrefix := func() error {
		if sign != "" {
			if err := writeChar(f.Writer, '+'); err != nil {
				return err
			}
		}
		if f.Options.Alternate {
			return writeText(f.Writer, prefix)
		}
		return nil
	}
	writePadding := func(count uint, ch rune) error {
		for n := uint(0); n < count; n++ {
			if err := writeChar(f.Writer, ch); err != nil {
				return err
			}
		}
		return nil
	}
	if padding > 0 && f.Options.ZeroPad {
		previous := f.Options
		f.Options.Fill = '0'
		f.Options.FillSet = true
		f.Options.Alignment = AlignRight
		if err := writePrefix(); err != nil {
			return err
		}
		if err := writePadding(padding, '0'); err != nil {
			return err
		}
		if err := writeText(f.Writer, digits); err != nil {
			return err
		}
		f.Options = previous
		return nil
	}
	before, after := padding, uint(0)
	switch alignment {
	case AlignLeft:
		before, after = 0, padding
	case AlignCenter:
		before, after = padding/2, padding-padding/2
	}
	if err := writePadding(before, fill); err != nil {
		return err
	}
	if err := writePrefix(); err != nil {
		return err
	}
	if err := writeText(f.Writer, digits); err != nil {
		return err
	}
	return writePadding(after, fill)
}

func (value Hash64) DebugString() string {
	var b strings.Builder
	_ = value.WriteDebug(&NumericFormatter{Writer: &b})
	return b.String()
}
func (value Hash128) DebugString() string {
	var b strings.Builder
	_ = value.WriteDebug(&NumericFormatter{Writer: &b})
	return b.String()
}

func goFormat(state fmt.State, verb rune, value FormatterValue) {
	if verb != 'v' && verb != 'x' {
		_, _ = fmt.Fprintf(state, "%%!%c(rustchashes.Hash=%s)", verb, value.DebugString())
		return
	}
	options := FormatOptions{Plus: state.Flag('+'), Alternate: state.Flag('#'), ZeroPad: state.Flag('0')}
	if width, ok := state.Width(); ok {
		options.Width = uint(width)
	}
	if state.Flag('-') {
		options.Alignment = AlignLeft
	}
	f := NumericFormatter{Writer: state, Options: options}
	if verb == 'x' {
		_ = value.WriteLowerHex(&f)
	} else {
		_ = value.WriteDebug(&f)
	}
}

type FormatterValue interface {
	WriteDebug(Formatter) error
	WriteLowerHex(Formatter) error
	DebugString() string
}

func (value Hash64) Format(state fmt.State, verb rune)  { goFormat(state, verb, value) }
func (value Hash128) Format(state fmt.State, verb rune) { goFormat(state, verb, value) }
