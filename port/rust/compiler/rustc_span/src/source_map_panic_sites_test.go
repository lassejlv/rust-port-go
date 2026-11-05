// SPDX-License-Identifier: MIT OR Apache-2.0
// Internal source panic payloads and explicit actual-profile hook locations.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func sourceMapNativePanicLocations() map[string]rustfmt.SourceLocation {
	out := map[string]rustfmt.SourceLocation{}
	for key, pos := range map[string][2]uint32{
		SourceMapPanicExpectCWD: {255, 14}, SourceMapPanicWorkingDirectory: {1244, 18}, SourceMapPanicLookupIndex: {1088, 9},
		SourceMapPanicDoctestNegate: {433, 28}, SourceMapPanicDoctestSubtract: {433, 21}, SourceMapPanicDoctestAdd: {435, 21},
		SourceMapPanicIndentSlice: {663, 30}, SourceMapPanicExtendSlice: {763, 22}, SourceMapPanicExtendPrevSlice: {781, 22}, SourceMapPanicStartSlice: {951, 26},
		"rustc_span::source_map::start_point.add": {965, 28}, "rustc_span::source_map::next_point.add": {1000, 50}, "rustc_span::source_map::find_width.subtract_length": {1045, 51}, "rustc_span::source_map::new_imported_source_file.expect_offset": {425, 14},
		"rustc_span::source_map::find_width.forward_index": {1061, 41}, "rustc_span::source_map::find_width.forward_width": {1061, 13},
		"rustc_span::source_map::find_width.backward_index": {1063, 54}, "rustc_span::source_map::find_width.backward_width": {1063, 13},
		"rustc_span::source_map::span_to_location_info.low_column": {509, 34}, "rustc_span::source_map::span_to_location_info.high_column": {509, 66},
	} {
		out[key] = rustfmt.SourceLocation{File: "original/compiler/rustc_span/src/source_map.rs", Line: pos[0], Column: pos[1]}
	}
	return out
}
func TestSourceMapNativePanicSites(t *testing.T) {
	sourceMapRunNativePanicSites(t, "testdata/source_map_panic_sites.tsv.gz")
}
func TestSourceMapNativePanicMaxDomain(t *testing.T) {
	sourceMapRunNativePanicSites(t, "testdata/source_map_panic_max.tsv.gz")
}
func TestSourceMapNativePanicImportedTextLength(t *testing.T) {
	sourceMapRunNativePanicSites(t, "testdata/source_map_panic_width.tsv.gz")
}
func TestSourceMapNativePanicSessionMap(t *testing.T) {
	sourceMapRunNativePanicSites(t, "testdata/source_map_panic_session.tsv.gz")
}
func TestSourceMapStableIdAssertionComposition(t *testing.T) {
	sourceMapRunNativePanicSites(t, "testdata/source_map_panic_assert.tsv.gz")
}
func sourceMapRunNativePanicSites(t *testing.T, path string) {
	selected, err := sourceMapPanicEvidencePaths(path, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	var profile sourceMapPanicProducerManifest
	if selected.Scope != "" {
		expectedSHA := ""
		key := "FERRUGO_SOURCE_MAP_PANIC_" + strings.ToUpper(selected.Scope) + "_MANIFEST_SHA256"
		if value, present := os.LookupEnv(key); present {
			if !sourceMapPanicValidSHA(value) {
				t.Fatalf("%s is not a SHA256", key)
			}
			expectedSHA = value
		}
		profile, err = sourceMapPanicProfile(selected, expectedSHA)
		if err != nil {
			t.Fatal(err)
		}
		path = selected.Fixture
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	sessions := map[string]*SessionGlobals{}
	defer func() {
		for _, s := range sessions {
			s.Close()
		}
	}()
	scan := bufio.NewScanner(z)
	count, panics := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 8 {
			t.Fatal("invalid panic site fixture")
		}
		config := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
		session := sessions[p[0]]
		if session == nil {
			session = NewSpanEncodingSession(config)
			sessions[p[0]] = session
		}
		lo, _ := strconv.ParseUint(p[2], 10, 64)
		hi, _ := strconv.ParseUint(p[3], 10, 64)
		offset, _ := strconv.ParseInt(p[5], 10, 64)
		var events []string
		locations := sourceMapNativePanicLocations()
		if p[4] == "getmap_empty" {
			// The selected manifest retains its actual producer filename verbatim.
			locations[ScopedSessionPanicSite] = profile.ScopedTLS
		}
		if p[4] == "assert_id" {
			locations[SourceMapPanicStableIdAssertion] = profile.Assertion
		}
		ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: locations, Hook: func(info rustfmt.SourcePanicInfo, ctx rustfmt.Context) {
			if info.Location == nil {
				t.Fatalf("missing explicit native location for %s", p[4])
			}
			events = append(events, fmt.Sprintf("%s|%s|%d|%d|%s", info.Kind, info.Location.File, info.Location.Line, info.Location.Column, hex.EncodeToString([]byte(fmt.Sprint(info.Payload)))))
		}})
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			sm := NewSourceMapWithConfig(EmptyFilePathMapping(), config)
			defer sm.Close()
			switch p[4] {
			case "assert_id":
				a, e := NewSourceFileWithContext(CustomFileName("assert-a"), "", SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, config)
				if e != nil {
					t.Fatal(e)
				}
				defer spanCloseOwnedCallback(ctx, a.CloseWithContext)
				b, e := NewSourceFileWithContext(CustomFileName("assert-b"), "", SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, config)
				if e != nil {
					t.Fatal(e)
				}
				defer spanCloseOwnedCallback(ctx, b.CloseWithContext)
				sourceMapAssertStableId(&a.StableId, &b.StableId, config, ctx)
				return
			case "getmap_empty":
				GetSourceMapInScopeWithContext(NewSessionScope(), ctx)
				return
			case "idx_empty":
				sm.LookupSourceFileIdxWithContext(BytePos(lo), ctx)
				return
			case "local_expect":
				name := RealFileNameFromVirtualPath(NewRustPath("virtual"))
				EmptyFilePathMappingValue := EmptyFilePathMapping()
				EmptyFilePathMappingValue.ToRealFilenameWithContext(&name, NewRustPath("local"), ctx)
				return
			case "doctest":
				name := DocTestSourceCode(RealFileNameFromVirtualPath(NewRustPath("doc")), int(offset))
				sm.DoctestOffsetLineWithContext(&name, uint(lo), ctx)
				return
			}
			if strings.HasPrefix(p[4], "max_") {
				length, _ := strconv.ParseUint(sourceMapOracleString(p[1]), 10, 32)
				sourceMapLockedImport(sm, "max-a", uint32(length), ctx)
				sp := WithRootCtxt(session, BytePos(lo), BytePos(hi), config)
				switch p[4] {
				case "max_start":
					sm.StartPointWithContext(session, sp, ctx)
				case "max_end":
					sm.EndPointWithContext(session, sp, ctx)
				case "max_next":
					sm.NextPointWithContext(session, sp, ctx)
				case "max_import":
					sourceMapLockedImport(sm, "max-b", 0, ctx)
				}
				return
			}
			if strings.HasPrefix(p[4], "width_") {
				src := sourceMapOracleString(p[1])
				template, failure := NewSourceFileWithContext(CustomFileName("width"), src, SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, config)
				if failure != nil {
					t.Fatal(failure)
				}
				if owner, ok := template.Src.Get(); ok {
					defer owner.CloseWithContext(ctx)
				}
				foreign := sm.NewImportedSourceFileWithContext(template.Name, template.SrcHash, template.ChecksumHash, template.StableId, uint32(offset), template.UnnormalizedSourceLen, CrateNumFromU32(1), template.Lines, template.MultibyteChars, template.NormalizedPos, 0, ctx)
				defer foreign.CloseWithContext(ctx)
				if !foreign.Borrow().AddExternalSrcWithContext(func() Option[string] { return Some(src) }, ctx, config) {
					t.Fatal("hash-matching external source rejected")
				}
				sp := WithRootCtxt(session, BytePos(lo), BytePos(hi), config)
				if p[4] == "width_next" {
					sm.NextPointWithContext(session, sp, ctx)
				} else {
					sm.EndPointWithContext(session, sp, ctx)
				}
				return
			}
			file := sm.NewSourceFile(CustomFileName("input.rs"), sourceMapOracleString(p[1]))
			defer file.Close()
			sp := WithRootCtxt(session, BytePos(lo), BytePos(hi), config)
			switch p[4] {
			case "indent":
				sm.IndentationBeforeWithContext(session, sp, ctx)
			case "while":
				sm.SpanExtendWhileWithContext(session, sp, func(rune) bool { return true }, ctx)
			case "prev_while":
				sm.SpanExtendPrevWhileWithContext(session, sp, func(rune) bool { return true }, ctx)
			case "start":
				sm.StartPointWithContext(session, sp, ctx)
			default:
				t.Fatal("invalid native operation")
			}
		}()
		want := sourceMapOracleString(p[7])
		if strconv.FormatBool(caught) != p[6] || strings.Join(events, "\n") != want {
			t.Fatalf("record%d %s/%s src%q lo%s offset%s caught%t events%q want%s/%q", count, p[0], p[4], sourceMapOracleString(p[1]), p[2], p[5], caught, events, p[6], want)
		}
		if caught {
			panics++
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	wantCounts := map[string][2]int{
		"source_map_panic_sites.tsv.gz":   {1648, 594},
		"source_map_panic_max.tsv.gz":     {784, 108},
		"source_map_panic_session.tsv.gz": {4, 4},
		"source_map_panic_assert.tsv.gz":  {4, 2},
		"source_map_panic_width.tsv.gz":   {1600, 234},
	}[filepath.Base(path)]
	if count != wantCounts[0] || panics != wantCounts[1] {
		t.Fatalf("native corpus counts %d/%d, want %d/%d", count, panics, wantCounts[0], wantCounts[1])
	}
	t.Logf("%d native operation records, %d caught panics", count, panics)
}
func TestSourceMapPanicLocationIsExplicit(t *testing.T) {
	var info rustfmt.SourcePanicInfo
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(value rustfmt.SourcePanicInfo, _ rustfmt.Context) { info = value }})
	func() {
		defer func() { recover() }()
		sourceMapPanic(ctx, SourceMapPanicLookupIndex, rustfmt.PanicStaticStr, "probe")
	}()
	if info.Location != nil {
		t.Fatal("unconfigured source profile invented a location")
	}
}
