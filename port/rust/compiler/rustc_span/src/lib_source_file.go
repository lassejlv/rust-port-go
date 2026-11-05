// SPDX-License-Identifier: MIT OR Apache-2.0
// SourceFile algorithms from pinned compiler/rustc_span/src/lib.rs.
package rustcspan

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	unicodewidth "github.com/lassejlv/rust-port-go/port/dependencies/unicode-width/v0_2_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

func NewSourceFile(name FileName, src string, hashKind SourceFileHashAlgorithm, checksumKind Option[SourceFileHashAlgorithm]) (SourceFile, error) {
	return NewSourceFileWithContext(name, src, hashKind, checksumKind, rustfmt.Context{})
}
func NewSourceFileWithContext(name FileName, src string, hashKind SourceFileHashAlgorithm, checksumKind Option[SourceFileHashAlgorithm], context rustfmt.Context, config ...SpanEncodingConfig) (SourceFile, error) {
	requireSpanUTF8(src, context)
	profile := spanEncodingConfig(nil, config...)
	srcHash := NewSourceFileHashInMemory(hashKind, []byte(src))
	checksum := None[SourceFileHash]()
	if kind, ok := checksumKind.Get(); ok {
		if kind == hashKind {
			checksum = Some(srcHash)
		} else {
			checksum = Some(NewSourceFileHashInMemory(kind, []byte(src)))
		}
	}
	if uint64(len(src)) > uint64(^uint32(0)-1) {
		return SourceFile{}, OffsetOverflowError{}
	}
	unnormalizedLen := uint32(len(src))
	normalized := normalizeSourceWithContext(&src, context, profile)
	stableId := StableSourceFileIdFromFilenameInCurrentCrate(&name)
	if uint64(len(src)) > uint64(^uint32(0)-1) {
		return SourceFile{}, OffsetOverflowError{}
	}
	analysisConfig := DefaultSourceFileAnalysisConfig()
	analysisConfig.DebugAssertions = profile.DebugAssertions
	analysisConfig.OverflowChecks = profile.OverflowChecks
	lines, multibyte := AnalyzeSourceFileWithContext(context, src, analysisConfig)
	return SourceFile{Name: name, Src: Some(NewSharedString(src)), SrcHash: srcHash, ChecksumHash: checksum, ExternalSrc: NewFreezeLockWithDrop(UnneededExternalSource(), true, func(value *ExternalSource, ctx rustfmt.Context) { value.CloseWithContext(ctx) }), NormalizedSourceLen: RelativeBytePosFromU32(uint32(len(src))), UnnormalizedSourceLen: unnormalizedLen, Lines: FrozenFreezeLock(LinesSourceFileLines(lines)), MultibyteChars: multibyte, NormalizedPos: normalized, StableId: stableId, Cnum: LocalCrate()}, nil
}
func (value *SourceFile) Clone() SourceFile { return value.CloneWithContext(rustfmt.Context{}) }
func (value *SourceFile) CloneWithContext(context rustfmt.Context) (out SourceFile) {
	out.Name = value.Name.Clone()
	// The partially initialized aggregate owns each completed field. Cleanup on
	// a later field's panic matches source struct construction.
	complete := false
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			if !complete {
				unwind := context
				unwind.Unwinding = true
				spanCleanupDuringUnwindWithContext(unwind, func() { out.CloseWithContext(unwind) })
			}
			context.ResumeUnwind(payload)
		}
	}()
	if src, ok := value.Src.Get(); ok {
		out.Src = Some(src.CloneWithContext(context))
	}
	out.SrcHash = value.SrcHash
	out.ChecksumHash = value.ChecksumHash
	out.ExternalSrc = value.ExternalSrc.CloneWithContext(func(v *ExternalSource, ctx rustfmt.Context) ExternalSource { return v.CloneWithContext(ctx) }, context)
	out.StartPos = value.StartPos
	out.NormalizedSourceLen = value.NormalizedSourceLen
	out.UnnormalizedSourceLen = value.UnnormalizedSourceLen
	out.Lines = value.Lines.CloneWithContext(func(v *SourceFileLines, _ rustfmt.Context) SourceFileLines { return v.Clone() }, context)
	out.MultibyteChars = make([]MultiByteChar, len(value.MultibyteChars))
	copy(out.MultibyteChars, value.MultibyteChars)
	out.NormalizedPos = make([]NormalizedPos, len(value.NormalizedPos))
	copy(out.NormalizedPos, value.NormalizedPos)
	out.StableId = value.StableId
	out.Cnum = value.Cnum
	complete = true
	return out
}
func (value *SourceFile) convertDiffsToLinesFrozen(context rustfmt.Context, profile SpanEncodingConfig) {
	guard, ok := value.Lines.TryWriteWithContext(context)
	if !ok {
		return
	}
	defer guard.Close()
	current := guard.Get()
	if current.IsLines() {
		guard.Freeze()
		return
	}
	diffs := &current.diffs
	numLines := spanCharPosAdd(diffs.numDiffs, 1, profile, context)
	if numLines > uint(^uint(0)>>1)/4 {
		context.Panic("capacity overflow")
	}
	lines := make([]RelativeBytePos, 0, numLines)
	lines = append(lines, 0)
	lineStart := RelativeBytePos(0)
	if diffs.bytesPerDiff == 0 {
		spanStaticPanicAt(context, spanLibLocation(2281, 32), "attempt to divide by zero")
	}
	if diffs.numDiffs != uint(len(diffs.rawDiffs))/diffs.bytesPerDiff {
		spanFormattedPanicAt(context, spanLibLocation(2281, 9), func(rustfmt.Context) any {
			return fmt.Sprintf("assertion `left == right` failed\n  left: %d\n right: %d", diffs.numDiffs, uint(len(diffs.rawDiffs))/diffs.bytesPerDiff)
		})
	}
	add := func(diff uint32) {
		lineStart = lineStart.AddWithContext(RelativeBytePos(diff), profile, context)
		lines = append(lines, lineStart)
	}
	switch diffs.bytesPerDiff {
	case 1:
		for _, diff := range diffs.rawDiffs {
			add(uint32(diff))
		}
	case 2, 4:
		for i := uint(0); i < diffs.numDiffs; i++ {
			if profile.OverflowChecks && i > ^uint(0)/diffs.bytesPerDiff {
				context.Panic("attempt to multiply with overflow")
			}
			pos := diffs.bytesPerDiff * i
			end := spanCharPosAdd(pos, diffs.bytesPerDiff, profile, context)
			if end > uint(len(diffs.rawDiffs)) {
				context.Panic("index out of bounds")
			}
			if diffs.bytesPerDiff == 2 {
				add(uint32(binary.LittleEndian.Uint16(diffs.rawDiffs[pos:end])))
			} else {
				add(binary.LittleEndian.Uint32(diffs.rawDiffs[pos:end]))
			}
		}
	default:
		spanStaticPanicAt(context, spanLibLocation(2312, 18), "internal error: entered unreachable code")
	}
	*current = LinesSourceFileLines(lines)
	guard.Freeze()
}
func (value *SourceFile) LinesWithContext(context rustfmt.Context, config ...SpanEncodingConfig) []RelativeBytePos {
	profile := spanEncodingConfig(nil, config...)
	if lines, ok := value.Lines.Get(); ok && lines.IsLines() {
		return lines.lines
	}
	value.convertDiffsToLinesFrozen(context, profile)
	if lines, ok := value.Lines.Get(); ok && lines.IsLines() {
		return lines.lines
	}
	context.Panic("internal error: entered unreachable code")
	return nil
}
func (value *SourceFile) IsEmpty() bool    { return value.NormalizedSourceLen.ToU32() == 0 }
func (value *SourceFile) IsImported() bool { return !value.Src.IsSome() }
func (value *SourceFile) IsRealFile() bool { return value.Name.IsReal() }
func (value *SourceFile) CountLines() uint {
	return uint(len(value.LinesWithContext(rustfmt.Context{})))
}
func (value *SourceFile) AbsolutePosition(pos RelativeBytePos, config ...SpanEncodingConfig) BytePos {
	return value.AbsolutePositionWithContext(pos, rustfmt.Context{}, config...)
}
func (value *SourceFile) AbsolutePositionWithContext(pos RelativeBytePos, context rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	return BytePos(spanPosAdd(pos.ToU32(), value.StartPos.ToU32(), spanEncodingConfig(nil, config...), context, spanLibLocation(2427, 27)))
}
func (value *SourceFile) RelativePosition(pos BytePos, config ...SpanEncodingConfig) RelativeBytePos {
	return value.RelativePositionWithContext(pos, rustfmt.Context{}, config...)
}
func (value *SourceFile) RelativePositionWithContext(pos BytePos, context rustfmt.Context, config ...SpanEncodingConfig) RelativeBytePos {
	return RelativeBytePos(spanPosSub(pos.ToU32(), value.StartPos.ToU32(), spanEncodingConfig(nil, config...), context, spanLibLocation(2432, 35)))
}
func (value *SourceFile) EndPosition(config ...SpanEncodingConfig) BytePos {
	return value.EndPositionWithContext(rustfmt.Context{}, config...)
}
func (value *SourceFile) EndPositionWithContext(context rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	return value.AbsolutePositionWithContext(value.NormalizedSourceLen, context, config...)
}
func (value *SourceFile) Contains(pos BytePos, config ...SpanEncodingConfig) bool {
	return value.ContainsWithContext(pos, rustfmt.Context{}, config...)
}
func (value *SourceFile) ContainsWithContext(pos BytePos, context rustfmt.Context, config ...SpanEncodingConfig) bool {
	return pos >= value.StartPos && pos <= value.EndPositionWithContext(context, config...)
}
func (value *SourceFile) LookupLine(pos RelativeBytePos) Option[uint] {
	return value.LookupLineWithContext(pos, rustfmt.Context{})
}
func (value *SourceFile) LookupLineWithContext(pos RelativeBytePos, context rustfmt.Context, config ...SpanEncodingConfig) Option[uint] {
	lines := value.LinesWithContext(context, config...)
	index := sort.Search(len(lines), func(i int) bool { return lines[i] > pos })
	if index == 0 {
		return None[uint]()
	}
	return Some(uint(index - 1))
}
func (value *SourceFile) LineBounds(line uint, config ...SpanEncodingConfig) BytePosRange {
	return value.LineBoundsWithContext(line, rustfmt.Context{}, config...)
}
func (value *SourceFile) LineBoundsWithContext(line uint, context rustfmt.Context, config ...SpanEncodingConfig) BytePosRange {
	if value.IsEmpty() {
		return BytePosRange{value.StartPos, value.StartPos}
	}
	lines := value.LinesWithContext(context, config...)
	if line >= uint(len(lines)) {
		spanStaticPanicAt(context, spanLibLocation(2454, 9), "assertion failed: line_index < lines.len()")
	}
	start := value.AbsolutePositionWithContext(lines[line], context, config...)
	if line == uint(len(lines)-1) {
		return BytePosRange{start, value.EndPositionWithContext(context, config...)}
	}
	return BytePosRange{start, value.AbsolutePositionWithContext(lines[line+1], context, config...)}
}
func (value *SourceFile) LineBeginPos(pos BytePos, config ...SpanEncodingConfig) BytePos {
	return value.LineBeginPosWithContext(pos, rustfmt.Context{}, config...)
}
func (value *SourceFile) LineBeginPosWithContext(pos BytePos, context rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	relative := value.RelativePositionWithContext(pos, context, config...)
	index, ok := value.LookupLineWithContext(relative, context, config...).Get()
	if !ok {
		spanStaticPanicAt(context, spanLibLocation(2337, 48), "called `Option::unwrap()` on a `None` value")
	}
	return value.AbsolutePositionWithContext(value.LinesWithContext(context, config...)[index], context, config...)
}

