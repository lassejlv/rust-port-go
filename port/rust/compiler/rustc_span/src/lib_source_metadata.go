// SPDX-License-Identifier: MIT OR Apache-2.0
// Source metadata blocks of pinned compiler/rustc_span/src/lib.rs.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type externalSourceKindTag uint8

const (
	externalPresent externalSourceKindTag = iota
	externalAbsentOk
	externalAbsentErr
)

type ExternalSourceKind struct {
	tag externalSourceKindTag
	src SharedString
}

func PresentExternalSourceKind(src SharedString) ExternalSourceKind {
	return ExternalSourceKind{tag: externalPresent, src: src}
}
func AbsentOkExternalSourceKind() ExternalSourceKind {
	return ExternalSourceKind{tag: externalAbsentOk}
}
func AbsentErrExternalSourceKind() ExternalSourceKind {
	return ExternalSourceKind{tag: externalAbsentErr}
}
func (value ExternalSourceKind) IsPresent() bool   { return value.tag == externalPresent }
func (value ExternalSourceKind) IsAbsentOk() bool  { return value.tag == externalAbsentOk }
func (value ExternalSourceKind) IsAbsentErr() bool { return value.tag == externalAbsentErr }
func (value ExternalSourceKind) GetSource() Option[string] {
	if value.IsPresent() {
		return Some(value.src.String())
	}
	return None[string]()
}
func (value ExternalSourceKind) Clone() ExternalSourceKind {
	return value.CloneWithContext(rustfmt.Context{})
}
func (value ExternalSourceKind) CloneWithContext(context rustfmt.Context) ExternalSourceKind {
	if value.IsPresent() {
		value.src = value.src.CloneWithContext(context)
	}
	return value
}
func (value ExternalSourceKind) Equal(other ExternalSourceKind) bool {
	return value.tag == other.tag && (!value.IsPresent() || value.src.String() == other.src.String())
}
func (value *ExternalSourceKind) CloseWithContext(context rustfmt.Context) {
	if value.IsPresent() {
		value.src.CloseWithContext(context)
	}
}

type ExternalSource struct {
	foreign       bool
	kind          ExternalSourceKind
	metadataIndex uint32
}

func UnneededExternalSource() ExternalSource { return ExternalSource{} }
func ForeignExternalSource(kind ExternalSourceKind, metadataIndex uint32) ExternalSource {
	return ExternalSource{foreign: true, kind: kind, metadataIndex: metadataIndex}
}
func (value ExternalSource) IsUnneeded() bool { return !value.foreign }
func (value ExternalSource) Foreign() (ExternalSourceKind, uint32, bool) {
	return value.kind, value.metadataIndex, value.foreign
}
func (value ExternalSource) GetSource() Option[string] {
	if value.foreign {
		return value.kind.GetSource()
	}
	return None[string]()
}
func (value ExternalSource) Clone() ExternalSource { return value.CloneWithContext(rustfmt.Context{}) }
func (value ExternalSource) CloneWithContext(context rustfmt.Context) ExternalSource {
	if value.foreign {
		value.kind = value.kind.CloneWithContext(context)
	}
	return value
}
func (value ExternalSource) Equal(other ExternalSource) bool {
	return value.foreign == other.foreign && (!value.foreign || value.kind.Equal(other.kind) && value.metadataIndex == other.metadataIndex)
}
func (value *ExternalSource) CloseWithContext(context rustfmt.Context) {
	if value.foreign {
		value.kind.CloseWithContext(context)
	}
}

// SourceFileLines alternatives consume their backing vectors. Returned views
// borrow that backing and must respect the enclosing FreezeLock's lifetime.
type SourceFileLines struct {
	isDiffs bool
	lines   []RelativeBytePos
	diffs   SourceFileDiffs
}

func LinesSourceFileLines(lines []RelativeBytePos) SourceFileLines {
	return SourceFileLines{lines: lines}
}
func DiffsSourceFileLines(diffs SourceFileDiffs) SourceFileLines {
	return SourceFileLines{isDiffs: true, diffs: diffs}
}
func (value SourceFileLines) IsLines() bool { return !value.isDiffs }
func (value SourceFileLines) Clone() SourceFileLines {
	if value.isDiffs {
		return DiffsSourceFileLines(value.diffs.Clone())
	}
	lines := make([]RelativeBytePos, len(value.lines))
	copy(lines, value.lines)
	return LinesSourceFileLines(lines)
}

type SourceFileDiffs struct {
	bytesPerDiff, numDiffs uint
	rawDiffs               []byte
}

