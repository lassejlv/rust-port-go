// SPDX-License-Identifier: MIT OR Apache-2.0
// Shared source-contract adapter; grants no Rust standard-library file credit.
package rustfmt

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Alignment uint8

const (
	AlignNone Alignment = iota
	AlignLeft
	AlignRight
	AlignCenter
)

// Options carries mutable source Formatter state. Fill zero is the lawful NUL
// fill, so use DefaultOptions for Rust's ordinary space default.
type Options struct {
	Fill                                    rune
	Align                                   Alignment
	Width, Precision                        uint
	HasWidth, HasPrecision                  bool
	Alternate, SignPlus, SignMinus, ZeroPad bool
	DebugLowerHex, DebugUpperHex            bool
}

func DefaultOptions() Options { return Options{Fill: ' '} }

// Context carries explicit ambient Rust panic state, without global Go panic
// interception. Component-owned cleanup guards distinguish a locally escaping
// second panic from an ambient panic that a source caller can still catch.
// RuntimeMetadata is an opaque borrowed ambient-runtime witness. It has no
// owned destructor or semantic Clone; copying Context preserves this pointer.
// The pointer keeps Context comparable even when the borrowed payload is not.
// Component adapters can carry caller-local tracing/session metadata without a
// dependency from this neutral formatter onto compiler/runtime packages.
type RuntimeMetadata struct {
	Value any
	Panic *PanicOperations
}
type Context struct {
	Unwinding, ProcessingPanic bool
	Runtime                    *RuntimeMetadata `json:"-"`
}

func (c Context) Panicking() bool { return c.Unwinding || c.ProcessingPanic }
func (c Context) Panic(value any) {
	c.PanicAt(SourcePanicInfo{Payload: value, Kind: PanicOpaque, CanUnwind: true})
}

// Formatter is the primary mutable formatter and fallible fmt::Write witness.
// Writer may independently implement CharWriter. It has no sticky error; only
// the Result returned by a Debug implementation controls its caller's builder.
type Formatter struct {
	Options Options
	Context Context
	Writer  io.Writer
}

func (f *Formatter) WriteString(s string) error { return WriteString(f.Writer, s) }
func (f *Formatter) WriteChar(c rune) error     { return WriteChar(f.Writer, c) }
func (f *Formatter) CloneWithWriter(writer io.Writer) *Formatter {
	child := *f
	child.Writer = writer
	return &child
}

// Value bridges the builder's writer to a formatter callback. Compact fields
// retain the actual formatter pointer; pretty fields wrap a copied formatter.
func (f *Formatter) Value(pretty bool, format func(*Formatter) error) Value {
	return func(writer io.Writer) error {
		if pretty {
			return format(f.CloneWithWriter(writer))
		}
		return format(f)
	}
}
func String(format func(*Formatter) error) string {
	var out strings.Builder
	f := Formatter{Options: DefaultOptions(), Writer: &out}
	if err := format(&f); err != nil {
		f.Context.Panic("rustfmt: formatting trait returned an error for a string sink")
	}
	return out.String()
}
func FormatDebugBool(value bool, f *Formatter) error { return f.PadString(strconv.FormatBool(value)) }

// FormatDebugInt and FormatDebugUint are the standard integer Debug witnesses.
// bits is the source integer width (8/16/32/64, or host usize width); signed
// hexadecimal Debug formats the two's complement representation of that width.
func FormatDebugInt(value int64, bits uint, f *Formatter) error {
	if bits == 0 || bits > 64 {
		panic("rustfmt: invalid integer Debug width")
	}
	if f.Options.DebugLowerHex || f.Options.DebugUpperHex {
		return FormatDebugUint(uint64(value), bits, f)
	}
	abs := uint64(value)
	if value < 0 {
		abs = uint64(-(value + 1)) + 1
	}
	return f.padIntegral(value >= 0, "", strconv.FormatUint(abs, 10))
}
func FormatDebugUint(value uint64, bits uint, f *Formatter) error {
	if bits == 0 || bits > 64 {
		panic("rustfmt: invalid integer Debug width")
	}
	if bits < 64 {
		value &= (uint64(1) << bits) - 1
	}
	base, prefix := 10, ""
	if f.Options.DebugLowerHex || f.Options.DebugUpperHex {
		base, prefix = 16, "0x"
	}
	digits := strconv.FormatUint(value, base)
	if !f.Options.DebugLowerHex && f.Options.DebugUpperHex {
		digits = strings.ToUpper(digits)
	}
	return f.padIntegral(true, prefix, digits)
}
func (f *Formatter) fill() rune { return f.Options.Fill }

func (f *Formatter) padding(n uint, align Alignment, fill rune) (uint, error) {
	left := n
	if align == AlignLeft {
		left = 0
	} else if align == AlignCenter {
		left = n / 2
	}
	for i := uint(0); i < left; i++ {
		if err := f.WriteChar(fill); err != nil {
			return 0, err
		}
	}
	return n - left, nil
}
func (f *Formatter) postPadding(n uint, fill rune) error {
	for i := uint(0); i < n; i++ {
		if err := f.WriteChar(fill); err != nil {
			return err
		}
	}
	return nil
}
func (f *Formatter) padIntegral(nonnegative bool, prefix, digits string) error {
	sign := ""
	if !nonnegative {
		sign = "-"
	} else if f.Options.SignPlus {
		sign = "+"
	}
	width := uint(len(digits) + len(sign))
	if f.Options.Alternate {
		width += uint(utf8.RuneCountInString(prefix))
	}
	writePrefix := func() error {
		if sign != "" {
			if err := f.WriteChar(rune(sign[0])); err != nil {
				return err
			}
		}
		if f.Options.Alternate {
			return f.WriteString(prefix)
		}
		return nil
	}
	minimum := uint(0)
	if f.Options.HasWidth {
		minimum = f.Options.Width
	}
	if width >= minimum {
		if err := writePrefix(); err != nil {
			return err
		}
		return f.WriteString(digits)
	}
	align, fill := f.Options.Align, f.fill()
	if align == AlignNone {
		align = AlignRight
	}
	if f.Options.ZeroPad {
		old := f.Options
		f.Options.Fill, f.Options.Align = '0', AlignRight
		if err := writePrefix(); err != nil {
			return err
		}
		post, err := f.padding(minimum-width, AlignRight, '0')
		if err != nil {
			return err
		}
		if err = f.WriteString(digits); err != nil {
			return err
		}
		if err = f.postPadding(post, '0'); err != nil {
			return err
		}
		f.Options = old
		return nil
	}
	post, err := f.padding(minimum-width, align, fill)
	if err != nil {
		return err
	}
	if err = writePrefix(); err != nil {
		return err
	}
	if err = f.WriteString(digits); err != nil {
		return err
	}
	return f.postPadding(post, fill)
}

// PadString is Formatter::pad, used by the standard bool/str Display witnesses.
func (f *Formatter) PadString(text string) error {
	if f.Options.HasPrecision {
		n := uint(0)
		for offset := range text {
			if n == f.Options.Precision {
				text = text[:offset]
				break
			}
			n++
		}
	}
	width := uint(utf8.RuneCountInString(text))
	if !f.Options.HasWidth || width >= f.Options.Width {
		return f.WriteString(text)
	}
	align := f.Options.Align
	if align == AlignNone {
		align = AlignLeft
	}
	post, err := f.padding(f.Options.Width-width, align, f.fill())
	if err != nil {
		return err
	}
	if err = f.WriteString(text); err != nil {
		return err
	}
	return f.postPadding(post, f.fill())
}