func (value *SourceFile) GetLine(line uint) Option[CowString] {
	return value.GetLineWithContext(line, rustfmt.Context{})
}

// AddExternalSrc is the trivial-destructor function convenience. The primary
// generic form below consumes a real owned environment, including the unused
// callback case when another thread has already frozen the external source.
func (value *SourceFile) AddExternalSrc(get func() Option[string]) bool {
	return value.AddExternalSrcWithContext(get, rustfmt.Context{})
}
func (value *SourceFile) AddExternalSrcWithContext(get func() Option[string], context rustfmt.Context, config ...SpanEncodingConfig) bool {
	owned := NewSpanOwned(struct{}{}, nil)
	return AddExternalSrcOwned(value, owned, func(*struct{}, rustfmt.Context) Option[string] { return get() }, context, config...)
}
func AddExternalSrcOwned[E any](value *SourceFile, environment *SpanOwned[E], call func(*E, rustfmt.Context) Option[string], context rustfmt.Context, config ...SpanEncodingConfig) bool {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	if !value.ExternalSrc.IsFrozen() {
		source := func() Option[string] {
			defer spanCloseOwnedCallback(context, env.DropWithContext)
			return call(env.Borrow(), context)
		}()
		if src, ok := source.Get(); ok {
			requireSpanUTF8(src, context)
			if value.SrcHash.MatchesWithContext(src, context) {
				normalizeSourceWithContext(&src, context, spanEncodingConfig(nil, config...))
				source = Some(src)
			} else {
				source = None[string]()
			}
		}
		if guard, ok := value.ExternalSrc.TryWriteWithContext(context); ok {
			func() {
				defer guard.Close()
				external := guard.Get()
				if !external.foreign || !external.kind.IsAbsentOk() {
					// panic!("...{:?}", value) begins processing its payload before
					// calling Debug. Keep the write guard held through that work;
					// a panic during an existing panic hook aborts at this site.
					spanFormattedPanicAt(context, spanLibLocation(2372, 21), func(processing rustfmt.Context) any {
						var text strings.Builder
						text.WriteString("unexpected state ")
						formatter := &rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: processing}
						_ = external.WriteDebugFormatter(formatter)
						return text.String()
					})
				}
				if src, present := source.Get(); present {
					external.kind = PresentExternalSourceKind(NewSharedString(src))
				} else {
					external.kind = AbsentErrExternalSourceKind()
				}
				guard.Freeze()
			}()
		}
	}
	if value.Src.IsSome() {
		return true
	}
	guard := value.ExternalSrc.ReadWithContext(context)
	defer guard.Close()
	return guard.Get().GetSource().IsSome()
}

