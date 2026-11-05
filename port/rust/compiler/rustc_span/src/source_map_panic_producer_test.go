// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const sourceMapPanicProofEnv = "FERRUGO_SOURCE_MAP_PANIC_PROOF"
const sourceMapPanicProofLimit = 2 << 20

type sourceMapPanicEvidence struct {
	Fixture, Manifest, Raw, Scope string
	Explicit                      bool
}
type sourceMapPanicProducerManifest struct {
	Schema         int
	Kind           string `json:"artifact_kind"`
	Scope          string `json:"scope_id"`
	Revision       string `json:"source_revision"`
	SourceSHA      string `json:"source_sha256"`
	WrapperSHA     string `json:"wrapper_sha256"`
	Records        int
	Panics         int `json:"caught_panics"`
	Profiles       []string
	RawSHA         string `json:"raw_sha256"`
	GzipSHA        string `json:"gzip_sha256"`
	Files          map[string]string
	NativeProfiles []struct {
		Name       string
		Provenance string `json:"reference_provenance_sha256"`
		Version    string `json:"rustc_version"`
	} `json:"native_profiles"`
	ScopedTLS rustfmt.SourceLocation `json:"scoped_tls_location"`
	Assertion rustfmt.SourceLocation `json:"assert_location"`
}

func sourceMapPanicEvidencePaths(path string, lookup func(string) (string, bool)) (sourceMapPanicEvidence, error) {
	s := sourceMapPanicEvidence{Fixture: path}
	switch filepath.Base(path) {
	case "source_map_panic_session.tsv.gz":
		s.Scope = "session"
	case "source_map_panic_assert.tsv.gz":
		s.Scope = "assert"
	default:
		return s, nil
	}
	directory := filepath.Dir(path)
	if value, present := lookup(sourceMapPanicProofEnv); present {
		if value == "" {
			return s, fmt.Errorf("%s explicitly empty", sourceMapPanicProofEnv)
		}
		var err error
		directory, err = filepath.Abs(value)
		if err != nil {
			return s, err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return s, fmt.Errorf("invalid explicit SourceMap panic producer directory %q", directory)
		}
		s.Explicit = true
	}
	stem := "source_map_panic_" + s.Scope
	s.Fixture = filepath.Join(directory, stem+".tsv.gz")
	s.Manifest = filepath.Join(directory, stem+"_manifest.json")
	if s.Explicit {
		s.Raw = filepath.Join(directory, stem+".tsv")
	}
	for _, name := range []string{s.Fixture, s.Manifest, s.Raw} {
		if name == "" {
			continue
		}
		if _, err := sourceMapPanicReadBounded(name); err != nil {
			return s, err
		}
	}
	return s, nil
}
func sourceMapPanicReadBounded(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > sourceMapPanicProofLimit {
		return nil, fmt.Errorf("selected SourceMap panic evidence is not a bounded regular file: %s", path)
	}
	return os.ReadFile(path)
}
func sourceMapPanicProofSHA(value []byte) string { return fmt.Sprintf("%x", sha256.Sum256(value)) }
func sourceMapPanicValidSHA(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size
}
func sourceMapPanicProfile(s sourceMapPanicEvidence, expectedSHA string) (sourceMapPanicProducerManifest, error) {
	var m sourceMapPanicProducerManifest
	body, err := sourceMapPanicReadBounded(s.Manifest)
	if err != nil {
		return m, err
	}
	if expectedSHA != "" && sourceMapPanicProofSHA(body) != expectedSHA {
		return m, fmt.Errorf("SourceMap panic producer manifest differs from selected SHA")
	}
	if err = json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	panics := 4
	if s.Scope == "assert" {
		panics = 2
	}
	if m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.SourceSHA != "9fa60e859a71b59b6787028a569a17d9fa0b5f047af9bb12de14e9c34af6eeb3" || m.Records != 4 || m.Panics != panics || strings.Join(m.Profiles, ",") != "d0o0,d0o1,d1o0,d1o1" || !sourceMapPanicValidSHA(m.RawSHA) || !sourceMapPanicValidSHA(m.GzipSHA) || !sourceMapPanicValidSHA(m.WrapperSHA) {
		return m, fmt.Errorf("invalid SourceMap panic producer profile")
	}
	if s.Explicit {
		if m.Schema != 2 || m.Kind != "source-map-panic-producer-v1" || m.Scope != s.Scope || len(m.Files) == 0 || len(m.NativeProfiles) != 4 {
			return m, fmt.Errorf("explicit SourceMap panic evidence is not a fresh producer bundle")
		}
		for i, p := range m.NativeProfiles {
			if p.Name != m.Profiles[i] || !sourceMapPanicValidSHA(p.Provenance) || !strings.HasPrefix(p.Version, "rustc 1.101.0-nightly") {
				return m, fmt.Errorf("invalid SourceMap panic native compiler profile")
			}
		}
		root := filepath.Join("..", "..", "..", "..", "..")
		for path, expected := range m.Files {
			clean := filepath.Clean(filepath.FromSlash(path))
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || !sourceMapPanicValidSHA(expected) {
				return m, fmt.Errorf("invalid SourceMap panic input binding")
			}
			value, err := sourceMapPanicReadBounded(filepath.Join(root, clean))
			if err != nil {
				return m, err
			}
			if sourceMapPanicProofSHA(value) != expected {
				return m, fmt.Errorf("SourceMap panic producer input changed: %s", path)
			}
		}
		for _, path := range []string{"tools/port-rust-source-map/generate_panic_sites.py", "tools/port-rust-source-map/panic_site_producer.py", "tools/port-rust-source-map/panic_sites.rs", "port/rust/compiler/rustc_span/src/source_map_panic_sites_test.go", "port/rust/compiler/rustc_span/src/source_map_panic_producer_test.go"} {
			if _, ok := m.Files[path]; !ok {
				return m, fmt.Errorf("missing SourceMap panic input binding: %s", path)
			}
		}
	} else if m.Schema != 1 {
		return m, fmt.Errorf("unsupported historical SourceMap panic schema")
	}
	location := m.ScopedTLS
	if s.Scope == "assert" {
		location = m.Assertion
	}
	if location.File == "" || !utf8.ValidString(location.File) || strings.ContainsRune(location.File, 0) || location.Line == 0 || location.Column == 0 {
		return m, fmt.Errorf("invalid explicit native SourceMap panic location")
	}
	compressed, err := sourceMapPanicReadBounded(s.Fixture)
	if err != nil {
		return m, err
	}
	if sourceMapPanicProofSHA(compressed) != m.GzipSHA {
		return m, fmt.Errorf("SourceMap panic compressed digest mismatch")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return m, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(z, sourceMapPanicProofLimit+1))
	closeErr := z.Close()
	if readErr != nil {
		return m, readErr
	}
	if closeErr != nil {
		return m, closeErr
	}
	if len(raw) > sourceMapPanicProofLimit || sourceMapPanicProofSHA(raw) != m.RawSHA {
		return m, fmt.Errorf("SourceMap panic raw digest mismatch")
	}
	if s.Explicit {
		selected, err := sourceMapPanicReadBounded(s.Raw)
		if err != nil {
			return m, err
		}
		if !bytes.Equal(selected, raw) {
			return m, fmt.Errorf("SourceMap panic raw artifact differs")
		}
	}
	rows := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(rows) != 4 {
		return m, fmt.Errorf("SourceMap panic row count mismatch")
	}
	for i, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 8 || fields[0] != m.Profiles[i] {
			return m, fmt.Errorf("invalid SourceMap panic profile row")
		}
		op := "getmap_empty"
		if s.Scope == "assert" {
			op = "assert_id"
		}
		wantCaught := s.Scope == "session" || i >= 2
		if fields[1] != "" || fields[2] != "0" || fields[3] != "0" || fields[4] != op || fields[5] != "0" || fields[6] != fmt.Sprint(wantCaught) {
			return m, fmt.Errorf("invalid SourceMap panic request row")
		}
		events, err := hex.DecodeString(fields[7])
		if err != nil || !utf8.Valid(events) {
			return m, fmt.Errorf("invalid SourceMap panic hook data")
		}
		if !wantCaught {
			if len(events) != 0 {
				return m, fmt.Errorf("unexpected disabled assertion hook")
			}
			continue
		}
		parts := strings.Split(string(events), "|")
		if len(parts) != 5 || parts[1] != location.File || parts[2] != fmt.Sprint(location.Line) || parts[3] != fmt.Sprint(location.Column) {
			return m, fmt.Errorf("SourceMap panic profile does not retain verbatim native location")
		}
	}
	return m, nil
}

