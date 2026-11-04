// SPDX-License-Identifier: MIT OR Apache-2.0
// Formatting trait adapters for the complete pinned slice.rs/vec.rs port.
// Builder and padding behavior follows library/core/src/fmt/{builders,mod,num}.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179; this is not a Rust ABI bridge.
package rustcindex

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type IndexFormatAlignment uint8

const (
	IndexAlignNone IndexFormatAlignment = iota
	IndexAlignLeft
	IndexAlignRight
	IndexAlignCenter
)

// IndexFormatOptions carries the complete Formatter options relevant to Debug.
// Width and precision are optional; lawful source values fit the pinned Rust
// formatter's u16 fields. Use DefaultIndexFormatOptions for the source defaults;
// Radix flags are private Rust Formatter bits but are forwarded to integer Debug.
type IndexFormatOptions struct {
	Fill                                    rune
	Align                                   IndexFormatAlignment
	Width, Precision                        uint
	HasWidth, HasPrecision                  bool
	Alternate, SignPlus, SignMinus, ZeroPad bool
	DebugLowerHex, DebugUpperHex            bool
}

func DefaultIndexFormatOptions() IndexFormatOptions { return IndexFormatOptions{Fill: ' '} }

// IndexFormatContext exposes the source thread panic state without global Go
// state. ProcessingPanic distinguishes formatting an assertion's panic message
// from ordinary unwinding. Translated Rust panic sites must call Panic: only
// panic-message processing aborts before local callback cleanup can run.
// Plain Go panic/defer callbacks retain Go's own local cleanup timing.
type IndexFormatContext struct{ ProcessingPanic, Unwinding bool }

func (c IndexFormatContext) Panicking() bool { return c.ProcessingPanic || c.Unwinding }
func (c IndexFormatContext) Panic(value any) {
	if c.ProcessingPanic {
		os.Exit(134)
	}
	panic(value)
}

// IndexDebugFormatter is the primary borrowed Debug::fmt adapter. Write is a
// fallible fmt::Write::write_str witness, not an io.Writer with partial writes.
// It has no sticky error: a user Debug implementation can handle a write error;
// source debug builders retain only the Result returned by that implementation.
// Options are forwarded unchanged to children, including through pretty padding.
type IndexDebugFormatter struct {
	Options   IndexFormatOptions
	Context   IndexFormatContext
	Write     func(string) error
	WriteChar func(rune) error
}

func (f *IndexDebugFormatter) WriteString(s string) error { return f.Write(s) }
func (f *IndexDebugFormatter) WriteRune(c rune) error {
	if f.WriteChar != nil {
		return f.WriteChar(c)
	}
	return f.Write(string(c))
}

func indexFormatString(format func(*IndexDebugFormatter) error, context IndexFormatContext) string {
	var result strings.Builder
	f := IndexDebugFormatter{Options: DefaultIndexFormatOptions(), Context: context, Write: func(s string) error { _, _ = result.WriteString(s); return nil }}
	if err := format(&f); err != nil {
		context.Panic("rustcindex: formatting trait returned an error for a string sink")
	}
	return result.String()
}

func (ops ValueOps[T]) formatAt(value *T, f *IndexDebugFormatter) error {
	if ops.FormatDebug != nil {
		return ops.FormatDebug(value, f)
	}
	if ops.DebugRefWithContext != nil {
		return f.WriteString(ops.DebugRefWithContext(value, f.Context))
	}
	if ops.DebugWithContext != nil {
		return f.WriteString(ops.DebugWithContext(*value, f.Context))
	}
	if ops.DebugRef != nil {
		return f.WriteString(ops.DebugRef(value))
	}
	if ops.Debug != nil {
		return f.WriteString(ops.Debug(*value))
	}
	// Built-in integer and bool witnesses are source-standard implementations.
	// Other representations require FormatDebug for full writer/option behavior;
	// the existing string Debug convenience is retained as the fallback.
	v := reflect.ValueOf(*value)
	if v.IsValid() {
		switch v.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return FormatDebugInt(v.Int(), uint(v.Type().Bits()), f)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return FormatDebugUint(v.Uint(), uint(v.Type().Bits()), f)
		case reflect.Bool:
			return f.PadString(strconv.FormatBool(v.Bool()))
		}
	}
	return f.WriteString(ops.debugLegacyAt(value))
}