func (value *SourceFile) byteposToFileCharpos(bpos RelativeBytePos, context rustfmt.Context, profile SpanEncodingConfig) CharPos {
	totalExtra := uint32(0)
	for index := range value.MultibyteChars {
		mbc := &value.MultibyteChars[index]
		libTraceMultiByteMessage(context, mbc)
		if mbc.Pos < bpos {
			extra := spanPosSub(uint32(mbc.Bytes), 1, profile, context, spanLibLocation(2523, 38))
			totalExtra = spanPosAdd(totalExtra, extra, profile, context, spanLibLocation(2523, 17))
			end := spanPosAdd(mbc.Pos.ToU32(), uint32(mbc.Bytes), profile, context, spanLibLocation(2526, 42))
			if bpos.ToU32() < end {
				spanStaticPanicAt(context, spanLibLocation(2526, 17), "assertion failed: bpos.to_u32() >= mbc.pos.to_u32() + mbc.bytes as u32")
			}
		} else {
			break
		}
	}
	if totalExtra > bpos.ToU32() {
		spanStaticPanicAt(context, spanLibLocation(2532, 9), "assertion failed: total_extra_bytes <= bpos.to_u32()")
	}
	return CharPos(spanCharPosSub(bpos.ToUsize(), uint(totalExtra), profile, context))
}
func (value *SourceFile) lookupFilePos(pos RelativeBytePos, context rustfmt.Context, profile SpanEncodingConfig) (uint, CharPos) {
	chpos := value.byteposToFileCharpos(pos, context, profile)
	if index, ok := value.LookupLineWithContext(pos, context, profile).Get(); ok {
		line := spanCharPosAdd(index, 1, profile, context)
		linebpos := value.LinesWithContext(context, profile)[index]
		linechpos := value.byteposToFileCharpos(linebpos, context, profile)
		col := chpos.SubWithContext(linechpos, profile, context)
		libTraceBytePosMessage(context, pos, linebpos)
		libTraceCharPosMessage(context, chpos, linechpos)
		libTraceLineMessage(context, line)
		if chpos < linechpos {
			context.Panic("assertion failed: chpos >= linechpos")
		}
		return line, col
	}
	return 0, chpos
}
func (value *SourceFile) LookupFilePosWithColDisplay(pos BytePos) (uint, CharPos, uint) {
	return value.LookupFilePosWithColDisplayWithContext(pos, rustfmt.Context{})
}
func (value *SourceFile) LookupFilePosWithColDisplayWithContext(pos BytePos, context rustfmt.Context, config ...SpanEncodingConfig) (uint, CharPos, uint) {
	profile := spanEncodingConfig(nil, config...)
	relative := value.RelativePositionWithContext(pos, context, profile)
	line, col := value.lookupFilePos(relative, context, profile)
	if line > 0 {
		code, ok := value.GetLineWithContext(line-1, context, profile).Get()
		if !ok {
			libTraceUnavailableMessage(context, line, &value.Name)
			return line, col, uint(col)
		}
		display := uint(0)
		count := uint(0)
		for _, ch := range code.String() {
			if count >= uint(col) {
				break
			}
			display = spanCharPosAdd(display, CharWidth(ch), profile, context)
			count++
		}
		return line, col, display
	}
	return 0, col, uint(col)
}
func (value *SourceFile) GetLineWithContext(line uint, context rustfmt.Context, config ...SpanEncodingConfig) Option[CowString] {
	lines := value.LinesWithContext(context, config...)
	if line >= uint(len(lines)) {
		return None[CowString]()
	}
	begin := lines[line].ToUsize()
	until := func(src string) string {
		tail := ruststr.SliceFrom(src, begin, context, LibPanicSite(2390, 29))
		if end := strings.IndexByte(tail, '\n'); end >= 0 {
			return tail[:end]
		}
		return tail
	}
	if src, ok := value.Src.Get(); ok {
		return Some(BorrowedCowString(until(src.String())))
	}
	guard := value.ExternalSrc.ReadWithContext(context)
	defer guard.Close()
	if src, ok := guard.Get().GetSource().Get(); ok {
		return Some(OwnedCowString(until(src)))
	}
	return None[CowString]()
}
func (value *SourceFile) OriginalRelativeBytePos(pos BytePos, config ...SpanEncodingConfig) RelativeBytePos {
	return value.OriginalRelativeBytePosWithContext(pos, rustfmt.Context{}, config...)
}
func (value *SourceFile) OriginalRelativeBytePosWithContext(pos BytePos, context rustfmt.Context, config ...SpanEncodingConfig) RelativeBytePos {
	profile := spanEncodingConfig(nil, config...)
	relative := value.RelativePositionWithContext(pos, context, profile)
	index, found := sourceNormalizedSearch(value.NormalizedPos, func(p NormalizedPos) int {
		if p.Pos < relative {
			return -1
		}
		if p.Pos > relative {
			return 1
		}
		return 0
	})
	diff := uint32(0)
	if found {
		diff = value.NormalizedPos[index].Diff
	} else if index > 0 {
		diff = value.NormalizedPos[index-1].Diff
	}
	return RelativeBytePos(spanPosAdd(uint32(relative), diff, profile, context, spanLibLocation(2490, 35)))
}
func (value *SourceFile) NormalizedBytePos(offset uint32, config ...SpanEncodingConfig) BytePos {
	return value.NormalizedBytePosWithContext(offset, rustfmt.Context{}, config...)
}
func (value *SourceFile) NormalizedBytePosWithContext(offset uint32, context rustfmt.Context, config ...SpanEncodingConfig) BytePos {
	profile := spanEncodingConfig(nil, config...)
	index, found := sourceNormalizedSearch(value.NormalizedPos, func(p NormalizedPos) int {
		n := spanPosAdd(uint32(p.Pos), p.Diff, profile, context, spanLibLocation(2504, 61))
		if n < offset {
			return -1
		}
		if n > offset {
			return 1
		}
		return 0
	})
	diff := uint32(0)
	if found {
		diff = value.NormalizedPos[index].Diff
	} else if index > 0 {
		diff = value.NormalizedPos[index-1].Diff
	}
	return BytePos(spanPosSub(spanPosAdd(uint32(value.StartPos), offset, profile, context, spanLibLocation(2510, 27)), diff, profile, context, spanLibLocation(2510, 27)))
}

