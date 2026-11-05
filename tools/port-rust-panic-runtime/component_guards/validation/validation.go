// SPDX-License-Identifier: MIT OR Apache-2.0
// Authored Rust caller/Drop-glue witnesses for independent component replay.
// No source algorithm is implemented here.
package validation

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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	lit "github.com/lassejlv/rust-port-go/port/dependencies/rustc-literal-escaper/v0_0_8/src"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	search "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src/binary_search_util"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	lexer "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_lexer/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type request struct{ Component, Case, Mode string }
type record struct {
	Request request
	Want    struct {
		Abort  bool
		Output string
	}
}
type metadata struct {
	Cases     int
	RawSHA256 string            `json:"raw_sha256"`
	Locations map[string]string `json:"source_locations"`
	Files     map[string]string
}
type config struct {
	call, drop, field string
	acc               bool
}

func choose(component, caseName string) config {
	call, drop := "call", "env"
	switch component {
	case "fingerprint":
		call = "convert_a"
		drop = "b"
	case "binary":
		call = "extract"
	case "interval":
		call = "start"
		drop = "range1"
	}
	switch caseName {
	case "ok":
		return config{}
	case "call":
		return config{call: call}
	case "call_b":
		return config{call: "convert_b"}
	case "drop":
		return config{drop: drop}
	case "call_drop":
		return config{call: call, drop: drop}
	case "drop_field":
		return config{drop: drop, field: drop}
	case "cmp":
		return config{call: "lt"}
	case "key":
		return config{drop: "key"}
	case "cmp_key":
		return config{call: "lt", drop: "key"}
	case "clone", "start", "end":
		return config{call: caseName}
	case "original_drop":
		return config{drop: "range0"}
	case "acc":
		return config{call: call, acc: true}
	}
	panic("invalid component request")
}

type runtimeValue struct {
	id            int
	noncomparable []byte
}

