// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL lib/identity formatting blocks consumed by span_encoding.rs.
// Source: rustc_span/src/lib.rs and rustc_index_macros/src/newtype.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Shared std formatting support has
// no independent source-file credit. Full source-map construction is unported.
package rustcspan

import (
	"io"
	"strings"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Internal span lookups can fail before a user callback is involved. Route
// those panics through the caller's explicit runtime context, including the
// immediate abort required while an existing panic is being processed.
func spanDebugContextGuard(context rustfmt.Context) {
	if failure := recover(); failure != nil {
		context.ResumeUnwind(failure)
	}
}

func (value BytePos) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	builder := rustfmt.BeginTuple(f.Writer, "BytePos", pretty)
	builder.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(value), 32, child) }))
	return builder.Finish()
}

func (value SyntaxContext) WriteDebugFormatter(f *rustfmt.Formatter) error {
	// The exact macro uses write!(fmt,"#{}", self.as_u32()), so the inner
	// numeric argument receives fresh default Display options.
	if err := f.WriteString("#"); err != nil {
		return err
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	return rustfmt.FormatDebugUint(uint64(value.AsU32()), 32, child)
}

func (value Span) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter, config ...SpanEncodingConfig) error {
	runtimeContext := f.Context
	defer spanDebugContextGuard(runtimeContext)
	profile := spanEncodingConfig(session, config...)
	return value.writeDebugFormatterWithContext(session, f, profile, runtimeContext)
}
func (value Span) writeDebugFormatterWithContext(session *SessionGlobals, f *rustfmt.Formatter, profile SpanEncodingConfig, runtimeContext rustfmt.Context) error {
	defer spanDebugContextGuard(runtimeContext)
	if session != nil {
		if sourceMap, present := session.sourceMap.Get(); present {
			text := sourceMap.BorrowWithContext(runtimeContext).SpanToDiagnosticStringWithContext(session, value, runtimeContext)
			context := value.Ctxt()
			if err := f.WriteString(text); err != nil {
				return err
			}
			if err := f.WriteString(" ("); err != nil {
				return err
			}
			child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: runtimeContext}
			if err := context.WriteDebugFormatter(child); err != nil {
				return err
			}
			return f.WriteString(")")
		}
	}
	if session != nil && session.spanSourceMap != nil {
		text := session.spanSourceMap.SpanToDiagnosticString(session, value)
		context := value.Ctxt()
		if err := f.WriteString(text); err != nil {
			return err
		}
		if err := f.WriteString(" ("); err != nil {
			return err
		}
		child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: runtimeContext}
		if err := context.WriteDebugFormatter(child); err != nil {
			return err
		}
		return f.WriteString(")")
	}
	pretty := f.Options.Alternate
	builder := rustfmt.BeginStruct(f.Writer, "Span", pretty)
	// Each getter runs before Field is invoked, even after remembered writer
	// errors. This preserves the source query-dependency callbacks and panics.
	lo := value.dataWithContext(session, profile, runtimeContext).Lo
	builder.Field("lo", f.Value(pretty, lo.WriteDebugFormatter))
	hi := value.dataWithContext(session, profile, runtimeContext).Hi
	builder.Field("hi", f.Value(pretty, hi.WriteDebugFormatter))
	context := value.Ctxt()
	builder.Field("ctxt", f.Value(pretty, context.WriteDebugFormatter))
	return builder.Finish()
}

func (value SpanData) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter, config ...SpanEncodingConfig) error {
	runtimeContext := f.Context
	defer spanDebugContextGuard(runtimeContext)
	profile := spanEncodingConfig(session, config...)
	return newSpanWithContext(session, value.Lo, value.Hi, value.Ctxt, value.Parent, profile, runtimeContext).writeDebugFormatterWithContext(session, f, profile, runtimeContext)
}
func (value OrdSpan) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter, config ...SpanEncodingConfig) error {
	runtimeContext := f.Context
	defer spanDebugContextGuard(runtimeContext)
	profile := spanEncodingConfig(session, config...)
	pretty := f.Options.Alternate
	builder := rustfmt.BeginTuple(f.Writer, "OrdSpan", pretty)
	builder.Field(f.Value(pretty, func(child *rustfmt.Formatter) error {
		return value.Span.writeDebugFormatterWithContext(session, child, profile, runtimeContext)
	}))
	return builder.Finish()
}
func (value Span) WriteDebug(session *SessionGlobals, w io.Writer, options rustfmt.Options, config ...SpanEncodingConfig) error {
	return value.WriteDebugFormatter(session, &rustfmt.Formatter{Writer: w, Options: options}, config...)
}
func (value OrdSpan) WriteDebug(session *SessionGlobals, w io.Writer, options rustfmt.Options, config ...SpanEncodingConfig) error {
	return value.WriteDebugFormatter(session, &rustfmt.Formatter{Writer: w, Options: options}, config...)
}
func (value Span) RustDebug(session *SessionGlobals, config ...SpanEncodingConfig) string {
	var text strings.Builder
	if err := value.WriteDebug(session, &text, rustfmt.DefaultOptions(), config...); err != nil {
		panic(err)
	}
	return text.String()
}
func (value OrdSpan) RustDebug(session *SessionGlobals, config ...SpanEncodingConfig) string {
	var text strings.Builder
	if err := value.WriteDebug(session, &text, rustfmt.DefaultOptions(), config...); err != nil {
		panic(err)
	}
	return text.String()
}
