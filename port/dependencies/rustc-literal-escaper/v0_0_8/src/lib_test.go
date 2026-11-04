package literalescaper

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
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type sourceManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	CrateSHA256 string `json:"crate_sha256"`
	Files       []struct {
		Path         string `json:"path"`
		SHA256       string `json:"sha256"`
		Role         string `json:"role"`
		Target       string `json:"target"`
		TargetSHA256 string `json:"target_sha256"`
		Status       string `json:"status"`
	} `json:"files"`
	UpstreamTests        int    `json:"upstream_tests"`
	UpstreamTestsPassed  bool   `json:"upstream_tests_passed"`
	PortedBenchmarks     int    `json:"ported_benchmark_workloads"`
	GoldenCases          int    `json:"golden_cases"`
	GoldenSHA256         string `json:"golden_sha256"`
	GoldenGzipSHA256     string `json:"golden_gzip_sha256"`
	MetadataHex          string `json:"metadata_hex"`
	MixedConstructorsHex string `json:"mixed_constructors_hex"`
	ScalarValues         int    `json:"scalar_values"`
	ScalarOperations     int    `json:"scalar_operations_per_value"`
	ScalarTraceSHA256    string `json:"scalar_trace_sha256"`
	ScalarTraceBytes     int64  `json:"scalar_trace_bytes"`
}

func literalDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func readManifest(t *testing.T) sourceManifest {
	t.Helper()
	data, err := os.ReadFile("../source-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest sourceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func encodeOperation(output []byte, op byte, source string) []byte {
	output = output[:0]
	output = append(output, 0, 0, 0, 0)
	appendEvent := func(span Range, kind byte, value uint32, err EscapeError) {
		if err != NoError {
			kind = 0
			value = uint32(err)
		}
		output = binary.LittleEndian.AppendUint32(output, uint32(span.Start))
		output = binary.LittleEndian.AppendUint32(output, uint32(span.End))
		output = append(output, kind)
		output = binary.LittleEndian.AppendUint32(output, value)
	}
	switch op {
	case 0:
		value, err := UnescapeChar(source)
		appendEvent(Range{0, len(source)}, 1, uint32(value), err)
	case 1:
		value, err := UnescapeByte(source)
		appendEvent(Range{0, len(source)}, 2, uint32(value), err)
	case 2, 5:
		callback := func(span Range, result Result[rune]) { appendEvent(span, 1, uint32(result.Value), result.Error) }
		if op == 2 {
			UnescapeStr(source, callback)
		} else {
			CheckRawStr(source, callback)
		}
	case 3, 6:
		callback := func(span Range, result Result[byte]) { appendEvent(span, 2, uint32(result.Value), result.Error) }
		if op == 3 {
			UnescapeByteStr(source, callback)
		} else {
			CheckRawByteStr(source, callback)
		}
	case 4:
		UnescapeCStr(source, func(span Range, result Result[MixedUnit]) {
			if result.Error != NoError {
				appendEvent(span, 0, 0, result.Error)
				return
			}
			if c, ok := result.Value.Char(); ok {
				appendEvent(span, 3, uint32(c.Get()), NoError)
			} else {
				b, _ := result.Value.HighByte()
				appendEvent(span, 4, uint32(b.Get()), NoError)
			}
		})
	case 7:
		CheckRawCStr(source, func(span Range, result Result[NonZeroChar]) {
			if result.Error != NoError {
				appendEvent(span, 0, 0, result.Error)
			} else {
				appendEvent(span, 5, uint32(result.Value.Get()), NoError)
			}
		})
	case 8, 9, 10, 11, 12, 13, 14, 15:
		CheckForErrors(source, Mode(op-8), func(span Range, err EscapeError) { appendEvent(span, 0, 0, err) })
	default:
		panic("invalid test operation")
	}
	binary.LittleEndian.PutUint32(output, uint32((len(output)-4)/13))
	return output
}

func TestCompleteArchiveManifest(t *testing.T) {
	manifest := readManifest(t)
	if manifest.Name != "rustc-literal-escaper" || manifest.Version != "0.0.8" || manifest.CrateSHA256 != "bfe6f213fb658c8fb95baabd5420393438cf5a98d707f5dd701d9197c705f71e" {
		t.Fatal("not the pinned dependency archive")
	}
	if len(manifest.Files) != 14 || manifest.UpstreamTests != 9 || !manifest.UpstreamTestsPassed || manifest.PortedBenchmarks != 27 {
		t.Fatal("incomplete source, test, or benchmark accounting")
	}
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if seen[file.Path] || len(file.SHA256) != 64 || file.Target == "" {
			t.Fatalf("invalid archive entry: %s", file.Path)
		}
		seen[file.Path] = true
		data, err := os.ReadFile(filepath.Join("..", file.Target))
		if err != nil {
			t.Fatal(err)
		}
		if literalDigest(data) != file.TargetSHA256 {
			t.Errorf("target for %s differs from source-manifest.json", file.Path)
		}
		if file.Status == "retained" && literalDigest(data) != file.SHA256 {
			t.Errorf("retained archive file %s differs", file.Path)
		}
	}
	for _, path := range []string{"src/lib.rs", "tests/test.rs", "benches/benches.rs", "LICENSE-MIT", "LICENSE-APACHE"} {
		if !seen[path] {
			t.Errorf("missing archive member %s", path)
		}
	}
}

