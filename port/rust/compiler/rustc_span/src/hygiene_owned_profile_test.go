// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hygieneOwnedProfileEnv = "FERRUGO_HYGIENE_OWNED_PROFILE_DIR"
const hygieneOwnedManifestSHAEnv = "FERRUGO_HYGIENE_OWNED_MANIFEST_SHA256"

type hygieneOwnedEvidence struct {
	Directory, Manifest, Fixture, ManifestSHA string
	Explicit                                  bool
}

func hygieneOwnedEvidencePaths(root string, lookup func(string) (string, bool)) (hygieneOwnedEvidence, error) {
	selection := hygieneOwnedEvidence{Manifest: filepath.Join(root, "tools/port-rust-hygiene/owned-manifest.json"), Fixture: filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/hygiene_owned.jsonl.gz")}
	if value, present := lookup(hygieneOwnedProfileEnv); present {
		if value == "" {
			return selection, fmt.Errorf("%s explicitly empty", hygieneOwnedProfileEnv)
		}
		directory, err := filepath.Abs(value)
		if err != nil {
			return selection, err
		}
		info, err := os.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return selection, fmt.Errorf("invalid explicit %s directory %q", hygieneOwnedProfileEnv, directory)
		}
		selection = hygieneOwnedEvidence{Directory: directory, Manifest: filepath.Join(directory, "owned-manifest.json"), Fixture: filepath.Join(directory, "hygiene_owned.jsonl.gz"), Explicit: true}
	}
	for _, path := range []string{selection.Manifest, selection.Fixture} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return selection, fmt.Errorf("selected owned Hygiene evidence is not a regular file: %s", path)
		}
		limit := int64(64 << 20)
		if path == selection.Manifest {
			limit = 4 << 20
		}
		if info.Size() > limit {
			return selection, fmt.Errorf("selected owned Hygiene evidence exceeds bound: %s", path)
		}
	}
	return selection, nil
}

