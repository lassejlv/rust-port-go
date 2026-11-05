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
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const libPanicProfileEnv = "FERRUGO_LIB_PANIC_PROOF"
const libPanicManifestSHAEnv = "FERRUGO_LIB_PANIC_MANIFEST_SHA256"

type libPanicEvidence struct {
	Manifest, Fixture string
	Explicit          bool
}
type libPanicManifest struct {
	Schema       int
	ArtifactKind string `json:"artifact_kind"`
	Revision     string `json:"source_revision"`
	Records      int
	RawSHA       string `json:"raw_sha256"`
	FixtureSHA   string `json:"fixture_sha256"`
	Profiles     []struct {
		Name    string
		Records int
	}
	Files map[string]string
}

func libPanicEvidencePaths(root string, lookup func(string) (string, bool)) (libPanicEvidence, error) {
	selection := libPanicEvidence{Manifest: filepath.Join(root, "tools/port-rust-span-lib/panic-manifest.json"), Fixture: filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/lib-panic.tsv.gz")}
	if value, present := lookup(libPanicProfileEnv); present {
		if value == "" {
			return selection, fmt.Errorf("%s explicitly empty", libPanicProfileEnv)
		}
		directory, err := filepath.Abs(value)
		if err != nil {
			return selection, err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return selection, fmt.Errorf("invalid explicit panic producer directory %q", directory)
		}
		selection = libPanicEvidence{Manifest: filepath.Join(directory, "panic-manifest.json"), Fixture: filepath.Join(directory, "lib-panic.tsv.gz"), Explicit: true}
	}
	for _, path := range []string{selection.Manifest, selection.Fixture} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return selection, fmt.Errorf("selected panic evidence is not a bounded regular file: %s", path)
		}
	}
	return selection, nil
}

func libPanicProfileMetadata(selection libPanicEvidence, expectedSHA string) (libPanicManifest, error) {
	var m libPanicManifest
	body, err := os.ReadFile(selection.Manifest)
	if err != nil {
		return m, err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(body))
	if expectedSHA != "" && expectedSHA != actual {
		return m, fmt.Errorf("panic replay selected a different producer manifest")
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	validSHA := func(value string) bool {
		data, err := hex.DecodeString(value)
		return err == nil && len(data) == sha256.Size
	}
	if m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Records != 256 || !validSHA(m.RawSHA) || !validSHA(m.FixtureSHA) || len(m.Profiles) != 4 {
		return m, fmt.Errorf("invalid selected panic producer profile")
	}
	for i, name := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		if m.Profiles[i].Name != name || m.Profiles[i].Records != 64 {
			return m, fmt.Errorf("invalid selected panic build profiles")
		}
	}
	if selection.Explicit {
		if m.Schema != 2 || m.ArtifactKind != "span-lib-panic-producer-v1" || len(m.Files) == 0 {
			return m, fmt.Errorf("explicit panic evidence is not a fresh producer profile")
		}
	} else if m.Schema != 1 {
		return m, fmt.Errorf("unsupported historical panic evidence schema")
	}
	return m, nil
}

// The native fixture's eighth hook field is the untouched actual filename.
// Its third field is retained legacy audit data, never the observed Go output.
// Install this explicit producer filename only on the already-declared semantic
// source sites with the same legacy file, line and column. The hook itself then
// compares the unmodified filename, payload, flags and source coordinates.
func libPanicVerbatimLocations(base map[string]rustfmt.SourceLocation, hooks string) (map[string]rustfmt.SourceLocation, []string, error) {
	locations := make(map[string]rustfmt.SourceLocation, len(base))
	for key, location := range base {
		locations[key] = location
	}
	expected := []string{}
	if hooks == "" {
		return locations, expected, nil
	}
	for _, event := range strings.Split(hooks, ",") {
		fields := strings.Split(event, ":")
		if len(fields) != 8 {
			return nil, nil, fmt.Errorf("native panic hook shape")
		}
		legacy, err := hex.DecodeString(fields[2])
		if err != nil || !utf8.Valid(legacy) {
			return nil, nil, fmt.Errorf("invalid legacy native filename")
		}
		raw, err := hex.DecodeString(fields[7])
		if err != nil || len(raw) == 0 || !utf8.Valid(raw) {
			return nil, nil, fmt.Errorf("invalid retained raw native filename")
		}
		line, err := strconv.ParseUint(fields[3], 10, 32)
		if err != nil || line == 0 {
			return nil, nil, fmt.Errorf("invalid native source line")
		}
		column, err := strconv.ParseUint(fields[4], 10, 32)
		if err != nil || column == 0 {
			return nil, nil, fmt.Errorf("invalid native source column")
		}
		matched := false
		for key, location := range base {
			if location.File == string(legacy) && location.Line == uint32(line) && location.Column == uint32(column) {
				location.File = string(raw)
				locations[key] = location
				matched = true
			}
		}
		if !matched {
			return nil, nil, fmt.Errorf("native panic has no declared semantic source site: %s:%d:%d", legacy, line, column)
		}
		fields[2] = fields[7]
		expected = append(expected, strings.Join(fields[:7], ":"))
	}
	return locations, expected, nil
}

