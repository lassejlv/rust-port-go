// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type spanProofFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func verifySpanProofFiles(t *testing.T, files []spanProofFile) {
	t.Helper()
	if len(files) == 0 {
		t.Fatal("normal Span proof has no production/replay bindings")
	}
	root := filepath.Join("..", "..", "..", "..", "..")
	for _, file := range files {
		clean := filepath.Clean(filepath.FromSlash(file.Path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			t.Fatalf("invalid Span proof input path %q", file.Path)
		}
		data, err := os.ReadFile(filepath.Join(root, clean))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != file.SHA256 {
			t.Fatalf("Span proof production/replay binding differs: %s", file.Path)
		}
	}
}
