package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRustSourceCorpusLiterals(t *testing.T) {
	source := `// "not a string"
let a = r"\u{0}";
let b = r##"quoted "# inside"##;
let c = "\\u{0}\n\u{a0}\x41\0";
let d = '"';
`
	want := []string{`\u{0}`, `quoted "# inside`, "\\u{0}\n\u00a0A\x00"}
	if got := rustStrings(source); !reflect.DeepEqual(got, want) {
		t.Fatalf("source strings = %#v, want %#v", got, want)
	}
}

func TestFixturePreservesExistingCompressionAndRejectsBadCRC(t *testing.T) {
	payload := bytes.Repeat([]byte("exact Rust reference"), 100)
	var encoded bytes.Buffer
	writer, err := gzip.NewWriterLevel(&encoded, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	writer.Name = "different Go compressor"
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "reference.bin.gz")
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{true, false} {
		got, err := oracleFixture(path, payload, check)
		if err != nil || !bytes.Equal(got, encoded.Bytes()) {
			t.Fatalf("check=%t rewrote existing exact-payload gzip: %v", check, err)
		}
	}
	corrupt := append([]byte(nil), encoded.Bytes()...)
	corrupt[len(corrupt)-8] ^= 0xff
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := oracleFixture(path, payload, true); err == nil {
		t.Fatal("check accepted an invalid CRC")
	}
	rebuilt, err := oracleFixture(path, payload, false)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(rebuilt))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decoded, payload) {
		t.Fatal("generation did not rebuild the exact payload")
	}
}
