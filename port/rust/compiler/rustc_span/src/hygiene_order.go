// SPDX-License-Identifier: MIT OR Apache-2.0
// Concrete inherited PartialOrd/Ord operations for the source hygiene types.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const HygieneOrdClampPanicSite = "rustc_span::hygiene::Ord::clamp"

func (v Transparency) Less(other Transparency) bool         { return v < other }
func (v Transparency) LessEqual(other Transparency) bool    { return v <= other }
func (v Transparency) Greater(other Transparency) bool      { return v > other }
func (v Transparency) GreaterEqual(other Transparency) bool { return v >= other }

func (v ExpnIndex) Less(other ExpnIndex) bool         { return v.Compare(other) < 0 }
func (v ExpnIndex) LessEqual(other ExpnIndex) bool    { return v.Compare(other) <= 0 }
func (v ExpnIndex) Greater(other ExpnIndex) bool      { return v.Compare(other) > 0 }
func (v ExpnIndex) GreaterEqual(other ExpnIndex) bool { return v.Compare(other) >= 0 }
func (v ExpnIndex) Min(other ExpnIndex) ExpnIndex {
	if v.LessEqual(other) {
		return v
	}
	return other
}
func (v ExpnIndex) Max(other ExpnIndex) ExpnIndex {
	if v.GreaterEqual(other) {
		return v
	}
	return other
}
func (v ExpnIndex) Clamp(minimum, maximum ExpnIndex) ExpnIndex {
	return v.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (v ExpnIndex) ClampWithContext(minimum, maximum ExpnIndex, ctx rustfmt.Context) ExpnIndex {
	if minimum.Greater(maximum) {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "assertion failed: min <= max", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(HygieneOrdClampPanicSite)})
	}
	if v.Less(minimum) {
		return minimum
	}
	if v.Greater(maximum) {
		return maximum
	}
	return v
}

func (v MacroKind) Less(other MacroKind) bool         { return v.Compare(other) < 0 }
func (v MacroKind) LessEqual(other MacroKind) bool    { return v.Compare(other) <= 0 }
func (v MacroKind) Greater(other MacroKind) bool      { return v.Compare(other) > 0 }
func (v MacroKind) GreaterEqual(other MacroKind) bool { return v.Compare(other) >= 0 }
func (v MacroKind) Min(other MacroKind) MacroKind {
	if v.LessEqual(other) {
		return v
	}
	return other
}
func (v MacroKind) Max(other MacroKind) MacroKind {
	if v.GreaterEqual(other) {
		return v
	}
	return other
}
func (v MacroKind) Clamp(minimum, maximum MacroKind) MacroKind {
	return v.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (v MacroKind) ClampWithContext(minimum, maximum MacroKind, ctx rustfmt.Context) MacroKind {
	if minimum.Greater(maximum) {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "assertion failed: min <= max", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(HygieneOrdClampPanicSite)})
	}
	if v.Less(minimum) {
		return minimum
	}
	if v.Greater(maximum) {
		return maximum
	}
	return v
}
