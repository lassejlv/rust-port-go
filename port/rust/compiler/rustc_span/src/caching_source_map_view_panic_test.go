// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type cachingPanicRecord struct {
	Profile string   `json:"profile"`
	Mode    string   `json:"mode"`
	Case    string   `json:"case"`
	Caller  string   `json:"caller"`
	Abort   bool     `json:"abort"`
	Events  []string `json:"events"`
}
type cachingPanicProfile struct {
	Records   int                                `json:"records"`
	Aborts    int                                `json:"actual_SIGABRT"`
	Hooks     int                                `json:"hook_records"`
	Locations map[string]*rustfmt.SourceLocation `json:"locations"`
	SHA       string                             `json:"gzip_sha256"`
	Bindings  map[string]string                  `json:"go_bindings"`
}
type cachingPanicState struct {
	request cachingPanicRecord
	profile cachingPanicProfile
	config  SpanEncodingConfig
	source  *SourceMap
	view    *CachingSourceMapView
	emit    func(string)
}

func cachingPanicLoadProfile(t *testing.T) cachingPanicProfile {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-caching-source-map-view/panic-sites/VALIDATION.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile cachingPanicProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	for path, want := range profile.Bindings {
		data, err := os.ReadFile("../../../../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
			t.Fatalf("cache hook source binding changed: %s; regenerate original evidence", path)
		}
	}
	return profile
}
func cachingPanicCaller(ctx rustfmt.Context) string {
	if ctx.Runtime != nil {
		if label, ok := ctx.Runtime.Value.(string); ok {
			return label
		}
	}
	return "nil"
}
func (s *cachingPanicState) counts() string {
	guard := s.source.Files()
	defer guard.Close()
	out := make([]string, 0, guard.Len())
	for i := uint(0); i < guard.Len(); i++ {
		out = append(out, fmt.Sprint(guard.Borrow(i).StrongCount()))
	}
	return strings.Join(out, "_")
}
func (s *cachingPanicState) event(tag string, ctx rustfmt.Context) {
	s.emit(fmt.Sprintf("E:%s:%s:%t:%s", tag, cachingPanicCaller(ctx), ctx.Panicking(), s.counts()))
}
func (s *cachingPanicState) current(label string) rustfmt.Context {
	if label == "nil" {
		return rustfmt.Context{}
	}
	return rustfmt.WithPanicOperations(rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: label}}, &rustfmt.PanicOperations{LocationCells: s.profile.Locations, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		if info.Location == nil {
			panic("cache proof: selected source location missing")
		}
		s.emit(fmt.Sprintf("H:%s:%s:%x:%x:%d:%d:%t:%t:%t:%s", cachingPanicCaller(current), info.Kind, fmt.Sprint(info.Payload), info.Location.File, info.Location.Line, info.Location.Column, current.Panicking(), info.CanUnwind, info.ForceNoBacktrace, s.counts()))
		if s.request.Mode == "processing" && info.Payload == "outer" {
			s.operate(current)
		}
	}})
}
func (s *cachingPanicState) importFile(name string, length uint32, positions []uint32, ctx rustfmt.Context) {
	seed, err := NewSourceFileWithContext(CustomFileName(name), "seed", SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, s.config)
	if err != nil {
		panic(err)
	}
	defer seed.CloseWithContext(ctx)
	lines := make([]RelativeBytePos, len(positions))
	for i, p := range positions {
		lines[i] = RelativeBytePos(p)
	}
	file := s.source.NewImportedSourceFileWithContext(seed.Name, seed.SrcHash, seed.ChecksumHash, seed.StableId, length, seed.UnnormalizedSourceLen, CrateNumZero(), NewFreezeLock(LinesSourceFileLines(lines)), seed.MultibyteChars, seed.NormalizedPos, 0, ctx)
	file.CloseWithContext(ctx)
}
func (s *cachingPanicState) point(tag string, pos BytePos, ctx rustfmt.Context) {
	result, ok := s.view.BytePosToLineAndColWithContext(pos, ctx)
	if !ok {
		s.event(tag+":none", ctx)
		return
	}
	defer spanCloseOwnedCallback(ctx, result.File.CloseWithContext)
	s.event(fmt.Sprintf("%s:b:%d:%d:%d", tag, result.File.Borrow().StartPos, result.Line, result.Column), ctx)
}
func (s *cachingPanicState) span(lo, hi BytePos, ctx rustfmt.Context) {
	result, ok := s.view.SpanDataToLinesAndColsWithContext(&SpanData{Lo: lo, Hi: hi, Ctxt: RootSyntaxContext()}, ctx)
	if !ok {
		s.event("span:none", ctx)
		return
	}
	s.event(fmt.Sprintf("span:%d:%d:%d:%d:%d", result.File.StartPos, result.LoLine, result.LoColumn, result.HiLine, result.HiColumn), ctx)
}
func (s *cachingPanicState) attempt(tag string, ctx rustfmt.Context, body func()) {
	caught := false
	func() {
		defer func() {
			if recover() != nil {
				caught = true
			}
		}()
		body()
	}()
	s.emit(fmt.Sprintf("C:%s:%t:%s:%s", tag, caught, cachingPanicCaller(ctx), s.counts()))
}
func (s *cachingPanicState) prepare() {
	ctx := s.current("A")
	s.source = NewSourceMapWithConfig(EmptyFilePathMapping(), s.config)
	switch s.request.Case {
	case "empty":
		return
	case "unwrap_byte":
		s.importFile("A", 10, nil, ctx)
	case "unwrap_span":
		s.importFile("A", 10, []uint32{2}, ctx)
	case "lo_upper":
		s.importFile("A", 0, nil, ctx)
		s.importFile("B", 10, []uint32{0, ^uint32(0)}, ctx)
	case "hi_upper":
		s.importFile("A", 0, nil, ctx)
		s.importFile("B", 10, []uint32{0, 5, ^uint32(0)}, ctx)
	case "stale_hi_lower", "stale_contains_lo", "stale_hit":
		s.importFile("A", 10, []uint32{0}, ctx)
		lines := []uint32{7}
		if s.request.Case == "stale_contains_lo" {
			lines = []uint32{^uint32(0)}
		}
		s.importFile("B", 10, lines, ctx)
	case "ok":
		s.importFile("A", 10, []uint32{0, 5}, ctx)
	default:
		panic("invalid cache proof case")
	}
	s.view = NewCachingSourceMapViewWithContext(s.source, ctx)
	if strings.HasPrefix(s.request.Case, "stale_") {
		s.attempt("prime0", ctx, func() { s.point("prime0", 0, ctx) })
		s.attempt("prime1", ctx, func() { s.point("prime1", 13, ctx) })
	}
	s.event("prepared", ctx)
}
func (s *cachingPanicState) operate(ctx rustfmt.Context) {
	defer spanCloseOwnedCallback(ctx, func(current rustfmt.Context) { s.event("local", current) })
	s.event("before", ctx)
	switch s.request.Case {
	case "empty":
		view := NewCachingSourceMapViewWithContext(s.source, ctx)
		defer spanCloseOwnedCallback(ctx, view.CloseWithContext)
		s.event("new", ctx)
	case "unwrap_byte":
		s.point("point", 0, ctx)
	case "unwrap_span":
		s.span(0, 1, ctx)
	case "lo_upper":
		s.span(1, 1, ctx)
	case "hi_upper":
		s.span(1, 6, ctx)
	case "stale_hi_lower", "stale_contains_lo":
		s.span(0, 10, ctx)
	case "stale_hit":
		s.point("point", 0, ctx)
	case "ok":
		s.point("point", 1, ctx)
	}
}
func (s *cachingPanicState) run() {
	s.prepare()
	ctx := s.current(s.request.Caller)
	defer s.source.CloseWithContext(ctx)
	outer := func() {
		if s.request.Mode == "processing" && ctx.Runtime == nil {
			// The explicit outer-runtime witness enters its source hook with
			// no observer, runs the operation, then resumes the outer payload.
			processing := ctx
			processing.ProcessingPanic = true
			processing.Unwinding = true
			s.operate(processing)
			ctx.ResumeUnwind("outer")
		}
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("outer"), CanUnwind: true})
	}
	switch s.request.Mode {
	case "normal":
		s.attempt("operation", ctx, func() { s.operate(ctx) })
	case "inherited":
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			defer func() {
				payload := recover()
				if payload != nil {
					unwind := ctx
					unwind.Unwinding = true
					s.event("outer-drop", unwind)
					s.attempt("operation", unwind, func() { s.operate(unwind) })
					s.event("outer-end", unwind)
					ctx.ResumeUnwind(payload)
				}
			}()
			outer()
		}()
		s.emit(fmt.Sprintf("C:outer:%t:%s:%s", caught, cachingPanicCaller(ctx), s.counts()))
	case "processing":
		s.attempt("outer", ctx, outer)
	}
	s.event("final", ctx)
	if s.view != nil {
		s.view.CloseWithContext(ctx)
	}
	s.event("closed", ctx)
}

