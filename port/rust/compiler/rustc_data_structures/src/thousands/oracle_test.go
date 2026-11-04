// SPDX-License-Identifier: MIT OR Apache-2.0
package thousands

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/oracle.tsv.gz
var pinnedOracle []byte

//go:embed oracle_manifest.json
var pinnedManifest []byte

func TestPinnedOriginalOracle(t *testing.T) {
	var meta struct {
		Revision      string         `json:"rust_revision"`
		SourceSHA     string         `json:"source_sha256"`
		TestsSHA      string         `json:"tests_sha256"`
		Tests         int            `json:"upstream_tests"`
		Cases         map[string]int `json:"cases"`
		RawSHA        string         `json:"oracle_raw_sha256"`
		CompressedSHA string         `json:"fixture_sha256"`
	}
	if err := json.Unmarshal(pinnedManifest, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || meta.SourceSHA != "adcd6db15c34f2fbf4aa9804427cd2f86c8e9e63be5a07316b1ab5e6240d21fb" || meta.TestsSHA != "9596a03a2af2075b20952c1483b31d43a6b9d76918b9184148a96bb60b6de56f" || meta.Tests != 4 {
		t.Fatal("fixture provenance does not match the pinned complete module and tests")
	}
	hash := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if hash(pinnedOracle) != meta.CompressedSHA {
		t.Fatal("compressed fixture hash differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(pinnedOracle))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if hash(raw) != meta.RawSHA {
		t.Fatal("original Rust output hash differs")
	}
	counts := map[string]int{"u": 0, "i": 0, "f": 0, "s": 0}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 3 {
			t.Fatalf("malformed reference record %q", scanner.Text())
		}
		counts[fields[0]]++
		wantBytes, err := hex.DecodeString(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		var got string
		switch fields[0] {
		case "u":
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if strconv.IntSize == 32 && value > math.MaxUint32 {
				continue
			}
			got = UsizeWithUnderscores(uint(value))
		case "i":
			value, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if strconv.IntSize == 32 && (value < math.MinInt32 || value > math.MaxInt32) {
				continue
			}
			got = IsizeWithUnderscores(int(value))
		case "f":
			bits, err := strconv.ParseUint(fields[1], 16, 64)
			if err != nil {
				t.Fatal(err)
			}
			got = F64p1WithUnderscores(math.Float64frombits(bits))
		case "s":
			value, err := hex.DecodeString(fields[1])
			if err != nil {
				t.Fatal(err)
			}
			got = formatWithUnderscores(string(value))
		default:
			t.Fatalf("unknown reference operation %q", fields[0])
		}
		if got != string(wantBytes) {
			t.Fatalf("original Rust mismatch for %s %s: Go %q; Rust %q", fields[0], fields[1], got, wantBytes)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for kind, want := range meta.Cases {
		if counts[kind] != want {
			t.Fatalf("reference %s cases = %d; want %d", kind, counts[kind], want)
		}
	}
	if counts["f"] < 10_000 {
		t.Fatal("float reference coverage unexpectedly reduced")
	}
}

func TestLiveOriginalRustOracle(t *testing.T) {
	compiler, requested := os.LookupEnv("FERRUGO_RUSTC")
	if !requested {
		t.Skip("set FERRUGO_RUSTC to compile the pinned whole Rust module and its original four tests")
	}
	if compiler == "" {
		t.Fatal("FERRUGO_RUSTC was explicitly empty")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repository root not found")
		}
		root = parent
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin/go"), "run", "./tools/port-rust-thousands", "-root", root, "-rustc", compiler, "-check")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("original Rust oracle: %v\n%s", err, output)
	} else {
		t.Log(strings.TrimSpace(string(output)))
	}
}
