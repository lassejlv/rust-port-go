// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit source panic-operation sites used by the complete SourceMap port.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
	"strings"
)

// These semantic keys resolve only through caller-supplied immutable profile
// metadata. No canonical filename or Go stack coordinate is inferred.
const (
	SourceMapPanicExpectCWD         = "rustc_span::source_map::with_inputs.expect_cwd"
	SourceMapPanicWorkingDirectory  = "rustc_span::source_map::to_real_filename.expect_local"
	SourceMapPanicLookupIndex       = "rustc_span::source_map::lookup_source_file_idx.subtract"
	SourceMapPanicDoctestNegate     = "rustc_span::source_map::doctest_offset_line.negate"
	SourceMapPanicDoctestSubtract   = "rustc_span::source_map::doctest_offset_line.subtract"
	SourceMapPanicDoctestAdd        = "rustc_span::source_map::doctest_offset_line.add"
	SourceMapPanicIndentSlice       = "rustc_span::source_map::indentation_before.slice"
	SourceMapPanicExtendSlice       = "rustc_span::source_map::span_extend_while.slice"
	SourceMapPanicExtendPrevSlice   = "rustc_span::source_map::span_extend_prev_while.slice"
	SourceMapPanicStartSlice        = "rustc_span::source_map::start_point.local_slice"
	SourceMapPanicStderr            = "std::io::stdio::print_to.stderr_failure"
	SourceMapPanicStableIdAssertion = "rustc_span::source_map::try_new_source_file.assert_stable_id"
)

func sourceMapPanic(context rustfmt.Context, site string, kind rustfmt.PanicPayloadKind, payload any) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: payload, Kind: kind, Location: context.SourcePanicLocation(site), CanUnwind: true})
}

func sourceMapAssertStableId(left, right *StableSourceFileId, config SpanEncodingConfig, context rustfmt.Context) {
	if !config.DebugAssertions || *left == *right {
		return
	}
	context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(SourceMapPanicStableIdAssertion), CanUnwind: true}, func(processing rustfmt.Context) any {
		var out strings.Builder
		out.WriteString("assertion `left == right` failed\n  left: ")
		if err := left.WriteDebugFormatter(&rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions(), Context: processing}); err != nil {
			processing.Panic(err)
		}
		out.WriteString("\n right: ")
		if err := right.WriteDebugFormatter(&rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions(), Context: processing}); err != nil {
			processing.Panic(err)
		}
		return out.String()
	})
}

// The full selected SDK str failure contract is supplied by the independently
// source-verified neutral helper. This adapter keeps each original SourceMap
// caller's explicit semantic site and returns the actual borrowed substring.
func sourceMapStringSlice(src string, begin, end uint, context rustfmt.Context, site string) string {
	return ruststr.Slice(src, begin, end, context, site)
}

func sourceMapU32Add(a, b uint32, config SpanEncodingConfig, context rustfmt.Context, site string) uint32 {
	if config.OverflowChecks && b > ^uint32(0)-a {
		sourceMapPanic(context, site, rustfmt.PanicStaticStr, "attempt to add with overflow")
	}
	return a + b
}
func sourceMapU32Sub(a, b uint32, config SpanEncodingConfig, context rustfmt.Context, site string) uint32 {
	if config.OverflowChecks && a < b {
		sourceMapPanic(context, site, rustfmt.PanicStaticStr, "attempt to subtract with overflow")
	}
	return a - b
}
func sourceMapUsizeAdd(a, b uint, config SpanEncodingConfig, context rustfmt.Context, site string) uint {
	if config.OverflowChecks && b > ^uint(0)-a {
		sourceMapPanic(context, site, rustfmt.PanicStaticStr, "attempt to add with overflow")
	}
	return a + b
}
func sourceMapUsizeSub(a, b uint, config SpanEncodingConfig, context rustfmt.Context, site string) uint {
	if config.OverflowChecks && a < b {
		sourceMapPanic(context, site, rustfmt.PanicStaticStr, "attempt to subtract with overflow")
	}
	return a - b
}
