package binarysearchutil

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type oracleElement struct {
	value int64
	index uint32
}
type comparison struct{ a, b int64 }

func oracleU32(t *testing.T, reader *bytes.Reader) uint32 {
	t.Helper()
	var value uint32
	if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func oracleI64(t *testing.T, reader *bytes.Reader) int64 {
	t.Helper()
	var value int64
	if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func oracleByte(t *testing.T, reader *bytes.Reader) byte {
	t.Helper()
	value, err := reader.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func oracleString(t *testing.T, reader *bytes.Reader) string {
	t.Helper()
	data := make([]byte, oracleU32(t, reader))
	if _, err := io.ReadFull(reader, data); err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func oracleFixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	compressed, err := os.ReadFile("testdata/oracle.bin.gz")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
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
	return raw, compressed
}
func sourceDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func unsignedAbs(value int64) uint64 {
	if value < 0 {
		return uint64(-(value + 1)) + 1
	}
	return uint64(value)
}
func euclideanRemainder(value, modulus int64) int64 {
	result := value % modulus
	if result < 0 {
		result += modulus
	}
	return result
}
func oracleCompare(mode byte, a, b int64, count int) int {
	switch mode {
	case 0:
		return cmp.Compare(a, b)
	case 1:
		return cmp.Compare(b, a)
	case 2:
		if first := cmp.Compare(unsignedAbs(a), unsignedAbs(b)); first != 0 {
			return first
		}
		return cmp.Compare(a, b)
	case 3:
		if first := cmp.Compare(euclideanRemainder(a, 5), euclideanRemainder(b, 5)); first != 0 {
			return first
		}
		return cmp.Compare(a, b)
	case 4:
		return 0
	case 5:
		x, y := euclideanRemainder(a, 3), euclideanRemainder(b, 3)
		if x == y {
			return 0
		}
		if (x+1)%3 == y {
			return -1
		}
		return 1
	case 6:
		result := cmp.Compare(a, b)
		if count%2 == 0 {
			return -result
		}
		return result
	default:
		panic("unknown oracle mode")
	}
}

type oracleTrace struct {
	result      []oracleElement
	panicValue  any
	keys        []uint32
	comparisons []comparison
	operators   []byte
}

func executeOracleCase(data []oracleElement, target int64, mode byte, keyPanic, comparePanic uint32, operatorPanic byte, operatorPanicAt uint32, comparatorAdapter bool) (trace oracleTrace) {
	defer func() { trace.panicValue = recover() }()
	counts := [7]uint32{}
	keyFn := func(value *oracleElement) int64 {
		trace.keys = append(trace.keys, value.index)
		if keyPanic != 0 && uint32(len(trace.keys)) == keyPanic {
			panic("key panic")
		}
		return value.value
	}
	operation := func(operator byte, a, b int64) int {
		trace.comparisons = append(trace.comparisons, comparison{a, b})
		trace.operators = append(trace.operators, operator)
		counts[operator]++
		if comparePanic != 0 && uint32(len(trace.comparisons)) == comparePanic {
			panic("compare panic")
		}
		if operatorPanicAt != 0 && operatorPanic == operator && counts[operator] == operatorPanicAt {
			switch operator {
			case 1:
				panic("lt panic")
			case 2:
				panic("ne panic")
			case 3:
				panic("le panic")
			default:
				panic("unexpected operator")
			}
		}
		result := oracleCompare(mode, a, b, len(trace.comparisons))
		if result < 0 {
			return -23
		}
		if result > 0 {
			return 71
		}
		return 0
	}
	if comparatorAdapter {
		trace.result = BinarySearchSliceBy(data, keyFn, target, func(a, b int64) int { return operation(0, a, b) })
	} else {
		trace.result = BinarySearchSliceWithOperators(data, keyFn, target, KeyOperators[int64]{
			Less:        func(a, b int64) bool { return operation(1, a, b) < 0 },
			NotEqual:    func(a, b int64) bool { return operation(2, a, b) != 0 },
			LessOrEqual: func(a, b int64) bool { return operation(3, a, b) <= 0 },
		})
	}
	return trace
}

func TestOriginalRustOracle(t *testing.T) {
	raw, _ := oracleFixture(t)
	reader := bytes.NewReader(raw)
	header := make([]byte, 6)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatal(err)
	}
	if string(header) != "RBSU2\x00" {
		t.Fatal("invalid Rust oracle header")
	}
	count := oracleU32(t, reader)
	panicCount, operatorPanicCount, comparatorCases := 0, 0, 0
	for caseIndex := range count {
		mode := oracleByte(t, reader)
		target := oracleI64(t, reader)
		data := make([]oracleElement, oracleU32(t, reader))
		for i := range data {
			data[i] = oracleElement{oracleI64(t, reader), uint32(i)}
		}
		keyPanic, comparePanic := oracleU32(t, reader), oracleU32(t, reader)
		operatorPanic, operatorPanicAt := oracleByte(t, reader), oracleU32(t, reader)
		wantPanic := oracleByte(t, reader) != 0
		expectedStart, expectedLength := uint32(0), uint32(0)
		expectedPanic := ""
		if wantPanic {
			expectedPanic = oracleString(t, reader)
		} else {
			expectedStart, expectedLength = oracleU32(t, reader), oracleU32(t, reader)
		}
		expectedKeys := make([]uint32, oracleU32(t, reader))
		for i := range expectedKeys {
			expectedKeys[i] = oracleU32(t, reader)
		}
		expectedComparisons := make([]comparison, oracleU32(t, reader))
		for i := range expectedComparisons {
			expectedComparisons[i] = comparison{oracleI64(t, reader), oracleI64(t, reader)}
		}
		expectedOperators := make([]byte, oracleU32(t, reader))
		if _, err := io.ReadFull(reader, expectedOperators); err != nil {
			t.Fatal(err)
		}
		check := func(trace oracleTrace, operators bool) {
			t.Helper()
			if (trace.panicValue != nil) != wantPanic || (wantPanic && fmt.Sprint(trace.panicValue) != expectedPanic) {
				t.Fatalf("case %d mode %d: panic=%v, want %q", caseIndex, mode, trace.panicValue, expectedPanic)
			}
			if !wantPanic {
				if len(trace.result) != int(expectedLength) {
					t.Fatalf("case %d mode %d: length %d, want %d", caseIndex, mode, len(trace.result), expectedLength)
				}
				if len(trace.result) > 0 {
					if &trace.result[0] != &data[expectedStart] || trace.result[0].index != expectedStart {
						t.Fatalf("case %d: result does not borrow expected input range", caseIndex)
					}
				} else if trace.result != nil {
					t.Fatalf("case %d: missing key must return nil", caseIndex)
				}
			}
			if !slices.Equal(trace.keys, expectedKeys) || !slices.Equal(trace.comparisons, expectedComparisons) || (operators && !slices.Equal(trace.operators, expectedOperators)) {
				t.Fatalf("case %d mode %d target %d: keys %v / comparisons %v / operators %v; want %v / %v / %v", caseIndex, mode, target, trace.keys, trace.comparisons, trace.operators, expectedKeys, expectedComparisons, expectedOperators)
			}
		}
		check(executeOracleCase(data, target, mode, keyPanic, comparePanic, operatorPanic, operatorPanicAt, false), true)
		if wantPanic {
			panicCount++
			if operatorPanic != 0 {
				operatorPanicCount++
			}
		}
		// The convenience comparator adapter models default/derived operators. It
		// cannot express an independent override, so only full-operator tests apply
		// to operator-specific panic cases.
		if operatorPanic == 0 {
			check(executeOracleCase(data, target, mode, keyPanic, comparePanic, 0, 0, true), false)
			comparatorCases++
		}
	}
	if reader.Len() != 0 {
		t.Fatal("trailing Rust oracle bytes")
	}
	t.Logf("matched %d original operator cases (%d panics, %d operator-specific) and %d comparator-adapter cases", count, panicCount, operatorPanicCount, comparatorCases)
}

func TestOracleProvenance(t *testing.T) {
	data, err := os.ReadFile("oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		RustRevision string `json:"rust_revision"`
		Sources      []struct {
			Path    string `json:"path"`
			GitBlob string `json:"git_blob"`
			SHA256  string `json:"sha256"`
		} `json:"sources"`
		OracleSHA256  string `json:"oracle_sha256"`
		FixtureSHA256 string `json:"fixture_sha256"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.RustRevision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || len(manifest.Sources) != 2 {
		t.Fatal("wrong upstream source pin")
	}
	if manifest.Sources[0].GitBlob != "1c6e227cec40870c0288f60270f609aeb4ebdc8e" || manifest.Sources[1].GitBlob != "d74febb5c0fc4a13e2530de71982520e3a944fcf" {
		t.Fatal("wrong source blobs")
	}
	raw, compressed := oracleFixture(t)
	if sourceDigest(raw) != manifest.OracleSHA256 || sourceDigest(compressed) != manifest.FixtureSHA256 {
		t.Fatal("oracle fixture checksum mismatch")
	}
}

func TestComparatorAdaptersAndBorrowing(t *testing.T) {
	type key struct {
		group int
		name  string
	}
	type record struct {
		key     key
		payload string
	}
	data := []record{{key{1, "a"}, "first"}, {key{1, "b"}, "left"}, {key{1, "b"}, "right"}, {key{2, "a"}, "last"}}
	compare := func(a, b key) int {
		if first := cmp.Compare(a.group, b.group); first != 0 {
			return first
		}
		return cmp.Compare(a.name, b.name)
	}
	result := BinarySearchSliceBy(data, func(value *record) key { return value.key }, key{1, "b"}, compare)
	if len(result) != 2 || result[0].payload != "left" || result[1].payload != "right" || &result[0] != &data[1] {
		t.Fatal("composite Ord adapter lost exact matching borrowed range")
	}
	if result := BinarySearchSliceBy[int, int](nil, nil, 0, nil); result != nil {
		t.Fatal("empty input called callbacks or returned a nonempty result")
	}
	if allocations := testing.AllocsPerRun(100, func() { _ = BinarySearchSlice(testMapForAlloc, getKey, uint(3)) }); allocations != 0 {
		t.Fatalf("search allocates %g times", allocations)
	}
}

var testMapForAlloc = []element{{0, "zero"}, {3, "a"}, {3, "b"}, {22, "twenty-two"}}

func TestLiveOriginalRust(t *testing.T) {
	requested, enabled := os.LookupEnv("FERRUGO_RUSTC")
	if !enabled {
		t.Skip("set FERRUGO_RUSTC to execute the pinned original Rust module")
	}
	if strings.TrimSpace(requested) == "" {
		t.Fatal("FERRUGO_RUSTC is set but empty")
	}
	compiler, err := exec.LookPath(requested)
	if err != nil {
		t.Fatalf("FERRUGO_RUSTC unavailable: %v", err)
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(version)) != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" {
		t.Fatalf("oracle requires pinned Rust 1.99.1, found %s", version)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checkout := os.Getenv("FERRUGO_RUST_SOURCE")
	if checkout == "" {
		checkout = filepath.Join(root, ".toolchain", "upstream-rust")
	} else if !filepath.IsAbs(checkout) {
		checkout = filepath.Join(root, checkout)
	}
	metadata, err := os.ReadFile("oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		RustRevision string `json:"rust_revision"`
		Sources      []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	for _, source := range manifest.Sources {
		data, err := exec.Command("git", "-C", checkout, "show", manifest.RustRevision+":"+source.Path).Output()
		if err != nil {
			t.Fatalf("read original %s: %v", source.Path, err)
		}
		if sourceDigest(data) != source.SHA256 {
			t.Fatalf("source hash mismatch: %s", source.Path)
		}
		if err := os.WriteFile(filepath.Join(temp, filepath.Base(source.Path)), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(temp, "mod.rs")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tests := filepath.Join(temp, "original-tests")
	command := exec.CommandContext(ctx, compiler, "--edition=2024", "--crate-name", "binary_search_original", "--test", "-C", "opt-level=2", source, "-o", tests)
	if diagnostics, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile original tests: %v\n%s", err, diagnostics)
	}
	if diagnostics, err := exec.CommandContext(ctx, tests).CombinedOutput(); err != nil {
		t.Fatalf("run original tests: %v\n%s", err, diagnostics)
	}
	executable := filepath.Join(temp, "oracle")
	command = exec.CommandContext(ctx, compiler, "--edition=2024", "-C", "opt-level=2", filepath.Join(root, "tools", "port-rust-binary-search", "oracle.rs"), "-o", executable)
	command.Env = append(os.Environ(), "BINARY_SEARCH_SOURCE="+source)
	if diagnostics, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile original oracle: %v\n%s", err, diagnostics)
	}
	actual, err := exec.CommandContext(ctx, executable).Output()
	if err != nil {
		t.Fatalf("execute original oracle: %v", err)
	}
	expected, _ := oracleFixture(t)
	if !bytes.Equal(actual, expected) {
		t.Fatal("live original Rust oracle differs from committed fixture")
	}
}

func TestIndependentRustOperators(t *testing.T) {
	data := []int{1, 2, 2, 3}
	var calls []string
	operators := KeyOperators[int]{
		Less:        func(a, b int) bool { calls = append(calls, "lt"); return a < b },
		NotEqual:    func(a, b int) bool { calls = append(calls, "ne"); return a != b },
		LessOrEqual: func(a, b int) bool { calls = append(calls, "le"); return a <= b },
	}
	result := BinarySearchSliceWithOperators(data, func(value *int) int { return *value }, 2, operators)
	if !slices.Equal(result, []int{2, 2}) || !slices.Equal(calls, []string{"lt", "lt", "lt", "ne", "le", "le"}) {
		t.Fatalf("independent operator result/trace: %v %v", result, calls)
	}
	calls = nil
	operators.NotEqual = func(a, b int) bool { calls = append(calls, "ne"); panic("ne panic") }
	func() {
		defer func() {
			if got := recover(); got != "ne panic" {
				t.Fatalf("ne override panic lost: %v", got)
			}
		}()
		BinarySearchSliceWithOperators(data, func(value *int) int { return *value }, 2, operators)
	}()
	if !slices.Equal(calls, []string{"lt", "lt", "lt", "ne"}) {
		t.Fatalf("ne panic operator order: %v", calls)
	}
	// A missing key above every entry never touches ne/le, even when those
	// independently overridden trait methods would panic.
	operators.NotEqual = func(int, int) bool { panic("unused ne") }
	operators.LessOrEqual = func(int, int) bool { panic("unused le") }
	if result := BinarySearchSliceWithOperators(data, func(value *int) int { return *value }, 10, operators); result != nil {
		t.Fatal("missing key returned data")
	}
	if result := BinarySearchSliceWithOperators[int, int](nil, nil, 0, KeyOperators[int]{}); result != nil {
		t.Fatal("empty input used absent callbacks")
	}
}

func FuzzBinarySearchSlice(f *testing.F) {
	for _, test := range []struct {
		data       []byte
		key        byte
		descending bool
	}{{[]byte{3, 0, 3, 22}, 3, false}, {nil, 1, false}, {[]byte{1, 1, 1}, 0, true}, {[]byte{0, 255}, 255, true}} {
		f.Add(test.data, test.key, test.descending)
	}
	f.Fuzz(func(t *testing.T, values []byte, key byte, descending bool) {
		if len(values) > 256 {
			t.Skip()
		}
		data := slices.Clone(values)
		compare := func(a, b byte) int {
			if descending {
				return cmp.Compare(b, a)
			}
			return cmp.Compare(a, b)
		}
		slices.SortFunc(data, compare)
		result := BinarySearchSliceBy(data, func(value *byte) byte { return *value }, key, compare)
		var expected []byte
		for _, value := range data {
			if value == key {
				expected = append(expected, value)
			}
		}
		if !slices.Equal(result, expected) {
			t.Fatalf("got %v, want %v", result, expected)
		}
	})
}
