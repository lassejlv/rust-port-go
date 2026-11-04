// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
	"testing"
)

type proofWriter struct {
	t            *testing.T
	block, whole hash.Hash
	lines        int
	expected     []string
}

func (w *proofWriter) Write(p []byte) (int, error) { w.block.Write(p); return w.whole.Write(p) }
func (w *proofWriter) endLine() {
	w.lines++
	if w.lines%64 == 0 {
		index := w.lines/64 - 1
		actual := "64\t" + hex.EncodeToString(w.block.Sum(nil))
		if index >= len(w.expected) || actual != w.expected[index] {
			w.t.Fatalf("original table differs at block%d (ending operation%d): got%s", index, w.lines, actual)
		}
		w.block.Reset()
	}
}
func tableRandom(state *uint64) uint64 {
	*state ^= *state << 13
	*state ^= *state >> 7
	*state ^= *state << 17
	return *state
}
func primitiveMap(capacity uint) *Map[uint64, uint64] {
	keys := KeyOps[uint64]{Hash: func(v *uint64, h Hasher, _ Context) { HashU64(h, *v) }, Equal: func(a, b *uint64, _ Context) bool { return *a == *b }, TrivialClone: true}
	values := ValueOps[uint64]{TrivialClone: true, Default: func(Context) uint64 { return 0 }}
	return NewMapWithCapacity(capacity, Layout{16, 8}, DefaultProfile(), keys, values, FxBuilder{64}, Context{})
}
func TestOriginalHashMapTable(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Blocks    int    `json:"blocks"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"fixture_raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Profiles  []struct {
			Operations int    `json:"operations"`
			Raw        string `json:"raw_output_sha256"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/generate.py": manifest.Generator} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("original source binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/table_blocks.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Blocks != 1500 || len(manifest.Profiles) != 2 {
		t.Fatal("original proof fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 1<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("original proof body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	expected := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(expected) != manifest.Blocks {
		t.Fatal("original proof block count differs")
	}
	w := &proofWriter{t: t, block: sha256.New(), whole: sha256.New(), expected: expected}
	c := Context{}
	for seed := uint64(1); seed <= 24; seed++ {
		rng := seed
		m := primitiveMap(0)
		for step := 0; step < 4000; step++ {
			x := tableRandom(&rng)
			key := (x >> 8) % 257
			value := tableRandom(&rng)
			result := ""
			switch x % 17 {
			case 0, 1, 2, 3, 4:
				old, ok := m.Insert(key, value, c)
				if ok {
					result = fmt.Sprintf("Some(%d)", old)
				} else {
					result = "None"
				}
			case 5, 6, 7:
				old, ok := m.Remove(&key, c)
				if ok {
					result = fmt.Sprintf("Some(%d)", old)
				} else {
					result = "None"
				}
			case 8:
				old, ok := m.Get(&key, c)
				if ok {
					result = fmt.Sprintf("Some(%d)", *old)
				} else {
					result = "None"
				}
			case 9:
				next := m.Clone(c)
				m.Close(c)
				m = next
			case 10:
				if step%997 == 0 {
					m.Clear(c)
				}
			case 11:
				result = fmt.Sprint(*m.Entry(key, c).OrInsert(value, c))
			case 12:
				result = fmt.Sprint(*m.Entry(key, c).AndModify(func(v *uint64) { *v += value }, c).OrDefault(c))
			case 13:
				newValue, occupied := m.TryInsert(key, value, c)
				if occupied == nil {
					result = fmt.Sprintf("Ok(%d)", *newValue)
				} else {
					result = fmt.Sprintf("Err(%d,%d,%d)", *occupied.Entry.Get(c), *occupied.Key.Borrow(), *occupied.Value.Borrow())
					occupied.Close(c)
				}
			case 14:
				entry := m.Entry(key, c).InsertEntry(value, c)
				result = fmt.Sprintf("%d:%d", *entry.Key(c), *entry.Get(c))
				entry.Close(c)
			case 15:
				entry := m.Entry(key, c)
				if occupied, ok := entry.TakeOccupied(c); ok {
					pair := occupied.RemoveEntry(c)
					result = fmt.Sprintf("(%d, %d)", pair.Key, pair.Value)
				} else {
					vacant, _ := entry.TakeVacant(c)
					result = fmt.Sprintf("Key(%d)", vacant.IntoKey(c))
				}
			case 16:
				dest := primitiveMap(500)
				dest.CloneFrom(m, c)
				m.Close(c)
				m = dest
			}
			fmt.Fprintf(w, "%d %d %d %d %s ", seed, step, m.Len(), m.Capacity(), result)
			iterator := m.Iter(c)
			for {
				pair, ok := iterator.Next(c)
				if !ok {
					break
				}
				fmt.Fprintf(w, "%d:%d,", *pair.Key, *pair.Value)
			}
			fmt.Fprintln(w)
			w.endLine()
		}
		m.Close(c)
	}
	for _, profile := range manifest.Profiles {
		if profile.Operations != 96000 || hex.EncodeToString(w.whole.Sum(nil)) != profile.Raw {
			t.Fatal("original profile output/coverage differs")
		}
	}
}
