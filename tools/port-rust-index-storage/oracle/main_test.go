// SPDX-License-Identifier: MIT OR Apache-2.0
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestToolSelectionUsesPATHAndRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "bin")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "original-cargo")
	if err := os.WriteFile(file, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	for _, input := range []string{"original-cargo", "bin/original-cargo", file} {
		got, err := resolveTool(root, input)
		if err != nil {
			t.Fatal(err)
		}
		if got != file {
			t.Fatalf("%q resolved to %q, want %q", input, got, file)
		}
	}
	for _, input := range []string{"", "bin/missing", "bin"} {
		if got, err := resolveTool(root, input); err == nil {
			t.Fatalf("invalid tool %q resolved to %s", input, got)
		}
	}
}

func TestExplicitEmptyToolOverrideFails(t *testing.T) {
	t.Setenv("FERRUGO_STORAGE_TEST_TOOL", "")
	if _, err := selected(t.TempDir(), "FERRUGO_STORAGE_TEST_TOOL", "go"); err == nil {
		t.Fatal("explicit empty tool must fail")
	}
}

func TestMinimumGoVersion(t *testing.T) {
	for _, test := range []struct {
		version string
		ok      bool
	}{{"go version go1.23.12 linux/amd64", true}, {"go version go1.27.3 darwin/arm64", true}, {"go version go1.22.9 linux/amd64", false}, {"go version go2.0 linux/amd64", false}, {"Go: Unknown option: version", false}, {"", false}} {
		if got := supportedGo(test.version); got != test.ok {
			t.Fatalf("supportedGo(%q)=%v", test.version, got)
		}
	}
}
