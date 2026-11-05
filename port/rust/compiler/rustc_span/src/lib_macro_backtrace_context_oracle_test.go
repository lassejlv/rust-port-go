// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type backtraceContextRequest struct {
	Case, Mode      string
	Debug, Overflow bool
}
type backtraceContextManifest struct {
	Records, Aborts int
	RawSHA          string            `json:"raw_sha256"`
	FixtureSHA      string            `json:"fixture_sha256"`
	Locations       map[string]string `json:"source_locations"`
	Files           map[string]string
	Tools           map[string]string `json:"tool_inputs_sha256"`
}
type backtraceContextHcx struct{}

func (backtraceContextHcx) StableHashSpan(s ds.RawSpan, h *ds.StableHasher) { h.WriteU64(s.Value) }
func (backtraceContextHcx) DefPathHash(d ds.RawDefId) ds.Fingerprint {
	return ds.FingerprintFromU64(uint64(d.Crate)+100, uint64(d.Index)+1000)
}
func (backtraceContextHcx) StableHashControls() ds.StableHashControls {
	return ds.StableHashControls{HashSpans: true}
}
func (backtraceContextHcx) AssertDefaultStableHashControls(string) {}

type backtraceContextState struct {
	q        backtraceContextRequest
	session  *sp.SessionGlobals
	features []sp.Shared[[]sp.Symbol]
	iter     *sp.MacroBacktraceIterator
	tracks   uint32
	fail     uint32
}

func backtraceContextRuntime(c rf.Context) int {
	if c.Runtime == nil {
		return 0
	}
	v, _ := c.Runtime.Value.(int)
	return v
}
func (s *backtraceContextState) event(c rf.Context, tag string) {
	fmt.Printf("%s:%t:%d:%d,%d\n", tag, c.Panicking(), backtraceContextRuntime(c), s.features[0].StrongCount(), s.features[1].StrongCount())
}

