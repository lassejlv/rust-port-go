// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped owned std io::Error contracts used by source_map.rs and lib.rs.
package rustcspan

import (
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// NewSourceIOErrorKind models io::Error::from(ErrorKind), whose Debug is
// Kind(Variant) and whose Display ignores outer padding options. The kind/name
// pair is a witness for one actual source ErrorKind; arbitrary extra names are
// outside that Rust enum contract.
func NewSourceIOErrorKind(kind, message string) *SourceMapIOError {
	requireSpanUTF8(kind, rustfmt.Context{})
	requireSpanUTF8(message, rustfmt.Context{})
	type state struct{ kind, message string }
	return NewSourceIOError(state{kind, message}, SourceIOErrorOps[state]{
		Display: func(value *state, f *rustfmt.Formatter) error { return f.WriteString(value.message) },
		Debug: func(value *state, f *rustfmt.Formatter) error {
			pretty := f.Options.Alternate
			b := rustfmt.BeginTuple(f.Writer, "Kind", pretty)
			b.Field(f.Value(pretty, func(f *rustfmt.Formatter) error { return f.WriteString(value.kind) }))
			return b.Finish()
		},
	})
}

func NewSourceIOErrorSimpleMessage(kind, message string) *SourceMapIOError {
	requireSpanUTF8(kind, rustfmt.Context{})
	requireSpanUTF8(message, rustfmt.Context{})
	type state struct{ kind, message string }
	return NewSourceIOError(state{kind, message}, SourceIOErrorOps[state]{Display: func(value *state, f *rustfmt.Formatter) error { return f.WriteString(value.message) }, Debug: func(value *state, f *rustfmt.Formatter) error {
		pretty := f.Options.Alternate
		b := rustfmt.BeginStruct(f.Writer, "Error", pretty)
		b.Field("kind", f.Value(pretty, func(f *rustfmt.Formatter) error { return f.WriteString(value.kind) }))
		b.Field("message", f.Value(pretty, func(f *rustfmt.Formatter) error { return FormatDebugStringPinnedUnicode18(value.message, f) }))
		return b.Finish()
	}})
}
func NewSourceIOErrorOS(code int32, kind, message string) *SourceMapIOError {
	requireSpanUTF8(message, rustfmt.Context{})
	return newSourceIOErrorOSWithFormat(code, kind, func(_ int32, f *rustfmt.Formatter) error { return f.WriteString(message) })
}
func newSourceIOErrorOSWithFormat(code int32, kind string, message func(int32, *rustfmt.Formatter) error) *SourceMapIOError {
	requireSpanUTF8(kind, rustfmt.Context{})
	type state struct {
		code int32
		kind string
	}
	return NewSourceIOError(state{code, kind}, SourceIOErrorOps[state]{
		Display: func(value *state, f *rustfmt.Formatter) error {
			child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
			if err := message(value.code, child); err != nil {
				return err
			}
			if err := f.WriteString(" (os error "); err != nil {
				return err
			}
			if err := rustfmt.FormatDebugInt(int64(value.code), 32, child); err != nil {
				return err
			}
			return f.WriteString(")")
		},
		Debug: func(value *state, f *rustfmt.Formatter) error {
			pretty := f.Options.Alternate
			b := rustfmt.BeginStruct(f.Writer, "Os", pretty)
			b.Field("code", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(int64(value.code), 32, f) }))
			b.Field("kind", f.Value(pretty, func(f *rustfmt.Formatter) error { return f.WriteString(value.kind) }))
			b.Field("message", f.Value(pretty, func(f *rustfmt.Formatter) error {
				// core io::Repr quotes format_os_error directly; it does not
				// apply str::Debug escaping. The nested write! argument has
				// fresh options, including when the outer caller is pretty.
				if err := f.WriteString("\""); err != nil {
					return err
				}
				child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
				if err := message(value.code, child); err != nil {
					return err
				}
				return f.WriteString("\"")
			}))
			return b.Finish()
		},
	})
}

// SourceIOErrorOps is a witness for a complete source io::Error value, not
// merely an arbitrary inner error. Callbacks borrow its actual state cell and
// must include the source variant's formatting/owned-field destruction. This
// supports nontrivial custom Error payloads without inventing a std I/O port.
// Formatting/source callbacks obey Rust shared-borrow laws; Drop borrows the
// actual mutable owning cell. Custom payload providers uphold the original
// Send/Sync/static bounds, and source() borrows cannot outlive their owner.
type SourceIOErrorOps[T any] struct {
	Display           func(*T, *rustfmt.Formatter) error
	Debug             func(*T, *rustfmt.Formatter) error
	Source            func(*T) error
	SourceWithContext func(*T, rustfmt.Context) error
	Drop              func(*T, rustfmt.Context)
}
type SourceMapIOError struct {
	display, debug func(*rustfmt.Formatter) error
	source         func(rustfmt.Context) error
	close          func(rustfmt.Context)
}

