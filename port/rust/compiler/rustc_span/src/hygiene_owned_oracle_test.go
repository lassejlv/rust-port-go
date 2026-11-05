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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type hygieneOwnedRequest struct {
	Operation, Case, Mode string
	Debug, Overflow       bool
}
type hygieneOwnedMetadata struct {
	Cases          int
	RawSHA256      string            `json:"raw_sha256"`
	FixtureSHA256  string            `json:"fixture_sha256"`
	ArtifactKind   string            `json:"artifact_kind"`
	SourceRevision string            `json:"source_revision"`
	ManifestSHA    string            `json:"-"`
	Locations      map[string]string `json:"source_locations"`
	Files          map[string]string
}
type hygieneOwnedRuntime struct{ id int }

func hygieneOwnedRuntimeID(c rf.Context) int {
	if c.Runtime == nil {
		return 0
	}
	r, _ := c.Runtime.Value.(*hygieneOwnedRuntime)
	if r == nil {
		return 0
	}
	return r.id
}

type hygieneOwnedCore struct {
	session                      *sp.SessionGlobals
	external                     sp.Shared[[]sp.Symbol]
	calls                        int
	reenter, failDrop, failField bool
	failAt                       int
	profile                      sp.HygieneConfig
}

func (s *hygieneOwnedCore) refs() uint { return s.external.StrongCount() }
func (s *hygieneOwnedCore) local(c rf.Context, name string) {
	fmt.Printf("local:%s:%t:%d:%d\n", name, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
}
func hygieneOwnedClose(c rf.Context, close func(rf.Context)) {
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
func hygieneOwnedRaise(c rf.Context, text string) {
	c.PanicAt(rf.SourcePanicInfo{Payload: text, Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation(text)})
}
func (s *hygieneOwnedCore) call(c rf.Context, stage, detail string) {
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local(c, "call") })
	s.calls++
	fmt.Printf("call:%s:%d:%x:%t:%d:%d\n", stage, s.calls, detail, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if s.failAt == s.calls {
		hygieneOwnedRaise(c, "callback")
	}
}
func (s *hygieneOwnedCore) drop(c rf.Context) {
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local(c, "drop") })
	fmt.Printf("drop:%d:%t:%d:%d\n", s.calls, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if s.reenter {
		id := sp.FreshEmptyLocalExpnIdWithContext(s.session, c)
		fmt.Printf("reentered:%d:%t\n", id.AsU32(), c.Panicking())
	}
	if s.failDrop {
		hygieneOwnedRaise(c, "drop")
	}
}
func (s *hygieneOwnedCore) field(c rf.Context) {
	fmt.Printf("field:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if s.failField {
		hygieneOwnedRaise(c, "field")
	}
}

type hygieneOwnedEnv struct {
	core     *hygieneOwnedCore
	features sp.Shared[[]sp.Symbol]
	metadata []byte
}

func (e *hygieneOwnedEnv) call(c rf.Context, stage, detail string) {
	if len(e.metadata) != 3 {
		panic("invalid owned environment")
	}
	e.core.call(c, stage, detail)
}
func (e *hygieneOwnedEnv) destroy(c rf.Context) {
	defer hygieneOwnedClose(c, func(c rf.Context) { e.features.CloseWithContext(c); e.metadata = nil; e.core.field(c) })
	e.core.drop(c)
}

type hygieneOwnedResult struct {
	core     *hygieneOwnedCore
	features sp.Shared[[]sp.Symbol]
}

