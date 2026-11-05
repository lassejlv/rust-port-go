// SPDX-License-Identifier: MIT OR Apache-2.0
// All four tests in pinned compiler/rustc_span/src/tests.rs, preserving each
// original input, expected position, and trim case. Supporting Go APIs adapt
// explicit sessions and borrowed line slices.
package rustcspan

import (
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"reflect"
	"testing"
)

func TestLibUpstreamLookupLine(t *testing.T) {
	sf, err := NewSourceFile(AnonFileName(rustchashes.ZeroHash64()), "abcdefghijklm\nabcdefghij\n...", SourceFileHashSha256, Some(SourceFileHashSha256))
	if err != nil {
		t.Fatal(err)
	}
	defer sf.CloseWithContext(rustfmt.Context{})
	sf.StartPos = 3
	if got := sf.LinesWithContext(rustfmt.Context{}); !reflect.DeepEqual(got, []RelativeBytePos{0, 14, 25}) {
		t.Fatal(got)
	}
	for _, c := range [][2]uint{{0, 0}, {1, 0}, {13, 0}, {14, 1}, {15, 1}, {25, 2}, {26, 2}} {
		if got := sf.LookupLine(RelativeBytePos(c[0])); got != Some(c[1]) {
			t.Fatalf("%v: %v", c, got)
		}
	}
}
func TestLibUpstreamNormalizeNewlines(t *testing.T) {
	for _, c := range []struct {
		before, after string
		positions     []uint32
	}{{"", "", []uint32{}}, {"\n", "\n", []uint32{}}, {"\r", "\r", []uint32{}}, {"\r\r", "\r\r", []uint32{}}, {"\r\n", "\n", []uint32{1}}, {"hello world", "hello world", []uint32{}}, {"hello\nworld", "hello\nworld", []uint32{}}, {"hello\r\nworld", "hello\nworld", []uint32{6}}, {"\r\nhello\r\nworld\r\n", "\nhello\nworld\n", []uint32{1, 7, 13}}, {"\r\r\n", "\r\n", []uint32{2}}, {"hello\rworld", "hello\rworld", []uint32{}}} {
		actual := c.before
		records := []NormalizedPos{}
		normalizeSourceNewlines(&actual, &records, rustfmt.Context{}, DefaultSpanEncodingConfig())
		positions := []uint32{}
		for _, record := range records {
			positions = append(positions, uint32(record.Pos))
		}
		if actual != c.after || !reflect.DeepEqual(positions, c.positions) {
			t.Fatalf("%q: %q %v", c.before, actual, positions)
		}
	}
}
func TestLibUpstreamTrim(t *testing.T) {
	span := func(lo, hi uint) Span {
		return NewSpan(nil, BytePosFromUsize(lo), BytePosFromUsize(hi), RootSyntaxContext(), None[LocalDefId]())
	}
	wellBefore, before, start, mid, end, after, wellAfter := uint(1), uint(3), uint(5), uint(7), uint(9), uint(11), uint(13)
	other := span(start, end).WithCtxt(nil, SyntaxContextFromU32(999))
	for _, c := range []struct {
		lo, hi uint
		want   Option[Span]
	}{{wellBefore, before, Some(span(wellBefore, before))}, {wellBefore, start, Some(span(wellBefore, start))}, {wellBefore, mid, Some(span(wellBefore, start))}, {wellBefore, end, Some(span(wellBefore, start))}, {wellBefore, after, Some(span(wellBefore, start))}, {start, mid, None[Span]()}, {start, end, None[Span]()}, {start, after, None[Span]()}, {mid, end, None[Span]()}, {mid, after, None[Span]()}, {end, after, None[Span]()}, {after, wellAfter, None[Span]()}} {
		if got := span(c.lo, c.hi).TrimEnd(nil, other); got != c.want {
			t.Fatalf("trim_end %v: %v != %v", c, got, c.want)
		}
	}
	for _, c := range []struct {
		lo, hi uint
		want   Option[Span]
	}{{after, wellAfter, Some(span(after, wellAfter))}, {end, wellAfter, Some(span(end, wellAfter))}, {mid, wellAfter, Some(span(end, wellAfter))}, {start, wellAfter, Some(span(end, wellAfter))}, {before, wellAfter, Some(span(end, wellAfter))}, {mid, end, None[Span]()}, {start, end, None[Span]()}, {before, end, None[Span]()}, {start, mid, None[Span]()}, {before, mid, None[Span]()}, {before, start, None[Span]()}, {wellBefore, before, None[Span]()}} {
		if got := span(c.lo, c.hi).TrimStart(nil, other); got != c.want {
			t.Fatalf("trim_start %v: %v != %v", c, got, c.want)
		}
	}
}
func TestLibUpstreamUnnormalizedSourceLength(t *testing.T) {
	sf, err := NewSourceFile(AnonFileName(rustchashes.ZeroHash64()), "\ufeffhello\r\nferries\r\n", SourceFileHashSha256, Some(SourceFileHashSha256))
	if err != nil {
		t.Fatal(err)
	}
	defer sf.CloseWithContext(rustfmt.Context{})
	if sf.UnnormalizedSourceLen != 19 || sf.NormalizedSourceLen != 14 {
		t.Fatal(sf.UnnormalizedSourceLen, sf.NormalizedSourceLen)
	}
}
