// SPDX-License-Identifier: MIT OR Apache-2.0
// Streaming formatting for restricted_names.rs and its exact thiserror 2.0.18
// expansion. Supporting std formatting helpers receive no source-file credit.
package cargoutilschemas

import (
	"fmt"
	"io"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func nameFormatString(format func(*rustfmt.Formatter) error) string { return rustfmt.String(format) }

// WriteDisplayFormatter preserves thiserror's transparent delegation. The
// inner error's write! format uses fresh default options for each argument;
// outer width, precision, fill and flags do not pad or truncate the message.
func (e *NameValidationError) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	return e.kind.WriteDisplayFormatter(f)
}

func (e errorKind) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	text := f.WriteString
	switch e.tag {
	case emptyName:
		if err := text(e.what); err != nil {
			return err
		}
		return text(" cannot be empty")
	case invalidCharacter:
		if err := text("invalid character `"); err != nil {
			return err
		}
		if err := f.WriteChar(e.ch); err != nil {
			return err
		}
		if err := text("` in "); err != nil {
			return err
		}
		if err := text(e.what); err != nil {
			return err
		}
		if err := text(": `"); err != nil {
			return err
		}
		if err := text(e.name); err != nil {
			return err
		}
		if err := text("`, "); err != nil {
			return err
		}
		return text(e.reason)
	case profileReserved:
		if err := text("profile name `"); err != nil {
			return err
		}
		if err := text(e.name); err != nil {
			return err
		}
		if err := text("` is reserved\n"); err != nil {
			return err
		}
		if err := text(e.help); err != nil {
			return err
		}
		return text("\nSee https://doc.rust-lang.org/cargo/reference/profiles.html for more on configuring profiles.")
	case featureDepColon:
		if err := text("feature named `"); err != nil {
			return err
		}
		if err := text(e.name); err != nil {
			return err
		}
		return text("` is not allowed to start with `dep:`")
	default:
		f.Context.Panic("restricted_names: invalid private error kind")
		return nil
	}
}

// WriteDebugFormatter uses the actual nested derived tuple/struct shape.
// Compact fields share the formatter; pretty fields inherit a copied child.
func (e *NameValidationError) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	builder := rustfmt.BeginTuple(f.Writer, "NameValidationError", pretty)
	builder.Field(f.Value(pretty, e.kind.WriteDebugFormatter))
	return builder.Finish()
}

func (e errorKind) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	str := func(value string) rustfmt.Value {
		return f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugString(value, child) })
	}
	switch e.tag {
	case emptyName:
		b := rustfmt.BeginTuple(f.Writer, "Empty", pretty)
		b.Field(str(e.what))
		return b.Finish()
	case invalidCharacter:
		b := rustfmt.BeginStruct(f.Writer, "InvalidCharacter", pretty)
		b.Field("ch", f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugChar(e.ch, child) }))
		b.Field("what", str(e.what))
		b.Field("name", str(e.name))
		b.Field("reason", str(e.reason))
		return b.Finish()
	case profileReserved:
		b := rustfmt.BeginStruct(f.Writer, "ProfileNameReservedKeyword", pretty)
		b.Field("name", str(e.name))
		b.Field("help", str(e.help))
		return b.Finish()
	case featureDepColon:
		b := rustfmt.BeginTuple(f.Writer, "FeatureNameStartsWithDepColon", pretty)
		b.Field(str(e.name))
		return b.Finish()
	default:
		f.Context.Panic("restricted_names: invalid private error kind")
		return nil
	}
}

func (e *NameValidationError) WriteDisplay(w io.Writer, options rustfmt.Options) error {
	return e.WriteDisplayFormatter(&rustfmt.Formatter{Writer: w, Options: options})
}
func (e *NameValidationError) WriteDebug(w io.Writer, options rustfmt.Options) error {
	return e.WriteDebugFormatter(&rustfmt.Formatter{Writer: w, Options: options})
}

// Format is a Go convenience bridge. Display ignores outer Go padding and
// precision as the source does; %#v selects compact Rust Debug. The primary
// formatter API carries all source flags and fallible writer behavior.
func (e *NameValidationError) Format(state fmt.State, verb rune) {
	f := &rustfmt.Formatter{Writer: state, Options: rustfmt.DefaultOptions()}
	if verb == 'v' && state.Flag('#') {
		_ = e.WriteDebugFormatter(f)
		return
	}
	_ = e.WriteDisplayFormatter(f)
}
