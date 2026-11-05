// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped used contract of anyhow 1.0.103's Error::msg(String), MessageError,
// ErrorImpl::{display,debug}, and String downcast. This is not a full anyhow
// crate port. See tools/port-cargo-time-span for source and runtime profiles.
package rustanyhow

import (
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

// BacktraceStatus mirrors the selected std BacktraceStatus values semantically;
// these tags do not promise the Rust enum ABI.
type BacktraceStatus uint8

const (
	BacktraceUnsupported BacktraceStatus = iota
	BacktraceDisabled
	BacktraceCaptured
)

// Backtrace is the immutable result of an explicit source-runtime capture.
// Text is the actual Backtrace Display result, not a Go-generated Rust stack.
// A Captured record must be supplied by a truthful runtime/profile witness.
type Backtrace struct {
	status BacktraceStatus
	text   string
}

func NewBacktraceRecord(status BacktraceStatus, text string) Backtrace {
	if status > BacktraceCaptured || !utf8.ValidString(text) {
		panic("rustanyhow: invalid source backtrace record")
	}
	return Backtrace{status: status, text: text}
}
func (b Backtrace) Status() BacktraceStatus { return b.status }
func (b Backtrace) DisplayText() string     { return b.text }

// BacktraceOperations borrows a complete runtime capture operation. Its code
// and metadata are not an owned Rust closure. Capture receives THIS caller's
// Context; the error never retains that context or a construction-time Scope.
// A nil Capture explicitly selects the disabled-backtrace convenience profile,
// equivalent to a source runtime whose RUST_LIB_BACKTRACE=0 was already chosen.
// It does not inspect Go/Rust process environment or fabricate a native stack.
type BacktraceOperations struct {
	Capture func(rustfmt.Context) Backtrace
}

func DisabledBacktraceProfile() BacktraceOperations { return BacktraceOperations{} }

type stringErrorCell struct {
	message   *ruststr.StringCell
	backtrace Backtrace
}
type stringErrorOwner struct{ cell *stringErrorCell }

// StringError owns the actual String message cell. Descriptor copies are not
// Clone: Move/Downcast/Close invalidate every descriptor sharing that owner.
// Source references from DowncastOwnedStringRef obey the original borrow lifetime;
// Go GC safety after a move/drop does not extend the source borrow contract.
// General anyhow contexts, arbitrary error payloads and stack unwinding are
// outside this scoped dependency implementation.
type StringError struct{ owner *stringErrorOwner }

func NewStringError(message string) *StringError {
	return NewStringErrorWithContext(message, DisabledBacktraceProfile(), rustfmt.Context{})
}
func NewStringErrorWithContext(message string, operations BacktraceOperations, context rustfmt.Context) *StringError {
	return NewOwnedStringErrorWithContext(ruststr.OwnedStringFromGo(message, context), operations, context)
}

// NewOwnedStringErrorWithContext is the concrete Error::msg(String) primary.
// It consumes the actual String owner, preserving its allocation and capacity.
func NewOwnedStringErrorWithContext(message *ruststr.OwnedString, operations BacktraceOperations, context rustfmt.Context) *StringError {
	argument := message.TakeCellWithContext(context)
	owned := true
	defer func() {
		if p := recover(); p != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(p)
			}
			if owned {
				unwind := context
				unwind.Unwinding = true
				argument.CloseWithContext(unwind)
			}
			context.ResumeUnwind(p)
		}
	}()
	trace := Backtrace{status: BacktraceDisabled, text: "disabled backtrace"}
	if operations.Capture != nil {
		trace = operations.Capture(context)
		if trace.status > BacktraceCaptured || !utf8.ValidString(trace.text) {
			context.Panic("rustanyhow: invalid capture result")
		}
	}
	cell := argument.MoveWithContext(context)
	owned = false
	return &StringError{owner: &stringErrorOwner{cell: &stringErrorCell{message: cell, backtrace: trace}}}
}
func (e *StringError) borrow(context rustfmt.Context) *stringErrorCell {
	if e == nil || e.owner == nil || e.owner.cell == nil {
		context.Panic("rustanyhow: consumed String error")
	}
	return e.owner.cell
}
func (e *StringError) Error() string {
	return strings.Clone(e.borrow(rustfmt.Context{}).message.AsStr())
}

