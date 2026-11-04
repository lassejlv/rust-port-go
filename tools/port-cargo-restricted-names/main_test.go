package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolSelectionAndDirectoryPrecedence(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "reference-tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RESTRICTED_NAMES_TEST_TOOL", "reference-tool")
	for _, value := range []string{"reference-tool", "./reference-tool", bin} {
		got, err := selected(root, "RESTRICTED_NAMES_TEST_TOOL", value, "absent", "absent", true)
		if err != nil || got != bin {
			t.Fatalf("%q: %q %v", value, got, err)
		}
	}
	if got, err := selected(root, "RESTRICTED_NAMES_TEST_TOOL", "", "absent", "absent", false); err != nil || got != bin {
		t.Fatalf("environment/PATH selection: %q %v", got, err)
	}
	for _, value := range []string{"", "definitely-missing-reference"} {
		if _, err := selected(root, "RESTRICTED_NAMES_TEST_TOOL", value, bin, bin, true); err == nil {
			t.Fatalf("explicit invalid %q fell back", value)
		}
	}
	t.Setenv("RESTRICTED_NAMES_TEST_TOOL", "")
	if _, err := selected(root, "RESTRICTED_NAMES_TEST_TOOL", "", bin, bin, false); err == nil {
		t.Fatal("empty environment fell back")
	}
	t.Setenv("RESTRICTED_NAMES_TEST_DIR", "cache")
	if got, err := selectedDirectory(root, "RESTRICTED_NAMES_TEST_DIR", "", "fallback", false); err != nil || got != filepath.Join(root, "cache") {
		t.Fatalf("directory env: %q %v", got, err)
	}
	if got, err := selectedDirectory(root, "RESTRICTED_NAMES_TEST_DIR", "explicit", "fallback", true); err != nil || got != filepath.Join(root, "explicit") {
		t.Fatalf("directory flag precedence: %q %v", got, err)
	}
	if _, err := selectedDirectory(root, "RESTRICTED_NAMES_TEST_DIR", "", "fallback", true); err == nil {
		t.Fatal("explicit empty directory fell back")
	}
	t.Setenv("RESTRICTED_NAMES_TEST_DIR", "")
	if _, err := selectedDirectory(root, "RESTRICTED_NAMES_TEST_DIR", "", "fallback", false); err == nil {
		t.Fatal("empty directory environment fell back")
	}
}

func TestRetainGzipAcrossGoVersionsAndRejectCorruption(t *testing.T) {
	raw := bytes.Repeat([]byte("retained Unicode\x00fixture雪\n"), 100)
	var original bytes.Buffer
	w, _ := gzip.NewWriterLevel(&original, gzip.BestSpeed)
	w.Name, w.Comment, w.ModTime = "recorded", "another Go compressor", time.Unix(123456789, 0)
	w.Write(raw)
	w.Close()
	got, err := retainGzip(raw, original.Bytes(), true)
	if err != nil || !bytes.Equal(got, original.Bytes()) {
		t.Fatalf("valid existing stream rewritten: %v", err)
	}
	corrupt := append([]byte(nil), original.Bytes()...)
	corrupt[len(corrupt)-8] ^= 1
	for _, invalid := range [][]byte{nil, []byte("invalid"), corrupt, original.Bytes()[:original.Len()-1]} {
		if _, err := retainGzip(raw, invalid, true); err == nil {
			t.Fatal("invalid gzip accepted in check mode")
		}
		if regenerated, err := retainGzip(raw, invalid, false); err != nil || len(regenerated) == 0 {
			t.Fatalf("regeneration failed: %v", err)
		}
	}
	if _, err := retainGzip(append(raw, '!'), original.Bytes(), true); err == nil {
		t.Fatal("different raw oracle accepted")
	}
}

func TestVerifyRegistryArchivesAndExpandedSource(t *testing.T) {
	home := t.TempDir()
	packageDir := filepath.Join(home, "registry", "src", "registry-name", "example-1.2.3")
	cache := filepath.Join(home, "registry", "cache", "registry-name")
	if err := os.MkdirAll(packageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("original dependency bytes")
	if err := os.WriteFile(filepath.Join(packageDir, "Cargo.toml"), original, 0600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	z := gzip.NewWriter(&archive)
	tw := tar.NewWriter(z)
	tw.WriteHeader(&tar.Header{Name: "example-1.2.3/Cargo.toml", Mode: 0600, Size: int64(len(original))})
	tw.Write(original)
	tw.Close()
	z.Close()
	path := filepath.Join(cache, "example-1.2.3.crate")
	if err := os.WriteFile(path, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(map[string]any{"packages": []any{map[string]string{"name": "example", "version": "1.2.3", "source": "registry+https://github.com/rust-lang/crates.io-index", "manifest_path": filepath.Join(packageDir, "Cargo.toml")}}})
	deps := []dependency{{"example", "1.2.3", digest(archive.Bytes())}}
	if err := verifyArchives(metadata, home, deps); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "Cargo.toml"), []byte("edited dependency"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchives(metadata, home, deps); err == nil || !strings.Contains(err.Error(), "expanded registry source changed") {
		t.Fatalf("edited dependency accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "Cargo.toml"), original, 0600); err != nil {
		t.Fatal(err)
	}
	deps[0].Checksum = strings.Repeat("0", 64)
	if err := verifyArchives(metadata, home, deps); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("wrong archive checksum accepted: %v", err)
	}
	if err := verifyArchives([]byte(`{"packages":[]}`), home, deps); err == nil {
		t.Fatal("missing selected registry package accepted")
	}
}

func TestDeterministicOriginalRequests(t *testing.T) {
	a, ac := requests()
	b, bc := requests()
	if ac != 32779 || ac != bc || !bytes.Equal(a, b) {
		t.Fatalf("request stream changed: %d %d", ac, bc)
	}
}
