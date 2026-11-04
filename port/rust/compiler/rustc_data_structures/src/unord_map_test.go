// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type unordSpanValue struct {
	value uint64
	read  bool
}

func unordTestRandom(s *uint64) uint64 { *s ^= *s << 13; *s ^= *s >> 7; *s ^= *s << 17; return *s }
func unordTestKeys() rusthashmap.KeyOps[uint64] {
	return rusthashmap.KeyOps[uint64]{Hash: func(v *uint64, h rusthashmap.Hasher, c rusthashmap.Context) { rusthashmap.HashU64(h, *v) }, Equal: func(a, b *uint64, c rusthashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func unordInputSnapshot(m *UnordMap[uint64, unordSpanValue]) string {
	var out strings.Builder
	out.WriteString("UnordBag { inner: [")
	i := m.inner.Iter(rusthashmap.Context{})
	count := 0
	for {
		pair, ok := i.Next(rusthashmap.Context{})
		if !ok {
			break
		}
		if count > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(&out, "(%d, (%d, %t))", *pair.Key, pair.Value.value, pair.Value.read)
		count++
	}
	out.WriteString("] }")
	return out.String()
}
func unordReadSnapshot(m *UnordMap[uint64, uint64]) string {
	var out strings.Builder
	out.WriteString("UnordBag { inner: [")
	i := m.inner.Iter(rusthashmap.Context{})
	count := 0
	for {
		pair, ok := i.Next(rusthashmap.Context{})
		if !ok {
			break
		}
		if count > 0 {
			out.WriteString(", ")
		}
		fmt.Fprintf(&out, "(%d, %d)", *pair.Key, *pair.Value)
		count++
	}
	out.WriteString("] }")
	return out.String()
}

func TestOriginalUnordMapFoundation(t *testing.T) {
	metadata, err := os.ReadFile("../../../../../tools/port-rust-unord/map/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int               `json:"cases"`
		Raw       string            `json:"raw_sha256"`
		Fixture   string            `json:"fixture_sha256"`
		Oracle    string            `json:"oracle_sha256"`
		Generator string            `json:"generator_sha256"`
		Reference map[string]string `json:"reference_tools"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	paths := map[string]string{"../../../../../tools/port-rust-unord/map/oracle.rs": manifest.Oracle, "../../../../../tools/port-rust-unord/map/generate.py": manifest.Generator}
	for name, want := range manifest.Reference {
		paths["../../../../../tools/port-rust-span-reference/"+name] = want
	}
	for path, want := range paths {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("original Unord proof binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/unord_map.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 2000 {
		t.Fatal("original Unord fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	want, err := io.ReadAll(io.LimitReader(reader, 32<<20))
	if err != nil || digest(want) != manifest.Raw {
		t.Fatalf("original Unord body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	config := UnordMapConfig[uint64, unordSpanValue]{Layout: rusthashmap.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: unordTestKeys(), Values: rusthashmap.ValueOps[unordSpanValue]{TrivialClone: true, Equal: func(a, b *unordSpanValue, c rusthashmap.Context) bool { return a.value == b.value && a.read == b.read }}}
	readConfig := UnordMapConfig[uint64, uint64]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: unordTestKeys(), Values: rusthashmap.ValueOps[uint64]{TrivialClone: true, Equal: func(a, b *uint64, c rusthashmap.Context) bool { return *a == *b }}}
	var output strings.Builder
	for seed := uint64(1); seed <= 4; seed++ {
		state := seed
		m := NewUnordMap(config)
		for step := 0; step < 500; step++ {
			x := unordTestRandom(&state)
			key := (x >> 8) % 41
			value := unordTestRandom(&state)
			result, frozen := "", "-"
			switch x % 7 {
			case 0:
				old, ok := m.Insert(key, unordSpanValue{value, false})
				if ok {
					result = fmt.Sprintf("Some((%d, %t))", old.value, old.read)
				} else {
					result = "None"
				}
			case 1:
				_, occupied := m.TryInsert(key, unordSpanValue{value, false})
				if occupied == nil {
					result = "true"
				} else {
					result = fmt.Sprint(occupied.Entry.Get(rusthashmap.Context{}).value == value)
					occupied.Close(rusthashmap.Context{})
				}
			case 2:
				v, ok := m.GetMut(&key)
				if ok {
					v.read = true
					result = fmt.Sprintf("Some(%d)", v.value)
				} else {
					result = "None"
				}
			case 3:
				read := CollectUnordMapFilterMap(m.Items(), func(k *uint64, v *unordSpanValue, c rusthashmap.Context) (rusthashmap.Pair[uint64, uint64], bool) {
					if !v.read {
						return rusthashmap.Pair[uint64, uint64]{}, false
					}
					return rusthashmap.Pair[uint64, uint64]{Key: *k, Value: v.value}, true
				}, readConfig)
				frozen = unordReadSnapshot(read)
				read.Close()
				result = "freeze"
			case 4:
				clone := m.Clone()
				m.CloneFrom(clone)
				clone.Close()
				result = "clone"
			case 5:
				if step%97 == 0 {
					m.Clear()
				}
				result = "clear"
			case 6:
				old, ok := m.Remove(&key)
				if ok {
					result = fmt.Sprintf("Some((%d, %t))", old.value, old.read)
				} else {
					result = "None"
				}
			}
			fmt.Fprintf(&output, "%d\t%d\t%d\t%s\t%s\t%s\n", seed, step, m.Len(), result, unordInputSnapshot(m), frozen)
		}
		m.Close()
	}
	if output.String() != string(want) {
		t.Fatal("actual original Unord operation, bucket order, or filtered-collection result differs")
	}
}