func hygieneOwnedProfileMetadata(selection hygieneOwnedEvidence, expectedSHA string) (hygieneOwnedMetadata, error) {
	var m hygieneOwnedMetadata
	data, err := os.ReadFile(selection.Manifest)
	if err != nil {
		return m, err
	}
	actualSHA := fmt.Sprintf("%x", sha256.Sum256(data))
	if expectedSHA != "" && expectedSHA != actualSHA {
		return m, fmt.Errorf("owned Hygiene child selected a different producer manifest")
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.ArtifactKind != "hygiene-owned-producer-profile-v1" || m.SourceRevision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Cases <= 0 || len(m.Files) == 0 || m.RawSHA256 == "" || m.FixtureSHA256 == "" {
		return m, fmt.Errorf("invalid selected owned Hygiene producer profile: %s", selection.Manifest)
	}
	m.ManifestSHA = actualSHA
	return m, nil
}

func hygieneOwnedChildEnvironment(environment []string, selection hygieneOwnedEvidence) []string {
	// Preserve the parent's captured choice even if its ambient environment
	// changes later. Explicitly empty/invalid choices never fall back to the
	// historical producer, and abort children receive the same absolute folder.
	result := make([]string, 0, len(environment)+1)
	for _, value := range environment {
		if !strings.HasPrefix(value, hygieneOwnedProfileEnv+"=") && !strings.HasPrefix(value, hygieneOwnedManifestSHAEnv+"=") {
			result = append(result, value)
		}
	}
	if selection.Explicit {
		result = append(result, hygieneOwnedProfileEnv+"="+selection.Directory)
	}
	if selection.ManifestSHA != "" {
		result = append(result, hygieneOwnedManifestSHAEnv+"="+selection.ManifestSHA)
	}
	return result
}

func hygieneOwnedVerifyFixture(compressed []byte, m hygieneOwnedMetadata) error {
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != m.FixtureSHA256 {
		return fmt.Errorf("selected owned Hygiene fixture checksum differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	defer z.Close()
	digest := sha256.New()
	const rawLimit = 256 << 20
	n, err := io.Copy(digest, io.LimitReader(z, rawLimit+1))
	if err != nil {
		return err
	}
	if n > rawLimit || fmt.Sprintf("%x", digest.Sum(nil)) != m.RawSHA256 {
		return fmt.Errorf("selected owned Hygiene raw fixture checksum/bound differs")
	}
	return nil
}

func TestHygieneOwnedEvidenceSelector(t *testing.T) {
	root := t.TempDir()
	manifest := filepath.Join(root, "tools/port-rust-hygiene/owned-manifest.json")
	fixture := filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/hygiene_owned.jsonl.gz")
	for _, path := range []string{manifest, fixture} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("historical"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	absent := func(string) (string, bool) { return "", false }
	selection, err := hygieneOwnedEvidencePaths(root, absent)
	if err != nil || selection.Explicit || selection.Manifest != manifest || selection.Fixture != fixture {
		t.Fatalf("wrong historical selection: %+v %v", selection, err)
	}
	for _, value := range []string{"", filepath.Join(root, "missing"), manifest} {
		if _, err := hygieneOwnedEvidencePaths(root, func(string) (string, bool) { return value, true }); err == nil {
			t.Fatalf("invalid explicit choice fell back to historical evidence: %q", value)
		}
	}
	directory := t.TempDir()
	for _, name := range []string{"owned-manifest.json", "hygiene_owned.jsonl.gz"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fresh"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	selection, err = hygieneOwnedEvidencePaths(root, func(string) (string, bool) { return directory, true })
	if err != nil || !selection.Explicit || selection.Directory != directory || selection.Manifest != filepath.Join(directory, "owned-manifest.json") {
		t.Fatalf("wrong explicit selection: %+v %v", selection, err)
	}
	if err := os.Remove(selection.Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := hygieneOwnedEvidencePaths(root, func(string) (string, bool) { return directory, true }); err == nil {
		t.Fatal("partial selected producer fell back to historical evidence")
	}
	if err := os.Symlink(fixture, selection.Fixture); err != nil {
		t.Fatal(err)
	}
	if _, err := hygieneOwnedEvidencePaths(root, func(string) (string, bool) { return directory, true }); err == nil {
		t.Fatal("selected symlink fixture was accepted")
	}
	if err := os.Remove(selection.Fixture); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(selection.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := hygieneOwnedEvidencePaths(root, func(string) (string, bool) { return directory, true }); err == nil {
		t.Fatal("oversized selected fixture was accepted")
	}
}

func TestHygieneOwnedEvidenceChildSelection(t *testing.T) {
	selection := hygieneOwnedEvidence{Explicit: true, Directory: "/captured/current producer", ManifestSHA: "captured-manifest"}
	environment := []string{"UNCHANGED=1", hygieneOwnedProfileEnv + "=/later/wrong", hygieneOwnedProfileEnv + "=", hygieneOwnedManifestSHAEnv + "=later-manifest"}
	actual := hygieneOwnedChildEnvironment(environment, selection)
	if strings.Join(actual, "\n") != "UNCHANGED=1\n"+hygieneOwnedProfileEnv+"=/captured/current producer\n"+hygieneOwnedManifestSHAEnv+"=captured-manifest" {
		t.Fatal("child lost the exact captured producer choice", actual)
	}
	if actual := hygieneOwnedChildEnvironment(environment, hygieneOwnedEvidence{}); len(actual) != 1 || actual[0] != "UNCHANGED=1" {
		t.Fatal("historical child inherited a later producer override", actual)
	}
}

func TestHygieneOwnedEvidenceManifestIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned-manifest.json")
	data := []byte(`{"artifact_kind":"hygiene-owned-producer-profile-v1","source_revision":"2d20b42b883dcfc014f3c2c56081eeb6091c2179","cases":1,"raw_sha256":"raw","fixture_sha256":"gzip","files":{"proof-input":"hash"}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	selection := hygieneOwnedEvidence{Manifest: path}
	parent, err := hygieneOwnedProfileMetadata(selection, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hygieneOwnedProfileMetadata(selection, parent.ManifestSHA); err != nil {
		t.Fatal("child rejected parent's exact selected profile", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := hygieneOwnedProfileMetadata(selection, parent.ManifestSHA); err == nil {
		t.Fatal("child accepted changed producer manifest bytes")
	}
	if err := os.WriteFile(path, []byte(`{"artifact_kind":"unrelated","cases":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := hygieneOwnedProfileMetadata(selection, ""); err == nil {
		t.Fatal("wrong producer schema/pin was accepted")
	}
}

func TestHygieneOwnedEvidenceFixtureIdentity(t *testing.T) {
	var buffer bytes.Buffer
	z := gzip.NewWriter(&buffer)
	raw := []byte("exact native producer rows\n")
	z.Write(raw)
	z.Close()
	compressed := buffer.Bytes()
	m := hygieneOwnedMetadata{FixtureSHA256: fmt.Sprintf("%x", sha256.Sum256(compressed)), RawSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	if err := hygieneOwnedVerifyFixture(compressed, m); err != nil {
		t.Fatal("exact producer fixture rejected", err)
	}
	m.RawSHA256 = "wrong-raw-profile"
	if err := hygieneOwnedVerifyFixture(compressed, m); err == nil {
		t.Fatal("parent accepted a different raw producer profile")
	}
	compressed[0] ^= 1
	if err := hygieneOwnedVerifyFixture(compressed, m); err == nil {
		t.Fatal("parent accepted a changed compressed producer fixture")
	}
}
