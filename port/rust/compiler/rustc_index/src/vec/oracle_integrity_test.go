// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func storageDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func storageVerifyCorpus(t *testing.T) {
	t.Helper()
	root := storageRoot()
	dir := filepath.Join(root, "tools/port-rust-index-storage/oracle")
	var m struct {
		Revision        string `json:"rust_revision"`
		Requests, Steps int
		Files           map[string]string `json:"sha256"`
		Decoded         string            `json:"decoded_sha256"`
		Compressed      string            `json:"compressed_sha256"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Requests == 0 || m.Steps == 0 {
		t.Fatal("invalid pinned storage corpus manifest")
	}
	for path, want := range m.Files {
		if strings.HasPrefix(path, "verified-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if storageDigest(data) != want {
			t.Fatalf("storage oracle input changed: %s; regenerate the original-source corpus", path)
		}
	}
	data, err = os.ReadFile(filepath.Join(dir, "cases.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if storageDigest(data) != m.Compressed {
		t.Fatal("storage compressed corpus checksum mismatch")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(digest.Sum(nil)) != m.Decoded {
		t.Fatal("storage decoded corpus checksum mismatch")
	}
}

func TestStorageCorpusIntegrity(t *testing.T) { storageVerifyCorpus(t) }

func TestOriginalStorageOracleLive(t *testing.T) {
	value, present := os.LookupEnv("FERRUGO_STORAGE_ORACLE")
	if !present {
		t.Skip("set FERRUGO_STORAGE_ORACLE=1 to rebuild the unchanged pinned Rust sources")
	}
	if value != "1" {
		t.Fatal("FERRUGO_STORAGE_ORACLE must be exactly 1")
	}
	root := storageRoot()
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if explicit, ok := os.LookupEnv("FERRUGO_GO"); ok {
		if explicit == "" {
			t.Fatal("FERRUGO_GO is empty")
		}
		if !strings.ContainsAny(explicit, "/\\") {
			var err error
			goBin, err = exec.LookPath(explicit)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			goBin = explicit
			if !filepath.IsAbs(goBin) {
				goBin = filepath.Join(root, goBin)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "run", "./tools/port-rust-index-storage/oracle", "-check")
	cmd.Dir = root
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live original storage oracle: %v\n%s", err, output)
	}
}
