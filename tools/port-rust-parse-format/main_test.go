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
)

func TestRetainGzipAcrossCompressors(t *testing.T) {
	raw := bytes.Repeat([]byte("public parser state\n"), 100)
	var original bytes.Buffer
	writer, err := gzip.NewWriterLevel(&original, gzip.NoCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		retained, err := retainGzip(raw, original.Bytes(), check)
		if err != nil || !bytes.Equal(retained, original.Bytes()) {
			t.Fatalf("valid gzip must be retained byte-for-byte: %v", err)
		}
	}
	before := append([]byte(nil), original.Bytes()...)
	if _, err := retainGzip([]byte("changed"), original.Bytes(), true); err == nil {
		t.Fatal("check accepted changed original results")
	}
	if !bytes.Equal(before, original.Bytes()) {
		t.Fatal("check mutated evidence")
	}
}

func TestExecutableSelection(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "bin")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(directory, "original-tool")
	if err := os.WriteFile(probe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("FERRUGO_PARSE_FORMAT_TEST_TOOL", "original-tool")
	for _, tc := range []struct {
		value    string
		explicit bool
	}{{"", false}, {"original-tool", true}, {"bin/original-tool", true}} {
		got, err := selected(root, "FERRUGO_PARSE_FORMAT_TEST_TOOL", tc.value, "/absent", "absent", tc.explicit)
		if err != nil || got != probe {
			t.Fatalf("selection %+v: %s %v", tc, got, err)
		}
	}
	t.Setenv("FERRUGO_PARSE_FORMAT_TEST_TOOL", "")
	if _, err := selected(root, "FERRUGO_PARSE_FORMAT_TEST_TOOL", "", probe, "absent", false); err == nil {
		t.Fatal("explicit empty override silently fell back")
	}
	if got, err := selected(root, "FERRUGO_PARSE_FORMAT_TEST_TOOL", "original-tool", probe, "absent", true); err != nil || got != probe {
		t.Fatal("explicit flag must override the environment")
	}
	if _, err := selected(root, "FERRUGO_PARSE_FORMAT_TEST_TOOL", "missing-original", probe, "absent", true); err == nil {
		t.Fatal("explicit missing override silently fell back")
	}
}

func TestRegistryArchiveAndExpandedSourceIntegrity(t *testing.T) {
	home := t.TempDir()
	registry := "test-index"
	pkg := "sample-1.0.0"
	packageDir := filepath.Join(home, "registry/src", registry, pkg)
	archivePath := filepath.Join(home, "registry/cache", registry, pkg+".crate")
	if err := os.MkdirAll(filepath.Join(packageDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"Cargo.toml": []byte("[package]\nname='sample'\nversion='1.0.0'\n"), "src/lib.rs": []byte("pub fn value()->u32{7}\n")}
	var archive bytes.Buffer
	compressed := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(compressed)
	for name, data := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: pkg + "/" + name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tarWriter.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(packageDir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, archive.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	metadata, err := json.Marshal(map[string]any{"packages": []any{map[string]any{"name": "sample", "version": "1.0.0", "source": "registry+https://github.com/rust-lang/crates.io-index", "manifest_path": filepath.Join(packageDir, "Cargo.toml")}}})
	if err != nil {
		t.Fatal(err)
	}
	deps := []dependency{{Name: "sample", Version: "1.0.0", Checksum: digest(archive.Bytes())}}
	if err := verifyArchives(metadata, home, deps); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "src/lib.rs"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchives(metadata, home, deps); err == nil || !strings.Contains(err.Error(), "expanded registry source changed") {
		t.Fatalf("expanded source tampering accepted: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchives(metadata, home, deps); err == nil || !strings.Contains(err.Error(), "archive checksum mismatch") {
		t.Fatalf("archive tampering accepted: %v", err)
	}
}
