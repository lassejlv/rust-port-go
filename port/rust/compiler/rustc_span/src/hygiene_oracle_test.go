// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type hygieneOracleRow struct {
	N, Old, Extra, Mode int
	Output              string
}

// The behavioral tracing replay is in the internal test package so it can
// create a fresh isolated registry process. This test binds its fixture and
// complete source set through the same independent proof reader.
func TestHygieneTracingEvidenceBindings(t *testing.T) {
	readHygieneOracle(t, "tracing", func(row hygieneOracleRow) string { return row.Output })
}

func readHygieneOracle(t *testing.T, suite string, replay func(hygieneOracleRow) string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	metadata, err := os.ReadFile(filepath.Join(root, "tools/port-rust-hygiene", suite+"-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases                 int
		UncompressedSHA256    string            `json:"uncompressed_sha256"`
		OracleSHA256          string            `json:"oracle_sha256"`
		GeneratorSHA256       string            `json:"generator_sha256"`
		GoBindings            map[string]string `json:"go_bindings"`
		SupportBindings       map[string]string `json:"support_bindings"`
		OracleSupportBindings map[string]string `json:"oracle_support_bindings"`
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(root, "port/rust/compiler/rustc_span/src/hygiene*.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != len(manifest.GoBindings) {
		t.Fatal("hygiene source binding set changed; regenerate actual reference")
	}
	check := func(name, expected string) {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("hygiene source binding changed:%s; regenerate actual source evidence", name)
		}
	}
	for name, expected := range manifest.GoBindings {
		check(name, expected)
	}
	for name, expected := range manifest.SupportBindings {
		check(name, expected)
	}
	check("tools/port-rust-hygiene/"+suite+".rs", manifest.OracleSHA256)
	for name, expected := range manifest.OracleSupportBindings {
		check(name, expected)
	}
	check("tools/port-rust-hygiene/generate.py", manifest.GeneratorSHA256)
	input, err := os.Open(filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/hygiene_"+suite+".jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	digest := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(reader, digest))
	scanner.Buffer(make([]byte, 65536), 8<<20)
	count := 0
	for scanner.Scan() {
		var row hygieneOracleRow
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		actual := replay(row)
		if actual != row.Output {
			t.Fatalf("native hygiene %s mismatch case%d input=%+v\nexpected:\n%s\nactual:\n%s", suite, count, hygieneOracleRow{N: row.N, Old: row.Old, Extra: row.Extra, Mode: row.Mode}, row.Output, actual)
		}
		count++
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Cases {
		t.Fatalf("hygiene %s cases %d != %d", suite, count, manifest.Cases)
	}
	if hex.EncodeToString(digest.Sum(nil)) != manifest.UncompressedSHA256 {
		t.Fatal("hygiene corpus checksum changed")
	}
}