func (s *hygieneOwnedCore) result(c rf.Context) *rv.Owned[hygieneOwnedResult] {
	value := hygieneOwnedResult{s, s.external.CloneWithContext(c)}
	fmt.Printf("prepared:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	return rv.NewOwned(value, rv.ValueOps[hygieneOwnedResult]{Drop: func(value *hygieneOwnedResult, c rf.Context) {
		defer hygieneOwnedClose(c, value.features.CloseWithContext)
		fmt.Printf("result_drop:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), value.core.refs())
		id := sp.FreshEmptyLocalExpnIdWithContext(value.core.session, c)
		fmt.Printf("result_reentered:%d:%t\n", id.AsU32(), c.Panicking())
	}})
}
func hygieneOwnedOps() ds.StableHashCtxtOps[hygieneOwnedEnv] {
	return ds.StableHashCtxtOps[hygieneOwnedEnv]{
		StableHashSpan: func(r rv.Ref[hygieneOwnedEnv], span ds.RawSpan, h *ds.StableHasher) {
			r.Value.call(r.Context, "span", strconv.FormatUint(span.Value, 10))
			h.WriteU64(span.Value)
		},
		DefPathHash: func(r rv.Ref[hygieneOwnedEnv], id ds.RawDefId) ds.Fingerprint {
			r.Value.call(r.Context, "path", fmt.Sprintf("%d/%d", id.Crate, id.Index))
			return (hygieneCoreHcx{}).DefPathHash(id)
		},
		StableHashControls: func(r rv.Ref[hygieneOwnedEnv]) ds.StableHashControls {
			r.Value.call(r.Context, "controls", "")
			return ds.StableHashControls{HashSpans: true}
		},
		AssertDefaultStableHashControls: func(r rv.Ref[hygieneOwnedEnv], message string) { r.Value.call(r.Context, "assert", message) },
	}
}
func (s *hygieneOwnedCore) newEnv(c rf.Context) *rv.Owned[hygieneOwnedEnv] {
	return rv.NewOwned(hygieneOwnedEnv{s, s.external.CloneWithContext(c), []byte{1, 2, 3}}, rv.ValueOps[hygieneOwnedEnv]{Drop: func(e *hygieneOwnedEnv, c rf.Context) { e.destroy(c) }}).MoveWithContext(c)
}
func (s *hygieneOwnedCore) data(c rf.Context) sp.ExpnData {
	return sp.ExpnDataAllowUnstable(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(s.session, "owned")}, sp.NewSpan(s.session, 11, 19, sp.RootSyntaxContext(), sp.None[sp.LocalDefId]()), sp.Edition2024, s.external.CloneWithContext(c), sp.None[sp.DefId](), sp.None[sp.ModId]())
}
func hygieneOwnedBuild(operation string, s *hygieneOwnedCore, construction rf.Context) func(rf.Context) {
	switch operation {
	case "fresh":
		data := s.data(construction)
		env := s.newEnv(construction)
		return func(c rf.Context) {
			id := sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
			fmt.Printf("returned:fresh:%d\n", id.AsU32())
		}
	case "collision":
		data := s.data(construction)
		_ = sp.FreshLocalExpnId(s.session, data.CloneWithContext(construction), hygieneCoreHcx{})
		env := s.newEnv(construction)
		return func(c rf.Context) {
			id := sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
			fmt.Printf("returned:collision:%d\n", id.AsU32())
		}
	case "set":
		id := sp.FreshEmptyLocalExpnId(s.session)
		data := s.data(construction)
		env := s.newEnv(construction)
		return func(c rf.Context) {
			sp.SetExpnDataOwned(s.session, id, data, env, hygieneOwnedOps(), c)
			fmt.Printf("returned:set:%d\n", id.AsU32())
		}
	case "mark":
		env := s.newEnv(construction)
		unstable := sp.Some(s.external.CloneWithContext(construction))
		span := sp.NewSpan(s.session, 11, 19, sp.RootSyntaxContext(), sp.None[sp.LocalDefId]())
		return func(c rf.Context) {
			marked := sp.MarkSpanWithReasonOwned(span, s.session, unstable, sp.DesugaringKind{Tag: sp.DesugarAsync}, sp.Edition2024, env, hygieneOwnedOps(), c)
			fmt.Printf("returned:mark:%d:%d:%d\n", marked.Lo(s.session), marked.Hi(s.session), marked.Ctxt().AsU32())
		}
	case "reset":
		id := sp.FreshLocalExpnId(s.session, s.data(construction), hygieneCoreHcx{})
		data := s.data(construction)
		env := s.newEnv(construction)
		return func(c rf.Context) {
			sp.SetExpnDataOwned(s.session, id, data, env, hygieneOwnedOps(), c)
			fmt.Println("returned:reset")
		}
	case "bad_parent", "bad_parent_set":
		krate, index := sp.CrateNumFromU32(3), sp.ExpnIndexFromU32(17)
		sp.RegisterExpnId(s.session, krate, index, sp.DefaultExpnData(sp.ExpnKind{Tag: sp.ExpnRoot}, sp.DummySpan(), sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId]()), hygieneCodecHash(9, 9))
		data := s.data(construction)
		data.Parent = sp.ExpnId{Krate: krate, LocalID: index}
		env := s.newEnv(construction)
		id := sp.RootLocalExpnId()
		if operation == "bad_parent_set" {
			id = sp.FreshEmptyLocalExpnId(s.session)
		}
		return func(c rf.Context) {
			if operation == "bad_parent_set" {
				sp.SetExpnDataOwned(s.session, id, data, env, hygieneOwnedOps(), c)
				fmt.Println("returned:bad_parent_set")
			} else {
				_ = sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
				fmt.Println("returned:bad_parent")
			}
		}
	case "already_set":
		first := s.data(construction)
		_ = sp.FreshLocalExpnId(s.session, first.CloneWithContext(construction), hygieneCoreHcx{})
		id := sp.FreshLocalExpnId(s.session, first, hygieneCoreHcx{})
		data := id.ExpnDataWithContext(s.session, construction)
		env := s.newEnv(construction)
		return func(c rf.Context) {
			_ = sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
			fmt.Println("returned:already_set")
		}
	case "update", "borrowed_update":
		ctxt := sp.RootSyntaxContext()
		for i := 0; i < 3; i++ {
			id := sp.FreshLocalExpnId(s.session, sp.DefaultExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(s.session, "resolve")}, sp.DummySpan(), sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId]()), hygieneCoreHcx{})
			ctxt = ctxt.ApplyMark(s.session, id.ToExpnId(), sp.Opaque)
		}
		env := s.newEnv(construction)
		return func(c rf.Context) {
			resolver := sp.HygieneNameResolver[hygieneOwnedEnv]{Env: env.Borrow(), Context: c, Call: func(e *hygieneOwnedEnv, ctxt sp.SyntaxContext, c rf.Context) sp.Symbol {
				e.call(c, "resolve", strconv.FormatUint(uint64(ctxt.AsU32()), 10))
				_ = ctxt.EditionWithContext(s.session, c)
				return sp.InternSymbolWithContext(s.session, fmt.Sprintf("name%d", ctxt.AsU32()), c)
			}, Drop: func(_ *hygieneOwnedEnv, c rf.Context) { env.DropWithContext(c) }}
			if operation == "borrowed_update" {
				defer hygieneOwnedClose(c, env.DropWithContext)
				sp.UpdateDollarCrateNamesOwned(s.session, resolver.Borrow())
				fmt.Println("returned:borrowed_update")
			} else {
				sp.UpdateDollarCrateNamesOwned(s.session, resolver)
				fmt.Println("returned:update")
			}
		}
	case "comparator":
		env := s.newEnv(construction)
		a, b := hygieneCodecHash(1, 9), hygieneCodecHash(2, 0)
		return func(c rf.Context) {
			cmp := sp.ExpnHashStableComparatorOwned(env, c)
			order := "Equal"
			switch n := cmp(&a, &b); {
			case n < 0:
				order = "Less"
			case n > 0:
				order = "Greater"
			}
			fmt.Printf("returned:comparator:%s\n", order)
		}
	case "remap_syntax", "remap_syntax_before", "remap_syntax_repeat", "remap_local", "remap_local_before", "remap_local_repeat":
		env := s.newEnv(construction)
		before, repeat := strings.HasSuffix(operation, "_before"), strings.HasSuffix(operation, "_repeat")
		return func(c rf.Context) {
			if strings.HasPrefix(operation, "remap_syntax") {
				sp.WithSyntaxContextRemapperOwned(s.session, hashmap.NewOnce(env, func(e *hygieneOwnedEnv, r sp.SyntaxContextRemapper, c rf.Context) struct{} {
					e.call(c, "remap", "syntax")
					if !before {
						r.CommitEndOfDeterminism(c)
					}
					if repeat {
						r.CommitEndOfDeterminism(c)
					}
					fmt.Printf("boundary:%d\n", r.FirstNonDetIndex(c))
					result := sp.ComputeHygieneRemappingResults[sp.SyntaxContext, uint32](r, sp.SyntaxContextFromU32WithContext, hygieneCoreHcx{}, c)
					result.CloseWithContext(c)
					return struct{}{}
				}), c)
				fmt.Println("returned:remap_syntax")
			} else {
				sp.WithLocalExpnIdRemapperOwned(s.session, hashmap.NewOnce(env, func(e *hygieneOwnedEnv, r sp.LocalExpnIdRemapper, c rf.Context) struct{} {
					e.call(c, "remap", "local")
					if !before {
						r.CommitEndOfDeterminism(c)
					}
					if repeat {
						r.CommitEndOfDeterminism(c)
					}
					fmt.Printf("boundary:%d\n", r.FirstNonDetIndex(c))
					result := sp.ComputeHygieneRemappingResults[sp.LocalExpnId, uint32](r, sp.LocalExpnIdFromU32WithContext, hygieneCoreHcx{}, c)
					result.CloseWithContext(c)
					return struct{}{}
				}), c)
				fmt.Println("returned:remap_local")
			}
		}
	case "remap_syntax_value", "remap_local_value":
		env := s.newEnv(construction)
		return func(c rf.Context) {
			var value *rv.Owned[hygieneOwnedResult]
			if operation == "remap_syntax_value" {
				value = sp.WithSyntaxContextRemapperOwned(s.session, hashmap.NewOnce(env, func(e *hygieneOwnedEnv, _ sp.SyntaxContextRemapper, c rf.Context) *rv.Owned[hygieneOwnedResult] {
					e.call(c, "remap", "syntax")
					return s.result(c)
				}), c)
				fmt.Println("returned:remap_syntax_value")
			} else {
				value = sp.WithLocalExpnIdRemapperOwned(s.session, hashmap.NewOnce(env, func(e *hygieneOwnedEnv, _ sp.LocalExpnIdRemapper, c rf.Context) *rv.Owned[hygieneOwnedResult] {
					e.call(c, "remap", "local")
					return s.result(c)
				}), c)
				fmt.Println("returned:remap_local_value")
			}
			value.DropWithContext(c)
		}
	}
	return hygieneFailureBuild(operation, s, construction)
}
func hygieneOwnedLocation(text string) rf.SourceLocation {
	p := strings.LastIndex(text, ":")
	column, e := strconv.ParseUint(text[p+1:], 10, 32)
	if e != nil {
		panic(e)
	}
	text = text[:p]
	p = strings.LastIndex(text, ":")
	line, e := strconv.ParseUint(text[p+1:], 10, 32)
	if e != nil {
		panic(e)
	}
	return rf.SourceLocation{File: text[:p], Line: uint32(line), Column: uint32(column)}
}
func hygieneOwnedReport(c rf.Context, s *hygieneOwnedCore, call func()) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Printf("caught:true:%x:%t:%d:%d\n", fmt.Sprint(p), c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		}
	}()
	call()
	fmt.Printf("caught:false::%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
}
func hygieneOwnedExecute(q hygieneOwnedRequest, m hygieneOwnedMetadata) {
	sp.SetDynThreadSafeMode(false)
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs](), sp.SpanEncodingConfig{DebugAssertions: q.Debug, OverflowChecks: q.Overflow})
	defer session.Close()
	external := sp.NewShared([]sp.Symbol{sp.InternSymbol(session, "feature")}, nil)
	defer external.Close()
	s := &hygieneOwnedCore{session: session, external: external, reenter: strings.HasPrefix(q.Case, "reenter"), failDrop: q.Case == "drop" || strings.HasSuffix(q.Case, "_drop") || q.Case == "drop_field", failField: q.Case == "field" || strings.HasSuffix(q.Case, "_field")}
	s.profile = sp.HygieneConfig{DebugAssertions: q.Debug, OverflowChecks: q.Overflow}
	if strings.HasPrefix(q.Case, "call") {
		number := strings.Split(strings.TrimPrefix(q.Case, "call"), "_")[0]
		s.failAt, _ = strconv.Atoi(number)
	}
	construction := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: &hygieneOwnedRuntime{1}}}, &rf.PanicOperations{Hook: func(rf.SourcePanicInfo, rf.Context) { fmt.Println("stale_hook") }})
	fmt.Printf("construct:1:%d\n", s.refs())
	action := hygieneOwnedBuild(q.Operation, s, construction)
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rf.SourceLocation{}}
	for site, text := range m.Locations {
		ops.Locations[site] = hygieneOwnedLocation(text)
	}
	ops.Hook = func(info rf.SourcePanicInfo, c rf.Context) {
		location := ""
		if info.Location != nil {
			location = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		fmt.Printf("hook:%s:%x:%t:%t:%t:%d:%d:%s\n", info.Kind, fmt.Sprint(info.Payload), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hygieneOwnedRuntimeID(c), s.refs(), location)
		if q.Mode == "processing" {
			action(c)
			fmt.Printf("hook_return:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		}
	}
	c := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: &hygieneOwnedRuntime{2}}}, ops)
	if q.Mode == "unobserved" {
		c = rf.Context{}
	}
	fmt.Printf("invoke:%d:%d\n", hygieneOwnedRuntimeID(c), s.refs())
	if q.Mode == "normal" || q.Mode == "unobserved" {
		hygieneOwnedReport(c, s, func() { action(c) })
	} else if q.Mode == "processing" {
		hygieneOwnedReport(c, s, func() {
			c.PanicAt(rf.SourcePanicInfo{Payload: "outer", Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation("outer_processing")})
		})
	} else {
		hygieneOwnedReport(c, s, func() {
			defer hygieneOwnedClose(c, func(u rf.Context) {
				fmt.Printf("outer:%t:%d:%d\n", u.Panicking(), hygieneOwnedRuntimeID(u), s.refs())
				if q.Mode == "inherited_caught" {
					hygieneOwnedReport(u, s, func() { action(u) })
				} else {
					action(u)
				}
			})
			c.PanicAt(rf.SourcePanicInfo{Payload: "outer", Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation("outer_unwind")})
		})
	}
	fmt.Printf("complete:%d:%d\n--final--\n%s\n", hygieneOwnedRuntimeID(c), s.refs(), sp.DebugHygieneDataWithContext(session, true, c))
}
func hygieneOwnedRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}
func TestOriginalHygieneOwnedChild(t *testing.T) {
	if os.Getenv("FERRUGO_HYGIENE_OWNED_CHILD") != "1" {
		return
	}
	var q hygieneOwnedRequest
	if err := json.Unmarshal([]byte(os.Getenv("FERRUGO_HYGIENE_OWNED_REQUEST")), &q); err != nil {
		t.Fatal(err)
	}
	selection, err := hygieneOwnedEvidencePaths(hygieneOwnedRoot(), os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	expectedSHA, present := os.LookupEnv(hygieneOwnedManifestSHAEnv)
	if !present || expectedSHA == "" {
		t.Fatal("owned Hygiene child has no captured parent producer manifest")
	}
	m, err := hygieneOwnedProfileMetadata(selection, expectedSHA)
	if err != nil {
		t.Fatal(err)
	}
	hygieneOwnedExecute(q, m)
	os.Exit(0)
}
func TestOriginalHygieneOwned(t *testing.T) {
	root := hygieneOwnedRoot()
	selection, err := hygieneOwnedEvidencePaths(root, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	m, err := hygieneOwnedProfileMetadata(selection, "")
	if err != nil {
		t.Fatal(err)
	}
	selection.ManifestSHA = m.ManifestSHA
	for name, want := range m.Files {
		clean := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			t.Fatalf("invalid owned Hygiene source binding path: %s", name)
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("owned hygiene source binding changed:%s", name)
		}
	}
	compressed, err := os.ReadFile(selection.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	// Anchor every child request to both exact producer checksums before any
	// child is spawned; children validate the captured immutable manifest SHA.
	if err := hygieneOwnedVerifyFixture(compressed, m); err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	digest := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(z, digest))
	scan.Buffer(make([]byte, 65536), 8<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for scan.Scan() {
		var row struct {
			Request hygieneOwnedRequest
			Want    struct {
				Abort  bool
				Output string
			}
		}
		if err = json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		request, _ := json.Marshal(row.Request)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestOriginalHygieneOwnedChild$")
		command.Env = append(hygieneOwnedChildEnvironment(os.Environ(), selection), "FERRUGO_HYGIENE_OWNED_CHILD=1", "FERRUGO_HYGIENE_OWNED_REQUEST="+string(request))
		output, err := command.Output()
		cancel()
		abort := false
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%+v hostfailure:%v\n%s", row.Request, err, output)
			}
			abort = true
		}
		if abort != row.Want.Abort || string(output) != row.Want.Output {
			t.Fatalf("%+v abort%t/%t\nactual:\n%s\nexpected:\n%s", row.Request, abort, row.Want.Abort, output, row.Want.Output)
		}
		count++
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != m.Cases || hex.EncodeToString(digest.Sum(nil)) != m.RawSHA256 {
		t.Fatal("owned hygiene proof count/checksum differs")
	}
	t.Logf("%d whole-source owned lifecycles", count)
}
