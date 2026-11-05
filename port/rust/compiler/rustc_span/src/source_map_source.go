// SPDX-License-Identifier: MIT OR Apache-2.0
// Source extraction and owned FnMut contracts from pinned source_map.rs.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strings"
)

// SpanToSourceOwned is the primary generic span_to_source adapter. It consumes
// the callback environment and returns an owned result. Callback borrows expire
// on return; source text must not be retained beyond the SourceFile owner. A
// prepared return place is not destroyed if the callback environment's normal
// destructor panics, matching Rust's function return-place ownership.
func SpanToSourceOwned[E, T any](sm *SourceMap, session *SessionGlobals, sp Span, environment *SpanOwned[E], call func(*E, string, uint, uint, rustfmt.Context) (*SpanOwned[T], *SpanSnippetError), context rustfmt.Context) (*SpanOwned[T], *SpanSnippetError) {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	begin := sm.LookupByteOffsetWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	defer sourceMapCloseSharedOnExit(begin.SF, context)
	end := sm.LookupByteOffsetWithContext(sourceMapSpanHi(sp, session, sm.config, context), context)
	defer sourceMapCloseSharedOnExit(end.SF, context)
	first, last := begin.SF.Borrow(), end.SF.Borrow()
	if first.StartPos != last.StartPos {
		return nil, DistinctSourcesSpanSnippetError(sourceMapDistinct(first, last))
	}
	sm.EnsureSourceFileSourcePresentWithContext(first, context)
	startIndex, endIndex := begin.Pos.ToUsize(), end.Pos.ToUsize()
	length := first.NormalizedSourceLen.ToUsize()
	if startIndex > endIndex || endIndex > length {
		return nil, MalformedSourceMapSpanSnippetError(MalformedSourceMapPositions{Name: first.Name.Clone(), SourceLen: length, BeginPos: begin.Pos, EndPos: end.Pos})
	}
	invoke := func(src string) (result *SpanOwned[T], failure *SpanSnippetError) {
		spanRunContext(context, func() { result, failure = call(env.Borrow(), src, startIndex, endIndex, context) })
		return
	}
	if src, ok := first.Src.Get(); ok {
		return invoke(src.String())
	}
	guard := first.ExternalSrc.ReadWithContext(context)
	defer guard.Close()
	if src, ok := guard.Get().GetSource().Get(); ok {
		return invoke(src)
	}
	return nil, SourceNotAvailableSpanSnippetError(first.Name.Clone())
}

// SpanToSource is the trivial-drop callback/result convenience. The owned form
// above is required when callback captures or the result have destructors.
func SpanToSource[T any](sm *SourceMap, session *SessionGlobals, sp Span, extract func(string, uint, uint) (T, *SpanSnippetError)) (T, *SpanSnippetError) {
	return SpanToSourceWithContext(sm, session, sp, extract, rustfmt.Context{})
}
func SpanToSourceWithContext[T any](sm *SourceMap, session *SessionGlobals, sp Span, extract func(string, uint, uint) (T, *SpanSnippetError), context rustfmt.Context) (T, *SpanSnippetError) {
	result, failure := SpanToSourceOwned(sm, session, sp, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, src string, begin, end uint, _ rustfmt.Context) (*SpanOwned[T], *SpanSnippetError) {
		value, err := extract(src, begin, end)
		if err != nil {
			return nil, err
		}
		return NewSpanOwned(value, nil), nil
	}, context)
	if failure != nil {
		var zero T
		return zero, failure
	}
	return result.Take(), nil
}

func sourceMapDistinct(begin, end *SourceFile) DistinctSources {
	return DistinctSources{Begin: FileNameAndBytePos{Name: begin.Name.Clone(), Pos: begin.StartPos}, End: FileNameAndBytePos{Name: end.Name.Clone(), Pos: end.StartPos}}
}

