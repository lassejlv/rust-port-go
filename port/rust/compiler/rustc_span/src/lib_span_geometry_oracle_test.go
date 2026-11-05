// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func geometryNumber(text string) uint64 {
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}
func geometryParent(text string) Option[LocalDefId] {
	if text == "-" {
		return None[LocalDefId]()
	}
	return Some(LocalDefId{LocalDefIndex: DefIndexFromU32(uint32(geometryNumber(text)))})
}
func geometrySpan(session *SessionGlobals, p []string, cfg SpanEncodingConfig) Span {
	return NewSpan(session, BytePos(geometryNumber(p[0])), BytePos(geometryNumber(p[1])), SyntaxContextFromU32(uint32(geometryNumber(p[2]))), geometryParent(p[3]), cfg)
}
func geometryRepr(session *SessionGlobals, value Span, cfg SpanEncodingConfig) string {
	d := value.DataUntracked(session, cfg)
	parent := "-"
	if p, ok := d.Parent.Get(); ok {
		parent = strconv.FormatUint(uint64(p.LocalDefIndex.AsU32()), 10)
	}
	return fmt.Sprintf("%d,%d,%d,%s", d.Lo, d.Hi, d.Ctxt.AsU32(), parent)
}
func geometryOptional(session *SessionGlobals, value Option[Span], cfg SpanEncodingConfig) string {
	if s, ok := value.Get(); ok {
		return geometryRepr(session, s, cfg)
	}
	return "_"
}

func geometryReplay(request string, tracked *[]uint32) string {
	parts := strings.Split(request, "\t")
	profile := parts[0]
	p := parts[1:]
	cfg := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	session := NewSessionGlobals(Edition2024, nil, None[SourceMapInputs](), cfg)
	defer session.Close()
	a, b := geometrySpan(session, p[1:5], cfg), geometrySpan(session, p[5:9], cfg)
	x, y := geometryNumber(p[9]), geometryNumber(p[10])
	mode := uint8(geometryNumber(p[11]))
	inserted := []string{}
	mapValue := session.metavarSpans
	if mode&1 != 0 {
		ctxt := b.Ctxt()
		if mode&4 != 0 {
			ctxt = SyntaxContextFromU32(2)
		}
		inserted = append(inserted, strconv.FormatBool(mapValue.Insert(a, NewSpan(session, BytePos(x), BytePos(y), ctxt, geometryParent(p[4]), cfg))))
	}
	if mode&2 != 0 {
		ctxt := a.Ctxt()
		if mode&4 != 0 {
			ctxt = SyntaxContextFromU32(2)
		}
		inserted = append(inserted, strconv.FormatBool(mapValue.Insert(b, NewSpan(session, BytePos(y), BytePos(x), ctxt, geometryParent(p[8]), cfg))))
	}
	if mode&16 != 0 {
		mapValue.Get(a)
	}
	if mode&8 != 0 {
		read := mapValue.FreezeAndGetReadSpans()
		read.Close()
	}
	*tracked = (*tracked)[:0]
	result := "panic"
	func() {
		defer func() {
			if recover() != nil {
				result = "panic"
			}
		}()
		var text string
		switch p[0] {
		case "lo":
			text = strconv.FormatUint(uint64(a.Lo(session, cfg)), 10)
		case "hi":
			text = strconv.FormatUint(uint64(a.Hi(session, cfg)), 10)
		case "lohi":
			lo, hi := a.LoHiWithContext(session, rustfmt.Context{}, cfg)
			text = fmt.Sprintf("%d,%d", lo, hi)
		case "withlo":
			text = geometryRepr(session, a.WithLoWithContext(session, BytePos(x), rustfmt.Context{}, cfg), cfg)
		case "withhi":
			text = geometryRepr(session, a.WithHiWithContext(session, BytePos(x), rustfmt.Context{}, cfg), cfg)
		case "withctxt":
			text = geometryRepr(session, a.WithCtxtWithContext(session, SyntaxContextFromU32(uint32(x)), rustfmt.Context{}, cfg), cfg)
		case "root":
			text = geometryRepr(session, WithRootCtxt(session, BytePos(x), BytePos(y), cfg), cfg)
		case "shlo":
			text = geometryRepr(session, a.ShrinkToLo(session, cfg), cfg)
		case "shhi":
			text = geometryRepr(session, a.ShrinkToHi(session, cfg), cfg)
		case "empty":
			text = strconv.FormatBool(a.IsEmpty(session, cfg))
		case "subdummy":
			text = geometryRepr(session, a.SubstituteDummy(session, b, cfg), cfg)
		case "contains":
			text = strconv.FormatBool(a.Contains(session, b, cfg))
		case "overlap":
			text = strconv.FormatBool(a.Overlaps(session, b, cfg))
		case "adjacent":
			text = strconv.FormatBool(a.OverlapsOrAdjacent(session, b, cfg))
		case "trimstart":
			text = geometryOptional(session, a.TrimStart(session, b, cfg), cfg)
		case "trimend":
			text = geometryOptional(session, a.TrimEnd(session, b, cfg), cfg)
		case "neighbor":
			text = geometryRepr(session, a.WithNeighbor(session, b, cfg), cfg)
		case "to":
			text = geometryRepr(session, a.To(session, b, cfg), cfg)
		case "between":
			text = geometryRepr(session, a.Between(session, b, cfg), cfg)
		case "until":
			text = geometryRepr(session, a.Until(session, b, cfg), cfg)
		case "split":
			lo, hi := a.SplitAt(session, uint32(x), cfg)
			text = geometryRepr(session, lo, cfg) + ";" + geometryRepr(session, hi, cfg)
		case "inner":
			text = geometryRepr(session, a.FromInner(session, InnerSpan{Start: uint(x), End: uint(y)}, cfg), cfg)
		default:
			panic("unknown geometry method")
		}
		result = "ok:" + text
	}()
	observed := make([]string, len(*tracked))
	for i, p := range *tracked {
		observed[i] = strconv.FormatUint(uint64(p), 10)
	}
	read := mapValue.FreezeAndGetReadSpans()
	defer read.Close()
	_, ra := read.Get(&a)
	_, rb := read.Get(&b)
	return fmt.Sprintf("%s\t%s\t%s\t%d:%t:%t", result, strings.Join(observed, ","), strings.Join(inserted, ","), read.Len(), ra, rb)
}

func TestOriginalSpanGeometry(t *testing.T) {
	data, err := os.ReadFile("../../../../../tools/port-rust-span-lib/span-geometry-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records int             `json:"records"`
		SHA     string          `json:"fixture_sha256"`
		Files   []spanProofFile `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	verifySpanProofFiles(t, manifest.Files)
	compressed, err := os.ReadFile("testdata/lib_span_geometry.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.SHA {
		t.Fatal("geometry fixture checksum differs")
	}
	file, err := os.Open("testdata/lib_span_geometry.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var tracked []uint32
	old := SwapSpanTrackHook(NewSpanTrackHook(func(parent LocalDefId, _ rustfmt.Context) { tracked = append(tracked, parent.LocalDefIndex.AsU32()) }))
	defer SwapSpanTrackHook(old)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	count := 0
	for scanner.Scan() {
		p := strings.SplitN(scanner.Text(), "\t|\t", 2)
		if len(p) != 2 {
			t.Fatal("geometry schema")
		}
		if got := geometryReplay(p[0], &tracked); got != p[1] {
			t.Fatalf("geometry row%d %s\ngot %s\nwant %s", count, p[0], got, p[1])
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records {
		t.Fatal("geometry record count differs")
	}
	t.Logf("%d actual whole-source Span geometry/track/metavar records match", count)
}
