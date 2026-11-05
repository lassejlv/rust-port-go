// SPDX-License-Identifier: MIT OR Apache-2.0
// Source panic sites owned by symbol.rs and its edit-distance implementation.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// These keys resolve only caller-supplied selected-build profiles. No native
// filename or compiler/std location is inferred by these source adapters.
const (
	PanicSymbolIndexSite            = "rustc_span.symbol.index_from_u32"
	PanicSymbolClampSite            = "rustc_span.symbol.clamp"
	PanicSymbolInternerBoundsSite   = "rustc_span.symbol.interner_index"
	PanicSymbolDuplicatePrefillSite = "rustc_span.symbol.prefill_duplicate"
	PanicSymbolSessionUnsetSite     = "rustc_span.symbol.session_unset"
	PanicIdentRawLifetimeSite       = "rustc_span.symbol.raw_lifetime_expect"
	PanicSymbolFormattingSite       = "rustc_span.symbol.to_string_error"
	PanicEditDistanceLimitAddSite   = "rustc_span.edit_distance.limit_add"
	PanicEditDistanceScoreAddSite   = "rustc_span.edit_distance.score_add"
	PanicEditDistanceLengthMulSite  = "rustc_span.edit_distance.length_mul"
)

func symbolStaticPanic(context rustfmt.Context, site, message string) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: message, Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(site), CanUnwind: true})
}
func symbolOwnedPanic(context rustfmt.Context, site string, render func(rustfmt.Context) any) {
	context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(site), CanUnwind: true}, render)
}

// ToString's generic Display fallback is selected by Ident::to_string.
// Its source fmt::Error Debug is the infallible literal Error.
func symbolToStringError(context rustfmt.Context) {
	symbolOwnedPanic(context, PanicSymbolFormattingSite, func(rustfmt.Context) any { return "a Display implementation returned an error unexpectedly: Error" })
}
