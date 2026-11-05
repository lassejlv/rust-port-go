// SPDX-License-Identifier: MIT OR Apache-2.0
// Character-point and statement helpers from pinned source_map.rs.
package rustcspan

import (
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

func (sm *SourceMap) StartPoint(session *SessionGlobals, sp Span) Span {
	return sm.StartPointWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) StartPointWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	width := func() uint {
		begin := sm.LookupByteOffsetWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
		defer sourceMapCloseSharedOnExit(begin.SF, context)
		index := begin.Pos.ToUsize()
		file := begin.SF.Borrow()
		guard := file.ExternalSrc.ReadWithContext(context)
		defer guard.Close()
		var text string
		site := "rustc_span::source_map::start_point.external_slice"
		present := false
		if src, ok := file.Src.Get(); ok {
			text, present = src.String(), true
			site = SourceMapPanicStartSlice
		} else {
			text, present = guard.Get().GetSource().Get()
		}
		if !present {
			return 1
		}
		text = sourceMapStringSlice(text, index, uint(len(text)), context, site)
		if text == "" {
			return 1
		}
		_, length := utf8.DecodeRuneInString(text)
		return uint(length)
	}()
	hi := BytePos(sourceMapU32Add(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(width), sm.config, context, "rustc_span::source_map::start_point.add"))
	return sourceMapSpanWithHi(sp, session, hi, sm.config, context)
}
func (sm *SourceMap) EndPoint(session *SessionGlobals, sp Span) Span {
	return sm.EndPointWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) EndPointWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	data := sp.dataWithContext(session, sm.config, context)
	width := sm.findWidthOfCharacterAtSpan(session, data, false, context)
	corrected := uint32(data.Hi)
	if width <= corrected {
		corrected -= width
	}
	if corrected < uint32(data.Lo) {
		corrected = uint32(data.Lo)
	}
	return newSpanWithContext(session, BytePos(corrected), data.Hi, data.Ctxt, data.Parent, sm.config, context)
}
func (sm *SourceMap) NextPoint(session *SessionGlobals, sp Span) Span {
	return sm.NextPointWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) NextPointWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	if sp.IsDummyWithContext(session, context) {
		return sp
	}
	data := sp.dataWithContext(session, sm.config, context)
	start := uint32(data.Hi)
	width := sm.findWidthOfCharacterAtSpan(session, data, true, context)
	end := start
	if width <= ^uint32(0)-start {
		end += width
	}
	next := sourceMapU32Add(start, 1, sm.config, context, "rustc_span::source_map::next_point.add")
	if end < next {
		end = next
	}
	return newSpanWithContext(session, BytePos(start), BytePos(end), data.Ctxt, None[LocalDefId](), sm.config, context)
}
func (sm *SourceMap) SpanFollowedBy(session *SessionGlobals, sp Span, target string) Option[Span] {
	return sm.SpanFollowedByWithContext(session, sp, target, rustfmt.Context{})
}
func (sm *SourceMap) SpanFollowedByWithContext(session *SessionGlobals, sp Span, target string, context rustfmt.Context) Option[Span] {
	requireSpanUTF8(target, context)
	sp = sm.SpanExtendWhileWhitespaceWithContext(session, sp, context)
	text, failure := sm.SpanToNextSourceWithContext(session, sp, context)
	if failure != nil || !strings.HasPrefix(text, target) {
		return None[Span]()
	}
	lo := sourceMapSpanHi(sp, session, sm.config, context)
	hi := sourceMapSpanHi(sp, session, sm.config, context).AddWithContext(BytePos(uint32(len(target))), sm.config, context)
	return Some(newSpanWithContext(session, lo, hi, sp.Ctxt(), None[LocalDefId](), sm.config, context))
}
func (sm *SourceMap) findWidthOfCharacterAtSpan(session *SessionGlobals, sp SpanData, forwards bool, context rustfmt.Context) uint32 {
	traceSpan, traceScope := sourceMapTraceSpan(context, sourceMapTraceWidth, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "forwards", Value: rusttracing.BoolField(forwards)}}
	})
	defer traceSpan.DeferCloseWithScope(traceScope, context)
	traceEntered := traceSpan.EnterWithScope(traceScope, context)
	defer traceEntered.DeferCloseWithScope(traceScope, context)
	if sp.Lo == sp.Hi && !forwards {
		sourceMapTraceLiteral(context, sourceMapTraceWidthEmpty, "early return empty span")
		return 1
	}
	begin := sm.LookupByteOffsetWithContext(sp.Lo, context)
	defer sourceMapCloseSharedOnExit(begin.SF, context)
	end := sm.LookupByteOffsetWithContext(sp.Hi, context)
	defer sourceMapCloseSharedOnExit(end.SF, context)
	sourceMapTraceMessage(context, sourceMapTraceWidthLoc, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("local_begin=`"); err != nil {
			return err
		}
		if err := begin.WriteDebugFormatter(f); err != nil {
			return err
		}
		if err := f.WriteString("`, local_end=`"); err != nil {
			return err
		}
		if err := end.WriteDebugFormatter(f); err != nil {
			return err
		}
		return f.WriteString("`")
	})
	if begin.SF.Borrow().StartPos != end.SF.Borrow().StartPos {
		sourceMapTraceLiteral(context, sourceMapTraceWidthFiles, "begin and end are in different files")
		return 1
	}
	startIndex, endIndex := begin.Pos.ToUsize(), end.Pos.ToUsize()
	sourceMapTraceMessage(context, sourceMapTraceWidthIndex, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("start_index=`"); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugUint(uint64(startIndex), uint(spanUintBits()), f); err != nil {
			return err
		}
		if err := f.WriteString("`, end_index=`"); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugUint(uint64(endIndex), uint(spanUintBits()), f); err != nil {
			return err
		}
		return f.WriteString("`")
	})
	if !forwards && endIndex == 0 || forwards && startIndex == ^uint(0) {
		sourceMapTraceLiteral(context, sourceMapTraceWidthLimit, "start or end of span, cannot be multibyte")
		return 1
	}
	sourceLen := begin.SF.Borrow().NormalizedSourceLen.ToUsize()
	sourceMapTraceMessage(context, sourceMapTraceWidthLen, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("source_len=`"); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugUint(uint64(sourceLen), uint(spanUintBits()), f); err != nil {
			return err
		}
		return f.WriteString("`")
	})
	// This is ordinary source subtraction, including the empty-file underflow
	// in overflow-checking profiles; it is intentionally not saturating.
	if startIndex > endIndex || endIndex > sourceMapUsizeSub(sourceLen, 1, sm.config, context, "rustc_span::source_map::find_width.subtract_length") {
		sourceMapTraceLiteral(context, sourceMapTraceWidthMalformed, "source indexes are malformed")
		return 1
	}
	file := begin.SF.Borrow()
	guard := file.ExternalSrc.ReadWithContext(context)
	defer guard.Close()
	var text string
	present := false
	if src, ok := file.Src.Get(); ok {
		text, present = src.String(), true
	} else {
		text, present = guard.Get().GetSource().Get()
	}
	if !present {
		return 1
	}
	if forwards {
		index := sourceMapUsizeAdd(endIndex, 1, sm.config, context, "rustc_span::source_map::find_width.forward_index")
		boundary := sourceMapCeilBoundary(text, index)
		return uint32(sourceMapUsizeSub(boundary, endIndex, sm.config, context, "rustc_span::source_map::find_width.forward_width"))
	}
	index := sourceMapUsizeSub(endIndex, 1, sm.config, context, "rustc_span::source_map::find_width.backward_index")
	boundary := sourceMapFloorBoundary(text, index)
	return uint32(sourceMapUsizeSub(endIndex, boundary, sm.config, context, "rustc_span::source_map::find_width.backward_width"))
}
func sourceMapCeilBoundary(text string, index uint) uint {
	if index >= uint(len(text)) {
		return uint(len(text))
	}
	for index < uint(len(text)) && !sourceMapStringBoundary(text, index) {
		index++
	}
	return index
}
func sourceMapFloorBoundary(text string, index uint) uint {
	if index >= uint(len(text)) {
		return uint(len(text))
	}
	for index > 0 && !sourceMapStringBoundary(text, index) {
		index--
	}
	return index
}
func (sm *SourceMap) StmtSpan(session *SessionGlobals, statement, block Span) Span {
	return sm.StmtSpanWithContext(session, statement, block, rustfmt.Context{})
}
func (sm *SourceMap) StmtSpanWithContext(session *SessionGlobals, statement, block Span, context rustfmt.Context) Span {
	if !statement.FromExpansion() {
		return statement
	}
	call := OriginalSpanWithContext(session, statement, block, context)
	if semi, ok := sm.MacCallStmtSemiSpanWithContext(session, call, context).Get(); ok {
		return sourceMapSpanWithHi(call, session, sourceMapSpanHi(semi, session, sm.config, context), sm.config, context)
	}
	return call
}
func (sm *SourceMap) MacCallStmtSemiSpan(session *SessionGlobals, call Span) Option[Span] {
	return sm.MacCallStmtSemiSpanWithContext(session, call, rustfmt.Context{})
}
func (sm *SourceMap) MacCallStmtSemiSpanWithContext(session *SessionGlobals, call Span, context rustfmt.Context) Option[Span] {
	span := sm.SpanExtendWhileWhitespaceWithContext(session, call, context)
	span = sm.NextPointWithContext(session, span, context)
	text, failure := sm.SpanToSnippetWithContext(session, span, context)
	if failure == nil && text == ";" {
		return Some(span)
	}
	return None[Span]()
}
