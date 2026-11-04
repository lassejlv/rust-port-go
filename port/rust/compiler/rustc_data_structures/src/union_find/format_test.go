// SPDX-License-Identifier: MIT OR Apache-2.0
package unionfind_test

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
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	probe "github.com/lassejlv/rust-port-go/tools/port-rust-union-find/format/validation"
)

func ufFormatRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test path unavailable")
	}
	root := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		next := filepath.Dir(root)
		if next == root {
			t.Fatal("repository root unavailable")
		}
		root = next
	}
}
func ufFormatDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func TestOriginalUnionFindFormatter(t *testing.T) {
	dir := filepath.Join(ufFormatRoot(t), "tools/port-rust-union-find/format")
	var manifest struct {
		Revision   string            `json:"revision"`
		Records    int               `json:"records"`
		Inputs     map[string]string `json:"inputs"`
		Decoded    string            `json:"decoded_sha256"`
		Compressed string            `json:"compressed_sha256"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" {
		t.Fatal("source revision changed")
	}
	for path, want := range manifest.Inputs {
		data, err = os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if ufFormatDigest(data) != want {
			t.Fatalf("formatter oracle input changed: %s; regenerate original output", path)
		}
	}
	data, err = os.ReadFile(filepath.Join(dir, "cases.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if ufFormatDigest(data) != manifest.Compressed {
		t.Fatal("compressed reference checksum changed")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	hash := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(gz, hash))
	scan.Buffer(make([]byte, 64*1024), 4*1024*1024)
	requests := probe.Requests()
	n := 0
	for scan.Scan() {
		var record probe.Record
		if err = json.Unmarshal(scan.Bytes(), &record); err != nil {
			t.Fatalf("record%d: %v", n, err)
		}
		if n >= len(requests) || record.Request != requests[n] {
			t.Fatalf("request%d differs from retained source request", n)
		}
		actual := probe.Replay(record.Request)
		if !reflect.DeepEqual(actual, record.Response) {
			t.Fatalf("case%d request%+v\nGo:%+v\nRust:%+v", n, record.Request, actual, record.Response)
		}
		n++
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hash.Sum(nil)) != manifest.Decoded {
		t.Fatal("decoded reference checksum changed")
	}
	if n != manifest.Records || n != len(requests) {
		t.Fatalf("reference count%d want%d/%d", n, manifest.Records, len(requests))
	}
}

func TestUnionFindFormatterBorrowedCellsAndLegacyDebug(t *testing.T) {
	sets := ds.NewUnionFind(3, ds.UnionFindUintIndex())
	sets.Unify(0, 1)
	var cells []*uint
	f := index.IndexDebugFormatter{Options: index.DefaultIndexFormatOptions(), Write: func(string) error { return nil }}
	if err := sets.FormatDebug(&f, func(key *uint, _ *index.IndexDebugFormatter) error { cells = append(cells, key); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(cells) != 3 || cells[0] == cells[1] || cells[1] == cells[2] || cells[0] == cells[2] {
		t.Fatal("distinct parent fields did not retain distinct cells")
	}
	sets.Unify(1, 2)
	position := 0
	if err := sets.FormatDebug(&f, func(key *uint, f *index.IndexDebugFormatter) error {
		if key != cells[position] {
			t.Fatalf("parent%d received a copied cell", position)
		}
		position++
		return index.FormatDebugUint(uint64(*key), uint(strconv.IntSize), f)
	}); err != nil {
		t.Fatal(err)
	}
	got := sets.Debug(func(key uint) string { return fmt.Sprint(key) })
	want := "UnionFind { table: [UnionFindEntry { parent: 0, rank: 1 }, UnionFindEntry { parent: 0, rank: 0 }, UnionFindEntry { parent: 0, rank: 0 }] }"
	if got != want {
		t.Fatalf("legacy Debug=%q", got)
	}
}

func TestOriginalUnionFindFormatterLive(t *testing.T) {
	value, exists := os.LookupEnv("FERRUGO_UF_FORMAT_ORACLE")
	if !exists {
		t.Skip("set FERRUGO_UF_FORMAT_ORACLE=1 to rebuild original source")
	}
	if value != "1" {
		t.Fatal("FERRUGO_UF_FORMAT_ORACLE must be1")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin/go")
	if custom, exists := os.LookupEnv("FERRUGO_GO"); exists {
		if custom == "" {
			t.Fatal("FERRUGO_GO cannot be empty")
		}
		goBin = custom
	}
	root := ufFormatRoot(t)
	if !filepath.IsAbs(goBin) && strings.ContainsAny(goBin, `/\`) {
		goBin = filepath.Join(root, goBin)
	}
	cmd := exec.Command(goBin, "run", "./tools/port-rust-union-find/format", "-check")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live reference: %v\n%s", err, output)
	}
}

func TestSnapshotVecFullTypedReturnAndTraits(t *testing.T) {
	type key struct {
		n        uint
		metadata []byte
	}
	var trace []string
	metadata := []byte{1, 2}
	idx := ds.UnionFindIndex[key]{FromIndex: func(n uint) key { trace = append(trace, fmt.Sprint("new:", n)); return key{n, metadata} }, ToIndex: func(k key) uint { trace = append(trace, fmt.Sprint("index:", k.n)); return k.n }, Equal: func(a, b key) bool { return a.n == b.n }}
	sets := ds.NewUnionFind(3, idx)
	sets.Unify(idx.FromIndex(0), idx.FromIndex(1))
	trace = nil
	old := sets.Snapshot()
	legacyTrace := append([]string(nil), trace...)
	var cloned []uint
	ops := index.ValueOps[key]{Copy: true, RustSize: 3 * uint(strconv.IntSize/8), CloneRef: func(k *key) key { cloned = append(cloned, k.n); return *k }, EqualRef: func(a, b *key) bool { return a.n == b.n }, FormatDebug: func(k *key, f *index.IndexDebugFormatter) error {
		return index.FormatDebugUint(uint64(k.n), uint(strconv.IntSize), f)
	}}
	trace = nil
	snapshot := sets.SnapshotVec(ops)
	if !reflect.DeepEqual(trace, legacyTrace) {
		t.Fatalf("primary snapshot trace=%v legacy=%v", trace, legacyTrace)
	}
	if snapshot.Len() != 3 || snapshot.Raw.Cap() != 3 || snapshot.Debug() != "[0, 0, 2]" {
		t.Fatalf("full snapshot len=%d cap=%d debug=%s", snapshot.Len(), snapshot.Raw.Cap(), snapshot.Debug())
	}
	for i, k := range old.Raw {
		if snapshot.Raw.At(uint(i)).n != k.n {
			t.Fatal("primary and compatibility roots differ")
		}
	}
	copy := snapshot.Clone()
	if !reflect.DeepEqual(cloned, []uint{0, 0, 2}) || !copy.Equal(snapshot) {
		t.Fatalf("returned Clone/Eq failed: %v", cloned)
	}
	var hashed []uint
	snapshot.Hash(func(raw index.IndexRaw[key]) {
		index.HashIndexRawRef(raw, &hashed, func(out *[]uint, n uint) { *out = append(*out, n) }, func(out *[]uint, k *key) {
			position := uint(len(*out) - 1)
			if k != snapshot.Raw.At(position) {
				t.Fatal("hash received copied snapshot cell")
			}
			*out = append(*out, k.n)
		}, nil)
	})
	if !reflect.DeepEqual(hashed, []uint{3, 0, 0, 2}) {
		t.Fatalf("hash=%v", hashed)
	}
	it := snapshot.IterEnumerated()
	i, value, ok := it.NextBack()
	if !ok || i.n != 2 || value != snapshot.Raw.At(2) {
		t.Fatal("returned typed iteration failed")
	}
	snapshot.At(idx.FromIndex(0)).n = 2
	if sets.Find(idx.FromIndex(0)).n != 0 || copy.Raw.At(0).n != 0 {
		t.Fatal("snapshot mutation changed forest or clone")
	}
}

func TestSnapshotVecZeroSizedKeyDomain(t *testing.T) {
	idx := ds.UnionFindIndex[struct{}]{FromIndex: func(n uint) struct{} {
		if n != 0 {
			panic("zero-sized key domain")
		}
		return struct{}{}
	}, ToIndex: func(struct{}) uint { return 0 }}
	sets := ds.NewUnionFind(0, idx)
	snapshot := sets.SnapshotVec(index.ValueOps[struct{}]{Copy: true, TrivialClone: true})
	if snapshot.Len() != 0 || snapshot.Raw.Cap() != ^uint(0) {
		t.Fatalf("empty ZST snapshot len=%d cap=%d", snapshot.Len(), snapshot.Raw.Cap())
	}
}

func TestSnapshotVecReturnedOverflowProfile(t *testing.T) {
	sets := ds.NewUnionFind(1, ds.UnionFindUintIndex())
	release := sets.SnapshotVecWithConfig(index.ValueOps[uint]{Copy: true}, index.IndexStorageConfig{})
	release.ResizeToElem(^uint(0), func() uint { t.Fatal("wrapped shrink called factory"); return 0 })
	if release.Len() != 0 || release.Config.OverflowChecks || release.Idx.Config.OverflowChecks {
		t.Fatal("returned release profile was not retained")
	}
	checked := sets.SnapshotVec(index.ValueOps[uint]{Copy: true})
	mustPanic(t, func() {
		checked.ResizeToElem(^uint(0), func() uint { t.Fatal("checked overflow called factory"); return 0 })
	})
	if checked.Len() != 1 {
		t.Fatal("checked overflow mutated returned snapshot")
	}
}