func TestOriginalRustGoldenCases(t *testing.T) {
	manifest := readManifest(t)
	compressed, err := os.ReadFile("testdata/oracle.bin.gz")
	if err != nil {
		t.Fatal(err)
	}
	if literalDigest(compressed) != manifest.GoldenGzipSHA256 {
		t.Fatal("golden gzip checksum differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 128<<20))
	if err != nil {
		t.Fatal(err)
	}
	if literalDigest(data) != manifest.GoldenSHA256 {
		t.Fatal("golden oracle checksum differs")
	}
	if len(data) < 16 || string(data[:8]) != "FERRUGOE" {
		t.Fatal("invalid golden header")
	}
	position := 8
	read32 := func() int {
		if position+4 > len(data) {
			t.Fatal("truncated golden field")
		}
		value := int(binary.LittleEndian.Uint32(data[position:]))
		position += 4
		return value
	}
	metadataLength := read32()
	if position+metadataLength > len(data) {
		t.Fatal("truncated metadata")
	}
	if hex.EncodeToString(data[position:position+metadataLength]) != manifest.MetadataHex {
		t.Fatal("metadata does not match manifest")
	}
	position += metadataLength
	count := read32()
	if count != manifest.GoldenCases {
		t.Fatal("golden case count differs")
	}
	var output []byte
	var errors [24]bool
	var operations [16]bool
	for i := 0; i < count; i++ {
		if position == len(data) {
			t.Fatal("truncated golden operation")
		}
		op := data[position]
		position++
		if op >= 16 {
			t.Fatal("invalid golden operation")
		}
		operations[op] = true
		inputLength := read32()
		if position+inputLength > len(data) {
			t.Fatal("truncated input")
		}
		source := string(data[position : position+inputLength])
		position += inputLength
		resultLength := read32()
		if position+resultLength > len(data) {
			t.Fatal("truncated result")
		}
		want := data[position : position+resultLength]
		position += resultLength
		output = encodeOperation(output, op, source)
		if !bytes.Equal(output, want) {
			t.Fatalf("case %d op%d input%q\nGo %x\nRust%x", i, op, source, output, want)
		}
		for j := 4; j < len(want); j += 13 {
			if want[j+8] == 0 {
				code := binary.LittleEndian.Uint32(want[j+9:])
				if code > 23 {
					t.Fatal("invalid error code")
				}
				errors[code] = true
			}
		}
	}
	if position != len(data) {
		t.Fatal("trailing golden bytes")
	}
	for code := 1; code <= 23; code++ {
		if !errors[code] {
			t.Errorf("golden corpus does not exercise EscapeError %s", EscapeError(code))
		}
	}
	for op, seen := range operations {
		if !seen {
			t.Errorf("golden corpus does not exercise op%d", op)
		}
	}
	t.Logf("%d original Rust cases match all callback values, kinds, errors, warnings, and UTF-8 byte ranges", count)
}

func TestAllUnicodeScalarsAgainstOriginalRust(t *testing.T) {
	manifest := readManifest(t)
	hash := sha256.New()
	var output []byte
	var byteCount int64
	checked := 0
	for c := rune(0); c <= utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		direct := string(c)
		for op := byte(0); op < 8; op++ {
			output = encodeOperation(output, op, direct)
			hash.Write(output)
			byteCount += int64(len(output))
		}
		escaped := `\u{` + strconv.FormatUint(uint64(c), 16) + `}`
		for op := byte(0); op < 5; op++ {
			output = encodeOperation(output, op, escaped)
			hash.Write(output)
			byteCount += int64(len(output))
		}
		continuation := "\\\n" + direct
		for op := byte(2); op < 5; op++ {
			output = encodeOperation(output, op, continuation)
			hash.Write(output)
			byteCount += int64(len(output))
		}
		checked++
	}
	if checked != 1_112_064 || checked != manifest.ScalarValues || manifest.ScalarOperations != 16 {
		t.Fatal("incomplete scalar coverage")
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != manifest.ScalarTraceSHA256 || byteCount != manifest.ScalarTraceBytes {
		t.Fatalf("scalar trace = %s/%d bytes, original Rust = %s/%d bytes", got, byteCount, manifest.ScalarTraceSHA256, manifest.ScalarTraceBytes)
	}
	t.Logf("all %d scalars match 16 direct/Unicode-escape/continuation operations (%d exact event bytes)", checked, byteCount)
}

func TestErrorsAndModesAgainstOriginalMetadata(t *testing.T) {
	manifest := readManifest(t)
	var metadata []byte
	for err := ZeroChars; err <= MultipleSkippedLinesWarning; err++ {
		fatal := byte(0)
		if err.IsFatal() {
			fatal = 1
		}
		metadata = append(metadata, fatal)
		if err.String() == "" {
			t.Fatal("missing error name")
		}
	}
	for mode := Char; mode <= RawCStr; mode++ {
		double := byte(0)
		if mode.InDoubleQuotes() {
			double = 1
		}
		prefix := mode.PrefixNoRaw()
		metadata = append(metadata, double, byte(len(prefix)))
		metadata = append(metadata, prefix...)
	}
	if hex.EncodeToString(metadata) != manifest.MetadataHex {
		t.Fatal("error severity or mode properties differ from unchanged Rust")
	}
}

func TestMixedUnitAndNonZeroConversions(t *testing.T) {
	for value := 0; value <= 255; value++ {
		unit, err := MixedUnitFromByte(byte(value))
		nonzero, ok := NewNonZeroByte(byte(value))
		if value == 0 {
			if err != NulInCStr || ok {
				t.Fatal("zero byte accepted")
			}
			continue
		}
		if err != NoError || !ok || unit != MixedByte(nonzero) {
			t.Fatalf("byte conversion %d differs", value)
		}
		direct := MixedHighByte(nonzero)
		stored, isHighByte := direct.HighByte()
		if !isHighByte || stored.Get() != byte(value) || direct.Kind() != HighByteUnit {
			t.Fatalf("direct HighByte(%d) does not retain its variant and payload", value)
		}
		if _, isChar := direct.Char(); isChar {
			t.Fatalf("direct HighByte(%d) exposes a character payload", value)
		}
		if (direct == unit) != (value >= 128) {
			t.Fatalf("direct HighByte(%d) equality loses the variant tag", value)
		}
		if value < 128 {
			c, ok := unit.Char()
			if !ok || c.Get() != rune(value) || unit.Kind() != CharUnit {
				t.Fatal("ASCII byte is not a character")
			}
		} else {
			b, ok := unit.HighByte()
			if !ok || b.Get() != byte(value) || unit.Kind() != HighByteUnit {
				t.Fatal("high byte loses byte identity")
			}
		}
	}
	for _, value := range []rune{0, 1, 'a', 'ы', '🦀', utf8.MaxRune} {
		unit, err := MixedUnitFromChar(value)
		nonzero, ok := NewNonZeroChar(value)
		if value == 0 {
			if err != NulInCStr || ok {
				t.Fatal("zero character accepted")
			}
			continue
		}
		if err != NoError || !ok || unit != MixedChar(nonzero) {
			t.Fatal("nonzero char conversion differs")
		}
		c, isChar := unit.Char()
		if !isChar || c.Get() != value {
			t.Fatal("character loses Unicode value")
		}
	}
}

func TestMixedPublicVariantConstructionAgainstOriginalRust(t *testing.T) {
	var trace []byte
	appendUnit := func(unit MixedUnit) {
		if c, ok := unit.Char(); ok {
			if unit.Kind() != CharUnit {
				t.Fatal("character tag differs from accessor")
			}
			if _, ok := unit.HighByte(); ok {
				t.Fatal("character exposes a byte payload")
			}
			trace = append(trace, 3)
			trace = binary.LittleEndian.AppendUint32(trace, uint32(c.Get()))
		} else {
			b, ok := unit.HighByte()
			if !ok || unit.Kind() != HighByteUnit {
				t.Fatal("byte tag differs from accessor")
			}
			trace = append(trace, 4)
			trace = binary.LittleEndian.AppendUint32(trace, uint32(b.Get()))
		}
	}
	appendBool := func(value bool) {
		if value {
			trace = append(trace, 1)
		} else {
			trace = append(trace, 0)
		}
	}
	for _, value := range []byte{1, 65, 127, 128, 255} {
		nonzero, _ := NewNonZeroByte(value)
		nonzeroChar, _ := NewNonZeroChar(rune(value))
		direct, normalized, character := MixedHighByte(nonzero), MixedByte(nonzero), MixedChar(nonzeroChar)
		appendUnit(direct)
		appendUnit(normalized)
		appendUnit(character)
		appendBool(direct == normalized)
		appendBool(direct == character)
		appendBool(normalized == character)
		appendBool(direct == direct)
	}
	if hex.EncodeToString(trace) != readManifest(t).MixedConstructorsHex {
		t.Fatal("public enum construction, variant payload access, or equality differs from unchanged Rust")
	}
}

func TestLiteralEscaperOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_REGENERATE_LITERAL_ESCAPER_ORACLE") != "1" {
		t.Skip("set FERRUGO_REGENERATE_LITERAL_ESCAPER_ORACLE=1 to rebuild unchanged Rust dependency references")
	}
	root, err := filepath.Abs("../../../../..")
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
	cmd := exec.Command(goTool, "run", "./tools/port-literal-escaper", "-check", "-offline")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("regenerate literal escaper oracle: %v\n%s", err, output)
	}
}
