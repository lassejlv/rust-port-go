// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"hash"
	"os"
	"testing"
	"unicode/utf8"
)

type unicode18Trace struct {
	calls uint32
	data  []byte
}

func (t *unicode18Trace) Write(p []byte) (int, error) { t.WriteString(string(p)); return len(p), nil }
func (t *unicode18Trace) WriteString(s string) (int, error) {
	t.calls++
	t.data = append(t.data, 'S')
	t.data = binary.LittleEndian.AppendUint32(t.data, uint32(len(s)))
	t.data = append(t.data, s...)
	return len(s), nil
}
func (t *unicode18Trace) WriteChar(c rune) error {
	t.calls++
	t.data = append(t.data, 'C')
	t.data = binary.LittleEndian.AppendUint32(t.data, uint32(c))
	return nil
}
func unicode18Number(h hash.Hash, n uint32) {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], n)
	h.Write(data[:])
}
func (t *unicode18Trace) finish(h hash.Hash) {
	unicode18Number(h, t.calls)
	h.Write(t.data)
	t.calls = 0
	t.data = t.data[:0]
}
func TestUnicode18OriginalScalarFormatting(t *testing.T) {
	data, err := os.ReadFile("testdata/unicode18-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Scalars int   `json:"scalar_values"`
		Unicode []int `json:"unicode_version"`
		Blocks  []struct {
			Start   uint32 `json:"start"`
			Scalars int    `json:"scalars"`
			Bytes   int    `json:"bytes"`
			SHA     string `json:"sha256"`
		}
	}
	if err = json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Scalars != 1112064 || len(proof.Blocks) != 272 || len(proof.Unicode) != 3 || proof.Unicode[0] != 18 || proof.Unicode[1] != 0 || proof.Unicode[2] != 0 {
		t.Fatal("wrong scalar/Unicode18 profile")
	}
	trace := unicode18Trace{data: make([]byte, 0, 128)}
	formatter := rf.Formatter{Writer: &trace, Options: rf.DefaultOptions()}
	total := 0
	for _, block := range proof.Blocks {
		h := sha256.New()
		count := 0
		for point := block.Start; point < block.Start+4096; point++ {
			c := rune(point)
			if !utf8.ValidRune(c) {
				continue
			}
			count++
			unicode18Number(h, point)
			if err = rf.FormatDebugCharUnicode18(c, &formatter); err != nil {
				t.Fatal(err)
			}
			trace.finish(h)
			if err = rf.FormatDebugStringUnicode18(string(c), &formatter); err != nil {
				t.Fatal(err)
			}
			trace.finish(h)
			escaped := rf.EscapeDebugUnicode18(c, true, true)
			unicode18Number(h, uint32(len(escaped)))
			h.Write([]byte(escaped))
		}
		if count != block.Scalars || hex.EncodeToString(h.Sum(nil)) != block.SHA {
			t.Fatalf("native Unicode18 writer/escape block %#x differs", block.Start)
		}
		total += count
	}
	if total != proof.Scalars {
		t.Fatal("incomplete Unicode18 scalar replay")
	}
}
