// SPDX-License-Identifier: MIT OR Apache-2.0
// Remaining concrete derive/default trait adapters from pinned lib.rs.
package rustcspan

import (
	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (value SourceFileHashAlgorithm) Clone() SourceFileHashAlgorithm            { return value }
func (value *SourceFileHashAlgorithm) CloneFrom(source SourceFileHashAlgorithm) { *value = source }
func (value SourceFileHashAlgorithm) Equal(other SourceFileHashAlgorithm) bool  { return value == other }
func (value SourceFileHashAlgorithm) Compare(other SourceFileHashAlgorithm) int {
	if value < other {
		return -1
	}
	if value > other {
		return 1
	}
	return 0
}
func (value SourceFileHashAlgorithm) PartialCompare(other SourceFileHashAlgorithm) Option[int] {
	return Some(value.Compare(other))
}
func (value SourceFileHashAlgorithm) Hash(h stablehash.Hasher) { stablehash.HashIsize(h, int(value)) }
func (value SourceFileHashAlgorithm) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(value))
}
func (value *SourceFileHash) CloneFrom(source SourceFileHash) { *value = source }
func (value SourceFileHash) Hash(h stablehash.Hasher) {
	value.Kind.Hash(h)
	stablehash.HashLengthPrefix(h, 32)
	h.Write(value.value[:])
}
func (value SourceFileHash) StableHash(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	value.Kind.StableHash(hcx, h)
	h.WriteUsize(32)
	h.Write(value.value[:])
}
func (value *ExternalSourceKind) CloneFrom(source ExternalSourceKind) {
	value.CloneFromWithContext(source, rustfmt.Context{})
}
func (value *ExternalSourceKind) CloneFromWithContext(source ExternalSourceKind, ctx rustfmt.Context) {
	replacement := source.CloneWithContext(ctx)
	defer func() {
		p := recover()
		if p != nil && ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		*value = replacement
		if p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	value.CloseWithContext(ctx)
}
func (value *ExternalSource) CloneFrom(source ExternalSource) {
	value.CloneFromWithContext(source, rustfmt.Context{})
}
func (value *ExternalSource) CloneFromWithContext(source ExternalSource, ctx rustfmt.Context) {
	replacement := source.CloneWithContext(ctx)
	defer func() {
		p := recover()
		if p != nil && ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		*value = replacement
		if p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	value.CloseWithContext(ctx)
}
func (value *SourceFileLines) CloneFrom(source SourceFileLines) { *value = source.Clone() }
func (value *SourceFileDiffs) CloneFrom(source SourceFileDiffs) { *value = source.Clone() }
func (value *SourceFile) CloneFrom(source *SourceFile) {
	value.CloneFromWithContext(source, rustfmt.Context{})
}
func (value *SourceFile) CloneFromWithContext(source *SourceFile, ctx rustfmt.Context) {
	replacement := source.CloneWithContext(ctx)
	defer func() {
		p := recover()
		if p != nil && ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		*value = replacement
		if p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	value.CloseWithContext(ctx)
}
func (value Loc) Clone() Loc { return value.CloneWithContext(rustfmt.Context{}) }
func (value Loc) CloneWithContext(ctx rustfmt.Context) Loc {
	return Loc{File: value.File.CloneWithContext(ctx), Line: value.Line, Col: value.Col, ColDisplay: value.ColDisplay}
}
func (value *Loc) CloneFrom(source Loc) { value.CloneFromWithContext(source, rustfmt.Context{}) }
func (value *Loc) CloneFromWithContext(source Loc, ctx rustfmt.Context) {
	replacement := source.CloneWithContext(ctx)
	defer func() {
		p := recover()
		if p != nil && ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		*value = replacement
		if p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	value.CloseWithContext(ctx)
}
func (value LineInfo) Clone() LineInfo                               { return value }
func (value *LineInfo) CloneFrom(source LineInfo)                    { *value = source }
func (value LineInfo) Equal(other LineInfo) bool                     { return value == other }
func (value *RealFileName) CloneFrom(source RealFileName)            { *value = source.Clone() }
func (value *FileName) CloneFrom(source FileName)                    { *value = source.Clone() }
func (value StableSourceFileId) Equal(other StableSourceFileId) bool { return value == other }
func (value StableSourceFileId) PartialCompare(other StableSourceFileId) Option[int] {
	return Some(value.Compare(other))
}
func (value BytePos) Equal(other BytePos) bool { return value == other }
func (value BytePos) Compare(other BytePos) int {
	return compareDefIndexValues(uint32(value), uint32(other))
}
func (value *BytePos) CloneFrom(source BytePos) { *value = source }
func (value RelativeBytePos) Compare(other RelativeBytePos) int {
	return compareDefIndexValues(uint32(value), uint32(other))
}
func (value RelativeBytePos) Equal(other RelativeBytePos) bool { return value == other }
func (value CharPos) Equal(other CharPos) bool                 { return value == other }
func (value CharPos) Compare(other CharPos) int {
	if value < other {
		return -1
	}
	if value > other {
		return 1
	}
	return 0
}
func (value ErrorGuaranteed) NotEqual(other ErrorGuaranteed) bool { return !value.Equal(other) }
func (value ErrorGuaranteed) PartialCompare(other ErrorGuaranteed) Option[int] {
	return Some(value.Compare(other))
}
func (value *SpanData) CloneFrom(source SpanData)                  { *value = source.Clone() }
func (value *SpanLinesError) CloneFrom(source *SpanLinesError)     { *value = *source.Clone() }
func (value *SpanSnippetError) CloneFrom(source *SpanSnippetError) { *value = *source.Clone() }
func (value *DistinctSources) CloneFrom(source DistinctSources)    { *value = source.Clone() }
func (value *MalformedSourceMapPositions) CloneFrom(source MalformedSourceMapPositions) {
	*value = source.Clone()
}
