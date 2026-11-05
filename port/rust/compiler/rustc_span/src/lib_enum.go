// SPDX-License-Identifier: MIT OR Apache-2.0
// Canonical borrowed pattern projections for public lib.rs enum alternatives.
// No projection clones an owned payload; references have the source borrow's
// lifetime and callers must preserve its ordinary exclusivity/UTF-8 rules.
package rustcspan

import rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"

type FileNameKind uint8

const (
	FileNameReal FileNameKind = iota
	FileNameCfgSpec
	FileNameAnon
	FileNameMacroExpansion
	FileNameProcMacroSourceCode
	FileNameCliCrateAttr
	FileNameCustom
	FileNameDocTest
	FileNameInlineAsm
)

func (value FileName) Kind() FileNameKind { return FileNameKind(value.kind) }
func (value *FileName) BorrowReal() (*RealFileName, bool) {
	if value.kind != fileNameReal {
		return nil, false
	}
	return &value.real, true
}
func (value *FileName) BorrowCustom() (*string, bool) {
	if value.kind != fileNameCustom {
		return nil, false
	}
	return &value.custom, true
}
func (value *FileName) BorrowDocTest() (*RealFileName, *int, bool) {
	if value.kind != fileNameDocTest {
		return nil, nil, false
	}
	return &value.real, &value.line, true
}
func (value *FileName) BorrowHash() (*rustchashes.Hash64, bool) {
	switch value.kind {
	case fileNameCfgSpec, fileNameAnon, fileNameMacroExpansion, fileNameProcMacroSourceCode, fileNameCliCrateAttr, fileNameInlineAsm:
		return &value.hash, true
	default:
		return nil, false
	}
}
func (value *ExternalSource) BorrowForeign() (*ExternalSourceKind, *uint32, bool) {
	if !value.foreign {
		return nil, nil, false
	}
	return &value.kind, &value.metadataIndex, true
}
func (value *ExternalSourceKind) BorrowPresent() (*SharedString, bool) {
	if !value.IsPresent() {
		return nil, false
	}
	return &value.src, true
}
func (value *SourceFileLines) BorrowLines() (*[]RelativeBytePos, bool) {
	if value.isDiffs {
		return nil, false
	}
	return &value.lines, true
}
func (value *SourceFileLines) BorrowDiffs() (*SourceFileDiffs, bool) {
	if !value.isDiffs {
		return nil, false
	}
	return &value.diffs, true
}

type SpanSnippetErrorKind uint8

const (
	SpanSnippetIllFormedSpan SpanSnippetErrorKind = iota
	SpanSnippetDistinctSources
	SpanSnippetMalformedForSourcemap
	SpanSnippetSourceNotAvailable
)

func (value *SpanSnippetError) Kind() SpanSnippetErrorKind { return SpanSnippetErrorKind(value.kind) }
func (value *SpanSnippetError) IllFormedSpan() (Span, bool) {
	return value.span, value.kind == snippetIllFormed
}
func (value *SpanSnippetError) BorrowDistinctSources() (*DistinctSources, bool) {
	if value.kind != snippetDistinct {
		return nil, false
	}
	return value.distinct, true
}
func (value *SpanSnippetError) BorrowMalformedForSourcemap() (*MalformedSourceMapPositions, bool) {
	if value.kind != snippetMalformed {
		return nil, false
	}
	return &value.malformed, true
}
func (value *SpanSnippetError) BorrowSourceNotAvailable() (*FileName, bool) {
	if value.kind != snippetUnavailable {
		return nil, false
	}
	return &value.filename, true
}
func (value *SpanLinesError) BorrowDistinctSources() *DistinctSources { return value.distinct }
