package rustclexer

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

type unicodeManifest struct {
	UnicodeVersion [3]uint8 `json:"unicode_version"`
	ScalarValues   int      `json:"scalar_values"`
	CodePoints     int      `json:"code_points"`
	OracleSHA256   string   `json:"oracle_flags_sha256"`
	FixtureSHA256  string   `json:"oracle_gzip_sha256"`
	Classes        []struct {
		Name         string `json:"name"`
		Count        int    `json:"scalar_count"`
		RangeCount   int    `json:"range_count"`
		RangesSHA256 string `json:"ranges_sha256"`
	} `json:"classes"`
}

func readUnicodeManifest(t *testing.T) unicodeManifest {
	t.Helper()
	data, err := os.ReadFile("../unicode_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest unicodeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func unicodeDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestUnicodeTablesWellFormed(t *testing.T) {
	manifest := readUnicodeManifest(t)
	if UnicodeVersion != [3]uint8{17, 0, 0} || UnicodeVersion != manifest.UnicodeVersion {
		t.Fatalf("Unicode version = %v, want pinned 17.0.0", UnicodeVersion)
	}
	tables := []struct {
		name   string
		ranges []unicodeRange
	}{
		{"XID_Start", xidStartRanges[:]},
		{"XID_Continue", xidContinueRanges[:]},
		{"Emoji", emojiRanges[:]},
	}
	if len(manifest.Classes) != len(tables) {
		t.Fatalf("manifest classes = %d, want %d", len(manifest.Classes), len(tables))
	}
	for i, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			var count int
			var encoded bytes.Buffer
			for index, interval := range table.ranges {
				if interval.low < 0 || interval.high > utf8.MaxRune || interval.low > interval.high {
					t.Fatalf("invalid range %d: %#v", index, interval)
				}
				if interval.low <= 0xdfff && interval.high >= 0xd800 {
					t.Fatalf("range %d includes non-scalar surrogate: %#v", index, interval)
				}
				if index > 0 && table.ranges[index-1].high+1 >= interval.low {
					t.Fatalf("range %d overlaps, adjoins, or precedes previous range", index)
				}
				count += int(interval.high - interval.low + 1)
				if err := binary.Write(&encoded, binary.LittleEndian, uint32(interval.low)); err != nil {
					t.Fatal(err)
				}
				if err := binary.Write(&encoded, binary.LittleEndian, uint32(interval.high)); err != nil {
					t.Fatal(err)
				}
			}
			want := manifest.Classes[i]
			if want.Name != table.name || count != want.Count || len(table.ranges) != want.RangeCount || unicodeDigest(encoded.Bytes()) != want.RangesSHA256 {
				t.Fatalf("%s table count, ranges, or SHA-256 differs from Rust oracle manifest", table.name)
			}
		})
	}
}

func TestUnicodePredicatesMatchPinnedRustEveryScalar(t *testing.T) {
	manifest := readUnicodeManifest(t)
	compressed, err := os.ReadFile("testdata/unicode_oracle.bin.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got := unicodeDigest(compressed); got != manifest.FixtureSHA256 {
		t.Fatalf("Rust reference fixture SHA-256 = %s, want %s", got, manifest.FixtureSHA256)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	flags, err := io.ReadAll(io.LimitReader(reader, utf8.MaxRune+2))
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != utf8.MaxRune+1 || len(flags) != manifest.CodePoints {
		t.Fatalf("Rust reference flags = %d bytes, want %d", len(flags), utf8.MaxRune+1)
	}
	if got := unicodeDigest(flags); got != manifest.OracleSHA256 {
		t.Fatalf("Rust reference flags SHA-256 = %s, want %s", got, manifest.OracleSHA256)
	}
	var checked int
	for cp, want := range flags {
		c := rune(cp)
		var got byte
		if isXIDStart(c) {
			got |= 1
		}
		if isXIDContinue(c) {
			got |= 2
		}
		if isEmojiChar(c) {
			got |= 4
		}
		if got != want {
			t.Fatalf("U+%04X: Go flags = %03b, pinned Rust flags = %03b", cp, got, want)
		}
		if utf8.ValidRune(c) {
			checked++
		}
	}
	if checked != 1_112_064 || checked != manifest.ScalarValues {
		t.Fatalf("checked %d scalar values, want 1112064", checked)
	}
	t.Logf("all %d Unicode scalar values match pinned Rust predicates", checked)
}

func TestUnicodePredicatesRejectInvalidRunes(t *testing.T) {
	for _, c := range []rune{-1 << 31, -1, 0xd800, 0xdfff, utf8.MaxRune + 1, 1<<31 - 1} {
		if isXIDStart(c) || isXIDContinue(c) || isEmojiChar(c) {
			t.Errorf("non-scalar U+%04X accepted", c)
		}
	}
}

func TestUnicodeOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_REGENERATE_UNICODE_ORACLE") != "1" {
		t.Skip("set FERRUGO_REGENERATE_UNICODE_ORACLE=1 to rebuild the pinned Rust oracle")
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	goTool := os.Getenv("FERRUGO_GO")
	if goTool == "" {
		if candidate, err := exec.LookPath("go"); err == nil {
			if output, err := exec.Command(candidate, "version").Output(); err == nil && bytes.HasPrefix(output, []byte("go version ")) {
				goTool = candidate
			}
		}
		if goTool == "" {
			goTool = filepath.Join(root, ".toolchain/go/bin/go")
		}
	} else if !strings.ContainsAny(goTool, `/\`) {
		goTool, err = exec.LookPath(goTool)
		if err != nil {
			t.Fatal(err)
		}
	} else if !filepath.IsAbs(goTool) {
		goTool = filepath.Join(root, goTool)
	}
	cmd := exec.Command(goTool, "run", "./tools/port-lexer-unicode", "-check", "-offline")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("regenerate pinned Rust Unicode oracle: %v\n%s", err, output)
	}
}