func (ops ValueOps[T]) debugAtContext(value *T, context IndexFormatContext) string {
	return indexFormatString(func(f *IndexDebugFormatter) error { return ops.formatAt(value, f) }, context)
}

// indexPadFormatter is the source PadAdapter: each write splits inclusively on
// newline, emits each indent separately, and updates state before the text write.
func indexPadFormatter(f *IndexDebugFormatter, newline *bool) IndexDebugFormatter {
	child := *f
	child.Write = func(s string) error {
		for len(s) != 0 {
			n := strings.IndexByte(s, '\n')
			if n < 0 {
				n = len(s)
			} else {
				n++
			}
			part := s[:n]
			if *newline {
				if err := f.WriteString("    "); err != nil {
					return err
				}
			}
			*newline = part[len(part)-1] == '\n'
			if err := f.WriteString(part); err != nil {
				return err
			}
			s = s[n:]
		}
		return nil
	}
	child.WriteChar = func(c rune) error {
		if *newline {
			if err := f.WriteString("    "); err != nil {
				return err
			}
		}
		*newline = c == '\n'
		return f.WriteRune(c)
	}
	return child
}

func (r IndexRaw[T]) FormatDebug(ops ValueOps[T], f *IndexDebugFormatter) error {
	err := f.WriteString("[")
	for i := uint(0); i < r.Len() && err == nil; i++ {
		if f.Options.Alternate {
			if i == 0 {
				err = f.WriteString("\n")
				if err != nil {
					break
				}
			}
			newline := true
			child := indexPadFormatter(f, &newline)
			err = ops.formatAt(r.At(i), &child)
			if err == nil {
				err = child.WriteString(",\n")
			}
		} else {
			if i != 0 {
				err = f.WriteString(", ")
				if err != nil {
					break
				}
			}
			err = ops.formatAt(r.At(i), f)
		}
	}
	if err != nil {
		return err
	}
	return f.WriteString("]")
}

func (s *IndexSlice[I, T]) FormatDebug(f *IndexDebugFormatter) error {
	return s.Raw.FormatDebug(s.Ops, f)
}
func (v *IndexVec[I, T]) FormatDebug(f *IndexDebugFormatter) error { return v.AsSlice().FormatDebug(f) }
func (s *IndexSlice[I, T]) DebugWithContext(context IndexFormatContext) string {
	return indexFormatString(s.FormatDebug, context)
}
func (v *IndexVec[I, T]) DebugWithContext(context IndexFormatContext) string {
	return v.AsSlice().DebugWithContext(context)
}

type indexDebugMapBuilder struct {
	f      *IndexDebugFormatter
	err    error
	fields bool
}

func newIndexDebugMap(f *IndexDebugFormatter) indexDebugMapBuilder {
	return indexDebugMapBuilder{f: f, err: f.WriteString("{")}
}
func (m *indexDebugMapBuilder) entry(key, value func(*IndexDebugFormatter) error) {
	defer func() { m.fields = true }()
	if m.err != nil {
		return
	}
	child := m.f
	if m.f.Options.Alternate {
		if !m.fields {
			m.err = m.f.WriteString("\n")
			if m.err != nil {
				return
			}
		}
		newline := true
		padded := indexPadFormatter(m.f, &newline)
		child = &padded
	} else if m.fields {
		m.err = m.f.WriteString(", ")
		if m.err != nil {
			return
		}
	}
	if m.err = key(child); m.err != nil {
		return
	}
	if m.err = child.WriteString(": "); m.err != nil {
		return
	}
	if m.err = value(child); m.err != nil {
		return
	}
	if m.f.Options.Alternate {
		m.err = child.WriteString(",\n")
	}
}
func (m *indexDebugMapBuilder) finish() error {
	if m.err != nil {
		return m.err
	}
	return m.f.WriteString("}")
}

