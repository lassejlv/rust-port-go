// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: compiler/rustc_span/src/analyze_source_file/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. All 12 macro-generated tests.
package analyze_test

import (
	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"reflect"
	"testing"
)

func TestOriginalAnalyzeCases(t *testing.T) {
	cases := []struct {
		name, text string
		lines      []span.RelativeBytePos
		multi      []span.MultiByteChar
	}{
		{"empty_text", "", []span.RelativeBytePos{}, []span.MultiByteChar{}},
		{"newlines_short", "a\nc", []span.RelativeBytePos{0, 2}, []span.MultiByteChar{}},
		{"newlines_long", "012345678\nabcdef012345678\na", []span.RelativeBytePos{0, 10, 26}, []span.MultiByteChar{}},
		{"newline_and_multi_byte_char_in_same_chunk", "01234β789\nbcdef0123456789abcdef", []span.RelativeBytePos{0, 11}, []span.MultiByteChar{{Pos: 5, Bytes: 2}}},
		{"newline_and_control_char_in_same_chunk", "01234\u00076789\nbcdef0123456789abcdef", []span.RelativeBytePos{0, 11}, []span.MultiByteChar{}},
		{"multi_byte_char_short", "aβc", []span.RelativeBytePos{0}, []span.MultiByteChar{{Pos: 1, Bytes: 2}}},
		{"multi_byte_char_long", "0123456789abcΔf012345β", []span.RelativeBytePos{0}, []span.MultiByteChar{{Pos: 13, Bytes: 2}, {Pos: 22, Bytes: 2}}},
		{"multi_byte_char_across_chunk_boundary", "0123456789abcdeΔ123456789abcdef01234", []span.RelativeBytePos{0}, []span.MultiByteChar{{Pos: 15, Bytes: 2}}},
		{"multi_byte_char_across_chunk_boundary_tail", "0123456789abcdeΔ....", []span.RelativeBytePos{0}, []span.MultiByteChar{{Pos: 15, Bytes: 2}}},
		{"non_narrow_short", "0\t2", []span.RelativeBytePos{0}, []span.MultiByteChar{}},
		{"non_narrow_long", "01\t3456789abcdef01234567\u00079", []span.RelativeBytePos{0}, []span.MultiByteChar{}},
		{"output_offset_all", "01\t345\n789abcΔf01234567\u00079\nbcΔf", []span.RelativeBytePos{0, 7, 27}, []span.MultiByteChar{{Pos: 13, Bytes: 2}, {Pos: 29, Bytes: 2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, arch := range []string{"generic", "amd64", "loong64"} {
				for _, checked := range []bool{false, true} {
					for _, debug := range []bool{false, true} {
						lines, multi := span.AnalyzeSourceFileWithConfig(c.text, span.SourceFileAnalysisConfig{Architecture: arch, SSE2Available: true, LSXAvailable: true, DebugAssertions: debug, OverflowChecks: checked})
						if !reflect.DeepEqual(lines, c.lines) || !reflect.DeepEqual(multi, c.multi) {
							t.Fatalf("%s d%v/o%v: (%v,%v), want (%v,%v)", arch, debug, checked, lines, multi, c.lines, c.multi)
						}
					}
				}
			}
		})
	}
}
