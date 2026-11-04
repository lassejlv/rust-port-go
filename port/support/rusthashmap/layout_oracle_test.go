// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestOriginalSetLayouts(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/layout_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/layout_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/layout_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("layout reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/layout.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 4096 {
		t.Fatal("layout reference fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	want, err := io.ReadAll(io.LimitReader(reader, 8<<20))
	if err != nil || digest(want) != manifest.Raw {
		t.Fatalf("layout proof body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	layouts := []Layout{{1, 1}, {2, 2}, {4, 4}, {8, 8}, {32, 32}, {0, 1}, {0, 32}, {1, 1}}
	var output strings.Builder
	c := Context{}
	for kind, layout := range layouts {
		keys := KeyOps[uint64]{Hash: func(v *uint64, h Hasher, c Context) {
			switch kind {
			case 0, 4, 7:
				HashU8(h, uint8(*v))
			case 1:
				HashU16(h, uint16(*v))
			case 2:
				HashU32(h, uint32(*v))
			case 3:
				HashU64(h, *v)
			case 5, 6:
			}
		}, Equal: func(a, b *uint64, c Context) bool { return *a == *b }, TrivialClone: true}
		set := NewSet(layout, DefaultProfile(), keys, FxBuilder{64})
		for step := uint64(0); step < 512; step++ {
			key := step * 31 % 256
			if kind == 5 || kind == 6 {
				key = 0
			}
			if kind == 7 {
				key %= 2
			}
			result := ""
			switch step % 7 {
			case 0, 1, 2:
				result = strconv.FormatBool(set.Insert(key, c))
			case 3:
				result = strconv.FormatBool(set.Remove(&key, c))
			case 4:
				result = strconv.FormatBool(set.Contains(&key, c))
			case 5:
				next := set.Clone(c)
				set.Close(c)
				set = next
				result = "clone"
			case 6:
				if step%63 == 0 {
					set.Clear(c)
				}
				result = "clear"
			}
			fmt.Fprintf(&output, "%d\t%d\t%d\t%d\t%s\t", kind, step, set.Len(), set.Capacity(), result)
			iterator := set.Iter(c)
			for {
				value, ok := iterator.Next(c)
				if !ok {
					break
				}
				fmt.Fprintf(&output, "%d,", *value)
			}
			fmt.Fprintln(&output)
		}
		set.Close(c)
	}
	if output.String() != string(want) {
		t.Fatal("narrow/aligned/ZST set state, layout capacity, or iteration order differs")
	}
}
