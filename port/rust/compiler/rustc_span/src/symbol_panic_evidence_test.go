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
	"runtime"
	"strings"
	"testing"
)

const symbolPanicProfileDirectory = "FERRUGO_SYMBOL_PANIC_PROFILE_DIR"

func symbolPanicSourceRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}

// A selected producer directory is explicit: errors never fall back to the
// historical producer. Source filenames inside the evidence remain verbatim.
func symbolPanicEvidencePaths(value string, selected bool) (manifest, fixture, directory string, err error) {
	root := symbolPanicSourceRoot()
	if !selected {
		return filepath.Join(root, "tools/port-rust-symbol/panic/manifest.json"), filepath.Join(root, "port/rust/compiler/rustc_span/src/testdata/symbol_panic.jsonl.gz"), "", nil
	}
	if value == "" {
		return "", "", "", fmt.Errorf("%s is explicitly empty", symbolPanicProfileDirectory)
	}
	info, err := os.Lstat(value)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", "", fmt.Errorf("invalid selected Symbol producer directory %q: %v", value, err)
	}
	directory, err = filepath.Abs(value)
	if err != nil {
		return "", "", "", err
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return "", "", "", err
	}
	manifest, fixture = filepath.Join(directory, "manifest.json"), filepath.Join(directory, "symbol_panic.jsonl.gz")
	for _, file := range []string{manifest, fixture} {
		info, err := os.Lstat(file)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", "", fmt.Errorf("invalid selected Symbol producer file %q: %v", file, err)
		}
	}
	return manifest, fixture, directory, nil
}

func symbolPanicLoadEvidence() (m symbolPanicManifest, raw []byte, err error) {
	value, selected := os.LookupEnv(symbolPanicProfileDirectory)
	manifest, fixture, _, err := symbolPanicEvidencePaths(value, selected)
	if err != nil {
		return m, nil, err
	}
	data, err := symbolPanicReadBounded(manifest, 2<<20)
	if err != nil {
		return m, nil, err
	}
	if err = json.Unmarshal(data, &m); err != nil {
		return m, nil, err
	}
	if m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Records != 660 || m.Rows != 177 || m.RawBytes > 16<<20 || m.RawBytes < 1 || len(m.Bindings) == 0 {
		return m, nil, fmt.Errorf("Symbol producer identity/coverage differs")
	}
	for name, want := range m.Bindings {
		if filepath.IsAbs(name) || filepath.ToSlash(filepath.Clean(name)) != name || strings.HasPrefix(name, "../") {
			return m, nil, fmt.Errorf("invalid Symbol source binding %q", name)
		}
		body, e := symbolPanicReadBounded(filepath.Join(symbolPanicSourceRoot(), name), 2<<20)
		if e != nil {
			return m, nil, e
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != want {
			return m, nil, fmt.Errorf("Symbol source binding differs: %s", name)
		}
	}
	compressed, err := symbolPanicReadBounded(fixture, 8<<20)
	if err != nil {
		return m, nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != m.FixtureSHA {
		return m, nil, fmt.Errorf("Symbol producer compressed hash differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return m, nil, err
	}
	defer z.Close()
	raw, err = io.ReadAll(io.LimitReader(z, (16<<20)+1))
	if err != nil || len(raw) > 16<<20 || len(raw) != m.RawBytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != m.RawSHA {
		return m, nil, fmt.Errorf("Symbol producer raw hash/bound differs: %v", err)
	}
	if bytes.Count(raw, []byte{'\n'}) != m.Rows || len(m.Profiles) != 8 {
		return m, nil, fmt.Errorf("Symbol producer row/profile coverage differs")
	}
	return m, raw, nil
}

func symbolPanicReadBounded(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, fmt.Errorf("invalid bounded Symbol evidence file %q: %v", path, err)
	}
	return os.ReadFile(path)
}

func TestSymbolPanicProducerSelectorFailsClosed(t *testing.T) {
	directory := t.TempDir()
	for _, input := range []string{"", filepath.Join(directory, "missing"), directory} {
		if _, _, _, err := symbolPanicEvidencePaths(input, true); err == nil {
			t.Fatalf("selected incomplete producer %q fell back", input)
		}
	}
	for _, name := range []string{"manifest.json", "symbol_panic.jsonl.gz"} {
		if err := os.WriteFile(filepath.Join(directory, name), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "producer-link")
	if err := os.Symlink(directory, link); err == nil {
		if _, _, _, err := symbolPanicEvidencePaths(link, true); err == nil {
			t.Fatal("symlink producer directory accepted")
		}
	}
	if err := os.Remove(filepath.Join(directory, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(directory, "symbol_panic.jsonl.gz"), filepath.Join(directory, "manifest.json")); err == nil {
		if _, _, _, err := symbolPanicEvidencePaths(directory, true); err == nil {
			t.Fatal("symlink producer manifest accepted")
		}
	}
}

func TestSymbolPanicProducerRelocationAndIntegrity(t *testing.T) {
	value, selected := os.LookupEnv(symbolPanicProfileDirectory)
	manifest, fixture, _, err := symbolPanicEvidencePaths(value, selected)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for _, file := range []string{manifest, fixture} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, filepath.Base(file)), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv(symbolPanicProfileDirectory, directory)
	m, raw, err := symbolPanicLoadEvidence()
	if err != nil {
		t.Fatal(err)
	}
	var profile symbolPanicProfile
	for _, p := range m.Profiles {
		if p.Name == "d1o1" {
			profile = p
		}
	}
	normal, abort := false, false
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var record symbolPanicRecord
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(record.Request, "no-session\t0\t") {
			result, trace := symbolPanicReplay(record.Request, profile, false)
			if result != record.Want.Result || strings.Join(trace, "|") != strings.Join(record.Want.Trace, "|") {
				t.Fatal("relocation changed verbatim native metadata")
			}
			normal = true
		}
		if !abort && record.Want.Abort {
			for _, name := range record.Profiles {
				if name == profile.Name {
					symbolPanicAssertAbort(t, record, profile)
					abort = true
				}
			}
		}
	}
	if !normal || !abort {
		t.Fatal("relocation did not cover ordinary and isolated abort replay")
	}
	metadata, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.Bindings["port/rust/compiler/rustc_span/src/symbol_panic.go"] = strings.Repeat("0", 64)
	tampered, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), tampered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := symbolPanicLoadEvidence(); err == nil || !strings.Contains(err.Error(), "binding differs") {
		t.Fatalf("tampered selected source binding was accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), metadata, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "symbol_panic.jsonl.gz"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := symbolPanicLoadEvidence(); err == nil {
		t.Fatal("corrupted selected profile fell back to historical data")
	}
}
