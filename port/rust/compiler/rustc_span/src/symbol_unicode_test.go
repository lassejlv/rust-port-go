// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode/utf8"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type symbolUnicodeTrace struct {
	count uint32
	data  []byte
}

func (trace *symbolUnicodeTrace) Write(text []byte) (int, error) {
	trace.writeString(string(text))
	return len(text), nil
}
func (trace *symbolUnicodeTrace) WriteString(text string) (int, error) {
	trace.writeString(text)
	return len(text), nil
}
func (trace *symbolUnicodeTrace) writeString(text string) {
	trace.count++
	trace.data = append(trace.data, 'S')
	trace.data = binary.LittleEndian.AppendUint32(trace.data, uint32(len(text)))
	trace.data = append(trace.data, text...)
}
func (trace *symbolUnicodeTrace) WriteChar(c rune) error {
	trace.count++
	trace.data = append(trace.data, 'C')
	trace.data = binary.LittleEndian.AppendUint32(trace.data, uint32(c))
	return nil
}
func symbolUnicodeWriteU32(h hash.Hash, value uint32) {
	var bytes [4]byte
	binary.LittleEndian.PutUint32(bytes[:], value)
	h.Write(bytes[:])
}
func symbolUnicodeWriteString(h hash.Hash, text string) {
	symbolUnicodeWriteU32(h, uint32(len(text)))
	h.Write([]byte(text))
}
func (trace *symbolUnicodeTrace) finish(h hash.Hash) {
	symbolUnicodeWriteU32(h, trace.count)
	h.Write(trace.data)
	trace.count = 0
	trace.data = trace.data[:0]
}

func TestSymbolPinnedUnicode18AllScalars(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "symbol_unicode18_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		UnicodeVersion []int `json:"unicode_version"`
		Scalars        int   `json:"scalar_values"`
		Blocks         []struct {
			Start   uint32 `json:"start"`
			Scalars int    `json:"scalars"`
			Bytes   int    `json:"bytes"`
			SHA256  string `json:"sha256"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.UnicodeVersion) != 3 || metadata.UnicodeVersion[0] != 18 || metadata.UnicodeVersion[1] != 0 || metadata.UnicodeVersion[2] != 0 {
		t.Fatal("wrong native Unicode profile")
	}
	if len(metadata.Blocks) != 272 || metadata.Scalars != 1112064 {
		t.Fatal("wrong native scalar/block count")
	}
	trace := symbolUnicodeTrace{data: make([]byte, 0, 128)}
	f := rustfmt.Formatter{Options: rustfmt.DefaultOptions(), Writer: &trace}
	total := 0
	for _, block := range metadata.Blocks {
		h := sha256.New()
		count := 0
		for point := block.Start; point < block.Start+4096; point++ {
			c := rune(point)
			if !utf8.ValidRune(c) {
				continue
			}
			count++
			symbolUnicodeWriteU32(h, point)
			if err := FormatDebugCharPinnedUnicode18(c, &f); err != nil {
				t.Fatal(err)
			}
			trace.finish(h)
			text := string(c)
			if err := FormatDebugStringPinnedUnicode18(text, &f); err != nil {
				t.Fatal(err)
			}
			trace.finish(h)
			for flags := 0; flags < 4; flags++ {
				symbolUnicodeWriteString(h, EscapeDebugPinnedUnicode18(c, flags&1 != 0, flags&2 != 0))
			}
			symbolUnicodeWriteString(h, symbolLowercase(text))
			symbolUnicodeWriteString(h, symbolUppercase(text))
			symbolUnicodeWriteString(h, symbolLowercase("A"+text+"Σ"))
			symbolUnicodeWriteString(h, symbolLowercase("AΣ"+text))
		}
		if count != block.Scalars {
			t.Fatalf("block %#x scalar count %d, want %d", block.Start, count, block.Scalars)
		}
		if actual := hex.EncodeToString(h.Sum(nil)); actual != block.SHA256 {
			t.Fatalf("block %#x: source18 writer/casing digest %s, want %s", block.Start, actual, block.SHA256)
		}
		total += count
	}
	if total != metadata.Scalars {
		t.Fatalf("scalars %d, want %d", total, metadata.Scalars)
	}
}
