// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type overrideBuilder struct {
	events *[]string
	live   bool
}

func (b *overrideBuilder) Build(c Context) *OwnedHasher {
	c.Panic("BuildHasher must be bypassed")
	return nil
}
func (b *overrideBuilder) Clone(c Context) Builder { return &overrideBuilder{b.events, true} }
func (b *overrideBuilder) CloneFrom(source Builder, c Context) {
	copy := source.Clone(c).(*overrideBuilder)
	b.Close(c)
	*b = *copy
}
func (b *overrideBuilder) Close(c Context) {
	if b.live {
		b.live = false
		*b.events = append(*b.events, "builder_drop")
	}
}

type overrideKey struct{ name string }

func TestOriginalHashOneOverride(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/hash_one_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Raw       string `json:"raw_sha256"`
		Source    string `json:"source_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/hash_one_oracle.rs": manifest.Source, "../../../tools/port-rust-unord/table/hash_one_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("hash_one binding differs:%s:%v", path, err)
		}
	}
	expected, err := os.ReadFile("testdata/hash_one.tsv")
	if err != nil || digest(expected) != manifest.Raw {
		t.Fatalf("hash_one reference differs:%v", err)
	}
	var events []string
	c := Context{}
	one := func(builder Builder) { b := builder.(*overrideBuilder); *b.events = append(*b.events, "one") }
	keys := KeyOps[overrideKey]{Hash: func(*overrideKey, Hasher, Context) { panic("key Hash must be bypassed") }, HashOne: func(b Builder, _ *overrideKey, _ Context) uint64 { one(b); return 0 }, Equal: func(a, b *overrideKey, c Context) bool { return a.name == b.name }, Drop: func(k *overrideKey, c Context) { events = append(events, "drop:"+k.name) }}
	m := NewMap[overrideKey, uint64](Layout{32, 8}, DefaultProfile(), keys, ValueOps[uint64]{TrivialClone: true}, &overrideBuilder{&events, true})
	lookup := BorrowedLookup[overrideKey, string]{Hash: func(*string, Hasher, Context) { panic("borrowed Hash must be bypassed") }, HashOne: func(b Builder, _ *string, _ Context) uint64 { one(b); return 0 }, Equal: func(q *string, k *overrideKey, c Context) bool { return *q == k.name }}
	m.Insert(overrideKey{"a"}, 1, c)
	m.Insert(overrideKey{"b"}, 2, c)
	a := "a"
	if v, ok := GetBorrowed(m, &a, lookup, c); !ok || *v != 1 {
		t.Fatal("borrowed override lookup failed")
	}
	if *m.Entry(overrideKey{"a"}, c).OrInsert(8, c) != 1 {
		t.Fatal("Entry override lookup failed")
	}
	_, occupied := m.TryInsert(overrideKey{"a"}, 9, c)
	if occupied == nil || *occupied.Entry.Get(c) != 1 {
		t.Fatal("TryInsert override lookup failed")
	}
	occupied.Close(c)
	if old, ok := RemoveBorrowed(m, &a, lookup, c); !ok || old != 1 {
		t.Fatal("borrowed override removal failed")
	}
	*m.Entry(overrideKey{"c"}, c).OrInsert(3, c) += 1
	var snapshot strings.Builder
	i := m.Iter(c)
	for {
		pair, ok := i.Next(c)
		if !ok {
			break
		}
		fmt.Fprintf(&snapshot, "%s:%d,", pair.Key.name, *pair.Value)
	}
	m.Close(c)
	actual := snapshot.String() + "\t" + strings.Join(events, "|") + "\n"
	if actual != string(expected) {
		t.Fatalf("hash_one override differs:\ngot%s\nwant%s", actual, expected)
	}
}
