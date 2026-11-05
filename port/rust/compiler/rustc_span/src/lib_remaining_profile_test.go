// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const libRemainingProfileEnv = "FERRUGO_LIB_REMAINING_PROOF"
const libRemainingManifestSHAEnv = "FERRUGO_LIB_REMAINING_MANIFEST_SHA256"

type libRemainingEvidence struct {
	Directory, Manifest, Fixture, ManifestSHA string
	Explicit                                  bool
}

func libRemainingEvidencePaths(root string, lookup func(string) (string, bool)) (libRemainingEvidence, error) {
	selection := libRemainingEvidence{Manifest: filepath.Join(root, "tools/port-rust-span-lib/remaining-context-manifest.json"), Fixture: filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/lib-remaining-context.jsonl.gz")}
	if value, present := lookup(libRemainingProfileEnv); present {
		if value == "" {
			return selection, fmt.Errorf("%s explicitly empty", libRemainingProfileEnv)
		}
		directory, err := filepath.Abs(value)
		if err != nil {
			return selection, err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return selection, fmt.Errorf("invalid explicit remaining-context directory %q", directory)
		}
		selection = libRemainingEvidence{Directory: directory, Manifest: filepath.Join(directory, "remaining-context-manifest.json"), Fixture: filepath.Join(directory, "lib-remaining-context.jsonl.gz"), Explicit: true}
	}
	for _, path := range []string{selection.Manifest, selection.Fixture} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return selection, fmt.Errorf("selected remaining-context evidence is not a regular file: %s", path)
		}
		limit := int64(16 << 20)
		if path == selection.Manifest {
			limit = 4 << 20
		}
		if info.Size() > limit {
			return selection, fmt.Errorf("selected remaining-context evidence exceeds bound: %s", path)
		}
	}
	return selection, nil
}
func libRemainingProfileMetadata(selection libRemainingEvidence, expectedSHA string) (libRemainingManifest, error) {
	var m libRemainingManifest
	body, err := os.ReadFile(selection.Manifest)
	if err != nil {
		return m, err
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(body))
	if expectedSHA != "" && expectedSHA != actual {
		return m, fmt.Errorf("remaining-context child selected a different producer manifest")
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	if m.ArtifactKind != "span-lib-remaining-producer-v1" || m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Records != 3520 || m.Aborts != 1324 || len(m.Files) == 0 || len(m.Locations) == 0 || len(m.RawSHA) != 64 || len(m.FixtureSHA) != 64 {
		return m, fmt.Errorf("invalid selected remaining-context producer profile")
	}
	for _, raw := range m.Locations {
		var file string
		var line, column uint32
		if len(raw) != 3 || json.Unmarshal(raw[0], &file) != nil || json.Unmarshal(raw[1], &line) != nil || json.Unmarshal(raw[2], &column) != nil || file == "" || line == 0 || column == 0 {
			return m, fmt.Errorf("invalid selected remaining-context source location")
		}
	}
	m.ManifestSHA = actual
	return m, nil
}
func libRemainingChildEnvironment(environment []string, selection libRemainingEvidence) []string {
	result := make([]string, 0, len(environment)+2)
	for _, value := range environment {
		if !strings.HasPrefix(value, libRemainingProfileEnv+"=") && !strings.HasPrefix(value, libRemainingManifestSHAEnv+"=") {
			result = append(result, value)
		}
	}
	if selection.Explicit {
		result = append(result, libRemainingProfileEnv+"="+selection.Directory)
	}
	if selection.ManifestSHA != "" {
		result = append(result, libRemainingManifestSHAEnv+"="+selection.ManifestSHA)
	}
	return result
}
func TestLibRemainingEvidenceSelector(t *testing.T) {
	root := t.TempDir()
	historical := libRemainingEvidence{Manifest: filepath.Join(root, "tools/port-rust-span-lib/remaining-context-manifest.json"), Fixture: filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/lib-remaining-context.jsonl.gz")}
	for _, p := range []string{historical.Manifest, historical.Fixture} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("historical"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := libRemainingEvidencePaths(root, func(string) (string, bool) { return "", false })
	if err != nil || s != historical {
		t.Fatal(s, err)
	}
	for _, value := range []string{"", filepath.Join(root, "missing"), historical.Manifest} {
		if _, err := libRemainingEvidencePaths(root, func(string) (string, bool) { return value, true }); err == nil {
			t.Fatalf("explicit %q fell back", value)
		}
	}
	fresh := t.TempDir()
	for _, name := range []string{"remaining-context-manifest.json", "lib-remaining-context.jsonl.gz"} {
		if err := os.WriteFile(filepath.Join(fresh, name), []byte("fresh"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err = libRemainingEvidencePaths(root, func(string) (string, bool) { return fresh, true })
	if err != nil || !s.Explicit || s.Directory != fresh {
		t.Fatal(s, err)
	}
	if err := os.Remove(s.Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := libRemainingEvidencePaths(root, func(string) (string, bool) { return fresh, true }); err == nil {
		t.Fatal("missing fresh fixture silently fell back")
	}
	if err := os.Symlink(historical.Fixture, s.Fixture); err == nil {
		if _, err := libRemainingEvidencePaths(root, func(string) (string, bool) { return fresh, true }); err == nil {
			t.Fatal("symlink fresh fixture accepted")
		}
	}
}
func TestLibRemainingChildChoiceAndManifestIdentity(t *testing.T) {
	selection := libRemainingEvidence{Directory: "/selected/current producer", Explicit: true, ManifestSHA: strings.Repeat("a", 64)}
	got := libRemainingChildEnvironment([]string{"KEEP=1", libRemainingProfileEnv + "=/stale", libRemainingManifestSHAEnv + "=stale"}, selection)
	want := []string{"KEEP=1", libRemainingProfileEnv + "=" + selection.Directory, libRemainingManifestSHAEnv + "=" + selection.ManifestSHA}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatal(got)
	}
	if got := libRemainingChildEnvironment([]string{libRemainingProfileEnv + "=/stale"}, libRemainingEvidence{ManifestSHA: "captured"}); len(got) != 1 || got[0] != libRemainingManifestSHAEnv+"=captured" {
		t.Fatal("historical parent inherited stale selection", got)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	body := []byte(`{}`)
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := libRemainingProfileMetadata(libRemainingEvidence{Manifest: path}, strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "different producer") {
		t.Fatal("child manifest replacement accepted", err)
	}
	if _, err := libRemainingProfileMetadata(libRemainingEvidence{Manifest: path}, fmt.Sprintf("%x", sha256.Sum256(body))); err == nil {
		t.Fatal("invalid explicit manifest accepted")
	}
}
