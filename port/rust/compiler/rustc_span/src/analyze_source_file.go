// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_span/src/analyze_source_file.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package rustcspan

import (
	"math/bits"
	"runtime"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// SourceFileAnalysisConfig carries the source build profile and the source
// host's actual feature witnesses. SSE2/LSX identify the original logical
// implementations; portable Go masks preserve their complete control flow,
// without promising SIMD code generation. Off amd64 the convenience default
// conservatively chooses generic decoding. Supply truthful feature witnesses
// to reproduce a particular x86/loongarch64 source host's dispatch exactly.
type SourceFileAnalysisConfig struct {
	Architecture                    string
	SSE2Available, LSXAvailable     bool
	DebugAssertions, OverflowChecks bool
}

func DefaultSourceFileAnalysisConfig() SourceFileAnalysisConfig {
	return SourceFileAnalysisConfig{Architecture: runtime.GOARCH, SSE2Available: runtime.GOARCH == "amd64", DebugAssertions: true, OverflowChecks: true}
}

// AnalyzeSourceFile returns local line starts and multibyte character metadata.
// Like Rust &str, src must be valid UTF-8. Empty files have no line start; the
// optimistic start after a trailing newline is removed. Ordinary returned
// slices own their vector backing; mutation must respect the source borrows.
func AnalyzeSourceFile(src string) ([]RelativeBytePos, []MultiByteChar) {
	return AnalyzeSourceFileWithConfig(src, DefaultSourceFileAnalysisConfig())
}
func AnalyzeSourceFileWithConfig(src string, config SourceFileAnalysisConfig) ([]RelativeBytePos, []MultiByteChar) {
	return AnalyzeSourceFileWithContext(rustfmt.Context{}, src, config)
}
func AnalyzeSourceFileWithContext(context rustfmt.Context, src string, config SourceFileAnalysisConfig) ([]RelativeBytePos, []MultiByteChar) {
	if !utf8.ValidString(src) {
		context.Panic("analyze_source_file: Rust str requires valid UTF-8")
	}
	lines := []RelativeBytePos{RelativeBytePosFromU32(0)}
	multibyte := []MultiByteChar{}
	analyzeSourceFileDispatch(src, &lines, &multibyte, config, context)
	if len(lines) > 0 {
		end := RelativeBytePosFromUsize(uint(len(src)))
		if end < lines[len(lines)-1] {
			analysisStaticPanic(context, "rustc_span::analyze_source_file::assert_end", "assertion failed: source_file_end >= last_line_start")
		}
		if end == lines[len(lines)-1] {
			lines = lines[:len(lines)-1]
		}
	}
	return lines, multibyte
}
func analyzeSourceFileDispatch(src string, lines *[]RelativeBytePos, multibyte *[]MultiByteChar, config SourceFileAnalysisConfig, context rustfmt.Context) {
	switch config.Architecture {
	case "x86", "x86_64", "386", "amd64":
		if config.SSE2Available {
			analyzeSourceFileSSE2(src, lines, multibyte, config, context)
			return
		}
	case "loongarch64", "loong64":
		if config.LSXAvailable {
			analyzeSourceFileLSX(src, lines, multibyte, config, context)
			return
		}
	}
	analyzeSourceFileGeneric(src, uint(len(src)), RelativeBytePosFromU32(0), lines, multibyte, config, context)
}
func analysisAdd32(a, b RelativeBytePos, config SourceFileAnalysisConfig, context rustfmt.Context) RelativeBytePos {
	return a.AddWithContext(b, SpanEncodingConfig{DebugAssertions: config.DebugAssertions, OverflowChecks: config.OverflowChecks}, context)
}

// These fixed Copy element types use the std Vec growth policy. Their source
// Rust sizes are four/eight bytes, independent of the host pointer width.
func analysisPush[T any](out *[]T, value T, rustSize uint, context rustfmt.Context) {
	if len(*out) == cap(*out) {
		minimum := uint(4)
		if rustSize == 1 {
			minimum = 8
		} else if rustSize > 1024 {
			minimum = 1
		}
		capacity := uint(cap(*out)) * 2
		required := uint(len(*out)) + 1
		if capacity < required {
			capacity = required
		}
		if capacity < minimum {
			capacity = minimum
		}
		if capacity > uint(^uint(0)>>1)/rustSize {
			analysisStaticPanic(context, "rustc_span::analysis_push::capacity", "capacity overflow")
		}
		backing := make([]T, len(*out), int(capacity))
		copy(backing, *out)
		*out = backing
	}
	*out = append(*out, value)
}

// SSE2 and LSX have the same 16-byte masks, chunk cursor and generic fallback.
// Keep separate source entry points so both dispatch branches remain explicit.
func analyzeSourceFileSSE2(src string, lines *[]RelativeBytePos, multibyte *[]MultiByteChar, config SourceFileAnalysisConfig, context rustfmt.Context) {
	analyzeSourceFileChunks(src, lines, multibyte, config, context, "rustc_span::analyze_source_file::sse2.assert_offset")
}
func analyzeSourceFileLSX(src string, lines *[]RelativeBytePos, multibyte *[]MultiByteChar, config SourceFileAnalysisConfig, context rustfmt.Context) {
	analyzeSourceFileChunks(src, lines, multibyte, config, context, "rustc_span::analyze_source_file::lsx.assert_offset")
}
func analyzeSourceFileChunks(src string, lines *[]RelativeBytePos, multibyte *[]MultiByteChar, config SourceFileAnalysisConfig, context rustfmt.Context, offsetSite string) {
	const chunkSize = 16
	chunks := len(src) / chunkSize
	intraChunkOffset := uint(0)
	for chunkIndex := 0; chunkIndex < chunks; chunkIndex++ {
		chunk := src[chunkIndex*chunkSize : (chunkIndex+1)*chunkSize]
		var multibyteMask uint16
		for i := 0; i < chunkSize; i++ {
			if chunk[i]&0x80 != 0 {
				multibyteMask |= uint16(1) << i
			}
		}
		if multibyteMask == 0 {
			if intraChunkOffset != 0 {
				analysisStaticPanic(context, offsetSite, "assertion failed: intra_chunk_offset == 0")
			}
			var newlineMask uint16
			for i := 0; i < chunkSize; i++ {
				if chunk[i] == '\n' {
					newlineMask |= uint16(1) << i
				}
			}
			outputOffset := RelativeBytePosFromUsize(uint(chunkIndex*chunkSize + 1))
			for newlineMask != 0 {
				index := bits.TrailingZeros16(newlineMask)
				analysisPush(lines, analysisAdd32(RelativeBytePos(uint32(index)), outputOffset, config, context), 4, context)
				newlineMask &= newlineMask - 1
			}
		} else {
			scanStart := uint(chunkIndex*chunkSize) + intraChunkOffset
			intraChunkOffset = analyzeSourceFileGeneric(src[scanStart:], chunkSize-intraChunkOffset, RelativeBytePosFromUsize(scanStart), lines, multibyte, config, context)
		}
	}
	tailStart := uint(chunks*chunkSize) + intraChunkOffset
	if tailStart < uint(len(src)) {
		analyzeSourceFileGeneric(src[tailStart:], uint(len(src))-tailStart, RelativeBytePosFromUsize(tailStart), lines, multibyte, config, context)
	}
}

// The decoder may read past scanLen to finish a scalar that begins inside the
// scan. Its returned overflow is carried into the following chunk, as upstream.
func analyzeSourceFileGeneric(src string, scanLen uint, outputOffset RelativeBytePos, lines *[]RelativeBytePos, multibyte *[]MultiByteChar, config SourceFileAnalysisConfig, context rustfmt.Context) uint {
	if uint(len(src)) < scanLen {
		analysisStaticPanic(context, "rustc_span::analyze_source_file::generic.assert_scan_length", "assertion failed: src.len() >= scan_len")
	}
	i := uint(0)
	for i < scanLen {
		ch := src[i]
		charLen := uint(1)
		if ch == '\n' {
			pos := analysisAdd32(RelativeBytePosFromUsize(i), outputOffset, config, context)
			analysisPush(lines, analysisAdd32(pos, RelativeBytePos(1), config, context), 4, context)
		} else if ch >= 128 {
			_, size := utf8.DecodeRuneInString(src[i:])
			charLen = uint(size)
			pos := analysisAdd32(RelativeBytePosFromUsize(i), outputOffset, config, context)
			if charLen < 2 || charLen > 4 {
				analysisStaticPanic(context, "rustc_span::analyze_source_file::generic.assert_char_length", "assertion failed: (2..=4).contains(&char_len)")
			}
			analysisPush(multibyte, MultiByteChar{Pos: pos, Bytes: uint8(charLen)}, 8, context)
		}
		i += charLen
	}
	return i - scanLen
}

// Source failures carry their actual payload type and an optional explicit
// profile location. The invalid-Go-UTF8 rejection remains a separate adapter
// precondition, since an invalid Rust &str cannot reach the original code.
func analysisStaticPanic(context rustfmt.Context, site, text string) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: text, Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(site), CanUnwind: true})
}
