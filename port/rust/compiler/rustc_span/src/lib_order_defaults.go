// SPDX-License-Identifier: MIT OR Apache-2.0
// Concrete derived PartialOrd and inherited Ord methods from pinned lib.rs.
// All listed concrete fields have total, non-overridable primitive/Path orders.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const LibOrdClampPanicSite = "rustc_span::Ord::clamp"

// min keeps its first equal owner; max keeps its second. No clone of an owned
// path/string is introduced. Callers relinquish the moved input values.
func libOrdMin[T interface{ Compare(T) int }](a, b T) T {
	if a.Compare(b) <= 0 {
		return a
	}
	return b
}
func libOrdMax[T interface{ Compare(T) int }](a, b T) T {
	if a.Compare(b) > 0 {
		return a
	}
	return b
}
func libOrdClamp[T interface{ Compare(T) int }](value, minimum, maximum T, ctx rustfmt.Context) T {
	if minimum.Compare(maximum) > 0 {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "assertion failed: min <= max", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation(LibOrdClampPanicSite), CanUnwind: true})
	}
	if value.Compare(minimum) < 0 {
		return minimum
	}
	if value.Compare(maximum) > 0 {
		return maximum
	}
	return value
}
func (value RemapPathScopeComponents) Equal(other RemapPathScopeComponents) bool {
	return value == other
}
func (value RemapPathScopeComponents) Compare(other RemapPathScopeComponents) int {
	if value < other {
		return -1
	}
	if value > other {
		return 1
	}
	return 0
}
func (value SourceFileHashAlgorithm) Less(other SourceFileHashAlgorithm) bool {
	return value.Compare(other) < 0
}
func (value SourceFileHashAlgorithm) LessEqual(other SourceFileHashAlgorithm) bool {
	return value.Compare(other) <= 0
}
func (value SourceFileHashAlgorithm) Greater(other SourceFileHashAlgorithm) bool {
	return value.Compare(other) > 0
}
func (value SourceFileHashAlgorithm) GreaterEqual(other SourceFileHashAlgorithm) bool {
	return value.Compare(other) >= 0
}
func (value SourceFileHashAlgorithm) Min(other SourceFileHashAlgorithm) SourceFileHashAlgorithm {
	return libOrdMin(value, other)
}
func (value SourceFileHashAlgorithm) Max(other SourceFileHashAlgorithm) SourceFileHashAlgorithm {
	return libOrdMax(value, other)
}
func (value SourceFileHashAlgorithm) Clamp(minimum, maximum SourceFileHashAlgorithm) SourceFileHashAlgorithm {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value SourceFileHashAlgorithm) ClampWithContext(minimum, maximum SourceFileHashAlgorithm, ctx rustfmt.Context) SourceFileHashAlgorithm {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value StableSourceFileId) Less(other StableSourceFileId) bool { return value.Compare(other) < 0 }
func (value StableSourceFileId) LessEqual(other StableSourceFileId) bool {
	return value.Compare(other) <= 0
}
func (value StableSourceFileId) Greater(other StableSourceFileId) bool {
	return value.Compare(other) > 0
}
func (value StableSourceFileId) GreaterEqual(other StableSourceFileId) bool {
	return value.Compare(other) >= 0
}
func (value StableSourceFileId) Min(other StableSourceFileId) StableSourceFileId {
	return libOrdMin(value, other)
}
func (value StableSourceFileId) Max(other StableSourceFileId) StableSourceFileId {
	return libOrdMax(value, other)
}
func (value StableSourceFileId) Clamp(minimum, maximum StableSourceFileId) StableSourceFileId {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value StableSourceFileId) ClampWithContext(minimum, maximum StableSourceFileId, ctx rustfmt.Context) StableSourceFileId {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value BytePos) PartialCompare(other BytePos) Option[int] { return Some(value.Compare(other)) }
func (value BytePos) Less(other BytePos) bool                  { return value.Compare(other) < 0 }
func (value BytePos) LessEqual(other BytePos) bool             { return value.Compare(other) <= 0 }
func (value BytePos) Greater(other BytePos) bool               { return value.Compare(other) > 0 }
func (value BytePos) GreaterEqual(other BytePos) bool          { return value.Compare(other) >= 0 }
func (value BytePos) Min(other BytePos) BytePos                { return libOrdMin(value, other) }
func (value BytePos) Max(other BytePos) BytePos                { return libOrdMax(value, other) }
func (value BytePos) Clamp(minimum, maximum BytePos) BytePos {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value BytePos) ClampWithContext(minimum, maximum BytePos, ctx rustfmt.Context) BytePos {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value RelativeBytePos) PartialCompare(other RelativeBytePos) Option[int] {
	return Some(value.Compare(other))
}
func (value RelativeBytePos) Less(other RelativeBytePos) bool      { return value.Compare(other) < 0 }
func (value RelativeBytePos) LessEqual(other RelativeBytePos) bool { return value.Compare(other) <= 0 }
func (value RelativeBytePos) Greater(other RelativeBytePos) bool   { return value.Compare(other) > 0 }
func (value RelativeBytePos) GreaterEqual(other RelativeBytePos) bool {
	return value.Compare(other) >= 0
}
func (value RelativeBytePos) Min(other RelativeBytePos) RelativeBytePos {
	return libOrdMin(value, other)
}
func (value RelativeBytePos) Max(other RelativeBytePos) RelativeBytePos {
	return libOrdMax(value, other)
}
func (value RelativeBytePos) Clamp(minimum, maximum RelativeBytePos) RelativeBytePos {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value RelativeBytePos) ClampWithContext(minimum, maximum RelativeBytePos, ctx rustfmt.Context) RelativeBytePos {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value CharPos) PartialCompare(other CharPos) Option[int] { return Some(value.Compare(other)) }
func (value CharPos) Less(other CharPos) bool                  { return value.Compare(other) < 0 }
func (value CharPos) LessEqual(other CharPos) bool             { return value.Compare(other) <= 0 }
func (value CharPos) Greater(other CharPos) bool               { return value.Compare(other) > 0 }
func (value CharPos) GreaterEqual(other CharPos) bool          { return value.Compare(other) >= 0 }
func (value CharPos) Min(other CharPos) CharPos                { return libOrdMin(value, other) }
func (value CharPos) Max(other CharPos) CharPos                { return libOrdMax(value, other) }
func (value CharPos) Clamp(minimum, maximum CharPos) CharPos {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value CharPos) ClampWithContext(minimum, maximum CharPos, ctx rustfmt.Context) CharPos {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value AttrId) Less(other AttrId) bool         { return value.Compare(other) < 0 }
func (value AttrId) LessEqual(other AttrId) bool    { return value.Compare(other) <= 0 }
func (value AttrId) Greater(other AttrId) bool      { return value.Compare(other) > 0 }
func (value AttrId) GreaterEqual(other AttrId) bool { return value.Compare(other) >= 0 }
func (value AttrId) Min(other AttrId) AttrId        { return libOrdMin(value, other) }
func (value AttrId) Max(other AttrId) AttrId        { return libOrdMax(value, other) }
func (value AttrId) Clamp(minimum, maximum AttrId) AttrId {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value AttrId) ClampWithContext(minimum, maximum AttrId, ctx rustfmt.Context) AttrId {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value RealFileName) PartialCompare(other RealFileName) Option[int] {
	return Some(value.Compare(other))
}
func (value RealFileName) Less(other RealFileName) bool         { return value.Compare(other) < 0 }
func (value RealFileName) LessEqual(other RealFileName) bool    { return value.Compare(other) <= 0 }
func (value RealFileName) Greater(other RealFileName) bool      { return value.Compare(other) > 0 }
func (value RealFileName) GreaterEqual(other RealFileName) bool { return value.Compare(other) >= 0 }
func (value RealFileName) Min(other RealFileName) RealFileName  { return libOrdMin(value, other) }
func (value RealFileName) Max(other RealFileName) RealFileName  { return libOrdMax(value, other) }
func (value RealFileName) Clamp(minimum, maximum RealFileName) RealFileName {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value RealFileName) ClampWithContext(minimum, maximum RealFileName, ctx rustfmt.Context) RealFileName {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value FileName) PartialCompare(other FileName) Option[int] { return Some(value.Compare(other)) }
func (value FileName) Less(other FileName) bool                  { return value.Compare(other) < 0 }
func (value FileName) LessEqual(other FileName) bool             { return value.Compare(other) <= 0 }
func (value FileName) Greater(other FileName) bool               { return value.Compare(other) > 0 }
func (value FileName) GreaterEqual(other FileName) bool          { return value.Compare(other) >= 0 }
func (value FileName) Min(other FileName) FileName               { return libOrdMin(value, other) }
func (value FileName) Max(other FileName) FileName               { return libOrdMax(value, other) }
func (value FileName) Clamp(minimum, maximum FileName) FileName {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value FileName) ClampWithContext(minimum, maximum FileName, ctx rustfmt.Context) FileName {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value ErrorGuaranteed) Less(other ErrorGuaranteed) bool      { return value.Compare(other) < 0 }
func (value ErrorGuaranteed) LessEqual(other ErrorGuaranteed) bool { return value.Compare(other) <= 0 }
func (value ErrorGuaranteed) Greater(other ErrorGuaranteed) bool   { return value.Compare(other) > 0 }
func (value ErrorGuaranteed) GreaterEqual(other ErrorGuaranteed) bool {
	return value.Compare(other) >= 0
}
func (value ErrorGuaranteed) Min(other ErrorGuaranteed) ErrorGuaranteed {
	return libOrdMin(value, other)
}
func (value ErrorGuaranteed) Max(other ErrorGuaranteed) ErrorGuaranteed {
	return libOrdMax(value, other)
}
func (value ErrorGuaranteed) Clamp(minimum, maximum ErrorGuaranteed) ErrorGuaranteed {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value ErrorGuaranteed) ClampWithContext(minimum, maximum ErrorGuaranteed, ctx rustfmt.Context) ErrorGuaranteed {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value innerRealFileName) PartialCompare(other innerRealFileName) Option[int] {
	return Some(value.Compare(other))
}
func (value innerRealFileName) Less(other innerRealFileName) bool { return value.Compare(other) < 0 }
func (value innerRealFileName) LessEqual(other innerRealFileName) bool {
	return value.Compare(other) <= 0
}
func (value innerRealFileName) Greater(other innerRealFileName) bool { return value.Compare(other) > 0 }
func (value innerRealFileName) GreaterEqual(other innerRealFileName) bool {
	return value.Compare(other) >= 0
}
func (value innerRealFileName) Min(other innerRealFileName) innerRealFileName {
	return libOrdMin(value, other)
}
func (value innerRealFileName) Max(other innerRealFileName) innerRealFileName {
	return libOrdMax(value, other)
}
func (value innerRealFileName) Clamp(minimum, maximum innerRealFileName) innerRealFileName {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value innerRealFileName) ClampWithContext(minimum, maximum innerRealFileName, ctx rustfmt.Context) innerRealFileName {
	return libOrdClamp(value, minimum, maximum, ctx)
}