func runtimeID(c rf.Context) int {
	if c.Runtime == nil {
		return 0
	}
	value, _ := c.Runtime.Value.(*runtimeValue)
	if value == nil {
		return 0
	}
	return value.id
}
func local(name string, c rf.Context) {
	fmt.Printf("local:%s:%t:%d\n", name, c.Panicking(), runtimeID(c))
}
func closeSource(c rf.Context, close func(rf.Context)) {
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
func raise(c rf.Context, text string) {
	c.PanicAt(rf.SourcePanicInfo{Payload: text, Kind: rf.PanicStaticStr, Location: c.SourcePanicLocation(text), CanUnwind: true})
}

type core struct {
	cfg   config
	calls int
}

func (s *core) call(c rf.Context, stage string) {
	defer closeSource(c, func(c rf.Context) { local("call", c) })
	s.calls++
	fmt.Printf("call:%s:%d:%t:%d\n", stage, s.calls, c.Panicking(), runtimeID(c))
	if s.cfg.call == stage {
		raise(c, "callback")
	}
}
func (s *core) drop(c rf.Context, label string) {
	defer closeSource(c, func(c rf.Context) { local("drop", c) })
	fmt.Printf("drop:%s:%t:%d\n", label, c.Panicking(), runtimeID(c))
	if s.cfg.drop == label {
		raise(c, "drop")
	}
}
func (s *core) field(c rf.Context, label string) {
	fmt.Printf("field:%s:%t:%d\n", label, c.Panicking(), runtimeID(c))
	if s.cfg.field == label {
		raise(c, "field")
	}
}

type env struct {
	core     *core
	label    string
	metadata []byte
}

func newEnv(s *core, label string) *env { return &env{s, label, []byte{1, 2, 3}} }
func (e *env) call(c rf.Context, stage string) {
	if len(e.metadata) != 3 {
		panic("lost environment state")
	}
	e.core.call(c, stage)
}
func (e *env) destroy(c rf.Context) {
	defer closeSource(c, func(c rf.Context) { e.core.field(c, e.label) })
	e.core.drop(c, e.label)
}

type accumulator struct {
	n    int
	core *core
}

func (a *accumulator) drop(c rf.Context) {
	fmt.Printf("acc:%d:%t:%d\n", a.n, c.Panicking(), runtimeID(c))
	if a.core.cfg.acc && c.Panicking() {
		raise(c, "acc")
	}
}

type key struct {
	n    int
	core *core
}

func (k *key) drop(c rf.Context) {
	defer closeSource(c, func(c rf.Context) { k.core.field(c, "key") })
	k.core.drop(c, "key")
}
func bounds(s *core, generation int) index.IntervalRangeBounds[uint] {
	label := "range0"
	if generation != 0 {
		label = "range1"
	}
	return index.IntervalRangeBounds[uint]{CloneWithContextFn: func(c rf.Context) index.IntervalRangeBounds[uint] { s.call(c, "clone"); return bounds(s, generation+1) }, StartBoundWithContextFn: func(c rf.Context) index.IntervalBound[uint] {
		s.call(c, "start")
		return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 1}
	}, EndBoundWithContextFn: func(c rf.Context) index.IntervalBound[uint] {
		s.call(c, "end")
		return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 3}
	}, CloseWithContext: func(c rf.Context) { defer closeSource(c, func(c rf.Context) { s.field(c, label) }); s.drop(c, label) }}
}
func build(component string, s *core, construction rf.Context) func(rf.Context) {
	switch component {
	case "predicate":
		cursor := lexer.NewCursorWithContext("ab!c", lexer.FrontmatterNo, construction)
		e := newEnv(s, "env")
		return func(c rf.Context) {
			lexer.EatWhileOwned(cursor, lexer.Predicate[env]{Env: e, Context: c, Call: func(e *env, c rf.Context, r rune) bool { e.call(c, "call"); return r != '!' }, Drop: func(e *env, c rf.Context) { e.destroy(c) }})
			fmt.Printf("returned:cursor:%x\n", cursor.AsStr())
		}
	case "foreach":
		it := lexer.NewTokenizerWithContext("a ! b", lexer.FrontmatterNo, construction)
		e := newEnv(s, "env")
		return func(c rf.Context) {
			lexer.ForEachTokenOwned(it, lexer.TokenConsumer[env]{Env: e, Context: c, Call: func(e *env, c rf.Context, _ lexer.Token) { e.call(c, "call") }, Drop: func(e *env, c rf.Context) { e.destroy(c) }})
			fmt.Println("returned:foreach")
		}
	case "fold":
		it := lexer.NewTokenizerWithContext("a ! b", lexer.FrontmatterNo, construction)
		e := newEnv(s, "env")
		return func(c rf.Context) {
			result := lexer.FoldTokensOwned(it, accumulator{0, s}, lexer.TokenFolder[env, accumulator]{Env: e, Context: c, Call: func(e *env, c rf.Context, a accumulator, _ lexer.Token) accumulator {
				owned := true
				defer closeSource(c, func(c rf.Context) {
					if owned {
						a.drop(c)
					}
				})
				e.call(c, "call")
				next := a.n + 1
				owned = false
				a.drop(c)
				return accumulator{next, s}
			}, Drop: func(e *env, c rf.Context) { e.destroy(c) }}, func(a *accumulator, c rf.Context) { a.drop(c) })
			fmt.Printf("returned:fold:%d\n", result.n)
			result.drop(c)
		}
	case "literal":
		e := newEnv(s, "env")
		return func(c rf.Context) {
			lit.UnescapeStrOwned("a\\nb", lit.UnitCallback[env, rune]{Env: e, Context: c, Call: func(e *env, c rf.Context, _ lit.Range, _ lit.Result[rune]) { e.call(c, "call") }, Drop: func(e *env, c rf.Context) { e.destroy(c) }})
			fmt.Println("returned:literal")
		}
	case "fingerprint":
		a, b := newEnv(s, "a"), newEnv(s, "b")
		component := func(e *env) ds.OwnedFingerprintComponent[env] {
			return ds.OwnedFingerprintComponent[env]{Value: *e, AsU64WithContext: func(e *env, c rf.Context) uint64 {
				stage, value := "convert_a", uint64(10)
				if e.label == "b" {
					stage, value = "convert_b", 20
				}
				e.call(c, stage)
				return value
			}, CloseWithContext: func(e *env, c rf.Context) { e.destroy(c) }}
		}
		return func(c rf.Context) {
			value := ds.NewFingerprintOwnedWithContext(c, component(a), component(b))
			a, b := value.Split()
			fmt.Printf("returned:fingerprint:%d:%d\n", a.AsU64(), b.AsU64())
		}
	case "binary":
		e := newEnv(s, "env")
		return func(c rf.Context) {
			data := []int{1, 2, 2, 3}
			query := key{2, s}
			extract := search.ExtractorFuncs[int, key]{Call: func(n *int, c rf.Context) search.ExtractedKey[key] {
				e.call(c, "extract")
				return search.ExtractedKey[key]{Value: key{*n, s}, Drop: func(k *key, c rf.Context) { k.drop(c) }}
			}, Drop: func(c rf.Context) { e.destroy(c) }}
			ops := search.BorrowedKeyOperators[key]{Less: func(a, b *key, c rf.Context) bool { s.call(c, "lt"); return a.n < b.n }, NotEqual: func(a, b *key, c rf.Context) bool { s.call(c, "ne"); return a.n != b.n }, LessOrEqual: func(a, b *key, c rf.Context) bool { s.call(c, "le"); return a.n <= b.n }}
			result := search.BinarySearchSliceOwnedWithContext(data, extract, &query, ops, c)
			fmt.Printf("returned:binary:%d\n", len(result))
		}
	case "interval":
		r := bounds(s, 0)
		return func(c rf.Context) {
			set := index.NewIntervalSet(index.UsizeIdx(), 16, index.UsizeIntervalStep())
			result := set.InsertRangeWithContext(c, r)
			values := []string{}
			it := set.Iter()
			for {
				n, ok := it.Next()
				if !ok {
					break
				}
				values = append(values, strconv.FormatUint(uint64(n), 10))
			}
			fmt.Printf("returned:interval:%t:[%s]\n", result, strings.Join(values, ", "))
		}
	case "stable_hash_context":
		return func(c rf.Context) {
			ops := ds.StableHashCtxtOps[core]{StableHashSpan: func(r rv.Ref[core], _ ds.RawSpan, _ *ds.StableHasher) {
				r.Value.call(r.Context, "call")
			}, DefPathHash: func(r rv.Ref[core], _ ds.RawDefId) ds.Fingerprint {
				r.Value.call(r.Context, "call")
				return ds.ZeroFingerprint()
			}, StableHashControls: func(r rv.Ref[core]) ds.StableHashControls {
				r.Value.call(r.Context, "call")
				return ds.StableHashControls{HashSpans: true}
			}, AssertDefaultStableHashControls: func(r rv.Ref[core], _ string) { r.Value.call(r.Context, "call") }}
			ds.NewStableHashCtxtRef(s, ops, c).AssertDefaultStableHashControls("probe")
			fmt.Println("returned:stable_hash_context")
		}
	case "internal":
		return func(c rf.Context) {
			defer func() {
				p := recover()
				drop := c
				drop.Unwinding = drop.Unwinding || p != nil
				local("internal", drop)
				if p != nil {
					c.ResumeUnwind(p)
				}
			}()
			lexer.NewCursor("λ", lexer.FrontmatterNo).EatUntilWithContext(0xbb, c)
			fmt.Println("returned:internal")
		}
	}
	panic("invalid component")
}
func report(c rf.Context, call func()) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Printf("caught:true:%x:%t:%d\n", fmt.Sprint(p), c.Panicking(), runtimeID(c))
		}
	}()
	call()
	fmt.Printf("caught:false::%t:%d\n", c.Panicking(), runtimeID(c))
}
func parseLocation(text string) rf.SourceLocation {
	p := strings.LastIndex(text, ":")
	column, err := strconv.ParseUint(text[p+1:], 10, 32)
	if err != nil {
		panic(err)
	}
	text = text[:p]
	p = strings.LastIndex(text, ":")
	line, err := strconv.ParseUint(text[p+1:], 10, 32)
	if err != nil {
		panic(err)
	}
	return rf.SourceLocation{File: text[:p], Line: uint32(line), Column: uint32(column)}
}
func execute(q request, m metadata) {
	s := &core{cfg: choose(q.Component, q.Case)}
	construction := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: &runtimeValue{1, []byte{1}}}}, &rf.PanicOperations{Hook: func(rf.SourcePanicInfo, rf.Context) { fmt.Println("stale_hook") }})
	fmt.Println("construct:1")
	action := build(q.Component, s, construction)
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rf.SourceLocation{}}
	for site, text := range m.Locations {
		ops.Locations[site] = parseLocation(text)
	}
	ops.Hook = func(info rf.SourcePanicInfo, c rf.Context) {
		location := ""
		if info.Location != nil {
			location = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		fmt.Printf("hook:%s:%x:%t:%t:%t:%d:%s\n", info.Kind, fmt.Sprint(info.Payload), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, runtimeID(c), location)
		if q.Mode == "processing" {
			action(c)
			fmt.Printf("hook_return:%t:%d\n", c.Panicking(), runtimeID(c))
		}
	}
	c := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: &runtimeValue{2, []byte{2}}}}, ops)
	if q.Mode == "unobserved" {
		c = rf.Context{}
	}
	fmt.Printf("invoke:%d\n", runtimeID(c))
	if q.Mode == "normal" || q.Mode == "unobserved" {
		report(c, func() { action(c) })
	} else if q.Mode == "processing" {
		report(c, func() {
			c.PanicAt(rf.SourcePanicInfo{Payload: "outer", Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation("outer_processing")})
		})
	} else {
		report(c, func() {
			defer closeSource(c, func(unwind rf.Context) {
				fmt.Printf("outer:%t:%d\n", unwind.Panicking(), runtimeID(unwind))
				if q.Mode == "inherited_caught" {
					report(unwind, func() { action(unwind) })
				} else {
					action(unwind)
				}
			})
			c.PanicAt(rf.SourcePanicInfo{Payload: "outer", Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation("outer_unwind")})
		})
	}
	fmt.Printf("complete:%d\n", runtimeID(c))
}
func rootPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../.."))
}

