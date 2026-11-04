// SPDX-License-Identifier: MIT OR Apache-2.0
package validation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CheckEvidence binds the replay and production adapters to their retained
// original-source observations without requiring Rust or a source cache.
func CheckEvidence(t *testing.T, directory, stem string) {
	t.Helper()
	data, e := os.ReadFile(filepath.Join(directory, stem+"-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var info struct {
		Revision, Rustc string
		Bound           map[string]string `json:"bound_sha256"`
	}
	if e = json.Unmarshal(data, &info); e != nil {
		t.Fatal(e)
	}
	if info.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || info.Rustc != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" || len(info.Bound) == 0 {
		t.Fatal("invalid original contract provenance")
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	for name, want := range info.Bound {
		data, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatalf("original evidence input changed:%s; rerun original contract oracle", name)
		}
	}
}
