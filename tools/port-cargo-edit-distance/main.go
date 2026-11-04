// Command port-cargo-edit-distance regenerates pinned Unicode tables and Rust
// reference fixtures for the complete Cargo edit_distance.rs source port.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const cargoRevision = "24de4721844c7a4d839072bab6fb66d41c9bc1f4"
const sourceHash = "073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407"
const sourceBlob = "e67259956fc26a53b1be30efb371161249dfff8a"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const points = 0x110000

type mapping struct {
	source uint32
	lower  []uint32
}
type interval struct{ first, last uint32 }
type metadata struct {
	Format               int      `json:"format"`
	CargoRevision        string   `json:"cargo_revision"`
	SourcePath           string   `json:"source_path"`
	SourceGitBlob        string   `json:"source_git_blob"`
	SourceSHA256         string   `json:"source_sha256"`
	Rustc                string   `json:"rustc"`
	UnicodeVersion       [3]uint8 `json:"unicode_version"`
	ScalarValues         int      `json:"scalar_values"`
	LowercaseMappings    int      `json:"lowercase_mappings"`
	SigmaCasedRanges     int      `json:"sigma_cased_ranges"`
	CaseIgnorableRanges  int      `json:"case_ignorable_ranges"`
	DistanceCases        uint32   `json:"distance_cases"`
	SuggestionCases      uint32   `json:"suggestion_cases"`
	OracleSourceSHA256   string   `json:"oracle_source_sha256"`
	UnicodeOracleSHA256  string   `json:"unicode_oracle_sha256"`
	UnicodeFixtureSHA256 string   `json:"unicode_fixture_sha256"`
	CaseOracleSHA256     string   `json:"case_oracle_sha256"`
	CaseFixtureSHA256    string   `json:"case_fixture_sha256"`
	TablesSHA256         string   `json:"tables_sha256"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	compilerDefault := os.Getenv("FERRUGO_RUSTC")
	if compilerDefault == "" {
		compilerDefault = ".toolchain/rust/bin/rustc"
	}
	sourceDefault := os.Getenv("FERRUGO_CARGO_SOURCE")
	if sourceDefault == "" {
		sourceDefault = ".toolchain/upstream-cargo"
	}
	rustc := flag.String("rustc", compilerDefault, "pinned Rust compiler name/path (defaults to FERRUGO_RUSTC)")
	cargoSource := flag.String("cargo-source", sourceDefault, "pinned Cargo checkout (defaults to FERRUGO_CARGO_SOURCE)")
	check := flag.Bool("check", false, "verify generated files without changing them")
	flag.Parse()
	if err := run(*root, *rustc, *cargoSource, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-cargo-edit-distance:", err)
		os.Exit(1)
	}
}
func run(root, rustc, cargoSource string, check bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(rustc) {
		if strings.ContainsAny(rustc, "/\\") {
			rustc = filepath.Join(root, rustc)
		} else {
			rustc, err = exec.LookPath(rustc)
			if err != nil {
				return fmt.Errorf("find Rust compiler: %w", err)
			}
		}
	}
	version, err := exec.Command(rustc, "--version").Output()
	if err != nil {
		return fmt.Errorf("read Rust compiler version: %w", err)
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, strings.TrimSpace(string(version)))
	}
	upstream := cargoSource
	if !filepath.IsAbs(upstream) {
		upstream = filepath.Join(root, upstream)
	}
	revision, err := exec.Command("git", "-C", upstream, "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(revision)) != cargoRevision {
		return fmt.Errorf("unexpected Cargo revision %s", strings.TrimSpace(string(revision)))
	}
	source := filepath.Join(upstream, "src/util/edit_distance.rs")
	original, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if digest(original) != sourceHash {
		return fmt.Errorf("pinned Cargo edit_distance.rs source hash mismatch")
	}
	oraclePath := filepath.Join(root, "tools/port-cargo-edit-distance/oracle.rs")
	oracleSource, err := os.ReadFile(oraclePath)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "cargo-edit-distance-oracle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	executable := filepath.Join(temp, "oracle")
	compile := exec.Command(rustc, "--edition=2024", "-C", "opt-level=2", oraclePath, "-o", executable)
	compile.Env = append(os.Environ(), "CARGO_EDIT_DISTANCE_SOURCE="+source)
	if diagnostics, err := compile.CombinedOutput(); err != nil {
		return fmt.Errorf("compile pinned oracle: %w\n%s", err, diagnostics)
	}
	upstreamTests := filepath.Join(temp, "upstream-tests")
	compileTests := exec.Command(rustc, "--edition=2024", "--test", "-C", "opt-level=2", source, "-o", upstreamTests)
	if diagnostics, err := compileTests.CombinedOutput(); err != nil {
		return fmt.Errorf("compile original upstream test: %w\n%s", err, diagnostics)
	}
	if diagnostics, err := exec.Command(upstreamTests).CombinedOutput(); err != nil {
		return fmt.Errorf("run original upstream test: %w\n%s", err, diagnostics)
	}
	unicode, err := exec.Command(executable, "unicode").Output()
	if err != nil {
		return fmt.Errorf("run Unicode oracle: %w", err)
	}
	flags, mappings, err := readUnicode(unicode)
	if err != nil {
		return err
	}
	cased, ignorable := ranges(flags, 1), ranges(flags, 2)
	tables, err := renderTables(mappings, cased, ignorable)
	if err != nil {
		return err
	}
	cases, err := exec.Command(executable, "cases").Output()
	if err != nil {
		return fmt.Errorf("run distance/suggestion oracle: %w", err)
	}
	distanceCount, suggestionCount, err := countCases(cases)
	if err != nil {
		return err
	}
	unicodeFixture, err := compressed(unicode)
	if err != nil {
		return err
	}
	caseFixture, err := compressed(cases)
	if err != nil {
		return err
	}
	meta := metadata{Format: 1, CargoRevision: cargoRevision, SourcePath: "src/util/edit_distance.rs", SourceGitBlob: sourceBlob, SourceSHA256: sourceHash, Rustc: rustVersion, UnicodeVersion: [3]uint8{17, 0, 0}, ScalarValues: points - 0x800, LowercaseMappings: len(mappings), SigmaCasedRanges: len(cased), CaseIgnorableRanges: len(ignorable), DistanceCases: distanceCount, SuggestionCases: suggestionCount, OracleSourceSHA256: digest(oracleSource), UnicodeOracleSHA256: digest(unicode), UnicodeFixtureSHA256: digest(unicodeFixture), CaseOracleSHA256: digest(cases), CaseFixtureSHA256: digest(caseFixture), TablesSHA256: digest(tables)}
	manifest, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	manifest = append(manifest, '\n')
	files := map[string][]byte{
		"port/cargo/src/util/edit_distance_unicode.go":              tables,
		"port/cargo/src/util/edit_distance_unicode_manifest.json":   manifest,
		"port/cargo/src/util/testdata/edit_distance_unicode.bin.gz": unicodeFixture,
		"port/cargo/src/util/testdata/edit_distance_cases.bin.gz":   caseFixture,
	}
	for path, data := range files {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if check {
			existing, err := os.ReadFile(absolute)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("generated file differs: %s", path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(absolute), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(absolute, data, 0644); err != nil {
				return err
			}
		}
	}
	action := "generated"
	if check {
		action = "verified"
	}
	fmt.Printf("%s Rust Unicode 17.0.0: %d scalar values, %d lowercase mappings, %d distance and %d suggestion cases\n", action, points-0x800, len(mappings), distanceCount, suggestionCount)
	return nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func compressed(data []byte) ([]byte, error) {
	var out bytes.Buffer
	writer, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(data); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func readUnicode(data []byte) ([]byte, []mapping, error) {
	reader := bytes.NewReader(data)
	header := make([]byte, 9)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(header, []byte{'C', 'E', 'D', 'U', '1', 0, 17, 0, 0}) {
		return nil, nil, fmt.Errorf("invalid Unicode oracle header")
	}
	flags := make([]byte, points)
	if _, err := io.ReadFull(reader, flags); err != nil {
		return nil, nil, err
	}
	count, err := u32(reader)
	if err != nil {
		return nil, nil, err
	}
	mappings := make([]mapping, 0, count)
	for range count {
		cp, err := u32(reader)
		if err != nil {
			return nil, nil, err
		}
		length, err := reader.ReadByte()
		if err != nil {
			return nil, nil, err
		}
		if length == 0 || length > 3 {
			return nil, nil, fmt.Errorf("invalid lowercase expansion length %d", length)
		}
		item := mapping{source: cp}
		for range length {
			value, err := u32(reader)
			if err != nil {
				return nil, nil, err
			}
			item.lower = append(item.lower, value)
		}
		mappings = append(mappings, item)
	}
	if reader.Len() != 0 {
		return nil, nil, fmt.Errorf("trailing Unicode oracle data")
	}
	return flags, mappings, nil
}
func ranges(flags []byte, mask byte) []interval {
	var result []interval
	for i := 0; i < len(flags); i++ {
		if flags[i]&mask == 0 {
			continue
		}
		start := i
		for i+1 < len(flags) && flags[i+1]&mask != 0 {
			i++
		}
		result = append(result, interval{uint32(start), uint32(i)})
	}
	return result
}
func renderTables(mappings []mapping, cased, ignorable []interval) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprint(&out, "// Code generated by tools/port-cargo-edit-distance; DO NOT EDIT.\n// Pinned Rust 1.99.1 / Unicode 17.0.0. MIT OR Apache-2.0.\n// Unicode data terms: edit_distance_unicode_LICENSE.\npackage util\n\n// EditDistanceUnicodeVersion identifies the pinned Rust lowercase behavior.\nconst EditDistanceUnicodeVersion = \"17.0.0\"\n\ntype editDistanceMapping struct { source rune; count uint8; lower [3]rune }\ntype editDistanceRuneRange struct { first,last rune }\n\nvar editDistanceLowercaseMappings = []editDistanceMapping{\n")
	for _, item := range mappings {
		fmt.Fprintf(&out, "{0x%X,%d,[3]rune{", item.source, len(item.lower))
		for _, value := range item.lower {
			fmt.Fprintf(&out, "0x%X,", value)
		}
		fmt.Fprintln(&out, "}},")
	}
	fmt.Fprintln(&out, "}")
	for _, group := range []struct {
		name, comment string
		values        []interval
	}{{"editDistanceSigmaCased", "Non-case-ignorable Cased scalars, exactly as Rust's final-sigma context observes them.", cased}, {"editDistanceCaseIgnorable", "Rust Case_Ignorable scalars, inferred by complementary final-sigma probes.", ignorable}} {
		fmt.Fprintf(&out, "\n// %s\nvar %s = []editDistanceRuneRange{\n", group.comment, group.name)
		for _, span := range group.values {
			fmt.Fprintf(&out, "{0x%X,0x%X},\n", span.first, span.last)
		}
		fmt.Fprintln(&out, "}")
	}
	return format.Source(out.Bytes())
}
func u32(reader *bytes.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(reader, binary.LittleEndian, &value)
	return value, err
}
func skipString(reader *bytes.Reader) error {
	length, err := u32(reader)
	if err != nil {
		return err
	}
	_, err = reader.Seek(int64(length), io.SeekCurrent)
	return err
}
func countCases(data []byte) (uint32, uint32, error) {
	reader := bytes.NewReader(data)
	header := make([]byte, 6)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, 0, err
	}
	if string(header) != "CEDC1\x00" {
		return 0, 0, fmt.Errorf("invalid case header")
	}
	distances, err := u32(reader)
	if err != nil {
		return 0, 0, err
	}
	for range distances {
		for range 2 {
			if err := skipString(reader); err != nil {
				return 0, 0, err
			}
		}
		if _, err := reader.Seek(8, io.SeekCurrent); err != nil {
			return 0, 0, err
		}
		present, err := reader.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		if present != 0 {
			if _, err := u32(reader); err != nil {
				return 0, 0, err
			}
		}
	}
	suggestions, err := u32(reader)
	if err != nil {
		return 0, 0, err
	}
	for range suggestions {
		for range 2 {
			if err := skipString(reader); err != nil {
				return 0, 0, err
			}
		}
		count, err := u32(reader)
		if err != nil {
			return 0, 0, err
		}
		for range count {
			if err := skipString(reader); err != nil {
				return 0, 0, err
			}
		}
		if _, err := u32(reader); err != nil {
			return 0, 0, err
		}
		if err := skipString(reader); err != nil {
			return 0, 0, err
		}
	}
	if reader.Len() != 0 {
		return 0, 0, fmt.Errorf("trailing case fixture data")
	}
	return distances, suggestions, nil
}
