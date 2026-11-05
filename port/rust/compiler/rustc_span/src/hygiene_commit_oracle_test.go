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
	"strings"
	"testing"
	"time"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type hygieneCommitCore struct {
	selected string
	external sp.Shared[[]byte]
}
type hygieneCommitKey struct {
	value    uint32
	core     *hygieneCommitCore
	features sp.Shared[[]byte]
	metadata []byte
}
type hygieneCommitState struct {
	core *hygieneCommitCore
	slot sp.Option[hygieneCommitKey]
}

func (s *hygieneCommitCore) refs() uint { return s.external.StrongCount() }
func (s *hygieneCommitCore) local(name string, c rf.Context) {
	fmt.Printf("local:%s:%t:%d:%d\n", name, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
}
func (s *hygieneCommitCore) raise(c rf.Context, stage string) {
	c.PanicAt(rf.SourcePanicInfo{Payload: stage, Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation(stage)})
}
func (s *hygieneCommitCore) from(value uint32, c rf.Context) hygieneCommitKey {
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local("from", c) })
	fmt.Printf("from:%d:%t:%d:%d\n", value, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if value == 2 && s.selected == "from" {
		s.raise(c, "from")
	}
	return hygieneCommitKey{value, s, s.external.CloneWithContext(c), []byte{1, 2, 3}}
}
func (k *hygieneCommitKey) drop(c rf.Context) {
	s := k.core
	defer hygieneOwnedClose(c, func(c rf.Context) {
		k.features.CloseWithContext(c)
		k.metadata = nil
		fmt.Printf("field:%d:%t:%d:%d\n", k.value, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		if k.value == 1 && (s.selected == "old_field" || s.selected == "old_drop_field") || k.value == 2 && (s.selected == "new_field" || s.selected == "new_drop_field") {
			s.raise(c, "key_field")
		}
	})
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local("drop", c) })
	if len(k.metadata) != 3 {
		panic("invalid commit key ownership")
	}
	fmt.Printf("drop:%d:%t:%d:%d\n", k.value, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if k.value == 1 && (s.selected == "old_drop" || s.selected == "old_drop_field") || k.value == 2 && (s.selected == "new_drop" || s.selected == "new_drop_field") {
		s.raise(c, "key_drop")
	}
}

type hygieneCommitRemapper struct {
	state *hygieneCommitState
}

