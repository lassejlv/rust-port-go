// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type markerBagAlias = datastructures.UnordBag[uint64]
type markerSetAlias = datastructures.UnordSet[uint64]
type markerMapAlias = datastructures.UnordMap[uint64, uint64]
type markerBagWrapper struct {
	*datastructures.UnordBag[uint64]
	ordered []uint64
}
type markerSetWrapper struct {
	*datastructures.UnordSet[uint64]
	other map[uint64]bool
}
type markerMapWrapper struct {
	*datastructures.UnordMap[uint64, uint64]
	other func()
}
type markerInput[T any] struct{ output *strings.Builder }

func markerItems[T any](output *strings.Builder) *datastructures.UnordItems[T] {
	state := rustvec.NewOwned(markerInput[T]{output}, rustvec.ValueOps[markerInput[T]]{Drop: func(state *markerInput[T], c rustfmt.Context) { state.output.WriteString("drop\n") }})
	return datastructures.NewUnordItems(state, datastructures.UnordIteratorOps[markerInput[T], T]{
		Next: func(ref rustvec.Ref[markerInput[T]]) *rustvec.Owned[T] {
			ref.Value.output.WriteString("next\n")
			return nil
		},
		SizeHint: func(rustvec.Ref[markerInput[T]]) rustvec.Hint { return rustvec.Hint{} },
		Count: func(state *rustvec.Owned[markerInput[T]], c rustfmt.Context) uint {
			state.Borrow().output.WriteString("next\n")
			return 0
		},
	})
}

func markerIntoIterator[T any](state *rustvec.Owned[markerInput[T]]) rustvec.IntoIterator[T] {
	return rustvec.FromIterator(rustvec.NewIteratorFromOwned(state, rustvec.IteratorOps[markerInput[T], T]{
		Next: func(ref rustvec.Ref[markerInput[T]]) (T, bool) {
			ref.Value.output.WriteString("next\n")
			var zero T
			return zero, false
		},
		SizeHint: func(rustvec.Ref[markerInput[T]]) rustvec.Hint { return rustvec.Hint{} },
	}))
}

func markerReject[C datastructures.UnordCollection](t *testing.T, collection C) {
	t.Helper()
	var output strings.Builder
	items := markerItems[uint64](&output)
	called := false
	message := ""
	func() {
		defer func() {
			if value := recover(); value != nil {
				message = fmt.Sprint(value)
			}
		}()
		datastructures.ExtendUnordItems(collection, items, func(C, *rustvec.Owned[markerInput[uint64]], rustfmt.Context) { called = true })
	}()
	if !strings.Contains(message, "source UnordCollection requires exact collection Self") || called || output.Len() != 0 {
		t.Fatalf("distinct source Self was consumed or extended: %q/%t/%q", message, called, output.String())
	}
	items.Close()
	if output.String() != "drop\n" {
		t.Fatalf("rejected input was not left owned: %q", output.String())
	}
}

func TestOriginalUnordCollectionMarker(t *testing.T) {
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/collection-marker/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Bounds []struct {
			Configuration, Stdout string
			Compiles              bool
		}
		Files     map[string]string
		Reference map[string]string `json:"reference_inputs"`
	}
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, expected := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != expected {
				t.Fatal("marker binding", path)
			}
		}
	}
	if len(metadata.Bounds) != 5 || !metadata.Bounds[0].Compiles || metadata.Bounds[0].Configuration != "positive" {
		t.Fatal("native marker bounds")
	}
	keys := rusthashmap.KeyOps[uint64]{Hash: func(value *uint64, h rusthashmap.Hasher, _ rustfmt.Context) { rusthashmap.HashU64(h, *value) }, Equal: func(a, b *uint64, _ rustfmt.Context) bool { return *a == *b }}
	bag := datastructures.NewUnordBag(datastructures.UnordBagConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}})
	set := datastructures.NewUnordSet(datastructures.UnordSetConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: keys})
	mapValue := datastructures.NewUnordMap(datastructures.UnordMapConfig[uint64, uint64]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys})
	defer bag.Close()
	defer set.Close()
	defer mapValue.Close()
	var bagAlias *markerBagAlias = bag
	var setAlias *markerSetAlias = set
	var mapAlias *markerMapAlias = mapValue
	var output strings.Builder
	datastructures.ExtendUnordItems(bagAlias, markerItems[uint64](&output), func(b *markerBagAlias, state *rustvec.Owned[markerInput[uint64]], c rustfmt.Context) {
		b.Extend(rustcindex.IndexIntoIterator[uint64]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[uint64] {
			actual := state.Take()
			return rustcindex.IndexValueIterator[uint64]{NextWithContext: func(rustfmt.Context) (uint64, bool) { actual.output.WriteString("next\n"); return 0, false }, CloseWithRuntimeContext: func(rustfmt.Context) { actual.output.WriteString("drop\n") }}
		}}, c)
	})
	datastructures.ExtendUnordItems(setAlias, markerItems[uint64](&output), func(s *markerSetAlias, state *rustvec.Owned[markerInput[uint64]], c rustfmt.Context) {
		s.ExtendScoped(markerIntoIterator(state), c)
	})
	datastructures.ExtendUnordItems(mapAlias, markerItems[rusthashmap.Pair[uint64, uint64]](&output), func(m *markerMapAlias, state *rustvec.Owned[markerInput[rusthashmap.Pair[uint64, uint64]]], c rustfmt.Context) {
		m.ExtendScoped(markerIntoIterator(state), c)
	})
	if output.String() != metadata.Bounds[0].Stdout {
		t.Fatalf("actual empty extension differs: %q != %q", output.String(), metadata.Bounds[0].Stdout)
	}
	markerReject(t, (*markerBagWrapper)(nil))
	markerReject(t, (*markerSetWrapper)(nil))
	markerReject(t, (*markerMapWrapper)(nil))
	markerReject(t, markerBagWrapper{UnordBag: bag, ordered: []uint64{1}})
	markerReject(t, &markerBagWrapper{UnordBag: bag, ordered: []uint64{1}})
	markerReject(t, markerSetWrapper{UnordSet: set, other: map[uint64]bool{1: true}})
	markerReject(t, &markerSetWrapper{UnordSet: set, other: map[uint64]bool{1: true}})
	markerReject(t, markerMapWrapper{UnordMap: mapValue, other: func() {}})
	markerReject(t, &markerMapWrapper{UnordMap: mapValue, other: func() {}})
}
