// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: compiler/rustc_span/src/caching_source_map_view.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Full-file validation is pending
// integration with the real source-map/source-file runtime.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// CachingSourceMapView owns one cloned SourceFile handle and borrows its map.
// Close consumes that handle; copying the Go descriptor is not an ownership
// clone. Borrowed results must follow the source's immutable borrow lifetimes.
type CachingSourceMapView struct {
	sourceMap  *SourceMap
	file       Shared[SourceFile]
	lineBounds BytePosRange
	lineNumber uint
	closed     bool
}

// CachedLineAndColumn owns its File handle. The caller must Close it.
type CachedLineAndColumn struct {
	File   Shared[SourceFile]
	Line   uint
	Column RelativeBytePos
}

// CachedSpanLines borrows File from the view without incrementing its owner
// count. Its columns are byte offsets, including positions within UTF-8 scalars.
type CachedSpanLines struct {
	File     *SourceFile
	LoLine   uint
	LoColumn BytePos
	HiLine   uint
	HiColumn BytePos
}

func NewCachingSourceMapView(sourceMap *SourceMap) *CachingSourceMapView {
	return NewCachingSourceMapViewWithContext(sourceMap, rustfmt.Context{})
}

func NewCachingSourceMapViewWithContext(sourceMap *SourceMap, context rustfmt.Context) *CachingSourceMapView {
	files := sourceMap.FilesWithContext(context)
	defer files.Close()
	first := cachingBorrowSourceFile(files, 0, context, CachingPanicConstructorIndex).CloneWithContext(context)
	return &CachingSourceMapView{sourceMap: sourceMap, file: first}
}

func (view *CachingSourceMapView) Close() { view.CloseWithContext(rustfmt.Context{}) }
func (view *CachingSourceMapView) CloseWithContext(context rustfmt.Context) {
	if !view.closed {
		view.closed = true
		view.file.CloseWithContext(context)
	}
}

func (view *CachingSourceMapView) check(context rustfmt.Context) {
	if view.closed {
		context.Panic("caching source-map view: use after close")
	}
}

func cachingContains(bounds BytePosRange, pos BytePos) bool {
	return bounds.Start <= pos && pos < bounds.End
}

func cachingSubtract(left, right BytePos, config SpanEncodingConfig, context rustfmt.Context) BytePos {
	return left.SubWithContext(right, config, context)
}

func cachingFileContains(file *SourceFile, pos BytePos, config SpanEncodingConfig, context rustfmt.Context) bool {
	return file.ContainsWithContext(pos, context, config) && !file.IsEmpty()
}

func (view *CachingSourceMapView) posToLine(pos BytePos, context rustfmt.Context) (BytePosRange, uint) {
	config := view.sourceMap.BuildConfig()
	file := view.file.BorrowWithContext(context)
	relative := file.RelativePositionWithContext(pos, context, config)
	line, present := file.LookupLineWithContext(relative, context, config).Get()
	if !present {
		cachingStaticPanic(context, CachingPanicLineUnwrap, "called `Option::unwrap()` on a `None` value")
	}
	bounds := file.LineBoundsWithContext(line, context, config)
	if config.OverflowChecks && line == ^uint(0) {
		cachingStaticPanic(context, CachingPanicLineNumberAdd, "attempt to add with overflow")
	}
	return bounds, line + 1
}

func (view *CachingSourceMapView) update(newFile Option[Shared[SourceFile]], pos BytePos, context rustfmt.Context) {
	if file, present := newFile.Get(); present {
		// Assignment destroys the old Arc before installing the new one.
		func() {
			defer func() { view.file = file }()
			view.file.CloseWithContext(context)
		}()
	}
	// The file assignment survives a later lookup failure; line fields change
	// together only after the complete source RHS has been evaluated.
	bounds, line := view.posToLine(pos, context)
	view.lineBounds, view.lineNumber = bounds, line
}

func (view *CachingSourceMapView) BytePosToLineAndCol(pos BytePos) (CachedLineAndColumn, bool) {
	return view.BytePosToLineAndColWithContext(pos, rustfmt.Context{})
}

func (view *CachingSourceMapView) BytePosToLineAndColWithContext(pos BytePos, context rustfmt.Context) (CachedLineAndColumn, bool) {
	view.check(context)
	config := view.sourceMap.BuildConfig()
	if !cachingContains(view.lineBounds, pos) {
		newFile := None[Shared[SourceFile]]()
		if !cachingFileContains(view.file.BorrowWithContext(context), pos, config, context) {
			file, present := view.fileForPosition(pos, context)
			if !present {
				return CachedLineAndColumn{}, false
			}
			newFile = Some(file)
		}
		view.update(newFile, pos, context)
	}
	column := cachingColumnSubtract(pos, view.lineBounds.Start, config, context)
	return CachedLineAndColumn{File: view.file.CloneWithContext(context), Line: view.lineNumber, Column: column}, true
}