func TestCachingSourceMapPanicNativeChild(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_CACHING_PANIC_CHILD")
	if !ok {
		return
	}
	var q cachingPanicRecord
	if err := json.Unmarshal([]byte(encoded), &q); err != nil {
		t.Fatal(err)
	}
	state := cachingPanicState{request: q, profile: cachingPanicLoadProfile(t), config: SpanEncodingConfig{DebugAssertions: q.Profile[1] == '1', OverflowChecks: q.Profile[3] == '1'}, emit: func(line string) { fmt.Println(line) }}
	state.run()
	os.Exit(0)
}
func TestCachingSourceMapFullPanicNativeOracle(t *testing.T) {
	profile := cachingPanicLoadProfile(t)
	if profile.Records != 324 || profile.Aborts != 84 || profile.Hooks != 364 {
		t.Fatalf("incomplete native cache hook corpus: %d/%d/%d", profile.Records, profile.Aborts, profile.Hooks)
	}
	data, err := os.ReadFile("testdata/caching_source_map_view_panic.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != profile.SHA {
		t.Fatal("caching panic fixture hash differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scanner := bufio.NewScanner(z)
	count, aborts, hooks := 0, 0, 0
	for scanner.Scan() {
		var q cachingPanicRecord
		if err := json.Unmarshal(scanner.Bytes(), &q); err != nil {
			t.Fatal(err)
		}
		var events []string
		if q.Abort {
			encoded, _ := json.Marshal(q)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCachingSourceMapPanicNativeChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_CACHING_PANIC_CHILD="+string(encoded))
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			cancel()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("record%d expected abort134, got%v stderr%s", count, err, stderr.String())
			}
			events = strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			aborts++
		} else {
			state := cachingPanicState{request: q, profile: profile, config: SpanEncodingConfig{DebugAssertions: q.Profile[1] == '1', OverflowChecks: q.Profile[3] == '1'}, emit: func(line string) { events = append(events, line) }}
			state.run()
		}
		if !reflect.DeepEqual(events, q.Events) {
			t.Fatalf("record%d %s/%s/%s/%s got%q want%q", count, q.Profile, q.Mode, q.Case, q.Caller, events, q.Events)
		}
		for _, event := range events {
			if strings.HasPrefix(event, "H:") {
				hooks++
			}
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != profile.Records || aborts != profile.Aborts || hooks != profile.Hooks {
		t.Fatalf("counts %d/%d/%d want%d/%d/%d", count, aborts, hooks, profile.Records, profile.Aborts, profile.Hooks)
	}
	t.Logf("%d whole-source hook records, %d hook observations, %d isolated aborts", count, hooks, aborts)
}

func TestCachingSourceMapPanicLocationRequiresProfile(t *testing.T) {
	var observed rustfmt.SourcePanicInfo
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) { observed = info }})
	mapValue := NewSourceMap(EmptyFilePathMapping())
	defer mapValue.Close()
	func() { defer func() { recover() }(); NewCachingSourceMapViewWithContext(mapValue, ctx) }()
	if observed.Kind != rustfmt.PanicOwnedString || observed.Location != nil || observed.Payload != "index out of bounds: the len is 0 but the index is 0" {
		t.Fatalf("unlocated profile %v", observed)
	}
}