// This is std slice binary_search_by's source algorithm, including duplicate
// selection. NormalizedPos is public and may contain duplicate positions.
func sourceNormalizedSearch(values []NormalizedPos, compare func(NormalizedPos) int) (int, bool) {
	size := len(values)
	if size == 0 {
		return 0, false
	}
	base := 0
	for size > 1 {
		half := size / 2
		mid := base + half
		if compare(values[mid]) <= 0 {
			base = mid
		}
		size -= half
	}
	c := compare(values[base])
	if c == 0 {
		return base, true
	}
	if c < 0 {
		return base + 1, false
	}
	return base, false
}

func CharWidth(ch rune) uint {
	if ch < 0 || ch > utf8.MaxRune || ch >= 0xd800 && ch <= 0xdfff {
		panic("rustcspan: char requires a Unicode scalar")
	}
	if ch == '\t' {
		return 4
	}
	if ch < 0x20 && ch != '\n' || ch == 0x7f || ch >= 0x202a && ch <= 0x202e || ch >= 0x2066 && ch <= 0x2069 {
		return 1
	}
	if width, ok := unicodewidth.CharWidth(ch); ok {
		return width
	}
	return 1
}
func StrWidth(text string) uint {
	return StrWidthWithContext(text, rustfmt.Context{})
}
func StrWidthWithContext(text string, context rustfmt.Context, config ...SpanEncodingConfig) uint {
	requireSpanUTF8(text, context)
	profile := spanEncodingConfig(nil, config...)
	sum := uint(0)
	for _, ch := range text {
		sum = spanCharPosAdd(sum, CharWidth(ch), profile, context)
	}
	return sum
}
func UnicodeVersion() [3]uint8 { return unicodewidth.UnicodeVersion() }

