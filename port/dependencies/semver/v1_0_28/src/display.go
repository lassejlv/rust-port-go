// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/display.rs. Only Version Display pads; the other
// Display implementations deliberately ignore width/alignment/precision flags.
package semver

import (
	"fmt"
	"io"
	"strconv"
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

type FormatOptions struct {
	Width Option[uint]
	Align Alignment
	Fill  Option[rune]
}
type DebugNumberFormat uint8

const (
	DebugDecimal DebugNumberFormat = iota
	DebugLowerHex
	DebugUpperHex
)

type DebugOptions struct {
	Pretty    bool
	Numbers   DebugNumberFormat
	Width     Option[uint]
	Precision Option[uint]
	Fill      Option[rune]
	Align     Alignment
	SignPlus  bool
	SignMinus bool
	ZeroPad   bool
}

// Display is the source Display trait exposed to serde's collect_str adapter.
type Display interface {
	String() string
	WriteDisplay(io.Writer, FormatOptions) error
}

func writeDisplay(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

// writeChar preserves fmt::Write::write_char independently from write_str.
// A writer can override it with WriteChar(rune) error; the default UTF-8
// encoding delegates to the string write as Rust's default implementation does.
func writeChar(writer io.Writer, char rune) error {
	if !utf8.ValidRune(char) {
		panic("semver: writer character must be a Unicode scalar")
	}
	if target, ok := writer.(interface{ WriteChar(rune) error }); ok {
		return target.WriteChar(char)
	}
	return writeDisplay(writer, string(char))
}
func rawVersion(w io.Writer, value Version) error {
	for _, text := range []string{strconv.FormatUint(value.Major, 10), ".", strconv.FormatUint(value.Minor, 10), ".", strconv.FormatUint(value.Patch, 10)} {
		if err := writeDisplay(w, text); err != nil {
			return err
		}
	}
	if !value.Pre.IsEmpty() {
		if err := writeDisplay(w, "-"); err != nil {
			return err
		}
		if err := writeDisplay(w, value.Pre.AsStr()); err != nil {
			return err
		}
	}
	if !value.Build.IsEmpty() {
		if err := writeDisplay(w, "+"); err != nil {
			return err
		}
		if err := writeDisplay(w, value.Build.AsStr()); err != nil {
			return err
		}
	}
	return nil
}
func (value Version) WriteDisplay(w io.Writer, options FormatOptions) error {
	padding := uint(0)
	if options.Width.Present {
		length := digits(value.Major) + 1 + digits(value.Minor) + 1 + digits(value.Patch) + value.Pre.Len() + value.Build.Len()
		if !value.Pre.IsEmpty() {
			length++
		}
		if !value.Build.IsEmpty() {
			length++
		}
		if options.Width.Value > uint(length) {
			padding = options.Width.Value - uint(length)
		}
	}
	if padding == 0 {
		return rawVersion(w, value)
	}
	var before, after uint
	switch options.Align {
	case AlignDefault, AlignLeft:
		after = padding
	case AlignRight:
		before = padding
	case AlignCenter:
		before = padding / 2
		after = (padding + 1) / 2
	default:
		panic("semver: invalid alignment")
	}
	fill := ' '
	if options.Fill.Present {
		fill = options.Fill.Value
		if !utf8.ValidRune(fill) {
			panic("semver: format fill must be a Unicode scalar")
		}
	}
	for count := uint(0); count < before; count++ {
		if err := writeChar(w, fill); err != nil {
			return err
		}
	}
	if err := rawVersion(w, value); err != nil {
		return err
	}
	for count := uint(0); count < after; count++ {
		if err := writeChar(w, fill); err != nil {
			return err
		}
	}
	return nil
}
func (value VersionReq) WriteDisplay(w io.Writer, _ FormatOptions) error {
	if len(value.Comparators) == 0 {
		return writeDisplay(w, "*")
	}
	for index, comparator := range value.Comparators {
		if index > 0 {
			if err := writeDisplay(w, ", "); err != nil {
				return err
			}
		}
		if err := comparator.WriteDisplay(w, FormatOptions{}); err != nil {
			return err
		}
	}
	return nil
}
func (value Comparator) WriteDisplay(w io.Writer, _ FormatOptions) error {
	value.Op.check()
	op := [...]string{"=", ">", ">=", "<", "<=", "~", "^", ""}[value.Op]
	if err := writeDisplay(w, op); err != nil {
		return err
	}
	if err := writeDisplay(w, strconv.FormatUint(value.Major, 10)); err != nil {
		return err
	}
	if value.Minor.Present {
		if err := writeDisplay(w, "."); err != nil {
			return err
		}
		if err := writeDisplay(w, strconv.FormatUint(value.Minor.Value, 10)); err != nil {
			return err
		}
		if value.Patch.Present {
			if err := writeDisplay(w, "."); err != nil {
				return err
			}
			if err := writeDisplay(w, strconv.FormatUint(value.Patch.Value, 10)); err != nil {
				return err
			}
			if !value.Pre.IsEmpty() {
				if err := writeDisplay(w, "-"); err != nil {
					return err
				}
				if err := writeDisplay(w, value.Pre.AsStr()); err != nil {
					return err
				}
			}
		} else if value.Op == Wildcard {
			return writeDisplay(w, ".*")
		}
	} else if value.Op == Wildcard {
		return writeDisplay(w, ".*")
	}
	return nil
}
func (value Prerelease) WriteDisplay(w io.Writer, _ FormatOptions) error {
	return writeDisplay(w, value.AsStr())
}
func (value BuildMetadata) WriteDisplay(w io.Writer, _ FormatOptions) error {
	return writeDisplay(w, value.AsStr())
}

func (value Version) String() string {
	var result strings.Builder
	_ = value.WriteDisplay(&result, FormatOptions{})
	return result.String()
}
func (value VersionReq) String() string {
	var result strings.Builder
	_ = value.WriteDisplay(&result, FormatOptions{})
	return result.String()
}
func (value Comparator) String() string {
	var result strings.Builder
	_ = value.WriteDisplay(&result, FormatOptions{})
	return result.String()
}
func (value Prerelease) String() string    { return value.AsStr() }
func (value BuildMetadata) String() string { return value.AsStr() }
func (value *Error) String() string        { return value.Error() }
func digits(value uint64) int {
	if value < 10 {
		return 1
	}
	return 1 + digits(value/10)
}

// Go formatting maps string/value verbs to Display and %#v to compact Rust
// Debug. Rust's arbitrary fill/center options are represented by WriteDisplay.
func formatValue(state fmt.State, verb rune, value Display, debug func() string) {
	if verb == 'v' && state.Flag('#') && debug != nil {
		_, _ = io.WriteString(state, debug())
		return
	}
	options := FormatOptions{}
	if width, ok := state.Width(); ok {
		options.Width = Some(uint(width))
	}
	_ = value.WriteDisplay(state, options)
}
func (value Version) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
func (value VersionReq) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
func (value Comparator) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
func (value Prerelease) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
func (value BuildMetadata) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
func (value *Error) Format(state fmt.State, verb rune) {
	formatValue(state, verb, value, value.RustDebug)
}
