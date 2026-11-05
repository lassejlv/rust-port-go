// SPDX-License-Identifier: MIT OR Apache-2.0
// Derived Debug bodies of the diagnostic source error types in pinned lib.rs.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func debugFilenameAndBytePos(value FileNameAndBytePos, f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "", p)
	b.Field(f.Value(p, value.Name.WriteDebugFormatter))
	b.Field(f.Value(p, value.Pos.WriteDebugFormatter))
	return b.Finish()
}
func (value DistinctSources) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "DistinctSources", p)
	b.Field("begin", f.Value(p, func(f *rustfmt.Formatter) error { return debugFilenameAndBytePos(value.Begin, f) }))
	b.Field("end", f.Value(p, func(f *rustfmt.Formatter) error { return debugFilenameAndBytePos(value.End, f) }))
	return b.Finish()
}
func (value MalformedSourceMapPositions) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "MalformedSourceMapPositions", p)
	b.Field("name", f.Value(p, value.Name.WriteDebugFormatter))
	b.Field("source_len", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.SourceLen), uint(spanUintBits()), f)
	}))
	b.Field("begin_pos", f.Value(p, value.BeginPos.WriteDebugFormatter))
	b.Field("end_pos", f.Value(p, value.EndPos.WriteDebugFormatter))
	return b.Finish()
}
func (value *SpanLinesError) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "DistinctSources", p)
	b.Field(f.Value(p, value.distinct.WriteDebugFormatter))
	return b.Finish()
}
func (value *SpanSnippetError) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	switch value.kind {
	case snippetIllFormed:
		b := rustfmt.BeginTuple(f.Writer, "IllFormedSpan", p)
		b.Field(f.Value(p, func(child *rustfmt.Formatter) error { return value.span.WriteDebugFormatter(session, child) }))
		return b.Finish()
	case snippetDistinct:
		b := rustfmt.BeginTuple(f.Writer, "DistinctSources", p)
		b.Field(f.Value(p, value.distinct.WriteDebugFormatter))
		return b.Finish()
	case snippetMalformed:
		b := rustfmt.BeginTuple(f.Writer, "MalformedForSourcemap", p)
		b.Field(f.Value(p, value.malformed.WriteDebugFormatter))
		return b.Finish()
	case snippetUnavailable:
		b := rustfmt.BeginStruct(f.Writer, "SourceNotAvailable", p)
		b.Field("filename", f.Value(p, value.filename.WriteDebugFormatter))
		return b.Finish()
	}
	f.Context.Panic("rustcspan: invalid SpanSnippetError")
	return nil
}
