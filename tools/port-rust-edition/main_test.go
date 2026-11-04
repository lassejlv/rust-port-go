package main

import (
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestEditionFixturePreservesEquivalentGzip(t *testing.T) {
	raw := bytes.Repeat([]byte("unchanged original edition oracle output\n"), 32)
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "edition.gz")
	if err := os.WriteFile(path, compressed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		got, err := editionFixture(path, raw, check)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, compressed.Bytes()) {
			t.Fatal("existing valid gzip bytes changed")
		}
	}
	if _, err := editionFixture(path, append(raw, '!'), true); err == nil {
		t.Fatal("check accepted changed original output")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, compressed.Bytes()) {
		t.Fatal("check rewrote retained fixture")
	}
}