// NewSourceFileDiffs mirrors the metadata-owned fields. Validation is lazy,
// exactly as in the original conversion routine, including malformed inputs.
func NewSourceFileDiffs(bytesPerDiff, numDiffs uint, raw []byte) SourceFileDiffs {
	return SourceFileDiffs{bytesPerDiff: bytesPerDiff, numDiffs: numDiffs, rawDiffs: raw}
}
func (value SourceFileDiffs) Clone() SourceFileDiffs {
	raw := make([]byte, len(value.rawDiffs))
	copy(raw, value.rawDiffs)
	return NewSourceFileDiffs(value.bytesPerDiff, value.numDiffs, raw)
}

type StableSourceFileId struct{ hash rustchashes.Hash128 }

func StableSourceFileIdFromHash(hash rustchashes.Hash128) StableSourceFileId {
	return StableSourceFileId{hash: hash}
}
func (value StableSourceFileId) AsHash() rustchashes.Hash128         { return value.hash }
func (value StableSourceFileId) Clone() StableSourceFileId           { return value }
func (value *StableSourceFileId) CloneFrom(other StableSourceFileId) { *value = other }
func (value StableSourceFileId) Compare(other StableSourceFileId) int {
	return value.hash.Compare(other.hash)
}
func (value StableSourceFileId) Hash(h rustchashes.U64Hasher) { value.hash.Hash(h) }
func (value StableSourceFileId) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	value.hash.Hash(h)
}
func StableSourceFileIdFromFilenameInCurrentCrate(filename *FileName) StableSourceFileId {
	return stableSourceFileIdFromFilename(filename, None[StableCrateId]())
}
func StableSourceFileIdFromFilenameForExport(filename *FileName, crate StableCrateId) StableSourceFileId {
	return stableSourceFileIdFromFilename(filename, Some(crate))
}
func stableSourceFileIdFromFilename(filename *FileName, crate Option[StableCrateId]) StableSourceFileId {
	h := datastructures.NewStableHasher()
	filename.StableHash(nil, h)
	if c, ok := crate.Get(); ok {
		h.WriteU8(1)
		c.StableHash(nil, h)
	} else {
		h.WriteU8(0)
	}
	return StableSourceFileId{hash: rustchashes.Hash128FromStableHash(rustchashes.StableHasherHash(h.FinishHash()))}
}

type SourceFile struct {
	Name                  FileName
	Src                   Option[SharedString]
	SrcHash               SourceFileHash
	ChecksumHash          Option[SourceFileHash]
	ExternalSrc           *FreezeLock[ExternalSource]
	StartPos              BytePos
	NormalizedSourceLen   RelativeBytePos
	UnnormalizedSourceLen uint32
	Lines                 *FreezeLock[SourceFileLines]
	MultibyteChars        []MultiByteChar
	NormalizedPos         []NormalizedPos
	StableId              StableSourceFileId
	Cnum                  CrateNum
}
type OffsetOverflowError struct{}

func (OffsetOverflowError) Error() string { return "OffsetOverflowError" }

type BytePosRange struct{ Start, End BytePos }
type SourceFileAndLine struct {
	SF   Shared[SourceFile]
	Line uint
}
type SourceFileAndBytePos struct {
	SF  Shared[SourceFile]
	Pos BytePos
}
type Loc struct {
	File       Shared[SourceFile]
	Line       uint
	Col        CharPos
	ColDisplay uint
}
type LineInfo struct {
	LineIndex        uint
	StartCol, EndCol CharPos
}
type FileLines struct {
	File  Shared[SourceFile]
	Lines []LineInfo
}

func (value *SourceFileAndLine) CloseWithContext(context rustfmt.Context) {
	value.SF.CloseWithContext(context)
}
func (value *SourceFileAndBytePos) CloseWithContext(context rustfmt.Context) {
	value.SF.CloseWithContext(context)
}
func (value *Loc) CloseWithContext(context rustfmt.Context) { value.File.CloseWithContext(context) }
func (value *FileLines) CloseWithContext(context rustfmt.Context) {
	value.File.CloseWithContext(context)
}
func (value *SourceFile) CloseWithContext(context rustfmt.Context) {
	// Field destruction proceeds even if an earlier destructor unwinds. A second
	// translated panic is handled by the shared Rust cleanup guard.
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			unwind := context
			unwind.Unwinding = true
			spanCleanupDuringUnwindWithContext(unwind, func() { value.ExternalSrc.CloseWithContext(unwind) })
			spanCleanupDuringUnwindWithContext(unwind, func() { value.Lines.CloseWithContext(unwind) })
			context.ResumeUnwind(payload)
		}
	}()
	if src, ok := value.Src.Get(); ok {
		src.CloseWithContext(context)
	}
	value.ExternalSrc.CloseWithContext(context)
	value.Lines.CloseWithContext(context)
}
