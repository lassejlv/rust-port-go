package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestFixturePreservesExistingValidGzip(t *testing.T) {
	raw := bytes.Repeat([]byte("original Rust output with a different valid compression\n"), 64)
	var encoded bytes.Buffer
	writer, err := gzip.NewWriterLevel(&encoded, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "oracle.tsv.gz")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		got, err := fixtureEncoding(path, raw, check)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, encoded.Bytes()) {
			t.Fatal("existing valid gzip representation changed")
		}
	}
	if _, err := fixtureEncoding(path, append(raw, '!'), true); err == nil {
		t.Fatal("check accepted changed original output")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, encoded.Bytes()) {
		t.Fatal("check rewrote the retained fixture")
	}
}