// This client witness supplies the complete local destruction of the safe
// static tracking function and the caller's ambient destructor. Production
// iterator cleanup remains inside NextWithContext and is independently observed
// through its real Shared fields, rather than reconstructed in this caller.
func backtraceContextClose(c rf.Context, close func(rf.Context)) {
	p := recover()
	if p == nil {
		close(c)
		return
	}
	if c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	u := c
	u.Unwinding = true
	func() {
		defer func() {
			if recover() != nil {
				c.AbortCleanup()
			}
		}()
		close(u)
	}()
	c.ResumeUnwind(p)
}
func backtraceContextRaise(c rf.Context, text string) {
	c.PanicAt(rf.SourcePanicInfo{Payload: text, Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation(text)})
}
func (s *backtraceContextState) track(id sp.LocalDefId, c rf.Context) {
	defer backtraceContextClose(c, func(c rf.Context) { s.event(c, "track_local_drop") })
	s.tracks++
	s.event(c, fmt.Sprintf("track:%d:%d", s.tracks, id.LocalDefIndex.AsU32()))
	if s.fail == s.tracks {
		if strings.HasPrefix(s.q.Case, "resume_") {
			c.ResumeUnwind("resumed")
		}
		backtraceContextRaise(c, "track")
	}
}
func (s *backtraceContextState) next(c rf.Context, label string) {
	s.event(c, "next:"+label)
	result := s.iter.NextWithContext(c)
	if data, ok := result.Get(); ok {
		s.event(c, fmt.Sprintf("yield:%s:%d:%s", label, data.Parent.LocalID.AsU32(), data.Kind.Descr(s.session)))
		s.event(c, "return_drop:"+label)
		data.CloseWithContext(c)
		s.event(c, "return_dropped:"+label)
	} else {
		s.event(c, "none:"+label)
	}
}
func (s *backtraceContextState) report(c rf.Context, label string, call func()) {
	defer func() {
		if p := recover(); p != nil {
			s.event(c, fmt.Sprintf("caught:%s:true:static_str:%x", label, fmt.Sprint(p)))
		}
	}()
	call()
	s.event(c, "caught:"+label+":false::")
}
func (s *backtraceContextState) setup(c rf.Context) {
	cfg := sp.SpanEncodingConfig{DebugAssertions: s.q.Debug, OverflowChecks: s.q.Overflow}
	s.session = sp.NewSessionGlobalsWithContext(sp.Edition2024, nil, sp.None[sp.SourceMapInputs](), c, cfg)
	s.features = []sp.Shared[[]sp.Symbol]{sp.NewShared([]sp.Symbol{sp.InternSymbol(s.session, "backtrace_a")}, nil), sp.NewShared([]sp.Symbol{sp.InternSymbol(s.session, "backtrace_b")}, nil)}
	parent := func(n uint32) sp.Option[sp.LocalDefId] {
		return sp.Some(sp.LocalDefId{LocalDefIndex: sp.DefIndexFromU32(n)})
	}
	a := sp.FreshLocalExpnId(s.session, sp.NewExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(s.session, "macro_a")}, sp.RootExpnId(), sp.NewSpan(s.session, 10, 11, sp.RootSyntaxContext(), parent(11)), sp.DummySpan(), sp.Some(s.features[0].CloneWithContext(c)), sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId](), false, false, false, false), backtraceContextHcx{}).ToExpnId()
	ctxtA := sp.RootSyntaxContext().ApplyMarkWithContext(s.session, a, sp.Opaque, c)
	lo, hi := sp.BytePos(20), sp.BytePos(21)
	if strings.Contains(s.q.Case, "recursive") {
		lo, hi = 0, 0
	}
	b := sp.FreshLocalExpnId(s.session, sp.NewExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(s.session, "macro_b")}, a, sp.NewSpan(s.session, lo, hi, ctxtA, parent(22)), sp.DummySpan(), sp.Some(s.features[1].CloneWithContext(c)), sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId](), false, false, false, false), backtraceContextHcx{}).ToExpnId()
	ctxtB := sp.RootSyntaxContext().ApplyMarkWithContext(s.session, b, sp.Opaque, c)
	span := sp.NewSpan(s.session, 100, 101, ctxtB, parent(33))
	s.iter = span.MacroBacktraceWithContext(s.session, c)
	s.event(c, "construct")
}
func backtraceContextLocation(text string) rf.SourceLocation {
	p := strings.LastIndexByte(text, ':')
	column, err := strconv.ParseUint(text[p+1:], 10, 32)
	if err != nil {
		panic(err)
	}
	text = text[:p]
	p = strings.LastIndexByte(text, ':')
	line, err := strconv.ParseUint(text[p+1:], 10, 32)
	if err != nil {
		panic(err)
	}
	return rf.SourceLocation{File: text[:p], Line: uint32(line), Column: uint32(column)}
}
func backtraceContextExecute(q backtraceContextRequest, m backtraceContextManifest) {
	s := &backtraceContextState{q: q}
	old := sp.SwapSpanTrackHook(sp.NewSpanTrackHook(s.track))
	defer sp.SwapSpanTrackHook(old)
	construction := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: 1}}, &rf.PanicOperations{Hook: func(rf.SourcePanicInfo, rf.Context) { fmt.Println("stale_hook") }})
	s.setup(construction)
	if strings.Contains(q.Case, "second") {
		s.next(construction, "warmup")
	}
	s.tracks = 0
	if q.Case != "ok" {
		s.fail = 1
		if strings.HasSuffix(q.Case, "2") {
			s.fail = 2
		} else if strings.HasSuffix(q.Case, "3") {
			s.fail = 3
		}
	}
	runtime := 2
	if q.Mode == "normal" {
		runtime = 1
	}
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rf.SourceLocation{}}
	for name, location := range m.Locations {
		ops.Locations[name] = backtraceContextLocation(location)
	}
	processing := q.Mode == "processing"
	ops.Hook = func(info rf.SourcePanicInfo, c rf.Context) {
		location := ""
		if info.Location != nil {
			location = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		s.event(c, fmt.Sprintf("hook:%s:%x:%t:%t:%s", info.Kind, fmt.Sprint(info.Payload), info.CanUnwind, info.ForceNoBacktrace, location))
		if processing {
			s.report(c, "target", func() { s.next(c, "target") })
			s.event(c, "hook_return")
		}
	}
	c := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: runtime}}, ops)
	if q.Mode == "nil" {
		c = rf.Context{}
	}
	s.event(c, "invoke")
	switch q.Mode {
	case "normal", "currentB", "nil":
		s.report(c, "target", func() { s.next(c, "target") })
	case "processing":
		s.report(c, "ambient", func() { backtraceContextRaise(c, "outer") })
	default:
		s.report(c, "ambient", func() {
			defer backtraceContextClose(c, func(u rf.Context) {
				s.event(u, "ambient_drop")
				if q.Mode == "inherited_caught" {
					s.report(u, "target", func() { s.next(u, "target") })
				} else {
					s.next(u, "target")
				}
			})
			backtraceContextRaise(c, "outer")
		})
	}
	processing = false
	s.fail = 0
	s.event(c, "after_target")
	for n := 0; n < 4; n++ {
		s.report(c, "drain", func() { s.next(c, fmt.Sprintf("resume%d", n)) })
	}
	s.iter = nil
	s.event(c, "iterator_dropped")
	s.session.CloseWithContext(c)
	s.event(c, "session_dropped")
	for _, features := range s.features {
		features.CloseWithContext(c)
	}
}

