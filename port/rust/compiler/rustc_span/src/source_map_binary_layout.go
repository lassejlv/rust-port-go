// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK Arc<[MaybeUninit<u8>]> allocation preflight used by RealFileLoader.
// No Arc/allocator source-file completion or Go/Rust physical layout is claimed.
package rustcspan

import (
	"strconv"
	"strings"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const (
	SourceMapPanicBinaryArrayLayout = "std::sync::Arc::new_uninit_slice.array_layout"
	SourceMapPanicBinaryArcCapacity = "std::sync::Arc::new_uninit_slice.header_capacity"
)

// The selected SDK's repr(C) ArcInner<()> has two atomic usize counters.
// Layout::array<u8> checks isize first; extend checks the counters plus final
// usize alignment. Metadata's u64-to-usize cast happens before either check.
func sourceMapBinaryArcLayout(metadata uint64, bits uint) (length, size, alignment uint64, kind string) {
	if bits != 32 && bits != 64 {
		panic("source-map: unsupported source usize width")
	}
	length = metadata
	if bits == 32 {
		length = uint64(uint32(metadata))
	}
	maximum := uint64(1)<<(bits-1) - 1
	alignment = uint64(bits / 8)
	if length > maximum {
		return length, 0, alignment, "array"
	}
	if length > maximum-(3*alignment-1) {
		return length, 0, alignment, "header"
	}
	size = (2*alignment + length + alignment - 1) &^ (alignment - 1)
	return length, size, alignment, "ok"
}
func sourceMapBinaryArcLength(metadata uint64, context rustfmt.Context) uint {
	length, _, _, kind := sourceMapBinaryArcLayout(metadata, uint(strconv.IntSize))
	switch kind {
	case "array":
		context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(SourceMapPanicBinaryArrayLayout), CanUnwind: true}, func(rustfmt.Context) any {
			return strings.Clone("called `Result::unwrap()` on an `Err` value: LayoutError")
		})
	case "header":
		sourceMapPanic(context, SourceMapPanicBinaryArcCapacity, rustfmt.PanicStaticStr, "capacity overflow")
	}
	return uint(length)
}
