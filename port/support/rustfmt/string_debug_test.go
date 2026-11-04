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

func TestOriginalStringDebugAllScalars(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	dir := filepath.Join(root, "tools/port-lexer-debug/string")
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
		t.Fatal("str Debug fixture hash mismatch")
	}
	source, err := os.ReadFile(filepath.Join(dir, "oracle.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(source) != manifest.Source || manifest.Rustc != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" {
		t.Fatal("str Debug source/profile mismatch")
	}
	if len(raw) != 20+4*4344*8 || string(raw[:4]) != "RDS1" {
		t.Fatal("invalid str Debug fixture")
	}
	for mode := 0; mode < 2; mode++ {
		for form := 0; form < 2; form++ {
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
				text := string(value)
				if form == 1 {
					text = "A\\\x00" + text + "\n\"'λ"
				}
				charMix(&sink.hash, []byte{'V'})
				charWord(&sink.hash, uint32(value))
				if err := FormatDebugString(text, &formatter); err != nil {
					t.Fatal(err)
				}
				count++
				if count%256 == 0 {
					expected := binary.LittleEndian.Uint64(raw[20+((mode*2+form)*4344+block)*8:])
					if sink.hash != expected {
						t.Fatalf("str Debug C/S trace differs: custom_char=%v form=%d block=%d ending_U+%04X Go=%x Rust=%x", mode == 1, form, block, value, sink.hash, expected)
					}
					sink.hash = charHashOffset
					block++
				}
			}
			if count != 1112064 || block != 4344 {
				t.Fatal("incomplete string scalar coverage")
			}
		}
	}
}