func normalizeSourceWithContext(src *string, context rustfmt.Context, profile SpanEncodingConfig) []NormalizedPos {
	records := []NormalizedPos{}
	if strings.HasPrefix(*src, "\ufeff") {
		*src = (*src)[3:]
		analysisPush(&records, NormalizedPos{Pos: 0, Diff: 3}, 8, context)
	}
	normalizeSourceNewlines(src, &records, context, profile)
	return records
}
func normalizeSourceNewlines(src *string, records *[]NormalizedPos, context rustfmt.Context, profile SpanEncodingConfig) {
	if !strings.ContainsRune(*src, '\r') {
		return
	}
	buf := []byte(*src)
	*src = ""
	gap := uint(0)
	tail := buf
	cursor := uint(0)
	originalGap := uint32(0)
	if n := len(*records); n > 0 {
		originalGap = (*records)[n-1].Diff
	}
	for {
		idx := uint(len(tail))
		for i := gap; i+1 < uint(len(tail)); i++ {
			if tail[i] == '\r' && tail[i+1] == '\n' {
				idx = i
				break
			}
		}
		copy(tail, tail[gap:idx])
		tail = tail[idx-gap:]
		if uint(len(tail)) == gap {
			break
		}
		cursor = spanCharPosAdd(cursor, idx-gap, profile, context)
		gap = spanCharPosAdd(gap, 1, profile, context)
		pos := spanCharPosAdd(cursor, 1, profile, context)
		diff := spanPosAdd(originalGap, uint32(gap), profile, context)
		analysisPush(records, NormalizedPos{Pos: RelativeBytePosFromUsize(pos), Diff: diff}, 8, context)
	}
	*src = string(buf[:uint(len(buf))-gap])
}
