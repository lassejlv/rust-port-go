// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit source panic sites for pinned caching_source_map_view.rs.
package rustcspan

import (
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Sites resolve through the current caller's immutable selected-build profile.
// No Rust filename or Go stack coordinate is inferred by the implementation.
const (
	CachingPanicConstructorIndex = "rustc_span::caching_source_map_view::new.first_file_index"
	CachingPanicLookupIndex      = "rustc_span::caching_source_map_view::file_for_position.file_index"
	CachingPanicLineUnwrap       = "rustc_span::caching_source_map_view::pos_to_line.lookup_unwrap"
	CachingPanicLineNumberAdd    = "rustc_span::caching_source_map_view::pos_to_line.line_number_add"
	CachingPanicColumnSubtract   = "rustc_span::caching_source_map_view::byte_pos_to_line_and_col.column_subtract"
	CachingPanicSpanLoLower      = "rustc_span::caching_source_map_view::span_data.lo_lower_bound"
	CachingPanicSpanLoUpper      = "rustc_span::caching_source_map_view::span_data.lo_upper_bound"
	CachingPanicSpanHiLower      = "rustc_span::caching_source_map_view::span_data.hi_lower_bound"
	CachingPanicSpanHiUpper      = "rustc_span::caching_source_map_view::span_data.hi_upper_bound"
	CachingPanicContainsLo       = "rustc_span::caching_source_map_view::span_data.file_contains_lo"
	CachingPanicContainsHi       = "rustc_span::caching_source_map_view::span_data.file_contains_hi"
)

func cachingStaticPanic(context rustfmt.Context, site, payload string) {
	context.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicStaticStr, Payload: payload, Location: context.SourcePanicLocation(site), CanUnwind: true})
}

// Indexing belongs to the original caching caller. The SourceMap accessor is
// borrowed only after this source bounds check, retaining the actual Arc cell.
func cachingBorrowSourceFile(files *SourceMapFilesReadGuard, index uint, context rustfmt.Context, site string) *Shared[SourceFile] {
	length := files.Len()
	if index >= length {
		context.PanicAt(rustfmt.SourcePanicInfo{
			Kind:     rustfmt.PanicOwnedString,
			Payload:  "index out of bounds: the len is " + strconv.FormatUint(uint64(length), 10) + " but the index is " + strconv.FormatUint(uint64(index), 10),
			Location: context.SourcePanicLocation(site), CanUnwind: true,
		})
	}
	return files.BorrowWithContext(index, context)
}

func cachingColumnSubtract(left, right BytePos, config SpanEncodingConfig, context rustfmt.Context) RelativeBytePos {
	if config.OverflowChecks && left < right {
		cachingStaticPanic(context, CachingPanicColumnSubtract, "attempt to subtract with overflow")
	}
	return RelativeBytePos(uint32(left) - uint32(right))
}
