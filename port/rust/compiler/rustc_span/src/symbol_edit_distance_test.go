// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSymbolOriginalEditDistanceProfiles(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained native oracle records the 64-bit usize profile; original source tests remain architecture independent")
	}
	encoded, err := os.ReadFile("symbol_edit_distance_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records       int    `json:"records"`
		FixtureSHA256 string `json:"fixture_sha256"`
		RawSHA256     string `json:"raw_sha256"`
	}
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/symbol_edit_distance.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if digest(compressed) != manifest.FixtureSHA256 {
		t.Fatal("edit-distance compressed fixture checksum mismatch")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, 8<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) >= 8<<20 || digest(raw) != manifest.RawSHA256 {
		t.Fatal("edit-distance raw fixture checksum/bound mismatch")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 65536)
	records := 0
	for scanner.Scan() {
		records++
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 6 || len(parts[0]) != 4 {
			t.Fatalf("record %d malformed", records)
		}
		a, err := hex.DecodeString(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		b, err := hex.DecodeString(parts[3])
		if err != nil {
			t.Fatal(err)
		}
		limit, err := strconv.ParseUint(parts[4], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		profile := SpanEncodingConfig{DebugAssertions: parts[0][1] == '1', OverflowChecks: parts[0][3] == '1'}
		actual := func() (out string) {
			defer func() {
				if recover() != nil {
					out = "panic"
				}
			}()
			var result Option[uint]
			switch parts[1] {
			case "D":
				result = EditDistance(string(a), string(b), uint(limit))
			case "S":
				result = EditDistanceWithSubstringsWithConfig(string(a), string(b), uint(limit), profile, rustfmt.Context{})
			default:
				t.Fatalf("unknown operation: %q", parts[1])
			}
			if distance, present := result.Get(); present {
				return strconv.FormatUint(uint64(distance), 10)
			}
			return "none"
		}()
		if actual != parts[5] {
			t.Fatalf("record %d %s: got %s, want %s", records, scanner.Text(), actual, parts[5])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if records != manifest.Records || records != 53248 {
		t.Fatalf("records: %d, want %d", records, manifest.Records)
	}
}

func TestSymbolEditDistanceOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_RUN_SYMBOL_EDIT_ORACLE") != "1" {
		t.Skip("set FERRUGO_RUN_SYMBOL_EDIT_ORACLE=1 to rebuild unchanged native edit-distance bodies")
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
	command := exec.CommandContext(ctx, "python3", filepath.Join(root, "tools/port-rust-symbol/generate_edit_distance.py"), "--check", "--offline")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatal(fmt.Sprintf("live edit-distance oracle: %v\n%s", err, output))
	}
}
