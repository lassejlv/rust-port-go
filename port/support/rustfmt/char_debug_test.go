// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode/utf8"
)

const charHashOffset uint64 = 14695981039346656037

func charMix(hash *uint64, data []byte) {
	for _, b := range data {
		*hash = (*hash ^ uint64(b)) * 1099511628211
	}
}
func charWord(hash *uint64, value uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], value)
	charMix(hash, b[:])
}

type charTraceSink struct{ hash uint64 }

func (s *charTraceSink) Write(b []byte) (int, error) { return s.WriteString(string(b)) }
func (s *charTraceSink) WriteString(text string) (int, error) {
	charMix(&s.hash, []byte{'S'})
	charWord(&s.hash, uint32(len(text)))
	charMix(&s.hash, []byte(text))
	return len(text), nil
}

type charIndependentSink struct{ *charTraceSink }

func (s charIndependentSink) WriteChar(c rune) error {
	charMix(&s.hash, []byte{'C'})
	charWord(&s.hash, uint32(c))
	return nil
}
func TestOriginalCharDebugAllScalars(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	dir := filepath.Join(root, "tools/port-lexer-debug/char")
	encoded, err := os.ReadFile(filepath.Join(dir, "oracle.bin.gz"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	var manifest struct {
		Rustc                   string `json:"rustc"`
		Raw, Compressed, Source string
	}
	body, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { hash := sha256.Sum256(data); return hex.EncodeToString(hash[:]) }
	if digest(raw) != manifest.Raw || digest(encoded) != manifest.Compressed {
		t.Fatal("char Debug fixture hash mismatch")
	}
	source, err := os.ReadFile(filepath.Join(dir, "oracle.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(source) != manifest.Source || manifest.Rustc != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" {
		t.Fatal("char Debug source/profile mismatch")
	}
	if len(raw) != 20+0x110000+2*4344*8 || string(raw[:4]) != "RDC1" {
		t.Fatal("invalid char Debug fixture")
	}
	flags := raw[20 : 20+0x110000]
	for mode := 0; mode < 2; mode++ {
		sink := &charTraceSink{charHashOffset}
		var writer io.Writer = sink
		if mode == 1 {
			writer = charIndependentSink{sink}
		}
		formatter := Formatter{Options: DefaultOptions(), Writer: writer}
		count, block := 0, 0
		for value := rune(0); value <= 0x10ffff; value++ {
			if !utf8.ValidRune(value) {
				continue
			}
			if mode == 0 && charUnicodeEscape(value) != (flags[value] != 0) {
				t.Fatalf("Unicode escape classification differs at U+%04X", value)
			}
			charMix(&sink.hash, []byte{'V'})
			charWord(&sink.hash, uint32(value))
			if err := FormatDebugChar(value, &formatter); err != nil {
				t.Fatal(err)
			}
			count++
			if count%256 == 0 {
				expected := binary.LittleEndian.Uint64(raw[20+0x110000+(mode*4344+block)*8:])
				if sink.hash != expected {
					t.Fatalf("char Debug C/S trace differs: custom_char=%v block=%d ending_U+%04X Go=%x Rust=%x", mode == 1, block, value, sink.hash, expected)
				}
				sink.hash = charHashOffset
				block++
			}
		}
		if count != 1112064 || block != 4344 {
			t.Fatalf("incomplete scalar coverage: %d/%d", count, block)
		}
	}
}