func (view *CachingSourceMapView) SpanDataToLinesAndCols(data *SpanData) (CachedSpanLines, bool) {
	return view.SpanDataToLinesAndColsWithContext(data, rustfmt.Context{})
}

func (view *CachingSourceMapView) SpanDataToLinesAndColsWithContext(data *SpanData, context rustfmt.Context) (CachedSpanLines, bool) {
	view.check(context)
	config := view.sourceMap.BuildConfig()
	loHit := cachingContains(view.lineBounds, data.Lo)
	hiHit := cachingContains(view.lineBounds, data.Hi)
	if loHit && hiHit {
		return CachedSpanLines{File: view.file.BorrowWithContext(context), LoLine: view.lineNumber, LoColumn: cachingSubtract(data.Lo, view.lineBounds.Start, config, context), HiLine: view.lineNumber, HiColumn: cachingSubtract(data.Hi, view.lineBounds.Start, config, context)}, true
	}
	newFile := None[Shared[SourceFile]]()
	ownsNewFile := false
	defer spanCloseOwnedCallback(context, func(dropContext rustfmt.Context) {
		if ownsNewFile {
			file, _ := newFile.Get()
			file.CloseWithContext(dropContext)
		}
	})
	if !cachingFileContains(view.file.BorrowWithContext(context), data.Lo, config, context) {
		file, present := view.fileForPosition(data.Lo, context)
		if !present {
			return CachedSpanLines{}, false
		}
		newFile = Some(file)
		ownsNewFile = true
		if !cachingFileContains(file.BorrowWithContext(context), data.Hi, config, context) {
			return CachedSpanLines{}, false
		}
	} else if !cachingFileContains(view.file.BorrowWithContext(context), data.Hi, config, context) {
		return CachedSpanLines{}, false
	}
	if !loHit {
		ownsNewFile = false // update consumes the optional Arc argument.
		view.update(newFile, data.Lo, context)
	}
	loBounds, loLine := view.lineBounds, view.lineNumber
	hiBounds, hiLine := view.lineBounds, view.lineNumber
	if !cachingContains(view.lineBounds, data.Hi) {
		hiBounds, hiLine = view.posToLine(data.Hi, context)
	}
	if data.Lo < loBounds.Start {
		cachingStaticPanic(context, CachingPanicSpanLoLower, "assertion failed: span_data.lo >= lo_line_bounds.start")
	}
	if data.Lo > loBounds.End {
		cachingStaticPanic(context, CachingPanicSpanLoUpper, "assertion failed: span_data.lo <= lo_line_bounds.end")
	}
	if data.Hi < hiBounds.Start {
		cachingStaticPanic(context, CachingPanicSpanHiLower, "assertion failed: span_data.hi >= hi_line_bounds.start")
	}
	if data.Hi > hiBounds.End {
		cachingStaticPanic(context, CachingPanicSpanHiUpper, "assertion failed: span_data.hi <= hi_line_bounds.end")
	}
	file := view.file.BorrowWithContext(context)
	if !file.ContainsWithContext(data.Lo, context, config) {
		cachingStaticPanic(context, CachingPanicContainsLo, "assertion failed: self.file.contains(span_data.lo)")
	}
	if !file.ContainsWithContext(data.Hi, context, config) {
		cachingStaticPanic(context, CachingPanicContainsHi, "assertion failed: self.file.contains(span_data.hi)")
	}
	return CachedSpanLines{File: file, LoLine: loLine, LoColumn: cachingSubtract(data.Lo, loBounds.Start, config, context), HiLine: hiLine, HiColumn: cachingSubtract(data.Hi, hiBounds.Start, config, context)}, true
}

func (view *CachingSourceMapView) fileForPosition(pos BytePos, context rustfmt.Context) (Shared[SourceFile], bool) {
	files := view.sourceMap.FilesWithContext(context)
	nonempty := files.Len() != 0
	files.Close()
	if !nonempty {
		return Shared[SourceFile]{}, false
	}
	index := view.sourceMap.LookupSourceFileIdxWithContext(pos, context)
	files = view.sourceMap.FilesWithContext(context)
	defer files.Close()
	file := cachingBorrowSourceFile(files, index, context, CachingPanicLookupIndex)
	if cachingFileContains(file.BorrowWithContext(context), pos, view.sourceMap.BuildConfig(), context) {
		return file.CloneWithContext(context), true
	}
	return Shared[SourceFile]{}, false
}
