// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual lib.rs and derive-macro codec bodies for source-file metadata.
package rustcspan

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"

	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func spanCodecContext(codec any) rustfmt.Context {
	if runtime, ok := codec.(interface{ RuntimeContext() rustfmt.Context }); ok {
		return runtime.RuntimeContext()
	}
	return rustfmt.Context{}
}
func spanCodecConfig(codec any) SpanEncodingConfig {
	if runtime, ok := codec.(interface{ BuildProfile() rustcserialize.Profile }); ok {
		p := runtime.BuildProfile()
		return SpanEncodingConfig{p.DebugAssertions, p.OverflowChecks}
	}
	return DefaultSpanEncodingConfig()
}
func spanCodecOptionTag(d rustcserialize.Decoder) bool {
	switch d.ReadU8() {
	case 0:
		return false
	case 1:
		return true
	}
	ctx := spanCodecContext(d)
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "Encountered invalid discriminant while decoding `Option`.", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation(SpanCodecOptionTagPanicSite), CanUnwind: true})
	return false
}
func (value RemapPathScopeComponents) Encode(e rustcserialize.Encoder) { e.EmitU8(value.Bits()) }
func DecodeRemapPathScopeComponents(d rustcserialize.Decoder) RemapPathScopeComponents {
	value, ok := RemapPathScopeComponentsFromBits(d.ReadU8()).Get()
	if !ok {
		spanFormattedPanicAt(spanCodecContext(d), spanLibLocation(273, 14), func(rustfmt.Context) any { return "invalid bits for RemapPathScopeComponents" })
	}
	return value
}
func encodeSourcePath(path RustPath, e rustcserialize.Encoder) {
	text := path.EncodedString()
	if !utf8.ValidString(text) {
		ctx := spanCodecContext(e)
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "called `Option::unwrap()` on a `None` value", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation(SpanCodecPathUTF8PanicSite), CanUnwind: true})
	}
	rustcserialize.EmitStr(e, text)
}
func decodeSourcePath(d rustcserialize.Decoder) RustPath {
	return NewRustPath(rustcserialize.ReadStr(d))
}
func (value innerRealFileName) encode(e rustcserialize.Encoder) {
	encodeSourcePath(value.name, e)
	encodeSourcePath(value.workingDirectory, e)
	encodeSourcePath(value.embeddableName, e)
}
func decodeInnerRealFileName(d rustcserialize.Decoder) innerRealFileName {
	name := decodeSourcePath(d)
	working := decodeSourcePath(d)
	embeddable := decodeSourcePath(d)
	return innerRealFileName{name: name, workingDirectory: working, embeddableName: embeddable}
}
func (value RealFileName) Encode(e rustcserialize.Encoder) {
	if local, ok := value.local.Get(); ok {
		e.EmitU8(1)
		local.encode(e)
	} else {
		e.EmitU8(0)
	}
	value.maybeRemapped.encode(e)
	value.scopes.Encode(e)
}
func DecodeRealFileName(d rustcserialize.Decoder) RealFileName {
	local := None[innerRealFileName]()
	if spanCodecOptionTag(d) {
		local = Some(decodeInnerRealFileName(d))
	}
	remapped := decodeInnerRealFileName(d)
	scopes := DecodeRemapPathScopeComponents(d)
	return RealFileName{local: local, maybeRemapped: remapped, scopes: scopes}
}
func (value FileName) Encode(e rustcserialize.Encoder) {
	e.EmitU8(uint8(value.kind))
	switch value.kind {
	case fileNameReal:
		value.real.Encode(e)
	case fileNameCustom:
		rustcserialize.EmitStr(e, value.custom)
	case fileNameDocTest:
		value.real.Encode(e)
		e.EmitIsize(value.line)
	default:
		rustcserialize.EncodeHash64(e, value.hash)
	}
}
func DecodeFileName(d rustcserialize.Decoder) FileName {
	tag := fileNameKind(d.ReadU8())
	switch tag {
	case fileNameReal:
		return RealFileNameFileName(DecodeRealFileName(d))
	case fileNameCustom:
		return CustomFileName(rustcserialize.ReadStr(d))
	case fileNameDocTest:
		path := DecodeRealFileName(d)
		line := d.ReadIsize()
		return DocTestSourceCode(path, line)
	case fileNameCfgSpec, fileNameAnon, fileNameMacroExpansion, fileNameProcMacroSourceCode, fileNameCliCrateAttr, fileNameInlineAsm:
		return FileName{kind: tag, hash: rustcserialize.DecodeHash64(d)}
	}
	spanFormattedPanicAt(spanCodecContext(d), spanLibLocation(511, 68), func(rustfmt.Context) any {
		return fmt.Sprintf("invalid enum variant tag while decoding `FileName`, expected 0..9, actual %d", tag)
	})
	return FileName{}
}
func (value SourceFileHashAlgorithm) Encode(e rustcserialize.Encoder) { e.EmitU8(uint8(value)) }
func DecodeSourceFileHashAlgorithm(d rustcserialize.Decoder) SourceFileHashAlgorithm {
	tag := d.ReadU8()
	if tag > 3 {
		spanFormattedPanicAt(spanCodecContext(d), spanLibLocation(1766, 79), func(rustfmt.Context) any {
			return fmt.Sprintf("invalid enum variant tag while decoding `SourceFileHashAlgorithm`, expected 0..4, actual %d", tag)
		})
	}
	return SourceFileHashAlgorithm(tag)
}
func (value SourceFileHash) Encode(e rustcserialize.Encoder) {
	value.EncodeWithByteSliceStrategy(e, spanByteSliceStrategy(e))
}
func (value SourceFileHash) EncodeWithByteSliceStrategy(e rustcserialize.Encoder, strategy rustcserialize.VectorEncodeOps[uint8]) {
	value.Kind.Encode(e)
	rustcserialize.EncodeBytesWithStrategy(e, value.value[:], strategy)
}
func DecodeSourceFileHash(d rustcserialize.Decoder) SourceFileHash {
	kind := DecodeSourceFileHashAlgorithm(d)
	n := d.ReadUsize()
	if n != 32 {
		ctx := spanCodecContext(d)
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "assertion failed: len == N", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation(SpanCodecArrayLengthPanicSite), CanUnwind: true})
	}
	value := [32]byte{}
	for i := range value {
		value[i] = d.ReadU8()
	}
	return SourceFileHash{Kind: kind, value: value}
}
func (value StableSourceFileId) Encode(e rustcserialize.Encoder) {
	rustcserialize.EncodeHash128(e, value.hash)
}
func DecodeStableSourceFileId(d rustcserialize.Decoder) StableSourceFileId {
	return StableSourceFileId{hash: rustcserialize.DecodeHash128(d)}
}
func (value MultiByteChar) Encode(e rustcserialize.Encoder) {
	value.Pos.Encode(e)
	e.EmitU8(value.Bytes)
}
func DecodeMultiByteChar(d rustcserialize.Decoder) MultiByteChar {
	pos := DecodeRelativeBytePos(d)
	bytes := d.ReadU8()
	return MultiByteChar{Pos: pos, Bytes: bytes}
}
func (value NormalizedPos) Encode(e rustcserialize.Encoder) {
	value.Pos.Encode(e)
	e.EmitU32(value.Diff)
}
func DecodeNormalizedPos(d rustcserialize.Decoder) NormalizedPos {
	pos := DecodeRelativeBytePos(d)
	diff := d.ReadU32()
	return NormalizedPos{Pos: pos, Diff: diff}
}
func (value *SourceFile) Encode(e SpanEncoder) {
	value.EncodeWithByteSliceStrategy(e, spanByteSliceStrategy(e))
}
func (value *SourceFile) EncodeWithByteSliceStrategy(e SpanEncoder, strategy rustcserialize.VectorEncodeOps[uint8]) {
	ctx := spanCodecContext(e)
	profile := spanCodecConfig(e)
	value.Name.Encode(e)
	value.SrcHash.EncodeWithByteSliceStrategy(e, strategy)
	if checksum, ok := value.ChecksumHash.Get(); ok {
		e.EmitU8(1)
		checksum.EncodeWithByteSliceStrategy(e, strategy)
	} else {
		e.EmitU8(0)
	}
	value.NormalizedSourceLen.Encode(e)
	e.EmitU32(value.UnnormalizedSourceLen)
	// The source assertion's condition temporary is destroyed before its panic
	// body. A hook may then safely reenter this same publicly accessible lock.
	guard := value.Lines.ReadWithContext(ctx)
	isLines := guard.Get().IsLines()
	guard.Close()
	if !isLines {
		spanStaticPanicAt(ctx, spanLibLocation(2047, 9), "assertion failed: self.lines.read().is_lines()")
	}
	lines := value.LinesWithContext(ctx, profile)
	e.EmitU32(uint32(len(lines)))
	if len(lines) != 0 {
		maxLine := uint(0)
		for i := 0; i+1 < len(lines); i++ {
			diff := lines[i+1].SubWithContext(lines[i], profile, ctx).ToUsize()
			if diff > maxLine {
				maxLine = diff
			}
		}
		bytesPerDiff := uint(4)
		if maxLine <= 0xff {
			bytesPerDiff = 1
		} else if maxLine <= 0xffff {
			bytesPerDiff = 2
		}
		e.EmitU8(uint8(bytesPerDiff))
		if lines[0] != 0 {
			spanFormattedPanicAt(ctx, spanLibLocation(2075, 13), func(rustfmt.Context) any {
				return fmt.Sprintf("assertion `left == right` failed\n  left: RelativeBytePos(%d)\n right: RelativeBytePos(0)", lines[0])
			})
		}
		n := uint(len(lines) - 1)
		if profile.OverflowChecks && n > ^uint(0)/bytesPerDiff {
			ctx.Panic("attempt to multiply with overflow")
		}
		raw := make([]byte, 0, n*bytesPerDiff)
		for i := 0; i+1 < len(lines); i++ {
			diff := lines[i+1].SubWithContext(lines[i], profile, ctx)
			switch bytesPerDiff {
			case 1:
				raw = append(raw, byte(diff))
			case 2:
				var b [2]byte
				binary.LittleEndian.PutUint16(b[:], uint16(diff))
				raw = append(raw, b[:]...)
			case 4:
				var b [4]byte
				binary.LittleEndian.PutUint32(b[:], uint32(diff))
				raw = append(raw, b[:]...)
			}
		}
		e.EmitRawBytes(raw)
	}
	e.EmitUsize(uint(len(value.MultibyteChars)))
	for _, char := range value.MultibyteChars {
		char.Encode(e)
	}
	value.StableId.Encode(e)
	e.EmitUsize(uint(len(value.NormalizedPos)))
	for _, pos := range value.NormalizedPos {
		pos.Encode(e)
	}
	value.Cnum.Encode(e)
}
func DecodeSourceFile(d SpanDecoder) SourceFile {
	ctx := spanCodecContext(d)
	profile := spanCodecConfig(d)
	name := DecodeFileName(d)
	hash := DecodeSourceFileHash(d)
	checksum := None[SourceFileHash]()
	if spanCodecOptionTag(d) {
		checksum = Some(DecodeSourceFileHash(d))
	}
	normalized := DecodeRelativeBytePos(d)
	unnormalized := d.ReadU32()
	numLines := d.ReadU32()
	var lines SourceFileLines
	if numLines > 0 {
		width := uint(d.ReadU8())
		numDiffs := uint(numLines) - 1
		if profile.OverflowChecks && width != 0 && numDiffs > ^uint(0)/width {
			ctx.Panic("attempt to multiply with overflow")
		}
		borrowed := d.ReadRawBytes(width * numDiffs)
		raw := make([]byte, len(borrowed))
		copy(raw, borrowed)
		lines = DiffsSourceFileLines(NewSourceFileDiffs(width, numDiffs, raw))
	} else {
		lines = LinesSourceFileLines([]RelativeBytePos{})
	}
	multibyte := rustcserialize.DecodeVector(d, rustcserialize.VectorDecodeOps[MultiByteChar, []MultiByteChar]{
		Element: func(decoder rustcserialize.Decoder, _ rustfmt.Context) *rustvec.Owned[MultiByteChar] {
			return rustvec.Own(DecodeMultiByteChar(decoder))
		},
		Collect: func(iterator rustvec.Iterator[MultiByteChar], context rustfmt.Context) []MultiByteChar {
			return rustvec.Collect(rustvec.FromIterator(iterator), rustvec.Config{Context: context, DebugAssertions: profile.DebugAssertions, ElementSize: 8}, rustvec.ElementOps[MultiByteChar]{})
		},
	})
	stable := DecodeStableSourceFileId(d)
	positions := rustcserialize.DecodeVector(d, rustcserialize.VectorDecodeOps[NormalizedPos, []NormalizedPos]{
		Element: func(decoder rustcserialize.Decoder, _ rustfmt.Context) *rustvec.Owned[NormalizedPos] {
			return rustvec.Own(DecodeNormalizedPos(decoder))
		},
		Collect: func(iterator rustvec.Iterator[NormalizedPos], context rustfmt.Context) []NormalizedPos {
			return rustvec.Collect(rustvec.FromIterator(iterator), rustvec.Config{Context: context, DebugAssertions: profile.DebugAssertions, ElementSize: 8}, rustvec.ElementOps[NormalizedPos]{})
		},
	})
	crate := DecodeCrateNum(d)
	return SourceFile{Name: name, SrcHash: hash, ChecksumHash: checksum, NormalizedSourceLen: normalized, UnnormalizedSourceLen: unnormalized, ExternalSrc: NewFreezeLockWithDrop(UnneededExternalSource(), true, func(value *ExternalSource, ctx rustfmt.Context) { value.CloseWithContext(ctx) }), Lines: NewFreezeLock(lines), MultibyteChars: multibyte, NormalizedPos: positions, StableId: stable, Cnum: crate}
}
