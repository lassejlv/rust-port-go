// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_data_structures/src/union_find/tests.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.
package unionfind_test

import (
	"reflect"
	"strconv"
	"testing"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

func TestEmpty(t *testing.T) {
	sets := datastructures.NewUnionFind(10, datastructures.UnionFindUint32Index())
	for i := uint32(1); i < 10; i++ {
		if got := sets.Find(i); got != i {
			t.Errorf("find(%d) = %d, want %d", i, got, i)
		}
	}
}

func TestTransitive(t *testing.T) {
	sets := datastructures.NewUnionFind(10, datastructures.UnionFindUint32Index())
	sets.Unify(3, 7)
	sets.Unify(4, 2)
	if sets.Find(7) != sets.Find(3) || sets.Find(2) != sets.Find(4) || sets.Find(3) == sets.Find(4) {
		t.Fatal("incorrect distinct sets before transitive unification")
	}
	sets.Unify(7, 4)
	if sets.Find(7) != sets.Find(3) || sets.Find(2) != sets.Find(4) || sets.Find(3) != sets.Find(4) {
		t.Fatal("incorrect transitive unification")
	}
	for _, i := range []uint32{0, 1, 5, 6, 8, 9} {
		if got := sets.Find(i); got != i {
			t.Errorf("unrelated find(%d) = %d, want %d", i, got, i)
		}
	}
}

func TestTypedIndexValidationOrderAndSnapshotOwnership(t *testing.T) {
	type key struct{ n uint }
	var calls []uint
	limit := uint(4)
	index := datastructures.UnionFindIndex[key]{
		FromIndex: func(i uint) key {
			calls = append(calls, i)
			if i > limit {
				panic("index exceeds limit")
			}
			return key{i}
		},
		ToIndex: func(k key) uint { return k.n },
	}
	sets := datastructures.NewUnionFind(4, index)
	if !reflect.DeepEqual(calls, []uint{4, 0, 1, 2, 3}) {
		t.Fatalf("constructor callback order = %v", calls)
	}
	sets.Unify(key{2}, key{3})
	calls = nil
	snapshot := sets.Snapshot()
	if !reflect.DeepEqual(calls, []uint{4, 0, 1, 2, 3}) {
		t.Fatalf("snapshot callback order = %v", calls)
	}
	if snapshot.Len() != 4 || snapshot.At(key{3}) != (key{2}) {
		t.Fatalf("typed snapshot = %#v", snapshot.Raw)
	}
	snapshot.Set(key{3}, key{1})
	if snapshot.At(key{3}) != (key{1}) || sets.Find(key{3}) != (key{2}) {
		t.Fatal("snapshot aliases forest")
	}
	sets.Unify(key{2}, key{1})
	if snapshot.At(key{1}) != (key{1}) {
		t.Fatal("later forest changes mutate snapshot")
	}
	limit = 3
	calls = nil
	mustPanic(t, func() { sets.Snapshot() })
	if !reflect.DeepEqual(calls, []uint{4}) {
		t.Fatalf("snapshot must validate len before per-key conversion: %v", calls)
	}
	calls = nil
	mustPanic(t, func() { datastructures.NewUnionFind(4, index) })
	if !reflect.DeepEqual(calls, []uint{4}) {
		t.Fatalf("constructor must validate len before per-key conversion: %v", calls)
	}
	calls = nil
	_ = datastructures.NewUnionFind(0, index)
	if !reflect.DeepEqual(calls, []uint{0}) {
		t.Fatalf("empty constructor validation = %v", calls)
	}
}

func TestRanksPathCompressionAndBounds(t *testing.T) {
	sets := datastructures.NewUnionFind(8, datastructures.UnionFindUintIndex())
	// Construct a depth-three tree. A single Find(7) must advance through every
	// original parent while replacing each with its original grandparent.
	for _, pair := range [][2]uint{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {0, 2}, {4, 6}, {0, 4}} {
		sets.Unify(pair[0], pair[1])
	}
	if root := sets.Find(7); root != 0 {
		t.Fatalf("root = %d", root)
	}
	formatKey := func(k uint) string { return strconv.FormatUint(uint64(k), 10) }
	want := "UnionFind { table: [UnionFindEntry { parent: 0, rank: 3 }, UnionFindEntry { parent: 0, rank: 0 }, UnionFindEntry { parent: 0, rank: 1 }, UnionFindEntry { parent: 2, rank: 0 }, UnionFindEntry { parent: 0, rank: 2 }, UnionFindEntry { parent: 4, rank: 0 }, UnionFindEntry { parent: 0, rank: 1 }, UnionFindEntry { parent: 4, rank: 0 }] }"
	if got := sets.Debug(formatKey); got != want {
		t.Fatalf("compression state = %s, want %s", got, want)
	}
	if sets.Unify(1, 7) != 0 {
		t.Fatal("same-set unification root")
	}
	if sets.Unify(5, 3) != 0 {
		t.Fatal("greater-rank root must win")
	}
	mustPanic(t, func() { sets.Find(8) })
	mustPanic(t, func() { sets.Unify(0, 8) })
	mustPanic(t, func() { sets.Unify(8, 0) })
	snapshot := sets.Snapshot()
	mustPanic(t, func() { snapshot.At(8) })
	mustPanic(t, func() { snapshot.Set(8, 0) })
	empty := datastructures.NewUnionFind(0, datastructures.UnionFindUintIndex())
	if empty.Snapshot().Len() != 0 {
		t.Fatal("empty snapshot")
	}
	mustPanic(t, func() { empty.Find(0) })
	if ^uint(0) > uint(^uint32(0)) {
		overU32 := uint(^uint32(0))
		overU32++
		mustPanic(t, func() { datastructures.UnionFindUint32Index().FromIndex(overU32) })
		mustPanic(t, func() { datastructures.NewUnionFind(overU32, datastructures.UnionFindUint32Index()) })
	}
}

func TestSliceMetadataKeysAndDefaultNotEqual(t *testing.T) {
	type key struct {
		index    uint
		metadata []byte
	}
	metadata := []byte{1, 2}
	equalCalls := 0
	index := datastructures.UnionFindIndex[key]{
		FromIndex: func(i uint) key { return key{i, metadata} },
		ToIndex:   func(k key) uint { return k.index },
		Equal:     func(a, b key) bool { equalCalls++; return a.index == b.index },
	}
	sets := datastructures.NewUnionFind(3, index)
	if root := sets.Unify(key{0, metadata}, key{1, metadata}); root.index != 0 {
		t.Fatalf("root=%#v", root)
	}
	snapshot := sets.Snapshot()
	for i, want := range []uint{0, 0, 2} {
		if snapshot.Raw[i].index != want {
			t.Fatalf("snapshot=%#v", snapshot.Raw)
		}
	}
	if equalCalls == 0 {
		t.Fatal("nil NotEqual must forward to Equal")
	}
	before := equalCalls
	sets.Find(key{2, metadata})
	if equalCalls != before+1 {
		t.Fatalf("default ne must call eq exactly once: %d→%d", before, equalCalls)
	}
}

func mustPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	f()
}
