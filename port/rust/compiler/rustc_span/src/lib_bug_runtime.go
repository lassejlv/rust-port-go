// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL actual macros.rs bug_impl/diagnostic callback dependency consumed by
// the span group. This grants no complete macros.rs or std formatting credit.
package rustcspan

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// SourceLocation is an immutable source caller-location witness. Host marks
// the Go convenience capture, whose column/provenance is not a Rust claim.
type SourceLocation struct {
	File         string
	Line, Column uint32
	Host         bool
	cell         *rustfmt.SourceLocation
}

func (l SourceLocation) String() string { return fmt.Sprintf("%s:%d:%d", l.File, l.Line, l.Column) }

// SourceFormatArguments borrows all referenced formatting state. It owns no
// capture environment and must not outlive those borrows. Rendering is lazy;
// a diagnostic callback may inspect the literal without formatting arguments.
type SourceFormatArguments struct {
	render  func(*rustfmt.Formatter) error
	literal Option[string]
}

func LiteralSourceFormatArguments(text string) SourceFormatArguments {
	requireSpanUTF8(text, rustfmt.Context{})
	return SourceFormatArguments{literal: Some(text)}
}
func NewSourceFormatArguments(render func(*rustfmt.Formatter) error) SourceFormatArguments {
	if render == nil {
		panic("nil format argument witness")
	}
	return SourceFormatArguments{render: render}
}
func (args SourceFormatArguments) AsStr() Option[string] { return args.literal }
func (args SourceFormatArguments) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	if literal, ok := args.literal.Get(); ok {
		return f.WriteString(literal)
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	return args.render(child)
}

// ExplicitBug is the actual typed panic marker selected when the diagnostic
// hook reports that it emitted an explicit compiler bug.
type ExplicitBug struct{}
type emitBugFunction func(*SessionGlobals, Option[Span], SourceFormatArguments, SourceLocation, rustfmt.Context) bool
type emitBugCallback struct{ call emitBugFunction }

var emitBugDiagnostic atomic.Pointer[emitBugCallback]

func defaultEmitBug(*SessionGlobals, Option[Span], SourceFormatArguments, SourceLocation, rustfmt.Context) bool {
	return false
}

// SwapEmitBugDiagnostic adapts the source atomic fn pointer. The witness owns
// no nontrivial capture; translated panic sites call the provided Context.
func SwapEmitBugDiagnostic(callback emitBugFunction) emitBugFunction {
	if callback == nil {
		panic("nil emit-bug function is not a Rust fn pointer")
	}
	old := emitBugDiagnostic.Swap(&emitBugCallback{callback})
	if old == nil {
		return defaultEmitBug
	}
	return old.call
}
func BugImplWithContext(session *SessionGlobals, span Option[Span], args SourceFormatArguments, location SourceLocation, context rustfmt.Context) {
	defer func() {
		if failure := recover(); failure != nil {
			context.ResumeUnwind(failure)
		}
	}()
	hook := defaultEmitBug
	if value := emitBugDiagnostic.Load(); value != nil {
		hook = value.call
	}
	panicLocation := location.cell
	if panicLocation == nil && !location.Host {
		panicLocation = &rustfmt.SourceLocation{File: location.File, Line: location.Line, Column: location.Column}
	}
	if hook(session, span, args, location, context) {
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: ExplicitBug{}, Kind: rustfmt.PanicOpaque, Location: panicLocation, CanUnwind: true})
	}
	// Unlike panic!("...{value:?}..."), this exact source performs format!
	// before panic_any, so argument formatting inherits the ambient state and
	// does not start processing a fresh panic in advance.
	var text strings.Builder
	f := &rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: context}
	if err := args.WriteDisplayFormatter(f); err != nil {
		context.Panic("a formatting trait implementation returned an error when the underlying stream did not: Error")
	}
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: text.String(), Kind: rustfmt.PanicOwnedString, Location: panicLocation, CanUnwind: true})
}
func SpanBugAtWithContext(session *SessionGlobals, span Span, message string, location SourceLocation, context rustfmt.Context) {
	BugImplWithContext(session, Some(span), LiteralSourceFormatArguments(message), location, context)
}
func SpanBugWithContext(session *SessionGlobals, span Span, message string, context rustfmt.Context) {
	_, file, line, _ := runtime.Caller(1)
	SpanBugAtWithContext(session, span, message, SourceLocation{File: file, Line: uint32(line), Host: true}, context)
}