func (value RemapPathScopeComponents) PartialCompare(other RemapPathScopeComponents) Option[int] {
	return Some(value.Compare(other))
}
func (value RemapPathScopeComponents) Less(other RemapPathScopeComponents) bool {
	return value.Compare(other) < 0
}
func (value RemapPathScopeComponents) LessEqual(other RemapPathScopeComponents) bool {
	return value.Compare(other) <= 0
}
func (value RemapPathScopeComponents) Greater(other RemapPathScopeComponents) bool {
	return value.Compare(other) > 0
}
func (value RemapPathScopeComponents) GreaterEqual(other RemapPathScopeComponents) bool {
	return value.Compare(other) >= 0
}
func (value RemapPathScopeComponents) Min(other RemapPathScopeComponents) RemapPathScopeComponents {
	return libOrdMin(value, other)
}
func (value RemapPathScopeComponents) Max(other RemapPathScopeComponents) RemapPathScopeComponents {
	return libOrdMax(value, other)
}
func (value RemapPathScopeComponents) Clamp(minimum, maximum RemapPathScopeComponents) RemapPathScopeComponents {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value RemapPathScopeComponents) ClampWithContext(minimum, maximum RemapPathScopeComponents, ctx rustfmt.Context) RemapPathScopeComponents {
	return libOrdClamp(value, minimum, maximum, ctx)
}