func TestSourceMapPanicProducerSelection(t *testing.T) {
	for _, scope := range []string{"session", "assert"} {
		base := filepath.Join("testdata", "source_map_panic_"+scope+".tsv.gz")
		s, err := sourceMapPanicEvidencePaths(base, func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sourceMapPanicProfile(s, ""); err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"", filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
			if _, err := sourceMapPanicEvidencePaths(base, func(string) (string, bool) { return value, true }); err == nil {
				t.Fatalf("explicit %q fell back", value)
			}
		}
		if _, err := sourceMapPanicProfile(s, strings.Repeat("0", 64)); err == nil {
			t.Fatal("wrong selected manifest SHA accepted")
		}
	}
}
func TestSourceMapPanicProducerRejectsCorruption(t *testing.T) {
	for _, scope := range []string{"session", "assert"} {
		stem := "source_map_panic_" + scope
		historical, err := sourceMapPanicEvidencePaths(filepath.Join("testdata", stem+".tsv.gz"), func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		for _, file := range []string{historical.Fixture, historical.Manifest} {
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, filepath.Base(file)), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
		compressed, _ := os.ReadFile(historical.Fixture)
		z, _ := gzip.NewReader(bytes.NewReader(compressed))
		raw, _ := io.ReadAll(z)
		z.Close()
		if err := os.WriteFile(filepath.Join(directory, stem+".tsv"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		selected, err := sourceMapPanicEvidencePaths(historical.Fixture, func(string) (string, bool) { return directory, true })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sourceMapPanicProfile(selected, ""); err == nil {
			t.Fatal("historical schema accepted as fresh producer")
		}
		if err := os.Remove(selected.Fixture); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(historical.Fixture, selected.Fixture); err == nil {
			if _, err := sourceMapPanicEvidencePaths(historical.Fixture, func(string) (string, bool) { return directory, true }); err == nil {
				t.Fatal("symlink producer accepted")
			}
		}
	}
}

func TestSourceMapPanicProducerManifestFailClosed(t *testing.T) {
	for _, scope := range []string{"session", "assert"} {
		historical, err := sourceMapPanicEvidencePaths(filepath.Join("testdata", "source_map_panic_"+scope+".tsv.gz"), func(string) (string, bool) { return "", false })
		if err != nil {
			t.Fatal(err)
		}
		m, err := sourceMapPanicProfile(historical, "")
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		stem := "source_map_panic_" + scope
		fixture, _ := os.ReadFile(historical.Fixture)
		z, _ := gzip.NewReader(bytes.NewReader(fixture))
		raw, _ := io.ReadAll(z)
		z.Close()
		for name, body := range map[string][]byte{stem + ".tsv.gz": fixture, stem + ".tsv": raw} {
			if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
		m.Schema = 2
		m.Kind = "source-map-panic-producer-v1"
		m.Scope = scope
		m.Files = map[string]string{}
		for _, path := range []string{"tools/port-rust-source-map/generate_panic_sites.py", "tools/port-rust-source-map/panic_site_producer.py", "tools/port-rust-source-map/panic_sites.rs", "port/rust/compiler/rustc_span/src/source_map_panic_sites_test.go", "port/rust/compiler/rustc_span/src/source_map_panic_producer_test.go"} {
			body, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			m.Files[path] = sourceMapPanicProofSHA(body)
		}
		for _, name := range m.Profiles {
			m.NativeProfiles = append(m.NativeProfiles, struct {
				Name       string
				Provenance string `json:"reference_provenance_sha256"`
				Version    string `json:"rustc_version"`
			}{name, strings.Repeat("a", 64), "rustc 1.101.0-nightly (selected)"})
		}
		write := func(value sourceMapPanicProducerManifest) {
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, stem+"_manifest.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
		write(m)
		selected, err := sourceMapPanicEvidencePaths(historical.Fixture, func(string) (string, bool) { return directory, true })
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sourceMapPanicProfile(selected, ""); err != nil {
			t.Fatal(err)
		}
		for _, change := range []func(*sourceMapPanicProducerManifest){
			func(m *sourceMapPanicProducerManifest) { m.Schema = 1 }, func(m *sourceMapPanicProducerManifest) { m.Kind = "historical" },
			func(m *sourceMapPanicProducerManifest) { m.Scope = "other" }, func(m *sourceMapPanicProducerManifest) { m.Files = nil },
			func(m *sourceMapPanicProducerManifest) { m.Records = 3 }, func(m *sourceMapPanicProducerManifest) { m.Panics = 0 },
			func(m *sourceMapPanicProducerManifest) { m.Profiles = nil }, func(m *sourceMapPanicProducerManifest) { m.RawSHA = strings.Repeat("0", 64) },
			func(m *sourceMapPanicProducerManifest) { m.GzipSHA = strings.Repeat("0", 64) }, func(m *sourceMapPanicProducerManifest) { m.NativeProfiles = nil },
			func(m *sourceMapPanicProducerManifest) {
				m.ScopedTLS.File = "/another/native/path"
				m.Assertion.File = "/another/native/path"
			},
			func(m *sourceMapPanicProducerManifest) { m.ScopedTLS.Line = 0; m.Assertion.Line = 0 },
		} {
			bad := m
			change(&bad)
			write(bad)
			if _, err := sourceMapPanicProfile(selected, ""); err == nil {
				t.Fatal("invalid explicit SourceMap panic manifest accepted")
			}
		}
		write(m)
		if err := os.WriteFile(selected.Raw, []byte("different producer"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := sourceMapPanicProfile(selected, ""); err == nil {
			t.Fatal("different raw producer accepted")
		}
	}
}