func (sm *SourceMap) EnsureSourceFileSourcePresent(file *SourceFile) bool {
	return sm.EnsureSourceFileSourcePresentWithContext(file, rustfmt.Context{})
}
func (sm *SourceMap) EnsureSourceFileSourcePresentWithContext(file *SourceFile, context rustfmt.Context) bool {
	return file.AddExternalSrcWithContext(func() Option[string] {
		if file.Name.kind != fileNameReal {
			return None[string]()
		}
		name := &file.Name.real
		path, local := name.LocalPath().Get()
		if !local {
			path = name.PathWithContext(RemapPathScopeDiagnostics, context)
			if reversed, ok := sm.pathMapping.ReverseMapPrefixHeuristicallyWithContext(path, context).Get(); ok {
				path = reversed
			}
		}
		src, err := sm.fileLoader.readFile(context, path)
		if err != nil {
			CloseSourceIOError(err, context)
			return None[string]()
		}
		return Some(src)
	}, context, sm.config)
}

func (sm *SourceMap) IsImported(session *SessionGlobals, sp Span) bool {
	return sm.IsImportedWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IsImportedWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) bool {
	index := sm.LookupSourceFileIdxWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	return guard.BorrowWithContext(index, context).BorrowWithContext(context).IsImported()
}

func sourceMapSpanLo(sp Span, session *SessionGlobals, config SpanEncodingConfig, context rustfmt.Context) BytePos {
	return sp.dataWithContext(session, config, context).Lo
}
func sourceMapSpanHi(sp Span, session *SessionGlobals, config SpanEncodingConfig, context rustfmt.Context) BytePos {
	return sp.dataWithContext(session, config, context).Hi
}
func sourceMapSpanWithLo(sp Span, session *SessionGlobals, lo BytePos, config SpanEncodingConfig, context rustfmt.Context) Span {
	data := sp.dataWithContext(session, config, context)
	return newSpanWithContext(session, lo, data.Hi, data.Ctxt, data.Parent, config, context)
}
func sourceMapSpanWithHi(sp Span, session *SessionGlobals, hi BytePos, config SpanEncodingConfig, context rustfmt.Context) Span {
	data := sp.dataWithContext(session, config, context)
	return newSpanWithContext(session, data.Lo, hi, data.Ctxt, data.Parent, config, context)
}
func (sm *SourceMap) IsSpanAccessible(session *SessionGlobals, sp Span) bool {
	return sm.IsSpanAccessibleWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IsSpanAccessibleWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) bool {
	accessible, failure := SpanToSourceWithContext(sm, session, sp, func(src string, start, end uint) (bool, *SpanSnippetError) {
		return sourceMapStringRangeValid(src, start, end), nil
	}, context)
	return failure == nil && accessible
}

func sourceMapStringBoundary(src string, index uint) bool {
	return index == uint(len(src)) || index < uint(len(src)) && (src[index]&0xc0) != 0x80
}
func sourceMapStringRangeValid(src string, begin, end uint) bool {
	return begin <= end && sourceMapStringBoundary(src, begin) && sourceMapStringBoundary(src, end)
}

func (sm *SourceMap) SpanToSnippet(session *SessionGlobals, sp Span) (string, *SpanSnippetError) {
	return sm.SpanToSnippetWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToSnippetWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (string, *SpanSnippetError) {
	return SpanToSourceWithContext(sm, session, sp, func(src string, begin, end uint) (string, *SpanSnippetError) {
		if !sourceMapStringRangeValid(src, begin, end) {
			return "", IllFormedSpanSnippetError(sp)
		}
		return strings.Clone(src[begin:end]), nil
	}, context)
}
func (sm *SourceMap) SpanToPrevSource(session *SessionGlobals, sp Span) (string, *SpanSnippetError) {
	return sm.SpanToPrevSourceWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToPrevSourceWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (string, *SpanSnippetError) {
	return SpanToSourceWithContext(sm, session, sp, func(src string, begin, _ uint) (string, *SpanSnippetError) {
		if !sourceMapStringBoundary(src, begin) {
			return "", IllFormedSpanSnippetError(sp)
		}
		return strings.Clone(src[:begin]), nil
	}, context)
}
func (sm *SourceMap) SpanToNextSource(session *SessionGlobals, sp Span) (string, *SpanSnippetError) {
	return sm.SpanToNextSourceWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToNextSourceWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (string, *SpanSnippetError) {
	return SpanToSourceWithContext(sm, session, sp, func(src string, _, end uint) (string, *SpanSnippetError) {
		if !sourceMapStringBoundary(src, end) {
			return "", IllFormedSpanSnippetError(sp)
		}
		return strings.Clone(src[end:]), nil
	}, context)
}
