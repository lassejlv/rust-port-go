package unicode

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

var corePredicates = []struct {
	name   string
	lookup func(rune) bool
}{
	{"AlphabeticLookup", AlphabeticLookup},
	{"CaseIgnorableLookup", CaseIgnorableLookup},
	{"CFLookup", CFLookup},
	{"CNPlanes03Lookup", CNPlanes03Lookup},
	{"DefaultIgnorableCodePointLookup", DefaultIgnorableCodePointLookup},
	{"GraphemeExtendLookup", GraphemeExtendLookup},
	{"LowercaseLookup", LowercaseLookup},
	{"LTLookup", LTLookup},
	{"NLookup", NLookup},
	{"UppercaseLookup", UppercaseLookup},
	{"WhiteSpaceLookup", WhiteSpaceLookup},
	{"XIDContinueLookup", XIDContinueLookup},
	{"XIDStartLookup", XIDStartLookup},
}

var coreConversions = []struct {
	name    string
	convert func(rune) [3]rune
}{
	{"ToLower", ToLower}, {"ToUpper", ToUpper},
	{"ToTitle", ToTitle}, {"ToCasefold", ToCasefold},
}

type coreManifest struct {
	SourceBlob     string   `json:"source_blob"`
	SourceLines    int      `json:"source_lines"`
	UnicodeVersion [3]uint8 `json:"unicode_version"`
	ScalarValues   int      `json:"scalar_values"`
	Predicates     []string `json:"predicates"`
	Conversions    []string `json:"conversions"`
	OracleSHA256   string   `json:"oracle_sha256"`
	FixtureSHA256  string   `json:"fixture_gzip_sha256"`
	GoFileSHA256   string   `json:"go_file_sha256"`
	License        string   `json:"license"`
	LicenseSHA256  string   `json:"license_sha256"`
	Tables         []struct {
		Module string `json:"module"`
		Name   string `json:"rust_name"`
	} `json:"tables"`
}

func coreDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readCoreManifest(t *testing.T) coreManifest {
	t.Helper()
	data, err := os.ReadFile("unicode_data_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest coreManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestCoreUnicodeProvenance(t *testing.T) {
	manifest := readCoreManifest(t)
	if manifest.SourceBlob != "84414a72df88c4718d040cb93777d7a4b265e8d4" || manifest.SourceLines != 1786 {
		t.Fatal("core Unicode source is not the complete pinned file")
	}
	if UnicodeVersion != [3]uint8{18, 0, 0} || manifest.UnicodeVersion != UnicodeVersion {
		t.Fatalf("Unicode version = %v, want pinned 18.0.0", UnicodeVersion)
	}
	if len(manifest.Tables) != 33 || len(manifest.Predicates) != len(corePredicates) || len(manifest.Conversions) != len(coreConversions) {
		t.Fatal("manifest must retain all 33 source tables, 13 predicates, and 4 conversions")
	}
	for i, predicate := range corePredicates {
		if predicate.name != manifest.Predicates[i] {
			t.Fatalf("predicate bit %d differs", i)
		}
	}
	for i, conversion := range coreConversions {
		if conversion.name != manifest.Conversions[i] {
			t.Fatalf("conversion bit %d differs", i)
		}
	}
	generated, err := os.ReadFile("unicode_data.go")
	if err != nil {
		t.Fatal(err)
	}
	if coreDigest(generated) != manifest.GoFileSHA256 {
		t.Fatal("complete generated Go file differs from manifest")
	}
	license, err := os.ReadFile("LICENSE-UNICODE")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.License != "Unicode-3.0" || coreDigest(license) != manifest.LicenseSHA256 {
		t.Fatal("pinned Unicode license differs")
	}
}

func TestCoreUnicodeAllScalarsAgainstUntouchedRust(t *testing.T) {
	manifest := readCoreManifest(t)
	compressed, err := os.ReadFile("testdata/core_unicode_oracle.bin.gz")
	if err != nil {
		t.Fatal(err)
	}
	if coreDigest(compressed) != manifest.FixtureSHA256 {
		t.Fatal("Rust fixture gzip SHA-256 differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if coreDigest(data) != manifest.OracleSHA256 {
		t.Fatal("Rust oracle SHA-256 differs")
	}
	if len(data) < 16 || string(data[:8]) != "FERRUGOC" || !bytes.Equal(data[8:12], []byte{18, 0, 0, 0}) {
		t.Fatal("invalid Rust oracle header")
	}
	if binary.LittleEndian.Uint32(data[12:16]) != 1_112_064 {
		t.Fatal("invalid scalar count in Rust oracle")
	}
	position, checked := 16, 0
	for c := rune(0); c <= utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		if position+3 > len(data) {
			t.Fatalf("truncated reference at U+%04X", c)
		}
		wantFlags := binary.LittleEndian.Uint16(data[position:])
		changes := data[position+2]
		position += 3
		if wantFlags>>len(corePredicates) != 0 || changes>>len(coreConversions) != 0 {
			t.Fatalf("invalid flags at U+%04X", c)
		}
		for i, predicate := range corePredicates {
			want := wantFlags&(1<<i) != 0
			if got := predicate.lookup(c); got != want {
				t.Fatalf("%s(U+%04X) = %t, original Rust = %t", predicate.name, c, got, want)
			}
		}
		for i, conversion := range coreConversions {
			want := [3]rune{c, 0, 0}
			if changes&(1<<i) != 0 {
				if position+12 > len(data) {
					t.Fatalf("truncated conversion at U+%04X", c)
				}
				for slot := range want {
					want[slot] = rune(binary.LittleEndian.Uint32(data[position:]))
					position += 4
				}
			}
			if got := conversion.convert(c); got != want {
				t.Fatalf("%s(U+%04X) = %U, original Rust = %U", conversion.name, c, got, want)
			}
		}
		checked++
	}
	if position != len(data) {
		t.Fatalf("Rust reference has %d trailing bytes", len(data)-position)
	}
	if checked != 1_112_064 || checked != manifest.ScalarValues {
		t.Fatalf("checked %d scalars, want 1112064", checked)
	}
	t.Logf("all %d scalar values match all 13 original non-ASCII predicates and all four [3]char conversions", checked)
}

func TestCoreUnicodeNonASCIIContractAndInvalidRunes(t *testing.T) {
	for _, predicate := range corePredicates {
		for c := rune(0); c < 128; c++ {
			if predicate.lookup(c) {
				t.Errorf("%s accepted ASCII U+%04X", predicate.name, c)
			}
		}
		for _, c := range []rune{-1 << 31, -1, 0xd800, 0xdfff, utf8.MaxRune + 1, 1<<31 - 1} {
			if predicate.lookup(c) {
				t.Errorf("%s accepted invalid rune U+%04X", predicate.name, c)
			}
		}
	}
	for _, conversion := range coreConversions {
		for _, c := range []rune{-1, 0xd800, 0xdfff, utf8.MaxRune + 1} {
			func() {
				defer func() {
					if recover() == nil {
						t.Errorf("%s accepted invalid rune U+%04X", conversion.name, c)
					}
				}()
				conversion.convert(c)
			}()
		}
	}
}

func TestCoreUnicodeCompressedTableInvariants(t *testing.T) {
	skiplists := []struct {
		name    string
		runs    []shortOffsetRunHeader
		offsets []uint8
	}{
		{"alphabetic", alphabeticShortOffsetRuns[:], alphabeticOffsets[:]},
		{"case_ignorable", caseIgnorableShortOffsetRuns[:], caseIgnorableOffsets[:]},
		{"cf", cfShortOffsetRuns[:], cfOffsets[:]},
		{"cn_planes_0_3", cnPlanes03ShortOffsetRuns[:], cnPlanes03Offsets[:]},
		{"default_ignorable_code_point", defaultIgnorableCodePointShortOffsetRuns[:], defaultIgnorableCodePointOffsets[:]},
		{"grapheme_extend", graphemeExtendShortOffsetRuns[:], graphemeExtendOffsets[:]},
		{"lt", ltShortOffsetRuns[:], ltOffsets[:]},
		{"n", nShortOffsetRuns[:], nOffsets[:]},
		{"xid_continue", xidContinueShortOffsetRuns[:], xidContinueOffsets[:]},
		{"xid_start", xidStartShortOffsetRuns[:], xidStartOffsets[:]},
	}
	for _, table := range skiplists {
		t.Run(table.name, func(t *testing.T) {
			if table.runs[0].startIndex() != 0 || table.runs[len(table.runs)-1].prefixSum() <= utf8.MaxRune {
				t.Fatal("missing skiplist boundaries")
			}
			for i, header := range table.runs {
				if header.startIndex() >= len(table.offsets) {
					t.Fatalf("run %d starts outside offsets", i)
				}
				if i > 0 && (header.startIndex() <= table.runs[i-1].startIndex() || header.prefixSum() <= table.runs[i-1].prefixSum()) {
					t.Fatalf("run %d is not strictly ordered", i)
				}
			}
		})
	}
	bitsets := []struct {
		name      string
		chunkMap  []uint8
		chunks    [][16]uint8
		canonical []uint64
		mapping   [][2]uint8
	}{
		{"lowercase", lowercaseBitsetChunksMap[:], lowercaseBitsetIndexChunks[:], lowercaseBitsetCanonical[:], lowercaseBitsetMapping[:]},
		{"uppercase", uppercaseBitsetChunksMap[:], uppercaseBitsetIndexChunks[:], uppercaseBitsetCanonical[:], uppercaseBitsetMapping[:]},
	}
	for _, table := range bitsets {
		t.Run(table.name, func(t *testing.T) {
			for i, chunk := range table.chunkMap {
				if int(chunk) >= len(table.chunks) {
					t.Fatalf("chunk map %d out of bounds", i)
				}
			}
			for i, chunk := range table.chunks {
				for _, index := range chunk {
					if int(index) >= len(table.canonical)+len(table.mapping) {
						t.Fatalf("chunk %d canonical index out of bounds", i)
					}
				}
			}
			for i, mapping := range table.mapping {
				if int(mapping[0]) >= len(table.canonical) {
					t.Fatalf("mapping %d out of bounds", i)
				}
			}
		})
	}
	for i, lut := range []*l1LUT{&lowercaseLUT, &uppercaseLUT, &titlecaseLUT, &casefoldLUT} {
		for plane, l2 := range lut.l2LUTs {
			for j, entry := range l2.singles {
				if j > 0 && entry.range_.start <= l2.singles[j-1].range_.end() {
					t.Fatalf("LUT %d plane %d overlapping single %d", i, plane, j)
				}
			}
			for j, entry := range l2.multis {
				if j > 0 && entry.key <= l2.multis[j-1].key {
					t.Fatalf("LUT %d plane %d unordered multi %d", i, plane, j)
				}
			}
			for _, entry := range l2.exceptions {
				for _, c := range entry.output {
					if !validScalar(c) {
						t.Fatalf("LUT %d invalid exception output", i)
					}
				}
			}
		}
	}
}

func TestCoreUnicodeOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_REGENERATE_CORE_UNICODE_ORACLE") != "1" {
		t.Skip("set FERRUGO_REGENERATE_CORE_UNICODE_ORACLE=1 to rebuild the untouched Rust core oracle")
	}
	root, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	goTool := os.Getenv("FERRUGO_GO")
	if goTool == "" {
		if candidate, err := exec.LookPath("go"); err == nil {
			if version, err := exec.Command(candidate, "version").Output(); err == nil && bytes.HasPrefix(version, []byte("go version ")) {
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
	cmd := exec.Command(goTool, "run", "./tools/port-core-unicode", "-check")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("regenerate core Unicode oracle: %v\n%s", err, output)
	}
}
