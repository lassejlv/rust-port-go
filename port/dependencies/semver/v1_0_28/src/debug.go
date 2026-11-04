// SPDX-License-Identifier: MIT OR Apache-2.0
// The Debug portions of semver src/display.rs and the scoped std debug-builder
// contract they invoke. All writes are fallible and preserve source boundaries.
package semver

import (
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type DebugFunc func(io.Writer, DebugOptions) error
type DebugField struct {
	Name  string
	Value DebugFunc
}

// DebugFormatter carries caller-visible formatting state as well as its sink.
// Compact builders share this pointer. Pretty builders isolate a copy, matching
// core::fmt::Debug builders. Numeric zero padding restores changed fill/align
// only on success; a writer error retains the source formatter mutation.
type DebugFormatter struct {
	Writer  io.Writer
	Options DebugOptions
}
type DebugFormatterFunc func(*DebugFormatter) error
type DebugFormatterField struct {
	Name  string
	Value DebugFormatterFunc
}

func WriteDebugStruct(w io.Writer, name string, fields []DebugField, o DebugOptions) error {
	adapted := make([]DebugFormatterField, len(fields))
	for i, field := range fields {
		adapted[i] = DebugFormatterField{field.Name, func(f *DebugFormatter) error { return field.Value(f.Writer, f.Options) }}
	}
	return WriteDebugStructFormatter(&DebugFormatter{Writer: w, Options: o}, name, adapted)
}
func WriteDebugTuple(w io.Writer, name string, fields []DebugFunc, o DebugOptions) error {
	adapted := make([]DebugFormatterFunc, len(fields))
	for i, field := range fields {
		adapted[i] = func(f *DebugFormatter) error { return field(f.Writer, f.Options) }
	}
	return WriteDebugTupleFormatter(&DebugFormatter{Writer: w, Options: o}, name, adapted)
}
func WriteDebugList(w io.Writer, values []DebugFunc, o DebugOptions) error {
	adapted := make([]DebugFormatterFunc, len(values))
	for i, value := range values {
		adapted[i] = func(f *DebugFormatter) error { return value(f.Writer, f.Options) }
	}
	return WriteDebugListFormatter(&DebugFormatter{Writer: w, Options: o}, adapted)
}

// adaptDebugValue keeps source-package formatter state out of the neutral
// builders: compact callbacks share the caller object, while pretty callbacks
// receive a copied option set and the supplied indentation writer.
func adaptDebugValue(formatter *DebugFormatter, pretty bool, value DebugFormatterFunc) rustfmt.Value {
	return func(writer io.Writer) error {
		if pretty {
			return value(&DebugFormatter{Writer: writer, Options: formatter.Options})
		}
		return value(formatter)
	}
}

func WriteDebugStructFormatter(formatter *DebugFormatter, name string, fields []DebugFormatterField) error {
	pretty := formatter.Options.Pretty
	adapted := make([]rustfmt.Field, len(fields))
	for i, field := range fields {
		adapted[i] = rustfmt.Field{Name: field.Name, Value: adaptDebugValue(formatter, pretty, field.Value)}
	}
	return rustfmt.WriteStruct(formatter.Writer, name, pretty, adapted)
}
func WriteDebugTupleFormatter(formatter *DebugFormatter, name string, fields []DebugFormatterFunc) error {
	pretty := formatter.Options.Pretty
	adapted := make([]rustfmt.Value, len(fields))
	for i, field := range fields {
		adapted[i] = adaptDebugValue(formatter, pretty, field)
	}
	return rustfmt.WriteTuple(formatter.Writer, name, pretty, adapted)
}
func WriteDebugListFormatter(formatter *DebugFormatter, values []DebugFormatterFunc) error {
	pretty := formatter.Options.Pretty
	adapted := make([]rustfmt.Value, len(values))
	for i, value := range values {
		adapted[i] = adaptDebugValue(formatter, pretty, value)
	}
	return rustfmt.WriteList(formatter.Writer, pretty, adapted)
}

func writeDebugUintFormatter(formatter *DebugFormatter, value uint64) error {
	writer, options := formatter.Writer, formatter.Options
	base := 10
	if options.Numbers == DebugLowerHex || options.Numbers == DebugUpperHex {
		base = 16
	} else if options.Numbers != DebugDecimal {
		panic("semver: invalid debug number format")
	}
	text := strconv.FormatUint(value, base)
	if options.Numbers == DebugUpperHex {
		text = strings.ToUpper(text)
	}
	sign := ""
	if options.SignPlus {
		sign = "+"
	}
	prefix := ""
	if base == 16 && options.Pretty {
		prefix = "0x"
	}
	length := uint(len(sign) + len(prefix) + len(text))
	var padding uint
	if options.Width.Present && options.Width.Value > length {
		padding = options.Width.Value - length
	}
	fill := ' '
	if options.Fill.Present {
		fill = options.Fill.Value
		if !utf8.ValidRune(fill) {
			panic("semver: debug fill must be a Unicode scalar")
		}
	}
	before, after := uint(0), uint(0)
	if options.ZeroPad && padding > 0 {
		formatter.Options.Fill = Some('0')
		formatter.Options.Align = AlignRight
		fill = '0'
		before = padding
	} else {
		switch options.Align {
		case AlignDefault, AlignRight:
			before = padding
		case AlignLeft:
			after = padding
		case AlignCenter:
			before = padding / 2
			after = (padding + 1) / 2
		default:
			panic("semver: invalid debug alignment")
		}
	}
	writeSign := func() error {
		if sign != "" {
			if err := writeChar(writer, '+'); err != nil {
				return err
			}
		}
		if options.Pretty {
			if err := writeDisplay(writer, prefix); err != nil {
				return err
			}
		}
		return nil
	}
	if options.ZeroPad {
		if err := writeSign(); err != nil {
			return err
		}
	}
	for count := uint(0); count < before; count++ {
		if err := writeChar(writer, fill); err != nil {
			return err
		}
	}
	if !options.ZeroPad {
		if err := writeSign(); err != nil {
			return err
		}
	}
	if err := writeDisplay(writer, text); err != nil {
		return err
	}
	for count := uint(0); count < after; count++ {
		if err := writeChar(writer, fill); err != nil {
			return err
		}
	}
	formatter.Options = options
	return nil
}
func writeDebugOptionFormatter(formatter *DebugFormatter, value Option[uint64]) error {
	if !value.Present {
		return writeDisplay(formatter.Writer, "None")
	}
	return WriteDebugTupleFormatter(formatter, "Some", []DebugFormatterFunc{func(f *DebugFormatter) error { return writeDebugUintFormatter(f, value.Value) }})
}
func (value Op) WriteDebugFormatter(f *DebugFormatter) error {
	value.check()
	return writeDisplay(f.Writer, [...]string{"Exact", "Greater", "GreaterEq", "Less", "LessEq", "Tilde", "Caret", "Wildcard"}[value])
}
func (value Prerelease) WriteDebugFormatter(f *DebugFormatter) error {
	if e := writeDisplay(f.Writer, "Prerelease(\""); e != nil {
		return e
	}
	if e := writeDisplay(f.Writer, value.AsStr()); e != nil {
		return e
	}
	return writeDisplay(f.Writer, "\")")
}
func (value BuildMetadata) WriteDebugFormatter(f *DebugFormatter) error {
	if e := writeDisplay(f.Writer, "BuildMetadata(\""); e != nil {
		return e
	}
	if e := writeDisplay(f.Writer, value.AsStr()); e != nil {
		return e
	}
	return writeDisplay(f.Writer, "\")")
}
func (value Version) WriteDebugFormatter(f *DebugFormatter) error {
	fields := []DebugFormatterField{{"major", func(f *DebugFormatter) error { return writeDebugUintFormatter(f, value.Major) }}, {"minor", func(f *DebugFormatter) error { return writeDebugUintFormatter(f, value.Minor) }}, {"patch", func(f *DebugFormatter) error { return writeDebugUintFormatter(f, value.Patch) }}}
	if !value.Pre.IsEmpty() {
		fields = append(fields, DebugFormatterField{"pre", value.Pre.WriteDebugFormatter})
	}
	if !value.Build.IsEmpty() {
		fields = append(fields, DebugFormatterField{"build", value.Build.WriteDebugFormatter})
	}
	return WriteDebugStructFormatter(f, "Version", fields)
}
func (value Comparator) WriteDebugFormatter(f *DebugFormatter) error {
	return WriteDebugStructFormatter(f, "Comparator", []DebugFormatterField{{"op", value.Op.WriteDebugFormatter}, {"major", func(f *DebugFormatter) error { return writeDebugUintFormatter(f, value.Major) }}, {"minor", func(f *DebugFormatter) error { return writeDebugOptionFormatter(f, value.Minor) }}, {"patch", func(f *DebugFormatter) error { return writeDebugOptionFormatter(f, value.Patch) }}, {"pre", value.Pre.WriteDebugFormatter}})
}
func (value VersionReq) WriteDebugFormatter(f *DebugFormatter) error {
	return WriteDebugStructFormatter(f, "VersionReq", []DebugFormatterField{{"comparators", func(f *DebugFormatter) error {
		values := make([]DebugFormatterFunc, len(value.Comparators))
		for i, c := range value.Comparators {
			values[i] = c.WriteDebugFormatter
		}
		return WriteDebugListFormatter(f, values)
	}}})
}
func (value *Error) WriteDebugFormatter(f *DebugFormatter) error {
	if e := writeDisplay(f.Writer, "Error(\""); e != nil {
		return e
	}
	if e := value.WriteDisplay(f.Writer, FormatOptions{}); e != nil {
		return e
	}
	return writeDisplay(f.Writer, "\")")
}
func (value Op) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value Version) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value VersionReq) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value Comparator) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value Prerelease) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value BuildMetadata) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}
func (value *Error) WriteDebug(w io.Writer, o DebugOptions) error {
	return value.WriteDebugFormatter(&DebugFormatter{Writer: w, Options: o})
}

type debugValue interface {
	WriteDebug(io.Writer, DebugOptions) error
}

func debugString(value debugValue, options DebugOptions) string {
	var result strings.Builder
	_ = value.WriteDebug(&result, options)
	return result.String()
}
func (value Op) RustDebug() string                              { return debugString(value, DebugOptions{}) }
func (value Version) RustDebug() string                         { return debugString(value, DebugOptions{}) }
func (value Comparator) RustDebug() string                      { return debugString(value, DebugOptions{}) }
func (value VersionReq) RustDebug() string                      { return debugString(value, DebugOptions{}) }
func (value Prerelease) RustDebug() string                      { return debugString(value, DebugOptions{}) }
func (value BuildMetadata) RustDebug() string                   { return debugString(value, DebugOptions{}) }
func (value *Error) RustDebug() string                          { return debugString(value, DebugOptions{}) }
func (value Version) RustDebugWith(options DebugOptions) string { return debugString(value, options) }
func (value Comparator) RustDebugWith(options DebugOptions) string {
	return debugString(value, options)
}
func (value VersionReq) RustDebugWith(options DebugOptions) string {
	return debugString(value, options)
}
