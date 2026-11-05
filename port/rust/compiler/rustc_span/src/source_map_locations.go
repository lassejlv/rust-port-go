// SPDX-License-Identifier: MIT OR Apache-2.0
// Location and line operations from source_map.rs at the pinned source revision.
package rustcspan

import (
	"strconv"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func OriginalSpan(session *SessionGlobals, sp, enclosing Span) Span {
	return OriginalSpanWithContext(session, sp, enclosing, rustfmt.Context{})
}
func OriginalSpanWithContext(session *SessionGlobals, sp, enclosing Span, context rustfmt.Context) Span {
	ctxt := sp.Ctxt()
	if ctxt.IsRoot() {
		return sp
	}
	enclosingCtxt := enclosing.Ctxt()
	data := ctxt.OuterExpnDataWithContext(session, context)
	defer spanCloseOwnedCallback(context, data.CloseWithContext)
	if !enclosingCtxt.IsRoot() {
		equal := func() bool {
			other := enclosingCtxt.OuterExpnDataWithContext(session, context)
			defer spanCloseOwnedCallback(context, other.CloseWithContext)
			return data.CallSite == other.CallSite
		}()
		if equal {
			return sp
		}
	}
	return OriginalSpanWithContext(session, data.CallSite, enclosing, context)
}
func GetSourceMap(session *SessionGlobals) Option[Shared[SourceMap]] {
	return GetSourceMapWithContext(session, rustfmt.Context{})
}
func GetSourceMapWithContext(session *SessionGlobals, context rustfmt.Context) Option[Shared[SourceMap]] {
	if session == nil {
		// The source function reaches the actual scoped-TLS accessor before
		// its closure. Preserve that operation's metadata and static payload.
		var absent *SessionScope
		session = absent.CurrentWithContext(context)
	}
	if value, ok := session.sourceMap.Get(); ok {
		return Some(value.CloneWithContext(context))
	}
	return None[Shared[SourceMap]]()
}

// GetSourceMapInScope is the primary scoped-TLS source operation. The explicit
// session convenience above assumes that the caller already borrowed it from
// its current source-thread scope. Arc ownership remains an explicit clone.
func GetSourceMapInScope(scope *SessionScope) Option[Shared[SourceMap]] {
	return GetSourceMapInScopeWithContext(scope, rustfmt.Context{})
}
func GetSourceMapInScopeWithContext(scope *SessionScope, context rustfmt.Context) Option[Shared[SourceMap]] {
	return GetSourceMapWithContext(scope.CurrentWithContext(context), context)
}

func (sm *SourceMap) DoctestOffsetLine(file *FileName, original uint) uint {
	return sm.DoctestOffsetLineWithContext(file, original, rustfmt.Context{})
}
func (sm *SourceMap) DoctestOffsetLineWithContext(file *FileName, original uint, context rustfmt.Context) uint {
	if file.kind != fileNameDocTest {
		return original
	}
	if file.line < 0 {
		if sm.config.OverflowChecks && file.line == -int(^uint(0)>>1)-1 {
			sourceMapPanic(context, SourceMapPanicDoctestNegate, rustfmt.PanicStaticStr, "attempt to negate with overflow")
		}
		offset := uint(-file.line)
		if sm.config.OverflowChecks && original < offset {
			sourceMapPanic(context, SourceMapPanicDoctestSubtract, rustfmt.PanicStaticStr, "attempt to subtract with overflow")
		}
		return original - offset
	}
	offset := uint(file.line)
	if sm.config.OverflowChecks && offset > ^uint(0)-original {
		sourceMapPanic(context, SourceMapPanicDoctestAdd, rustfmt.PanicStaticStr, "attempt to add with overflow")
	}
	return original + offset
}

func (sm *SourceMap) LookupCharPos(pos BytePos) Loc {
	return sm.LookupCharPosWithContext(pos, rustfmt.Context{})
}
func (sm *SourceMap) LookupCharPosWithContext(pos BytePos, context rustfmt.Context) Loc {
	file := sm.LookupSourceFileWithContext(pos, context)
	moved := false
	defer sourceMapCloseSharedUnlessMoved(file, &moved, context)
	line, col, display := file.Borrow().LookupFilePosWithColDisplayWithContext(pos, context, sm.config)
	moved = true
	return Loc{File: file, Line: line, Col: col, ColDisplay: display}
}

// LookupLine returns either (line, zero, true) or (zero, owned-file, false),
// preserving both owned alternatives of Result<SourceFileAndLine, Arc<_>>.
func (sm *SourceMap) LookupLine(pos BytePos) (SourceFileAndLine, Shared[SourceFile], bool) {
	return sm.LookupLineWithContext(pos, rustfmt.Context{})
}
func (sm *SourceMap) LookupLineWithContext(pos BytePos, context rustfmt.Context) (SourceFileAndLine, Shared[SourceFile], bool) {
	file := sm.LookupSourceFileWithContext(pos, context)
	moved := false
	defer sourceMapCloseSharedUnlessMoved(file, &moved, context)
	relative := file.Borrow().RelativePositionWithContext(pos, context, sm.config)
	line, ok := file.Borrow().LookupLineWithContext(relative, context, sm.config).Get()
	moved = true
	if ok {
		return SourceFileAndLine{SF: file, Line: line}, Shared[SourceFile]{}, true
	}
	return SourceFileAndLine{}, file, false
}
func (sm *SourceMap) SpanToLocationInfo(session *SessionGlobals, sp Span) (Option[Shared[SourceFile]], uint, uint, uint, uint) {
	return sm.SpanToLocationInfoWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToLocationInfoWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (Option[Shared[SourceFile]], uint, uint, uint, uint) {
	guard := sm.FilesWithContext(context)
	empty := guard.Len() == 0
	guard.Close()
	if empty || sp.IsDummyWithContext(session, context) {
		return None[Shared[SourceFile]](), 0, 0, 0, 0
	}
	lo := sm.LookupCharPosWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	moved := false
	defer sourceMapCloseSharedUnlessMoved(lo.File, &moved, context)
	hi := sm.LookupCharPosWithContext(sourceMapSpanHi(sp, session, sm.config, context), context)
	defer sourceMapCloseSharedOnExit(hi.File, context)
	loCol := sourceMapUsizeAdd(lo.Col.ToUsize(), 1, sm.config, context, "rustc_span::source_map::span_to_location_info.low_column")
	hiCol := sourceMapUsizeAdd(hi.Col.ToUsize(), 1, sm.config, context, "rustc_span::source_map::span_to_location_info.high_column")
	moved = true
	return Some(lo.File), lo.Line, loCol, hi.Line, hiCol
}
func (sm *SourceMap) SpanToString(session *SessionGlobals, sp Span, scope RemapPathScopeComponents) string {
	return sm.SpanToStringWithContext(session, sp, scope, rustfmt.Context{})
}
func (sm *SourceMap) SpanToStringWithContext(session *SessionGlobals, sp Span, scope RemapPathScopeComponents, context rustfmt.Context) string {
	return sm.spanToStringExt(session, sp, scope, false, context)
}
func (sm *SourceMap) SpanToShortString(session *SessionGlobals, sp Span, scope RemapPathScopeComponents) string {
	return sm.SpanToShortStringWithContext(session, sp, scope, rustfmt.Context{})
}
func (sm *SourceMap) SpanToShortStringWithContext(session *SessionGlobals, sp Span, scope RemapPathScopeComponents, context rustfmt.Context) string {
	return sm.spanToStringExt(session, sp, scope, true, context)
}
func (sm *SourceMap) spanToStringExt(session *SessionGlobals, sp Span, scope RemapPathScopeComponents, short bool, context rustfmt.Context) string {
	file, ll, lc, hl, hc := sm.SpanToLocationInfoWithContext(session, sp, context)
	owner, ok := file.Get()
	if !ok {
		return "no-location"
	}
	name := func() string {
		defer sourceMapCloseSharedOnExit(owner, context)
		display := owner.Borrow().Name.Display(scope)
		if short {
			display = owner.Borrow().Name.Short()
		}
		return rustfmt.String(func(f *rustfmt.Formatter) error { f.Context = context; return display.WriteDisplayFormatter(f) })
	}()
	name += ":" + strconv.FormatUint(uint64(ll), 10) + ":" + strconv.FormatUint(uint64(lc), 10)
	if !short {
		name += ": " + strconv.FormatUint(uint64(hl), 10) + ":" + strconv.FormatUint(uint64(hc), 10)
	}
	return name
}
func (sm *SourceMap) SpanToDiagnosticString(session *SessionGlobals, sp Span) string {
	return sm.SpanToDiagnosticStringWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToDiagnosticStringWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) string {
	return sm.SpanToStringWithContext(session, sp, RemapPathScopeDiagnostics, context)
}
func (sm *SourceMap) SpanToFilename(session *SessionGlobals, sp Span) FileName {
	return sm.SpanToFilenameWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToFilenameWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) FileName {
	loc := sm.LookupCharPosWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	defer sourceMapCloseSharedOnExit(loc.File, context)
	return loc.File.Borrow().Name.Clone()
}
func (sm *SourceMap) FilenameForDiagnostics(file *FileName) FileNameDisplay {
	return file.Display(RemapPathScopeDiagnostics)
}
func (sm *SourceMap) IsMultiline(session *SessionGlobals, sp Span) bool {
	return sm.IsMultilineWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IsMultilineWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) bool {
	lo := sm.LookupSourceFileIdxWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	hi := sm.LookupSourceFileIdxWithContext(sourceMapSpanHi(sp, session, sm.config, context), context)
	if lo != hi {
		return true
	}
	guard := sm.FilesWithContext(context)
	file := guard.CloneAtWithContext(lo, context)
	guard.Close()
	defer sourceMapCloseSharedOnExit(file, context)
	first := file.Borrow().RelativePositionWithContext(sourceMapSpanLo(sp, session, sm.config, context), context, sm.config)
	last := file.Borrow().RelativePositionWithContext(sourceMapSpanHi(sp, session, sm.config, context), context, sm.config)
	a := file.Borrow().LookupLineWithContext(first, context, sm.config)
	b := file.Borrow().LookupLineWithContext(last, context, sm.config)
	av, ap := a.Get()
	bv, bp := b.Get()
	return ap != bp || ap && av != bv
}

func (sm *SourceMap) IsValidSpan(session *SessionGlobals, sp Span) (Loc, Loc, *SpanLinesError) {
	return sm.IsValidSpanWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IsValidSpanWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (Loc, Loc, *SpanLinesError) {
	traceSpan, traceScope := sourceMapTraceSpan(context, sourceMapTraceValid, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "sp", Value: rusttracing.DebugField(func(f *rustfmt.Formatter) error { return sp.WriteDebugFormatter(session, f, sm.config) })}}
	})
	defer traceSpan.DeferCloseWithScope(traceScope, context)
	traceEntered := traceSpan.EnterWithScope(traceScope, context)
	defer traceEntered.DeferCloseWithScope(traceScope, context)
	lo := sm.LookupCharPosWithContext(sourceMapSpanLo(sp, session, sm.config, context), context)
	moved := false
	defer sourceMapCloseSharedUnlessMoved(lo.File, &moved, context)
	sourceMapTraceEvent(context, sourceMapTraceLo, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "lo", Value: rusttracing.DebugField(lo.WriteDebugFormatter)}}
	})
	hi := sm.LookupCharPosWithContext(sourceMapSpanHi(sp, session, sm.config, context), context)
	defer sourceMapCloseSharedUnlessMoved(hi.File, &moved, context)
	sourceMapTraceEvent(context, sourceMapTraceHi, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "hi", Value: rusttracing.DebugField(hi.WriteDebugFormatter)}}
	})
	if lo.File.Borrow().StartPos != hi.File.Borrow().StartPos {
		return Loc{}, Loc{}, DistinctSourcesSpanLinesError(sourceMapDistinct(lo.File.Borrow(), hi.File.Borrow()))
	}
	moved = true
	return lo, hi, nil
}
func (sm *SourceMap) SpanToLines(session *SessionGlobals, sp Span) (FileLines, *SpanLinesError) {
	return sm.SpanToLinesWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToLinesWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) (FileLines, *SpanLinesError) {
	sourceMapTraceMessage(context, sourceMapTraceLines, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("span_to_lines(sp="); err != nil {
			return err
		}
		if err := sp.WriteDebugFormatter(session, f, sm.config); err != nil {
			return err
		}
		return f.WriteString(")")
	})
	lo, hi, failure := sm.IsValidSpanWithContext(session, sp, context)
	if failure != nil {
		return FileLines{}, failure
	}
	moved := false
	defer sourceMapCloseSharedUnlessMoved(lo.File, &moved, context)
	defer sourceMapCloseSharedOnExit(hi.File, context)
	if hi.Line < lo.Line {
		sourceMapPanic(context, "rustc_span::source_map::span_to_lines.assert_order", rustfmt.PanicStaticStr, "assertion failed: hi.line >= lo.line")
	}
	if sp.IsDummyWithContext(session, context) {
		moved = true
		return FileLines{File: lo.File, Lines: []LineInfo{}}, nil
	}
	capacity := sourceMapUsizeAdd(hi.Line-lo.Line, 1, sm.config, context, "rustc_span::source_map::span_to_lines.capacity_add")
	// Vec<LineInfo> holds three usize fields, so its array layout must fit
	// isize before allocation, independently of source overflow-check flags.
	if capacity > (^uint(0)>>1)/(3*uint(strconv.IntSize/8)) {
		sourceMapPanic(context, rustvec.PanicCapacityOverflowSite, rustfmt.PanicStaticStr, "capacity overflow")
	}
	lines := make([]LineInfo, 0, int(capacity))
	start := lo.Col
	hiLine := uint(0)
	if hi.Line > 0 {
		hiLine = hi.Line - 1
	}
	loLine := uint(0)
	if lo.Line > 0 {
		loLine = lo.Line - 1
	}
	for index := loLine; index < hiLine; index++ {
		length := 0
		if text, ok := lo.File.Borrow().GetLineWithContext(index, context, sm.config).Get(); ok {
			length = utf8.RuneCountInString(text.String())
		}
		lines = append(lines, LineInfo{LineIndex: index, StartCol: start, EndCol: CharPos(length)})
		start = 0
	}
	lines = append(lines, LineInfo{LineIndex: hiLine, StartCol: start, EndCol: hi.Col})
	moved = true
	return FileLines{File: lo.File, Lines: lines}, nil
}
