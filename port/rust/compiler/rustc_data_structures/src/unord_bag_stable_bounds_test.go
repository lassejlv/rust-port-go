// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

// Both the direct method dictionary and the existing KeyOps adapter must
// preserve unused Hash/Eq: only StableHash has effects in the original body.
type bagStableBoundsValue struct{ output *strings.Builder }

func (bagStableBoundsValue) Hash(*uint64, rusthashmap.Hasher, rustfmt.Context) {
	panic("UNEXPECTED Hash")
}
func (bagStableBoundsValue) Equal(*uint64, *uint64, rustfmt.Context) bool {
	panic("UNEXPECTED Eq")
}
func (ops bagStableBoundsValue) StableHash(value *uint64, _ StableHashCtxt, hasher *StableHasher, _ rustfmt.Context) {
	fmt.Fprintf(ops.output, "stable:%d\n", *value)
	hasher.WriteU64(*value)
}
func TestOriginalUnordBagStableBounds(t *testing.T) {
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/bag-stable-bounds/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Bounds []struct {
			Configuration string
			Compiles      bool
			Stdout        string
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
			sum := sha256.Sum256(data)
			if err != nil || hex.EncodeToString(sum[:]) != expected {
				t.Fatalf("Bag StableHash bound binding %s: %v", path, err)
			}
		}
	}
	if len(metadata.Bounds) != 4 || metadata.Bounds[0].Configuration != "positive" || !metadata.Bounds[0].Compiles {
		t.Fatal("original Bag StableHash bounds")
	}
	for _, adapted := range []bool{false, true} {
		var output strings.Builder
		direct := bagStableBoundsValue{&output}
		var selected UnordBagStableHashValue[uint64] = direct
		if adapted {
			selected = UnordBagStableHashOps[uint64]{HashEq: rusthashmap.KeyOps[uint64]{Hash: direct.Hash, Equal: direct.Equal}, HashStable: direct.StableHash}
		}
		for _, length := range []int{0, 1, 2, 4} {
			fmt.Fprintf(&output, "length:%d\n", length)
			config := UnordBagConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}}
			bag := NewUnordBag(config)
			for n := 0; n < length; n++ {
				bag.Push(uint64(n))
			}
			hasher := NewStableHasher()
			UnordBagStableHash(bag, nil, hasher, selected)
			fingerprint := hasher.FinishHash()
			fmt.Fprintf(&output, "fingerprint:%d:%d\n", fingerprint[0], fingerprint[1])
			bag.Close()
		}
		if output.String() != metadata.Bounds[0].Stdout {
			t.Fatalf("Bag StableHash adapted=%t\nGOT\n%sWANT\n%s", adapted, output.String(), metadata.Bounds[0].Stdout)
		}
	}
}