func TestLibPanicProducerSelection(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"", filepath.Join(root, "missing"), root} {
		if _, err := libPanicEvidencePaths(root, func(string) (string, bool) { return value, true }); err == nil {
			t.Fatalf("explicit panic selection %q fell back", value)
		}
	}
	fresh := t.TempDir()
	selection := libPanicEvidence{Manifest: filepath.Join(fresh, "panic-manifest.json"), Fixture: filepath.Join(fresh, "lib-panic.tsv.gz"), Explicit: true}
	for _, path := range []string{selection.Manifest, selection.Fixture} {
		if err := os.WriteFile(path, []byte("fresh"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := libPanicEvidencePaths(root, func(string) (string, bool) { return fresh, true })
	if err != nil || got != selection {
		t.Fatal(got, err)
	}
	if _, err := libPanicProfileMetadata(selection, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "different producer") {
		t.Fatal(err)
	}
	if err := os.Remove(selection.Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := libPanicEvidencePaths(root, func(string) (string, bool) { return fresh, true }); err == nil {
		t.Fatal("missing producer fixture silently fell back")
	}
	if err := os.Symlink(selection.Manifest, selection.Fixture); err == nil {
		if _, err := libPanicEvidencePaths(root, func(string) (string, bool) { return fresh, true }); err == nil {
			t.Fatal("symlink fixture accepted")
		}
	}
}

func TestLibPanicProducerVerbatimFilename(t *testing.T) {
	base := map[string]rustfmt.SourceLocation{"selected": {File: "registry/crate/src/lib.rs", Line: 168, Column: 9}}
	actual := "/independent producer/cache/registry/src/index/crate/src/lib.rs"
	header := "static_str:61:" + hex.EncodeToString([]byte(base["selected"].File)) + ":168:9:true:false:"
	locations, expected, err := libPanicVerbatimLocations(base, header+hex.EncodeToString([]byte(actual)))
	if err != nil || locations["selected"].File != actual || base["selected"].File == actual || len(expected) != 1 || strings.Split(expected[0], ":")[2] != hex.EncodeToString([]byte(actual)) {
		t.Fatal(locations, expected, err)
	}
	for _, hooks := range []string{header + "zz", header + "", strings.Replace(header, ":168:", ":169:", 1) + hex.EncodeToString([]byte(actual))} {
		if _, _, err := libPanicVerbatimLocations(base, hooks); err == nil {
			t.Fatal("invalid producer location accepted", hooks)
		}
	}
}

// Every historical observation remains usable without normalizing the actual
// hook output. This checks the selected source dictionary independently of the
// full runtime replay, including absolute scoped-tls producer paths.
func TestLibPanicRetainedVerbatimProfiles(t *testing.T) {
	compressed, err := os.ReadFile("testdata/lib-panic.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	g, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(g, 1<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1<<20 {
		t.Fatal("historical panic bound")
	}
	base := LibPanicLocationsAtPinnedRevision("original/compiler/rustc_span/src/lib.rs", "registry/scoped-tls-1.0.1/src/lib.rs")
	for key, location := range AttrIdStepPanicLocationsAtSDK20261104("/rustc/708c94ef093ecc42fb8851e17d8383d999727abc/library/core/src/iter/range.rs") {
		base[key] = location
	}
	for key, location := range SpanCodecPanicLocationsAtPinnedRevision("original/compiler/rustc_serialize/src/serialize.rs") {
		base[key] = location
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			t.Fatal("historical panic row shape")
		}
		if _, _, err := libPanicVerbatimLocations(base, fields[3]); err != nil {
			t.Fatal(fields[:2], err)
		}
		count++
	}
	if count != 256 {
		t.Fatal(count)
	}
}

func TestLibPanicProducerManifestFailClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "panic-manifest.json")
	selection := libPanicEvidence{Manifest: path, Explicit: true}
	m := libPanicManifest{Schema: 2, ArtifactKind: "span-lib-panic-producer-v1", Revision: "2d20b42b883dcfc014f3c2c56081eeb6091c2179", Records: 256, RawSHA: strings.Repeat("a", 64), FixtureSHA: strings.Repeat("b", 64), Files: map[string]string{"selected.go": strings.Repeat("c", 64)}}
	for _, name := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		m.Profiles = append(m.Profiles, struct {
			Name    string
			Records int
		}{name, 64})
	}
	write := func(m libPanicManifest) string {
		t.Helper()
		body, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0644); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(body))
	}
	if _, err := libPanicProfileMetadata(selection, write(m)); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*libPanicManifest){
		func(m *libPanicManifest) { m.Schema = 1 },
		func(m *libPanicManifest) { m.ArtifactKind = "historical" },
		func(m *libPanicManifest) { m.Files = nil },
		func(m *libPanicManifest) { m.Revision = strings.Repeat("0", 40) },
		func(m *libPanicManifest) { m.Records-- },
		func(m *libPanicManifest) { m.RawSHA = "not-a-hash" },
		func(m *libPanicManifest) { m.Profiles = nil },
	} {
		bad := m
		change(&bad)
		if _, err := libPanicProfileMetadata(selection, write(bad)); err == nil {
			t.Fatal("invalid explicit producer manifest accepted")
		}
	}
}
