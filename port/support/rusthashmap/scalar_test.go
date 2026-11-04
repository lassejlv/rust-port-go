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

type directHasher struct{ value uint64 }

func (h *directHasher) Write([]byte)          { panic("unexpected byte Hash") }
func (h *directHasher) WriteU64(value uint64) { h.value = value }
func (h *directHasher) Finish64() uint64      { return h.value }

type directBuilder struct{}

func (directBuilder) Build(Context) *OwnedHasher { return NewOwnedHasher(&directHasher{}, nil) }
func (b directBuilder) Clone(Context) Builder    { return b }
func (directBuilder) CloneFrom(Builder, Context) {}
func (directBuilder) Close(Context)              {}

type scalarKey struct{ identity, hash uint64 }

func TestOriginalScalarGroupFalsePositives(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/scalar_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Raw       string `json:"raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/scalar_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/scalar_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("scalar reference binding differs:%s:%v", path, err)
		}
	}
	want, err := os.ReadFile("testdata/scalar.tsv")
	if err != nil || digest(want) != manifest.Raw {
		t.Fatalf("scalar reference differs:%v", err)
	}
	var output strings.Builder
	for _, tags := range [][4]uint64{{0, 1, 0, 1}, {1, 0, 1, 0}, {127, 126, 127, 126}} {
		var events []string
		keys := KeyOps[scalarKey]{Hash: func(k *scalarKey, h Hasher, c Context) { HashU64(h, k.hash) }, Equal: func(a, b *scalarKey, c Context) bool {
			events = append(events, fmt.Sprintf("%d:%d", a.identity, b.identity))
			return a.identity == b.identity
		}}
		m := NewMap[scalarKey, uint64](Layout{24, 8}, Profile{64, 8}, keys, ValueOps[uint64]{TrivialClone: true}, directBuilder{})
		for i := uint64(0); i < 4; i++ {
			m.Insert(scalarKey{i + 1, tags[i]<<57 | i}, i, Context{})
		}
		events = nil
		query := scalarKey{99, tags[0] << 57}
		if _, ok := m.Get(&query, Context{}); ok {
			t.Fatal("false-positive tag comparison became false equality")
		}
		fmt.Fprintf(&output, "%d\t%s\n", m.Capacity(), strings.Join(events, "|"))
		m.Close(Context{})
	}
	if output.String() != string(want) {
		t.Fatalf("scalar false-positive traces differ:\ngot%s\nwant%s", output.String(), want)
	}
}
