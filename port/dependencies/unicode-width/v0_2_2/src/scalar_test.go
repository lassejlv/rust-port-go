// SPDX-License-Identifier: MIT OR Apache-2.0
package unicodewidth

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"unicode/utf8"
)

func TestExactOriginalScalarDomain(t *testing.T) {
	metadata, err := os.ReadFile("../source-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ResponseSHA string `json:"response_sha256"`
		FixtureSHA  string `json:"fixture_sha256"`
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/scalars.bin.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("native fixture checksum differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(z, 0x110000+8))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if len(data) != 0x110000+7 || string(data[:4]) != "UWC1" || !bytes.Equal(data[4:7], []byte{17, 0, 0}) {
		t.Fatal("invalid native scalar stream")
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != manifest.ResponseSHA {
		t.Fatal("native scalar stream checksum differs")
	}
	count := 0
	for value := rune(0); value <= utf8.MaxRune; value++ {
		if !utf8.ValidRune(value) {
			continue
		}
		width, present := CharWidth(value)
		actual := byte(0)
		if present {
			actual = byte(width + 1)
		}
		if actual != data[7+int(value)] {
			t.Fatalf("U+%04X: Go(%d,%t), original code%d", value, width, present, data[7+int(value)])
		}
		count++
	}
	if count != 1112064 {
		t.Fatal("incomplete valid-scalar coverage")
	}
	if UnicodeVersion() != [3]uint8{17, 0, 0} {
		t.Fatal("Unicode version differs")
	}
}

func TestInvalidCharIsOutsideRustDomain(t *testing.T) {
	for _, value := range []rune{-1, 0xD800, 0xDFFF, 0x110000} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted invalid Rust char U+%X", value)
				}
			}()
			CharWidth(value)
		}()
	}
}