// Source delegates the actual MessageError<String>::source(), which is None.
// No Unwrap chain is invented around the underlying String.
func (e *StringError) Source() error { e.borrow(rustfmt.Context{}); return nil }
func (e *StringError) Backtrace() *Backtrace {
	return e.BacktraceWithContext(rustfmt.Context{})
}
func (e *StringError) BacktraceWithContext(context rustfmt.Context) *Backtrace {
	return &e.borrow(context).backtrace
}
func (e *StringError) IsString() bool { e.borrow(rustfmt.Context{}); return true }

// IsStaticString distinguishes the actual String payload from &'static str,
// even though both would use Go strings in a general error adapter.
func (e *StringError) IsStaticString() bool { e.borrow(rustfmt.Context{}); return false }

// DowncastOwnedStringRef is the actual borrowed/mutable String cell. Reserve
// and push operations take their current Context explicitly; no runtime is
// captured by the payload. Text-only Go methods remain conveniences.
func (e *StringError) DowncastOwnedStringRef() *ruststr.StringCell {
	return e.DowncastOwnedStringRefWithContext(rustfmt.Context{})
}
func (e *StringError) DowncastOwnedStringRefWithContext(context rustfmt.Context) *ruststr.StringCell {
	return e.borrow(context).message
}
func (e *StringError) Move() *StringError { return e.MoveWithContext(rustfmt.Context{}) }
func (e *StringError) MoveWithContext(context rustfmt.Context) *StringError {
	cell := e.borrow(context)
	e.owner.cell = nil
	return &StringError{owner: &stringErrorOwner{cell: cell}}
}

// DowncastOwnedString consumes the error and moves the SAME String buffer
// into the returned owner. Its old String header is retired, not cloned.
func (e *StringError) DowncastOwnedString() *ruststr.OwnedString {
	return e.DowncastOwnedStringWithContext(rustfmt.Context{})
}
func (e *StringError) DowncastOwnedStringWithContext(context rustfmt.Context) *ruststr.OwnedString {
	cell := e.borrow(context)
	result := ruststr.OwnedStringFromMovedCell(cell.message, context)
	e.owner.cell = nil
	cell.backtrace = Backtrace{}
	return result
}

// DowncastString is a text-only Go convenience; use DowncastOwnedString for
// source String ownership, buffer identity, len/capacity and mutable methods.
func (e *StringError) DowncastString() string { return e.DowncastStringWithContext(rustfmt.Context{}) }
func (e *StringError) DowncastStringWithContext(context rustfmt.Context) string {
	owned := e.DowncastOwnedStringWithContext(context)
	text := strings.Clone(owned.BorrowWithContext(context).AsStrWithContext(context))
	owned.CloseWithContext(context)
	return text
}
func (e *StringError) Close() { e.CloseWithContext(rustfmt.Context{}) }
func (e *StringError) CloseWithContext(context rustfmt.Context) {
	cell := e.borrow(context)
	e.owner.cell = nil
	cell.backtrace = Backtrace{}
	cell.message.CloseWithContext(context)
}

// The outer write!("{}", error) resets string formatting options. There is no
// cause after MessageError<String>, so alternate Display has no extra output.
func (e *StringError) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	cell := e.borrow(f.Context)
	child := *f
	child.Options = rustfmt.DefaultOptions()
	return child.PadString(cell.message.AsStrWithContext(f.Context))
}
func (e *StringError) WriteDebugFormatter(f *rustfmt.Formatter) error {
	cell := e.borrow(f.Context)
	if f.Options.Alternate {
		return rustfmt.FormatDebugString(cell.message.AsStrWithContext(f.Context), f)
	}
	if err := e.WriteDisplayFormatter(f); err != nil {
		return err
	}
	if cell.backtrace.status != BacktraceCaptured {
		return nil
	}
	text := cell.backtrace.text
	if err := f.WriteString("\n\n"); err != nil {
		return err
	}
	if strings.HasPrefix(text, "stack backtrace:") {
		text = "S" + text[1:]
	} else if err := f.WriteString("Stack backtrace:\n"); err != nil {
		return err
	}
	return f.WriteString(strings.TrimRightFunc(text, sourceWhitespace17))
}
func (e *StringError) RustDebug() string { return rustfmt.String(e.WriteDebugFormatter) }

// Exact char::is_whitespace set of the declared Unicode17 SDK. This small
// stable predicate is a scoped used method, not a Go Unicode-version fallback.
func sourceWhitespace17(c rune) bool {
	return c >= '\t' && c <= '\r' || c == ' ' || c == 0x85 || c == 0xa0 || c == 0x1680 ||
		c >= 0x2000 && c <= 0x200a || c == 0x2028 || c == 0x2029 || c == 0x202f || c == 0x205f || c == 0x3000
}