// Replay launches a fresh process even for successful scripts, so tracing/hook
// witness state and real aborts never leak into another test's registry.
func Replay(t *testing.T, component string) {
	t.Helper()
	root := rootPath()
	directory := filepath.Join(root, "tools/port-rust-panic-runtime/component_guards")
	data, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m metadata
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FERRUGO_COMPONENT_GUARD_CHILD") == "1" {
		var q request
		if err = json.Unmarshal([]byte(os.Getenv("FERRUGO_COMPONENT_GUARD_REQUEST")), &q); err != nil {
			t.Fatal(err)
		}
		execute(q, m)
		os.Exit(0)
	}
	for name, expected := range m.Files {
		bytes, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(bytes)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("component proof binding changed:%s", name)
		}
	}
	f, err := os.Open(filepath.Join(directory, "cases.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	digest := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(z, digest))
	scan.Buffer(make([]byte, 65536), 1<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	total, count := 0, 0
	for scan.Scan() {
		var row record
		if err = json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		total++
		if row.Request.Component != component && !(component == "predicate" && row.Request.Component == "internal") {
			continue
		}
		count++
		requestBytes, _ := json.Marshal(row.Request)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^"+regexp.QuoteMeta(t.Name())+"$")
		command.Env = append(os.Environ(), "FERRUGO_COMPONENT_GUARD_CHILD=1", "FERRUGO_COMPONENT_GUARD_REQUEST="+string(requestBytes))
		output, err := command.Output()
		cancel()
		abort := false
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%+v unexpected host failure:%v\n%s", row.Request, err, output)
			}
			abort = true
		}
		if abort != row.Want.Abort || string(output) != row.Want.Output {
			t.Fatalf("%+v abort%t/%t\nactual:\n%s\nexpected:\n%s", row.Request, abort, row.Want.Abort, output, row.Want.Output)
		}
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if total != m.Cases || hex.EncodeToString(digest.Sum(nil)) != m.RawSHA256 {
		t.Fatal("native component fixture count/checksum differs")
	}
	t.Logf("%d original component scripts", count)
}