func backtraceContextRoot() string {
	root, err := filepath.Abs("../../../../../")
	if err != nil {
		panic(err)
	}
	return root
}
func backtraceContextMetadata(t *testing.T) backtraceContextManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(backtraceContextRoot(), "tools/port-rust-span-lib/macro-backtrace-context-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m backtraceContextManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func TestOriginalMacroBacktraceContextChild(t *testing.T) {
	if os.Getenv("FERRUGO_BACKTRACE_CONTEXT_CHILD") != "1" {
		return
	}
	var q backtraceContextRequest
	if err := json.Unmarshal([]byte(os.Getenv("FERRUGO_BACKTRACE_CONTEXT_REQUEST")), &q); err != nil {
		t.Fatal(err)
	}
	backtraceContextExecute(q, backtraceContextMetadata(t))
	os.Exit(0)
}
func TestOriginalMacroBacktraceContext(t *testing.T) {
	m := backtraceContextMetadata(t)
	root := backtraceContextRoot()
	for _, bindings := range []map[string]string{m.Files, m.Tools} {
		for name, want := range bindings {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
				t.Fatalf("backtrace selected source differs: %s", name)
			}
		}
	}
	blob, err := os.ReadFile(filepath.Join(root, "tools/port-rust-span-lib/macro-backtrace-context.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(blob)) != m.FixtureSHA {
		t.Fatal("backtrace fixture checksum differs")
	}
	z, err := gzip.NewReader(strings.NewReader(string(blob)))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	digest := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(z, digest))
	scanner.Buffer(make([]byte, 65536), 1<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count, aborts := 0, 0
	for scanner.Scan() {
		var row struct {
			Request backtraceContextRequest
			Want    struct {
				Abort        bool
				NativeStatus int `json:"native_status"`
				Output       string
			}
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if row.Want.Abort && row.Want.NativeStatus != -6 || !row.Want.Abort && row.Want.NativeStatus != 0 {
			t.Fatal("backtrace native termination class differs")
		}
		request, _ := json.Marshal(row.Request)
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(c, binary, "-test.run=^TestOriginalMacroBacktraceContextChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_BACKTRACE_CONTEXT_CHILD=1", "FERRUGO_BACKTRACE_CONTEXT_REQUEST="+string(request))
		out, runErr := cmd.Output()
		cancel()
		aborted := false
		if runErr != nil {
			exit, ok := runErr.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("backtrace %+v host failure: %v\n%s", row.Request, runErr, out)
			}
			aborted = true
		}
		if aborted != row.Want.Abort || string(out) != row.Want.Output {
			t.Fatalf("backtrace %+v abort %t/%t\nGo:\n%s\nRust:\n%s", row.Request, aborted, row.Want.Abort, out, row.Want.Output)
		}
		if aborted {
			aborts++
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != m.Records || aborts != m.Aborts || hex.EncodeToString(digest.Sum(nil)) != m.RawSHA {
		t.Fatal("backtrace retained coverage differs")
	}
	t.Logf("%d original backtrace runtime records, %d native SIGABRT prefixes match", count, aborts)
}
