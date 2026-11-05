// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type indexContextCase struct {
	Mode, Observer, Action string
	Resume                 bool
	Outcome                string
	Stdout                 string
}
type indexContextManifest struct {
	Cases, Aborts    int
	Fixture          string `json:"fixture_sha256"`
	Raw              string `json:"raw_sha256"`
	Locations        map[string]rustfmt.SourceLocation
	GoSources        map[string]string `json:"go_sources"`
	ReferenceSources map[string]string `json:"reference_sources"`
	Selector         string            `json:"sdk_selector_sha256"`
}

func indexContextDigest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func indexLoadContext(t *testing.T) (indexContextManifest, []indexContextCase) {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/index-context/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m indexContextManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	paths := map[string]string{"../../../../../tools/port-rust-unord/sort/generate.py": m.Selector}
	for n, h := range m.GoSources {
		paths[n] = h
	}
	for n, h := range m.ReferenceSources {
		paths["../../../../../tools/port-rust-unord/index-context/"+n] = h
	}
	for n, h := range paths {
		b, e := os.ReadFile(n)
		if e != nil || indexContextDigest(b) != h {
			t.Fatalf("context binding changed: %s (%v)", n, e)
		}
	}
	b, e = os.ReadFile("testdata/runtime_context.jsonl.gz")
	if e != nil || indexContextDigest(b) != m.Fixture {
		t.Fatal("context fixture digest", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 2<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if indexContextDigest(raw) != m.Raw {
		t.Fatal("raw context digest")
	}
	var rows []indexContextCase
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	for scanner.Scan() {
		var c indexContextCase
		if e = json.Unmarshal(scanner.Bytes(), &c); e != nil {
			t.Fatal(e)
		}
		rows = append(rows, c)
	}
	if scanner.Err() != nil || len(rows) != 228 || m.Cases != 228 || m.Aborts != 88 {
		t.Fatal("context coverage", len(rows), scanner.Err())
	}
	return m, rows
}

type indexContextRunner struct {
	spec      indexContextCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *indexContextRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func indexContextLabel(c rustfmt.Context) string {
	if c.Runtime != nil && c.Runtime.Value != nil {
		return c.Runtime.Value.(string)
	}
	return "none"
}
func (r *indexContextRunner) raise(c rustfmt.Context, payload string) {
	if r.spec.Resume {
		c.ResumeUnwind(payload)
	}
	location := r.locations[payload]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: payload, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func indexContextCatch(body func()) (message string) {
	defer func() {
		if p := recover(); p != nil {
			message = fmt.Sprint(p)
		}
	}()
	body()
	return
}
func (r *indexContextRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	ops := ValueOps[uint8]{RustSize: 1, CloneRefWithContext: func(v *uint8, c rustfmt.Context) uint8 {
		r.event("clone:%d:%s:%t", *v, indexContextLabel(c), c.Panicking())
		if *v == 2 && (strings.HasPrefix(r.spec.Action, "clone_fail") || r.spec.Action == "clone_from_fail") {
			r.raise(c, "clone")
		}
		return *v
	}, DropWithRuntimeContext: func(v *uint8, c rustfmt.Context) {
		r.event("drop:%d:%s:%t", *v, indexContextLabel(c), c.Panicking())
		if *v == 10 && r.spec.Action == "clone_from_old_drop" || *v == 1 && (r.spec.Action == "drop_fail" || r.spec.Action == "clone_fail_drop" || r.spec.Action == "iter_drop_fail") {
			r.raise(c, "element_drop")
		}
	}}
	// The dictionary contains no A runtime pointer. Source setup happens under A;
	// the operation's B/no-observer runtime is supplied only to each call below.
	setup := func(values ...uint8) *IndexVec[uint, uint8] {
		v := NewIndexVec(UsizeIdx(), ops)
		for _, value := range values {
			v.Push(value)
		}
		return v
	}
	result := func(message string) { r.event("result:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking()) }
	switch r.spec.Action {
	case "extend_trusted_none", "collect_trusted_none":
		field := struct{}{}
		live := true
		ops := ValueOps[struct{}]{RustZeroSized: true, DropWithRuntimeContext: func(_ *struct{}, c rustfmt.Context) {
			r.event("zst_drop:%s:%t", indexContextLabel(c), c.Panicking())
		}}
		source := IndexIntoIterator[struct{}]{IntoIterWithContext: func(rustfmt.Context) IndexValueIterator[struct{}] {
			return IndexValueIterator[struct{}]{TrustedLength: true, NextWithContext: func(rustfmt.Context) (struct{}, bool) {
				panic("overflowing trusted range must not be advanced")
			}, SizeHintWithContext: func(rustfmt.Context) (uint, uint, bool) { return ^uint(0), 0, false }, CloseWithRuntimeContext: func(c rustfmt.Context) {
				if live {
					live = false
					ops.DropWithRuntimeContext(&field, c)
				}
			}}
		}}
		result(indexContextCatch(func() {
			if r.spec.Action == "extend_trusted_none" {
				v := NewIndexVec(UsizeIdx(), ops)
				v.ExtendWithContext(source, c)
			} else {
				v := IndexVecFromIteratorWithContext(source, UsizeIdx(), ops, c)
				v.CloseWithRuntimeContext(c)
			}
		}))
	case "clone_zst_max":
		prototype := struct{}{}
		v := IndexVecFromStorage(IndexRawZeroSized(&prototype, ^uint(0)), UsizeIdx(), ValueOps[struct{}]{RustZeroSized: true, TrivialClone: true})
		cloned := v.CloneWithContext(c)
		r.event("sizes:%d:%d", cloned.Len(), cloned.Raw.Cap())
	case "extend_capacity_generic", "extend_capacity_trusted", "extend_capacity_owned":
		prototype := struct{}{}
		ops := ValueOps[struct{}]{RustZeroSized: true, DropWithRuntimeContext: func(_ *struct{}, c rustfmt.Context) {
			r.event("zst_drop:%s:%t", indexContextLabel(c), c.Panicking())
		}}
		v := IndexVecFromStorage(IndexRawZeroSized(&prototype, ^uint(0)), UsizeIdx(), ops)
		var source IndexIntoIterator[struct{}]
		if r.spec.Action == "extend_capacity_owned" {
			input := NewIndexVec(UsizeIdx(), ops)
			input.Push(struct{}{})
			source = input.IntoIter().IntoIterator()
		} else {
			next := uint(0)
			source.IntoIterWithContext = func(rustfmt.Context) IndexValueIterator[struct{}] {
				return IndexValueIterator[struct{}]{TrustedLength: r.spec.Action == "extend_capacity_trusted", NextWithContext: func(c rustfmt.Context) (struct{}, bool) {
					r.event("znext:%d:%s:%t", next, indexContextLabel(c), c.Panicking())
					if next == 0 {
						next++
						return struct{}{}, true
					}
					return struct{}{}, false
				}, SizeHintWithContext: func(c rustfmt.Context) (uint, uint, bool) {
					r.event("zhint:%s:%t", indexContextLabel(c), c.Panicking())
					return 1 - next, 1 - next, true
				}, CloseWithRuntimeContext: func(c rustfmt.Context) {
					r.event("zinput_drop:%s:%t", indexContextLabel(c), c.Panicking())
				}}
			}
		}
		message := indexContextCatch(func() { v.ExtendWithContext(source, c) })
		v.Raw = IndexRawZeroSized(&prototype, 0)
		result(message)
	case "capacity":
		prototype := struct{}{}
		zops := ValueOps[struct{}]{RustZeroSized: true, DropWithRuntimeContext: func(_ *struct{}, c rustfmt.Context) { r.event("zst_drop:%s:%t", indexContextLabel(c), c.Panicking()) }}
		v := IndexVecFromStorage(IndexRawZeroSized(&prototype, ^uint(0)), UsizeIdx(), zops)
		message := indexContextCatch(func() { v.PushWithContext(struct{}{}, c) })
		v.Raw = IndexRawZeroSized(&prototype, 0)
		result(message)
	case "extend_fail", "extend_double", "collect_fail", "collect_double":
		var v *IndexVec[uint, uint8]
		if !strings.HasPrefix(r.spec.Action, "collect") {
			v = setup(10)
		}
		next := uint8(0)
		source := IndexIntoIterator[uint8]{IntoIterWithContext: func(rustfmt.Context) IndexValueIterator[uint8] {
			return IndexValueIterator[uint8]{NextWithContext: func(c rustfmt.Context) (uint8, bool) {
				r.event("next:%d:%s:%t", next, indexContextLabel(c), c.Panicking())
				next++
				if next == 2 {
					r.raise(c, "next")
				}
				return next, true
			}, SizeHintWithContext: func(c rustfmt.Context) (uint, uint, bool) {
				r.event("hint:%s:%t", indexContextLabel(c), c.Panicking())
				return 0, 0, false
			}, CloseWithRuntimeContext: func(c rustfmt.Context) {
				r.event("input_drop:%s:%t", indexContextLabel(c), c.Panicking())
				if r.spec.Action == "extend_double" || r.spec.Action == "collect_double" {
					r.raise(c, "iterator_drop")
				}
			}}
		}}
		if strings.HasPrefix(r.spec.Action, "collect") {
			result(indexContextCatch(func() {
				collected := IndexVecFromIteratorWithContext(source, UsizeIdx(), ops, c)
				collected.CloseWithRuntimeContext(c)
			}))
			return
		}
		message := indexContextCatch(func() { v.ExtendWithContext(source, c) })
		result(message)
		v.CloseWithRuntimeContext(c)
	case "iter_drop_fail":
		v := setup(1, 2, 3)
		it := v.IntoIter()
		result(indexContextCatch(func() { it.CloseWithRuntimeContext(c) }))
	case "drop_fail":
		v := setup(1, 2, 3)
		result(indexContextCatch(func() { v.CloseWithRuntimeContext(c) }))
	case "clone_from_ok", "clone_from_fail", "clone_from_old_drop":
		source, destination := setup(1, 2, 3), setup(10)
		result(indexContextCatch(func() { destination.CloneFromWithContext(source, c) }))
		destination.CloseWithRuntimeContext(c)
		source.CloseWithRuntimeContext(c)
	default:
		v := setup(1, 2, 3)
		message := indexContextCatch(func() { copy := v.CloneWithContext(c); copy.CloseWithRuntimeContext(c) })
		result(message)
		v.CloseWithRuntimeContext(c)
	}
}
func (r *indexContextRunner) run() {
	var c rustfmt.Context
	if r.spec.Observer == "B" {
		c.Runtime = &rustfmt.RuntimeMetadata{Value: "B"}
	}
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	capacity := r.locations["capacity overflow"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c = rustfmt.WithPanicOperations(c, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rustfmt.SourceLocation{PanicVectorCapacitySite: capacity, PanicExtendTrustedOverflowSite: r.locations["capacity overflow.extend_trusted_none"], PanicCollectTrustedOverflowSite: r.locations["capacity overflow.collect_trusted_none"]}, Hook: func(info rustfmt.SourcePanicInfo, hook rustfmt.Context) {
		if r.spec.Observer == "B" {
			loc := rustfmt.SourceLocation{}
			if info.Location != nil {
				loc = *info.Location
			}
			r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), hook.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(loc.File)), loc.Line, loc.Column)
			r.event("hook_local:%t", hook.Panicking())
		}
		if r.spec.Mode == "processing" {
			r.body(hook)
		}
	}})
	if r.spec.Mode == "normal" {
		r.body(c)
		return
	}
	outer := r.locations["outer."+r.spec.Mode]
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &outer, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func TestOriginalIndexRuntimeContext(t *testing.T) {
	m, rows := indexLoadContext(t)
	aborts := 0
	for i, c := range rows {
		var got string
		if c.Outcome == "abort" {
			aborts++
			b, _ := json.Marshal(c)
			cmd := exec.Command(os.Args[0], "-test.run=^TestIndexRuntimeContextChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_INDEX_CONTEXT_CASE="+string(b))
			out, e := cmd.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("case %d abort: %v %s", i, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			r := indexContextRunner{c, m.Locations, &out}
			r.run()
			got = out.String()
		}
		if got != c.Stdout {
			t.Fatalf("case %d %#v\nGOT:\n%sWANT:\n%s", i, c, got, c.Stdout)
		}
	}
	if aborts != 88 {
		t.Fatal("abort coverage", aborts)
	}
}
func TestIndexRuntimeContextChild(t *testing.T) {
	value := os.Getenv("FERRUGO_INDEX_CONTEXT_CASE")
	if value == "" {
		return
	}
	m, _ := indexLoadContext(t)
	var c indexContextCase
	if e := json.Unmarshal([]byte(value), &c); e != nil {
		t.Fatal(e)
	}
	r := indexContextRunner{c, m.Locations, os.Stdout}
	r.run()
	os.Exit(0)
}
