// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// These semantic keys accept the ACTUAL selected source profile's locations.
// Constructor/add sites are the newtype macro invocation. ExpectLocal is
// #[track_caller], so that key must identify the current source caller, not the
// line containing def_id.rs's panic expression. Missing metadata stays nil;
// these adapters never invent a Rust filename or inspect the Go stack.
const (
	CrateNumPanicSite          = "rustc_span.def_id.CrateNum.newtype"
	DefIndexPanicSite          = "rustc_span.def_id.DefIndex.newtype"
	DefIdStepForwardPanicSite  = "rustc_span.def_id.Step.forward"
	DefIdStepBackwardPanicSite = "rustc_span.def_id.Step.backward"
	DefIdExpectLocalPanicSite  = "rustc_span.def_id.expect_local.caller"
	DefIdFormatErrorPanicSite  = "rustc_span.def_id.format_error"
	DefIdIdxPlusPanicSite      = "source:rustc_index/src/idx.rs:22:19"
	LocalDefIdIdxPlusPanicSite = DefIdIdxPlusPanicSite
	ModIdExpectLocalPanicSite  = "rustc_span.def_id.ModId.expect_local"
)

func defIdRaise(context rustfmt.Context, site, message string, kind rustfmt.PanicPayloadKind) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: message, Kind: kind, Location: context.SourcePanicLocation(site), CanUnwind: true})
}
