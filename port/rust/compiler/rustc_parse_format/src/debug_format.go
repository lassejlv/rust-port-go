// SPDX-License-Identifier: MIT OR Apache-2.0
// The nine derived Debug implementations from the pinned rustc_parse_format
// source, including their delegated std formatting and fallible writer calls.
package parseformat

import (
	"io"
	"strconv"
	"strings"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// DebugFormatter retains the mutable source Formatter and its caller-owned
// writer. Compact fields share this pointer; pretty fields receive copies.
type DebugFormatter = rustfmt.Formatter
type DebugOptions = rustfmt.Options
type DebugContext = rustfmt.Context

func DefaultDebugOptions() DebugOptions { return rustfmt.DefaultOptions() }

func debugField(f *DebugFormatter, name string, show func(*DebugFormatter) error) rustfmt.Field {
	return rustfmt.Field{Name: name, Value: f.Value(f.Options.Alternate, show)}
}
func debugTuple(f *DebugFormatter, name string, fields ...func(*DebugFormatter) error) error {
	values := make([]rustfmt.Value, len(fields))
	for i, show := range fields {
		values[i] = f.Value(f.Options.Alternate, show)
	}
	return rustfmt.WriteTuple(f.Writer, name, f.Options.Alternate, values)
}
func debugOption[T any](value Option[T], f *DebugFormatter, show func(T, *DebugFormatter) error) error {
	if !value.Present {
		return f.WriteString("None")
	}
	return debugTuple(f, "Some", func(child *DebugFormatter) error { return show(value.Value, child) })
}
func debugUint(value uint, f *DebugFormatter) error {
	return rustfmt.FormatDebugUint(uint64(value), strconv.IntSize, f)
}
func debugRange(value Range, f *DebugFormatter) error {
	if err := debugUint(value.Start, f); err != nil {
		return err
	}
	if err := f.WriteString(".."); err != nil {
		return err
	}
	return debugUint(value.End, f)
}
func debugRune(value rune) string {
	return rustfmt.String(func(f *DebugFormatter) error { return rustfmt.FormatDebugChar(value, f) })
}
func debugString(value string) string {
	return rustfmt.String(func(f *DebugFormatter) error { return rustfmt.FormatDebugString(value, f) })
}

func (value ParseMode) WriteDebugFormatter(f *DebugFormatter) error {
	switch value {
	case Format:
		return f.WriteString("Format")
	case InlineAsm:
		return f.WriteString("InlineAsm")
	case Diagnostic:
		return f.WriteString("Diagnostic")
	default:
		panic("parseformat: invalid parse mode")
	}
}
func (value Alignment) WriteDebugFormatter(f *DebugFormatter) error {
	switch value {
	case AlignUnknown:
		return f.WriteString("AlignUnknown")
	case AlignLeft:
		return f.WriteString("AlignLeft")
	case AlignRight:
		return f.WriteString("AlignRight")
	case AlignCenter:
		return f.WriteString("AlignCenter")
	default:
		panic("parseformat: invalid alignment")
	}
}
func (value Sign) WriteDebugFormatter(f *DebugFormatter) error {
	switch value {
	case Plus:
		return f.WriteString("Plus")
	case Minus:
		return f.WriteString("Minus")
	default:
		panic("parseformat: invalid sign")
	}
}
func (value DebugHex) WriteDebugFormatter(f *DebugFormatter) error {
	switch value {
	case Lower:
		return f.WriteString("Lower")
	case Upper:
		return f.WriteString("Upper")
	default:
		panic("parseformat: invalid debug hex")
	}
}
func (value Position) WriteDebugFormatter(f *DebugFormatter) error {
	switch value.Kind {
	case ArgumentImplicitlyIs, ArgumentIs:
		name := "ArgumentImplicitlyIs"
		if value.Kind == ArgumentIs {
			name = "ArgumentIs"
		}
		return debugTuple(f, name, func(child *DebugFormatter) error { return debugUint(value.Index, child) })
	case ArgumentNamed:
		return debugTuple(f, "ArgumentNamed", func(child *DebugFormatter) error { return rustfmt.FormatDebugString(value.Name, child) })
	default:
		panic("parseformat: invalid position")
	}
}
func (value Count) WriteDebugFormatter(f *DebugFormatter) error {
	switch value.Kind {
	case CountIs:
		return debugTuple(f, "CountIs", func(child *DebugFormatter) error { return rustfmt.FormatDebugUint(uint64(value.Value), 16, child) })
	case CountIsName:
		return debugTuple(f, "CountIsName", func(child *DebugFormatter) error { return rustfmt.FormatDebugString(value.Name, child) }, func(child *DebugFormatter) error { return debugRange(value.Span, child) })
	case CountIsParam, CountIsStar:
		name := "CountIsParam"
		if value.Kind == CountIsStar {
			name = "CountIsStar"
		}
		return debugTuple(f, name, func(child *DebugFormatter) error { return debugUint(value.Index, child) })
	case CountImplied:
		return f.WriteString("CountImplied")
	default:
		panic("parseformat: invalid count")
	}
}
func (value FormatSpec) WriteDebugFormatter(f *DebugFormatter) error {
	return rustfmt.WriteStruct(f.Writer, "FormatSpec", f.Options.Alternate, []rustfmt.Field{
		debugField(f, "fill", func(child *DebugFormatter) error { return debugOption(value.Fill, child, rustfmt.FormatDebugChar) }),
		debugField(f, "fill_span", func(child *DebugFormatter) error { return debugOption(value.FillSpan, child, debugRange) }),
		debugField(f, "align", value.Align.WriteDebugFormatter),
		debugField(f, "sign", func(child *DebugFormatter) error {
			return debugOption(value.Sign, child, func(v Sign, f *DebugFormatter) error { return v.WriteDebugFormatter(f) })
		}),
		debugField(f, "alternate", func(child *DebugFormatter) error { return rustfmt.FormatDebugBool(value.Alternate, child) }),
		debugField(f, "zero_pad", func(child *DebugFormatter) error { return rustfmt.FormatDebugBool(value.ZeroPad, child) }),
		debugField(f, "debug_hex", func(child *DebugFormatter) error {
			return debugOption(value.DebugHex, child, func(v DebugHex, f *DebugFormatter) error { return v.WriteDebugFormatter(f) })
		}),
		debugField(f, "precision", value.Precision.WriteDebugFormatter),
		debugField(f, "precision_span", func(child *DebugFormatter) error { return debugOption(value.PrecisionSpan, child, debugRange) }),
		debugField(f, "width", value.Width.WriteDebugFormatter),
		debugField(f, "width_span", func(child *DebugFormatter) error { return debugOption(value.WidthSpan, child, debugRange) }),
		debugField(f, "ty", func(child *DebugFormatter) error { return rustfmt.FormatDebugString(value.Ty, child) }),
		debugField(f, "ty_span", func(child *DebugFormatter) error { return debugOption(value.TySpan, child, debugRange) }),
	})
}
func (value Argument) WriteDebugFormatter(f *DebugFormatter) error {
	return rustfmt.WriteStruct(f.Writer, "Argument", f.Options.Alternate, []rustfmt.Field{
		debugField(f, "position", value.Position.WriteDebugFormatter),
		debugField(f, "position_span", func(child *DebugFormatter) error { return debugRange(value.PositionSpan, child) }),
		debugField(f, "format", value.Format.WriteDebugFormatter),
	})
}
func (value Piece) WriteDebugFormatter(f *DebugFormatter) error {
	switch value.Kind {
	case Lit:
		return debugTuple(f, "Lit", func(child *DebugFormatter) error { return rustfmt.FormatDebugString(value.Text, child) })
	case NextArgument:
		if value.Argument != nil {
			return debugTuple(f, "NextArgument", value.Argument.WriteDebugFormatter)
		}
	}
	panic("parseformat: invalid piece")
}

func debugWithOptions(show func(*DebugFormatter) error, options DebugOptions) string {
	var output strings.Builder
	f := DebugFormatter{Options: options, Writer: &output}
	if err := show(&f); err != nil {
		f.Context.Panic("parseformat: Debug returned an error for a string sink")
	}
	return output.String()
}

func (value ParseMode) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value ParseMode) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value ParseMode) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Piece) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Piece) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Piece) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Argument) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Argument) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Argument) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value FormatSpec) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value FormatSpec) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value FormatSpec) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Position) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Position) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Position) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Alignment) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Alignment) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Alignment) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Sign) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Sign) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Sign) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value DebugHex) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value DebugHex) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value DebugHex) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}

func (value Count) WriteDebug(writer io.Writer, options DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: writer, Options: options})
}
func (value Count) RustDebugWith(options DebugOptions) string {
	return debugWithOptions(value.WriteDebugFormatter, options)
}
func (value Count) RustDebug() string {
	return value.RustDebugWith(DefaultDebugOptions())
}
