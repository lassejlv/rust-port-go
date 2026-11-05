// SPDX-License-Identifier: MIT OR Apache-2.0
// Diagnostic location/error declarations from pinned rustc_span/src/lib.rs.
package rustcspan

type FileNameAndBytePos struct {
	Name FileName
	Pos  BytePos
}
type DistinctSources struct{ Begin, End FileNameAndBytePos }
type MalformedSourceMapPositions struct {
	Name             FileName
	SourceLen        uint
	BeginPos, EndPos BytePos
}

// The source variants own a Box. Keeping its pointee separate preserves
// identity when the box is moved between enum wrappers. A consumed box pointer
// has one logical owner; borrowed pointers do not extend that ownership.
type SpanLinesError struct{ distinct *DistinctSources }

func DistinctSourcesSpanLinesError(value DistinctSources) *SpanLinesError {
	return DistinctSourcesBoxSpanLinesError(&value)
}
func DistinctSourcesBoxSpanLinesError(box *DistinctSources) *SpanLinesError {
	if box == nil {
		panic("rustcspan: a source Box cannot be nil")
	}
	return &SpanLinesError{distinct: box}
}

// DistinctSources is a borrowed value view retained for compatibility; it is
// not an independent owner. BorrowDistinctSources exposes the actual cell.
func (value *SpanLinesError) DistinctSources() DistinctSources { return *value.distinct }

// TakeDistinctSources consumes the wrapper and returns its owning Box pointer.
// The old wrapper and all aliases must be relinquished after this move.
func (value *SpanLinesError) TakeDistinctSources() *DistinctSources {
	box := value.distinct
	value.distinct = nil
	if box == nil {
		panic("rustcspan: DistinctSources box was already consumed")
	}
	return box
}

type spanSnippetErrorKind uint8

const (
	snippetIllFormed spanSnippetErrorKind = iota
	snippetDistinct
	snippetMalformed
	snippetUnavailable
)

type SpanSnippetError struct {
	kind      spanSnippetErrorKind
	span      Span
	distinct  *DistinctSources
	malformed MalformedSourceMapPositions
	filename  FileName
}

func IllFormedSpanSnippetError(value Span) *SpanSnippetError {
	return &SpanSnippetError{kind: snippetIllFormed, span: value}
}
func DistinctSourcesSpanSnippetError(value DistinctSources) *SpanSnippetError {
	return DistinctSourcesBoxSpanSnippetError(&value)
}
func DistinctSourcesBoxSpanSnippetError(box *DistinctSources) *SpanSnippetError {
	if box == nil {
		panic("rustcspan: a source Box cannot be nil")
	}
	return &SpanSnippetError{kind: snippetDistinct, distinct: box}
}

// TakeDistinctSources consumes a matching wrapper and preserves its pointee.
// A false result leaves the nonmatching variant owned by the caller.
func (value *SpanSnippetError) TakeDistinctSources() (*DistinctSources, bool) {
	if value.kind != snippetDistinct {
		return nil, false
	}
	box := value.distinct
	value.distinct = nil
	if box == nil {
		panic("rustcspan: DistinctSources box was already consumed")
	}
	return box, true
}
func MalformedSourceMapSpanSnippetError(value MalformedSourceMapPositions) *SpanSnippetError {
	return &SpanSnippetError{kind: snippetMalformed, malformed: value}
}
func SourceNotAvailableSpanSnippetError(value FileName) *SpanSnippetError {
	return &SpanSnippetError{kind: snippetUnavailable, filename: value}
}
func (value DistinctSources) Clone() DistinctSources {
	return DistinctSources{Begin: FileNameAndBytePos{Name: value.Begin.Name.Clone(), Pos: value.Begin.Pos}, End: FileNameAndBytePos{Name: value.End.Name.Clone(), Pos: value.End.Pos}}
}
func (value DistinctSources) Equal(other DistinctSources) bool {
	return value.Begin.Name.Equal(other.Begin.Name) && value.Begin.Pos == other.Begin.Pos && value.End.Name.Equal(other.End.Name) && value.End.Pos == other.End.Pos
}
func (value MalformedSourceMapPositions) Clone() MalformedSourceMapPositions {
	value.Name = value.Name.Clone()
	return value
}
func (value MalformedSourceMapPositions) Equal(other MalformedSourceMapPositions) bool {
	return value.Name.Equal(other.Name) && value.SourceLen == other.SourceLen && value.BeginPos == other.BeginPos && value.EndPos == other.EndPos
}
func (value *SpanLinesError) Clone() *SpanLinesError {
	return DistinctSourcesSpanLinesError(value.distinct.Clone())
}
func (value *SpanLinesError) Equal(other *SpanLinesError) bool {
	return value.distinct.Equal(*other.distinct)
}
func (value *SpanSnippetError) Clone() *SpanSnippetError {
	out := *value
	switch value.kind {
	case snippetDistinct:
		cloned := value.distinct.Clone()
		out.distinct = &cloned
	case snippetMalformed:
		out.malformed = value.malformed.Clone()
	case snippetUnavailable:
		out.filename = value.filename.Clone()
	}
	return &out
}
func (value *SpanSnippetError) Equal(other *SpanSnippetError) bool {
	if value.kind != other.kind {
		return false
	}
	switch value.kind {
	case snippetIllFormed:
		return value.span == other.span
	case snippetDistinct:
		return value.distinct.Equal(*other.distinct)
	case snippetMalformed:
		return value.malformed.Equal(other.malformed)
	case snippetUnavailable:
		return value.filename.Equal(other.filename)
	}
	panic("rustcspan: invalid SpanSnippetError")
}

type InnerSpan struct{ Start, End uint }

func NewInnerSpan(start, end uint) InnerSpan { return InnerSpan{Start: start, End: end} }
