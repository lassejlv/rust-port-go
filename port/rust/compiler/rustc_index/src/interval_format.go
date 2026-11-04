// SPDX-License-Identifier: MIT OR Apache-2.0
// Derived Debug bridge of pinned interval.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The shared scoped builders grant no
// Rust standard-library file completion or layout ABI credit.
package rustcindex

import (
	"io"
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// intervalFormatWriter preserves the index formatter's independently
// overridable write_str/write_char witnesses through shared pretty builders.
type intervalFormatWriter struct{ formatter *IndexDebugFormatter }

func (w intervalFormatWriter) Write(p []byte) (int, error) { return w.WriteString(string(p)) }
func (w intervalFormatWriter) WriteString(text string) (int, error) {
	if err := w.formatter.WriteString(text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (w intervalFormatWriter) WriteChar(c rune) error { return w.formatter.WriteRune(c) }
func intervalFormatValue(f *IndexDebugFormatter, pretty bool, format func(*IndexDebugFormatter) error) rustfmt.Value {
	return func(writer io.Writer) error {
		if !pretty {
			return format(f)
		}
		child := *f
		child.Write = func(s string) error { return rustfmt.WriteString(writer, s) }
		child.WriteChar = func(c rune) error { return rustfmt.WriteChar(writer, c) }
		return format(&child)
	}
}

func formatIntervalPair(pair *intervalPair, f *IndexDebugFormatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(intervalFormatWriter{f}, "", pretty)
	b.Field(intervalFormatValue(f, pretty, func(f *IndexDebugFormatter) error { return FormatDebugUint(uint64(pair.start), 32, f) }))
	b.Field(intervalFormatValue(f, pretty, func(f *IndexDebugFormatter) error { return FormatDebugUint(uint64(pair.end), 32, f) }))
	return b.Finish()
}

// FormatDebug is the complete derived struct Debug: SmallVec's pair list,
// source-width domain and PhantomData type name. TypeName is the exact source
// type_name::<I>() witness; primitive step factories supply "usize"/"u32".
// A custom key must supply its canonical Rust name rather than a Go spelling.
func (set *IntervalSet[I]) FormatDebug(f *IndexDebugFormatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(intervalFormatWriter{f}, "IntervalSet", pretty)
	b.Field("map", intervalFormatValue(f, pretty, func(f *IndexDebugFormatter) error {
		ops := ValueOps[intervalPair]{FormatDebug: formatIntervalPair}
		return rawFromSlice(set.intervals, ops).FormatDebug(ops, f)
	}))
	b.Field("domain", intervalFormatValue(f, pretty, func(f *IndexDebugFormatter) error {
		return FormatDebugUint(uint64(set.domain), uint(strconv.IntSize), f)
	}))
	b.Field("_data", intervalFormatValue(f, pretty, func(f *IndexDebugFormatter) error {
		if set.step.TypeName == "" {
			f.Context.Panic("rustcindex: source PhantomData type-name witness required")
		}
		if err := f.WriteString("PhantomData<"); err != nil {
			return err
		}
		// The inner format_args displays the type-name string with default
		// options, so caller width/precision and debug radix do not affect it.
		if err := f.WriteString(set.step.TypeName); err != nil {
			return err
		}
		return f.WriteString(">")
	}))
	return b.Finish()
}
func (set IntervalSet[I]) Debug() string {
	return indexFormatString(set.FormatDebug, IndexFormatContext{})
}
func (set IntervalSet[I]) String() string { return set.Debug() }
