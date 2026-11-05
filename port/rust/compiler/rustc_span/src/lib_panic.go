// SPDX-License-Identifier: MIT OR Apache-2.0
// Source panic sites for the pinned full lib.rs runtime. Actual filenames are
// explicit caller profile witnesses, never inferred from a Go host stack or a
// presumed Rust build/cache path. Without a witness, location is honestly nil.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strconv"
)

type spanLibCodePoint struct{ line, column uint32 }

func spanLibLocation(line, column uint32) spanLibCodePoint { return spanLibCodePoint{line, column} }
func LibPanicSite(line, column uint32) string {
	return "rustc_span::lib.rs:" + strconv.FormatUint(uint64(line), 10) + ":" + strconv.FormatUint(uint64(column), 10)
}

const ScopedSessionPanicSite = "rustc_span::scoped_tls::with"
const AttrIdStepForwardPanicSite = "rustc_span::AttrId::Step::forward.expect"
const AttrIdStepBackwardPanicSite = "rustc_span::AttrId::Step::backward.expect"
const SpanCodecOptionTagPanicSite = "rustc_serialize::Option::decode.invalid_discriminant"
const SpanCodecArrayLengthPanicSite = "rustc_serialize::array::decode.assert_length"
const SpanCodecPathUTF8PanicSite = "rustc_serialize::Path::encode.to_str.unwrap"

// SpanCodecPanicLocationsAtPinnedRevision names only these used serialization
// dependency sites. The surrounding serializer source file remains partial.
func SpanCodecPanicLocationsAtPinnedRevision(serializeFile string) map[string]rustfmt.SourceLocation {
	return map[string]rustfmt.SourceLocation{
		SpanCodecOptionTagPanicSite:   {File: serializeFile, Line: 459, Column: 18},
		SpanCodecArrayLengthPanicSite: {File: serializeFile, Line: 403, Column: 9},
		SpanCodecPathUTF8PanicSite:    {File: serializeFile, Line: 516, Column: 23},
	}
}

// AttrIdStepPanicLocationsAtSDK20261104 supplies the explicitly selected SDK's
// default Step implementation sites. The caller provides its actual std file
// spelling, independently of lib.rs's build location.
func AttrIdStepPanicLocationsAtSDK20261104(rangeFile string) map[string]rustfmt.SourceLocation {
	return map[string]rustfmt.SourceLocation{
		AttrIdStepForwardPanicSite:  {File: rangeFile, Line: 119, Column: 45},
		AttrIdStepBackwardPanicSite: {File: rangeFile, Line: 214, Column: 46},
	}
}

// LibPanicLocationsAtPinnedRevision is an explicit location-profile factory.
// Both filenames are retained verbatim; this map is immutable while borrowed.
func LibPanicLocationsAtPinnedRevision(libFile, scopedTLSFile string) map[string]rustfmt.SourceLocation {
	result := make(map[string]rustfmt.SourceLocation)
	for _, point := range [][2]uint32{{273, 14}, {511, 68}, {1766, 79}, {1084, 19}, {1085, 9}, {1087, 33}, {2490, 35}, {2504, 61}, {2510, 27}, {2523, 38}, {2523, 17}, {2526, 42}, {354, 9}, {379, 9}, {1370, 1}, {1382, 9}, {1431, 9}, {1435, 9}, {1443, 9}, {1468, 9}, {1472, 9}, {1480, 9}, {1590, 9}, {1607, 9}, {1611, 9}, {1623, 9}, {2917, 9}, {2926, 9}, {154, 5}, {164, 5}, {1860, 41}, {2047, 9}, {2075, 13}, {2281, 9}, {2281, 32}, {2312, 18}, {2337, 48}, {2372, 21}, {2390, 29}, {2427, 27}, {2432, 35}, {2454, 9}, {2526, 17}, {2532, 9}, {2747, 1}} {
		result[LibPanicSite(point[0], point[1])] = rustfmt.SourceLocation{File: libFile, Line: point[0], Column: point[1]}
	}
	result[ScopedSessionPanicSite] = rustfmt.SourceLocation{File: scopedTLSFile, Line: 168, Column: 9}
	return result
}
func spanStaticPanicAt(ctx rustfmt.Context, location spanLibCodePoint, text string) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: text, Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation(LibPanicSite(location.line, location.column)), CanUnwind: true})
}
func spanFormattedPanicAt(ctx rustfmt.Context, location spanLibCodePoint, render func(rustfmt.Context) any) {
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: ctx.SourcePanicLocation(LibPanicSite(location.line, location.column)), CanUnwind: true}, render)
}
func spanPosPanicLocation(locations []spanLibCodePoint) spanLibCodePoint {
	if len(locations) != 0 {
		return locations[0]
	}
	return spanLibLocation(0, 0)
}