func (view IndexSliceMapView[I, T]) WithKeyFormatter(format func(*I, *IndexDebugFormatter) error) IndexSliceMapView[I, T] {
	view.formatKeyRef = format
	return view
}
func (view IndexSliceMapView[I, T]) FormatDebug(f *IndexDebugFormatter) error {
	mapWriter := newIndexDebugMap(f)
	it := view.slice.IterEnumerated() // even a failed opening write precedes I::new(len)
	for {
		key, value, ok := it.Next()
		if !ok {
			break
		}
		mapWriter.entry(func(f *IndexDebugFormatter) error {
			if view.formatKeyRef != nil {
				return view.formatKeyRef(&key, f)
			}
			if view.formatKey != nil {
				return f.WriteString(view.formatKey(key))
			}
			return (ValueOps[I]{}).formatAt(&key, f)
		}, func(f *IndexDebugFormatter) error { return view.slice.Ops.formatAt(value, f) })
	}
	return mapWriter.finish()
}
func (view IndexSliceMapViewCompact[I, T]) WithKeyFormatter(format func(*I, *IndexDebugFormatter) error) IndexSliceMapViewCompact[I, T] {
	view.formatKeyRef = format
	return view
}
func (view IndexSliceMapViewCompact[I, T]) FormatDebug(f *IndexDebugFormatter) error {
	mapWriter := newIndexDebugMap(f)
	it := view.slice.IterEnumerated()
	for {
		key, value, ok := it.Next()
		if !ok {
			break
		}
		if !value.Present {
			continue
		}
		mapWriter.entry(func(f *IndexDebugFormatter) error {
			if view.formatKeyRef != nil {
				return view.formatKeyRef(&key, f)
			}
			if view.formatKey != nil {
				return f.WriteString(view.formatKey(key))
			}
			return (ValueOps[I]{}).formatAt(&key, f)
		}, func(f *IndexDebugFormatter) error { return view.ops.formatAt(&value.Value, f) })
	}
	return mapWriter.finish()
}

// formatIndexOption follows Option's derived Debug tuple builder, including its
// separate name/'('/value/')' writes and nested pretty indentation.
func formatIndexOption[T any](value *Option[T], inner ValueOps[T], f *IndexDebugFormatter) error {
	if !value.Present {
		return f.WriteString("None")
	}
	if err := f.WriteString("Some"); err != nil {
		return err
	}
	if f.Options.Alternate {
		if err := f.WriteString("(\n"); err != nil {
			return err
		}
		newline := true
		child := indexPadFormatter(f, &newline)
		if err := inner.formatAt(&value.Value, &child); err != nil {
			return err
		}
		if err := child.WriteString(",\n"); err != nil {
			return err
		}
	} else {
		if err := f.WriteString("("); err != nil {
			return err
		}
		if err := inner.formatAt(&value.Value, f); err != nil {
			return err
		}
	}
	return f.WriteString(")")
}

// FormatDebugInt and FormatDebugUint are the standard integer Debug witnesses.
// bits is the source integer width (8/16/32/64, or host usize width); signed
// hexadecimal Debug formats the two's complement representation of that width.
func FormatDebugInt(value int64, bits uint, f *IndexDebugFormatter) error {
	if bits == 0 || bits > 64 {
		panic("rustcindex: invalid integer Debug width")
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
func FormatDebugUint(value uint64, bits uint, f *IndexDebugFormatter) error {
	if bits == 0 || bits > 64 {
		panic("rustcindex: invalid integer Debug width")
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
func (f *IndexDebugFormatter) fill() rune { return f.Options.Fill }

func (f *IndexDebugFormatter) padding(n uint, align IndexFormatAlignment, fill rune) (uint, error) {
	left := n
	if align == IndexAlignLeft {
		left = 0
	} else if align == IndexAlignCenter {
		left = n / 2
	}
	for i := uint(0); i < left; i++ {
		if err := f.WriteRune(fill); err != nil {
			return 0, err
		}
	}
	return n - left, nil
}
func (f *IndexDebugFormatter) postPadding(n uint, fill rune) error {
	for i := uint(0); i < n; i++ {
		if err := f.WriteRune(fill); err != nil {
			return err
		}
	}
	return nil
}
func (f *IndexDebugFormatter) padIntegral(nonnegative bool, prefix, digits string) error {
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
			if err := f.WriteRune(rune(sign[0])); err != nil {
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
	if align == IndexAlignNone {
		align = IndexAlignRight
	}
	if f.Options.ZeroPad {
		old := f.Options
		f.Options.Fill, f.Options.Align = '0', IndexAlignRight
		if err := writePrefix(); err != nil {
			return err
		}
		post, err := f.padding(minimum-width, IndexAlignRight, '0')
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
func (f *IndexDebugFormatter) PadString(text string) error {
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
	if align == IndexAlignNone {
		align = IndexAlignLeft
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