func NewSourceIOError[T any](value T, ops SourceIOErrorOps[T]) *SourceMapIOError {
	if ops.Display == nil || ops.Debug == nil {
		panic("source-map: complete io::Error Display/Debug witnesses required")
	}
	owned := NewSpanOwned(value, ops.Drop)
	return &SourceMapIOError{
		display: func(f *rustfmt.Formatter) (err error) {
			spanRunContext(f.Context, func() { err = ops.Display(owned.Borrow(), f) })
			return
		},
		debug: func(f *rustfmt.Formatter) (err error) {
			spanRunContext(f.Context, func() { err = ops.Debug(owned.Borrow(), f) })
			return
		},
		source: func(context rustfmt.Context) (result error) {
			spanRunContext(context, func() {
				if ops.SourceWithContext != nil {
					result = ops.SourceWithContext(owned.Borrow(), context)
				} else if ops.Source != nil {
					result = ops.Source(owned.Borrow())
				}
			})
			return
		},
		close: owned.DropWithContext,
	}
}
func (failure *SourceMapIOError) Error() string {
	return rustfmt.String(failure.WriteSourceIOErrorDisplay)
}
func (failure *SourceMapIOError) Source() error { return failure.SourceWithContext(rustfmt.Context{}) }
func (failure *SourceMapIOError) SourceWithContext(context rustfmt.Context) error {
	return failure.source(context)
}
func (failure *SourceMapIOError) WriteSourceIOErrorDisplay(f *rustfmt.Formatter) error {
	return failure.display(f)
}
func (failure *SourceMapIOError) WriteSourceIOErrorDebug(f *rustfmt.Formatter) error {
	return failure.debug(f)
}
func (failure *SourceMapIOError) CloseWithContext(context rustfmt.Context) { failure.close(context) }
func (failure *SourceMapIOError) Close()                                   { failure.CloseWithContext(rustfmt.Context{}) }

type ownedSourceIOError interface {
	error
	WriteSourceIOErrorDisplay(*rustfmt.Formatter) error
	WriteSourceIOErrorDebug(*rustfmt.Formatter) error
	CloseWithContext(rustfmt.Context)
}

func CloseSourceIOError(failure error, context rustfmt.Context) {
	if owned, ok := failure.(ownedSourceIOError); ok {
		owned.CloseWithContext(context)
	}
}
func FormatSourceIOErrorDebug(failure error, f *rustfmt.Formatter) (result error) {
	spanRunContext(f.Context, func() {
		if primary, ok := failure.(ownedSourceIOError); ok {
			result = primary.WriteSourceIOErrorDebug(f)
			return
		}
		// Plain Go errors are the trivial-drop convenience, mapped explicitly to
		// io::Error::other(error text), rather than pretending Go Debug is Rust Debug.
		result = formatSourceIOErrorOtherString(failure.Error(), f)
	})
	return
}
func FormatSourceIOErrorDisplay(failure error, f *rustfmt.Formatter) (result error) {
	spanRunContext(f.Context, func() {
		if primary, ok := failure.(ownedSourceIOError); ok {
			result = primary.WriteSourceIOErrorDisplay(f)
			return
		}
		text := failure.Error()
		if !utf8.ValidString(text) {
			f.Context.Panic("source-map: Rust error text requires valid UTF-8")
		}
		result = f.PadString(text)
	})
	return
}
func NewSourceIOErrorOtherString(message string) *SourceMapIOError {
	if !utf8.ValidString(message) {
		panic("source-map: Rust error text requires valid UTF-8")
	}
	return NewSourceIOError(message, SourceIOErrorOps[string]{
		Display: func(message *string, f *rustfmt.Formatter) error { return f.PadString(*message) },
		Debug:   func(message *string, f *rustfmt.Formatter) error { return formatSourceIOErrorOtherString(*message, f) },
	})
}
func formatSourceIOErrorOtherString(message string, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	custom := rustfmt.BeginStruct(f.Writer, "Custom", pretty)
	custom.Field("kind", f.Value(pretty, func(f *rustfmt.Formatter) error { return f.WriteString("Other") }))
	custom.Field("error", f.Value(pretty, func(f *rustfmt.Formatter) error { return FormatDebugStringPinnedUnicode18(message, f) }))
	return custom.Finish()
}

// expect owns its io::Error. Panic-message Debug runs while processing the new
// panic; that value is destroyed before the caller's loader on the unwind edge.
func sourceMapExpectIOFailure(context rustfmt.Context, message string, failure error) {
	defer spanCloseOwnedCallback(context, func(context rustfmt.Context) {
		CloseSourceIOError(failure, context)
	})
	context.PanicFormattedAt(rustfmt.SourcePanicInfo{
		Kind:      rustfmt.PanicOwnedString,
		Location:  context.SourcePanicLocation(SourceMapPanicExpectCWD),
		CanUnwind: true,
	}, func(processing rustfmt.Context) any {
		var text strings.Builder
		f := &rustfmt.Formatter{Writer: &text, Context: processing, Options: rustfmt.DefaultOptions()}
		_ = f.WriteString(message)
		_ = f.WriteString(": ")
		// The SDK panic payload builder retains partial output if a user
		// formatting trait returns fmt::Error. A new panic is different and
		// still aborts immediately in the processing context.
		_ = FormatSourceIOErrorDebug(failure, f)
		return text.String()
	})
}
