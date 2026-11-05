// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const tracingProducerDirectory = "FERRUGO_TRACING_PRODUCER_DIR"
const tracingProducerExpectedSHA = "FERRUGO_TRACING_PRODUCER_MANIFEST_SHA"

type tracingProducerEvidence struct {
	Schema    int                                `json:"schema"`
	Records   int                                `json:"records"`
	Aborts    int                                `json:"actual_SIGABRT"`
	Revision  string                             `json:"source_revision"`
	RawBytes  int                                `json:"raw_bytes"`
	RawSHA    string                             `json:"raw_sha256"`
	Bindings  map[string]string                  `json:"source_bindings"`
	Locations map[string]*rustfmt.SourceLocation `json:"locations"`
	Digest    string                             `json:"-"`
}

func tracingProducerRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}
func tracingProducerNames(kind string) (string, string, int, int) {
	if kind == "borrow" {
		return "BORROW_VALIDATION.json", "borrow_panic.tsv", 9, 3
	}
	if kind == "registry" {
		return "REGISTRY_VALIDATION.json", "registry_poison.tsv", 8, 2
	}
	panic("invalid tracing proof kind")
}
func tracingProducerPaths(kind, selected string, explicit bool) (manifest, fixture, directory string, err error) {
	name, data, _, _ := tracingProducerNames(kind)
	if !explicit {
		return filepath.Join(tracingProducerRoot(), "tools/port-rust-tracing/processing", name), filepath.Join(tracingProducerRoot(), "port/support/rusttracing/testdata", data), "", nil
	}
	if selected == "" {
		return "", "", "", fmt.Errorf("%s explicitly empty", tracingProducerDirectory)
	}
	info, err := os.Lstat(selected)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", fmt.Errorf("invalid selected tracing producer directory %q: %v", selected, err)
	}
	directory, err = filepath.Abs(selected)
	if err != nil {
		return "", "", "", err
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return "", "", "", err
	}
	manifest, fixture = filepath.Join(directory, name), filepath.Join(directory, data)
	for _, path := range []string{manifest, fixture} {
		if _, err := tracingProducerRead(path); err != nil {
			return "", "", "", err
		}
	}
	return manifest, fixture, directory, nil
}
func tracingProducerRead(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
		return nil, fmt.Errorf("invalid bounded tracing evidence %q: %v", path, err)
	}
	return os.ReadFile(path)
}
func tracingProducerLoad(kind string) (m tracingProducerEvidence, raw []byte, directory string, err error) {
	value, explicit := os.LookupEnv(tracingProducerDirectory)
	manifest, fixture, directory, err := tracingProducerPaths(kind, value, explicit)
	if err != nil {
		return m, nil, "", err
	}
	data, err := tracingProducerRead(manifest)
	if err != nil {
		return m, nil, "", err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, nil, "", err
	}
	checksum := sha256.Sum256(data)
	m.Digest = hex.EncodeToString(checksum[:])
	if expected, present := os.LookupEnv(tracingProducerExpectedSHA); present && (expected == "" || expected != m.Digest) {
		return m, nil, "", fmt.Errorf("tracing producer changed between parent and child")
	}
	_, _, count, aborts := tracingProducerNames(kind)
	if m.Schema != 1 || m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Records != count || m.Aborts != aborts || m.RawBytes < 1 || m.RawBytes > 1<<20 || len(m.Bindings) == 0 || len(m.Locations) == 0 {
		return m, nil, "", fmt.Errorf("tracing producer identity/coverage differs")
	}
	for name, expected := range m.Bindings {
		if filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || strings.HasPrefix(name, "../") {
			return m, nil, "", fmt.Errorf("invalid tracing source binding %q", name)
		}
		body, e := tracingProducerRead(filepath.Join(tracingProducerRoot(), name))
		if e != nil {
			return m, nil, "", e
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != expected {
			return m, nil, "", fmt.Errorf("tracing source binding differs: %s", name)
		}
	}
	raw, err = tracingProducerRead(fixture)
	if err != nil {
		return m, nil, "", err
	}
	sum := sha256.Sum256(raw)
	if len(raw) != m.RawBytes || hex.EncodeToString(sum[:]) != m.RawSHA {
		return m, nil, "", fmt.Errorf("tracing producer raw hash/bound differs")
	}
	rows := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(rows) != count {
		return m, nil, "", fmt.Errorf("tracing producer row count differs")
	}
	aborted := 0
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		if len(fields) != 4 || fields[2] != "ok" && fields[2] != "abort" {
			return m, nil, "", fmt.Errorf("tracing producer row schema differs")
		}
		if _, e := hex.DecodeString(fields[3]); e != nil {
			return m, nil, "", e
		}
		if fields[2] == "abort" {
			aborted++
		}
	}
	if aborted != aborts {
		return m, nil, "", fmt.Errorf("tracing producer abort coverage differs")
	}
	return m, raw, directory, nil
}

func tracingProducerChildEnv(m tracingProducerEvidence, directory string) []string {
	env := append(os.Environ(), tracingProducerExpectedSHA+"="+m.Digest)
	if _, explicit := os.LookupEnv(tracingProducerDirectory); explicit {
		env = append(env, tracingProducerDirectory+"="+directory)
	}
	return env
}

func TestTracingProducerSelectorFailsClosed(t *testing.T) {
	for _, kind := range []string{"borrow", "registry"} {
		for _, directory := range []string{"", filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
			if _, _, _, err := tracingProducerPaths(kind, directory, true); err == nil {
				t.Fatalf("%s selected incomplete producer fell back", kind)
			}
		}
	}
	directory := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(directory, link); err == nil {
		if _, _, _, err := tracingProducerPaths("borrow", link, true); err == nil {
			t.Fatal("symlink selection accepted")
		}
	}
}

func TestTracingProducerRelocationAndIntegrity(t *testing.T) {
	directory := t.TempDir()
	value, explicit := os.LookupEnv(tracingProducerDirectory)
	for _, kind := range []string{"borrow", "registry"} {
		manifest, fixture, _, err := tracingProducerPaths(kind, value, explicit)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{manifest, fixture} {
			body, err := tracingProducerRead(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, filepath.Base(path)), body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv(tracingProducerDirectory, directory)
	t.Run("borrow", TestBorrowAndZeroIDNativeOracle)
	t.Run("registry", TestRegistryPoisonNativeOracle)
	m, _, _, err := tracingProducerLoad("borrow")
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, "BORROW_VALIDATION.json")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	m.Bindings["port/support/rusttracing/dispatch.go"] = strings.Repeat("0", 64)
	tampered, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := tracingProducerLoad("borrow"); err == nil || !strings.Contains(err.Error(), "binding differs") {
		t.Fatalf("source tamper accepted: %v", err)
	}
	if err := os.WriteFile(manifest, original, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(tracingProducerExpectedSHA, "changed")
	if _, _, _, err := tracingProducerLoad("borrow"); err == nil {
		t.Fatal("changed parent-child manifest accepted")
	}
	t.Setenv(tracingProducerExpectedSHA, m.Digest)
	if err := os.WriteFile(filepath.Join(directory, "borrow_panic.tsv"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := tracingProducerLoad("borrow"); err == nil {
		t.Fatal("corrupt explicit data fell back")
	}
}
