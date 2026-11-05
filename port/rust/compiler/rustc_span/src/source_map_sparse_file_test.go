//go:build linux

// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSourceMapRealLoaderOversizedSparseFileNativeOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/source_map_sparse_file.tsv")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input")
	count := 0
	for _, row := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		p := strings.Split(row, "\t")
		if len(p) != 4 {
			t.Fatal("invalid sparse-file fixture")
		}
		length, err := strconv.ParseInt(p[0], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(length); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		loader := RealFileLoader{}
		nativePath := NewRustPath(path)
		text, failure := loader.ReadFile(nativePath)
		if failure == nil || text != "" {
			t.Fatal("oversized file was not rejected before reading")
		}
		debug := rustfmt.String(func(f *rustfmt.Formatter) error { return FormatSourceIOErrorDebug(failure, f) })
		display := failure.Error()
		CloseSourceIOError(failure, rustfmt.Context{})
		if strconv.FormatBool(loader.FileExists(nativePath)) != p[1] || debug != sourceMapOracleString(p[2]) || display != sourceMapOracleString(p[3]) {
			t.Fatalf("length%d: %q/%q", length, debug, display)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("native sparse-file records %d", count)
	}
}