func (r hygieneCommitRemapper) StableHash(hygieneCommitKey, ds.StableHashCtxt, *ds.StableHasher, rf.Context) {
	panic("unexpected stable_hash")
}
func (r hygieneCommitRemapper) Len() uint {
	return r.LenWithContext(rf.Context{})
}
func (r hygieneCommitRemapper) LenWithContext(c rf.Context) uint {
	s := r.state.core
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local("len", c) })
	fmt.Printf("len:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if s.selected == "len" {
		s.raise(c, "len")
	}
	return 2
}
func (r hygieneCommitRemapper) FirstNonDetIndexMut() *sp.Option[hygieneCommitKey] {
	return r.FirstNonDetIndexMutWithContext(rf.Context{})
}
func (r hygieneCommitRemapper) FirstNonDetIndexMutWithContext(c rf.Context) *sp.Option[hygieneCommitKey] {
	s := r.state.core
	defer hygieneOwnedClose(c, func(c rf.Context) { s.local("slot", c) })
	fmt.Printf("slot:%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
	if s.selected == "slot" {
		s.raise(c, "slot")
	}
	return &r.state.slot
}
func (r hygieneCommitRemapper) FirstNonDetIndex(rf.Context) uint32 {
	k, _ := r.state.slot.Get()
	return k.value
}
func hygieneCommitReport(c rf.Context, s *hygieneCommitCore, call func()) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Printf("caught:true:%x:%t:%d:%d\n", fmt.Sprint(p), c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		}
	}()
	call()
	fmt.Printf("caught:false::%t:%d:%d\n", c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
}
func hygieneCommitExecute(q hygieneOwnedRequest, m hygieneOwnedMetadata) {
	s := &hygieneCommitCore{external: sp.NewShared([]byte{7}, nil)}
	defer s.external.Close()
	construction := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: &hygieneOwnedRuntime{1}}}, &rf.PanicOperations{Hook: func(rf.SourcePanicInfo, rf.Context) { fmt.Println("stale_hook") }})
	fmt.Printf("construct:1:%d\n", s.refs())
	state := &hygieneCommitState{core: s}
	if strings.HasPrefix(q.Operation, "repeat") {
		state.slot = sp.Some(s.from(1, construction))
	}
	s.selected = q.Case
	r := hygieneCommitRemapper{state: state}
	ref := sp.BorrowHygieneRemapper[hygieneCommitRemapper, hygieneCommitKey, uint32](&r, sp.HygieneRemapperOps[hygieneCommitRemapper, hygieneCommitKey, uint32]{StableHash: func(*hygieneCommitRemapper, hygieneCommitKey, ds.StableHashCtxt, *ds.StableHasher, rf.Context) {
		panic("unexpected stable_hash")
	}, Len: func(r *hygieneCommitRemapper, c rf.Context) uint { return r.LenWithContext(c) }, FirstNonDetIndexMut: func(r *hygieneCommitRemapper, c rf.Context) *sp.Option[hygieneCommitKey] {
		return r.FirstNonDetIndexMutWithContext(c)
	}, FirstNonDetIndex: func(r *hygieneCommitRemapper, c rf.Context) uint32 { return r.FirstNonDetIndex(c) }, DropCommitValue: func(k *hygieneCommitKey, c rf.Context) { k.drop(c) }}, s.from)
	action := func(c rf.Context) {
		if strings.HasSuffix(q.Operation, "_primary") {
			sp.CommitHygieneEndOfDeterminismWithOps[hygieneCommitKey](r, sp.HygieneCommitValueOps[hygieneCommitKey]{FromU32: s.from, Drop: func(k *hygieneCommitKey, c rf.Context) { k.drop(c) }}, c)
		} else {
			sp.CommitHygieneEndOfDeterminism[hygieneCommitKey](ref, s.from, c)
		}
	}
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rf.SourceLocation{}}
	for site, text := range m.Locations {
		ops.Locations[site] = hygieneOwnedLocation(text)
	}
	finished := false
	ops.Hook = func(info rf.SourcePanicInfo, c rf.Context) {
		location := ""
		if info.Location != nil {
			location = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		fmt.Printf("hook:%s:%x:%t:%t:%t:%d:%d:%s\n", info.Kind, fmt.Sprint(info.Payload), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hygieneOwnedRuntimeID(c), s.refs(), location)
		if q.Mode == "processing" && !finished {
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
		hygieneCommitReport(c, s, func() { action(c) })
	} else if q.Mode == "processing" {
		hygieneCommitReport(c, s, func() { s.raise(c, "outer") })
	} else {
		hygieneCommitReport(c, s, func() {
			defer hygieneOwnedClose(c, func(u rf.Context) {
				fmt.Printf("outer:%t:%d:%d\n", u.Panicking(), hygieneOwnedRuntimeID(u), s.refs())
				if q.Mode == "inherited_caught" {
					hygieneCommitReport(u, s, func() { action(u) })
				} else {
					action(u)
				}
			})
			s.raise(c, "outer")
		})
	}
	finished = true
	slot := uint32(0)
	if k, ok := state.slot.Get(); ok {
		slot = k.value
	}
	fmt.Printf("after:%d:%d:%d\n", slot, hygieneOwnedRuntimeID(c), s.refs())
	hygieneCommitReport(c, s, func() {
		fmt.Printf("remapper_drop:%d:%t:%d:%d\n", slot, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		if k, ok := state.slot.Get(); ok {
			state.slot = sp.None[hygieneCommitKey]()
			k.drop(c)
		}
	})
	fmt.Printf("complete:%d:%d\n", hygieneOwnedRuntimeID(c), s.refs())
}
func TestOriginalHygieneCommitChild(t *testing.T) {
	if os.Getenv("FERRUGO_HYGIENE_COMMIT_CHILD") != "1" {
		return
	}
	var q hygieneOwnedRequest
	if e := json.Unmarshal([]byte(os.Getenv("FERRUGO_HYGIENE_COMMIT_REQUEST")), &q); e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(filepath.Join(hygieneOwnedRoot(), "tools/port-rust-hygiene/commit-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m hygieneOwnedMetadata
	if e = json.Unmarshal(data, &m); e != nil {
		t.Fatal(e)
	}
	hygieneCommitExecute(q, m)
	os.Exit(0)
}
func TestOriginalHygieneCommit(t *testing.T) {
	root := hygieneOwnedRoot()
	data, e := os.ReadFile(filepath.Join(root, "tools/port-rust-hygiene/commit-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var m hygieneOwnedMetadata
	if e = json.Unmarshal(data, &m); e != nil {
		t.Fatal(e)
	}
	for name, want := range m.Files {
		data, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("commit source binding changed:%s", name)
		}
	}
	f, e := os.Open(filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/hygiene_commit.jsonl.gz"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	digest := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(z, digest))
	scan.Buffer(make([]byte, 65536), 8<<20)
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
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
		if e = json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		request, _ := json.Marshal(row.Request)
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(c, binary, "-test.run=^TestOriginalHygieneCommitChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_HYGIENE_COMMIT_CHILD=1", "FERRUGO_HYGIENE_COMMIT_REQUEST="+string(request))
		out, e := cmd.Output()
		cancel()
		abort := false
		if e != nil {
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%+v host failure:%v\n%s", row.Request, e, out)
			}
			abort = true
		}
		if abort != row.Want.Abort || string(out) != row.Want.Output {
			t.Fatalf("%+v abort%t/%t\nactual:\n%s\nexpected:\n%s", row.Request, abort, row.Want.Abort, out, row.Want.Output)
		}
		count++
	}
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	if count != m.Cases || hex.EncodeToString(digest.Sum(nil)) != m.RawSHA256 {
		t.Fatal("commit proof count/checksum differs")
	}
}
