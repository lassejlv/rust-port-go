// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_data_structures/src/base_n/tests.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179, plus original-source oracle checks.
package basen_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

// TestLimits is the entire upstream limits test, using math/big for u128 and
// strconv for the native unsigned widths in place of Rust's from_str_radix.
func TestLimits(t *testing.T) {
	max128 := ds.BaseNU128{Hi: ^uint64(0), Lo: ^uint64(0)}
	encoded := ds.BaseNToBase128(max128, 36)
	parsed, ok := new(big.Int).SetString(encoded.String(), 36)
	if !ok || parsed.Cmp(bigValue(max128)) != 0 {
		t.Fatalf("u128::MAX round trip: %q", encoded)
	}
	for _, width := range []int{64, 32} {
		var text string
		var max uint64
		if width == 64 {
			max = ^uint64(0)
			text = ds.BaseNToBase64(max, 36).String()
		} else {
			max = uint64(^uint32(0))
			text = ds.BaseNToBase32(uint32(max), 36).String()
		}
		parsed, err := strconv.ParseUint(text, 36, width)
		if err != nil || parsed != max {
			t.Fatalf("u%d::MAX round trip: got %d, %v", width, parsed, err)
		}
	}
}

// TestToBase preserves every upstream case and the full non-Miri loop bound.
func TestToBase(t *testing.T) {
	test := func(n ds.BaseNU128, base uint) {
		t.Helper()
		for _, text := range []string{
			ds.BaseNToBase128(n, base).String(),
			ds.BaseNToBaseFixedLen128(n, base).String(),
		} {
			parsed, ok := new(big.Int).SetString(text, int(base))
			if !ok || parsed.Cmp(bigValue(n)) != 0 {
				t.Fatalf("n=%+v base=%d did not round trip: %q", n, base, text)
			}
		}
	}
	for base := uint(2); base < 37; base++ {
		for _, n := range []uint64{0, 1, 35, 36, 37, ^uint64(0)} {
			test(ds.BaseNU128{Lo: n}, base)
		}
		test(ds.BaseNU128{Hi: ^uint64(0), Lo: ^uint64(0)}, base)
		for i := uint64(0); i < 1000; i++ {
			test(ds.BaseNU128{Lo: i * 983}, base)
		}
	}
}

func TestBaseNOriginalSourceOracle(t *testing.T) {
	root := repositoryRoot(t)
	dir := filepath.Join(root, "tools/port-rust-base-n")
	var manifest struct {
		Format               int    `json:"format"`
		RustRevision         string `json:"rust_revision"`
		SourceGitBlob        string `json:"source_git_blob"`
		TestsGitBlob         string `json:"tests_git_blob"`
		SourceSHA256         string `json:"source_sha256"`
		TestsSHA256          string `json:"tests_sha256"`
		OracleSourceSHA256   string `json:"oracle_source_sha256"`
		OracleRequestSHA256  string `json:"oracle_request_sha256"`
		OracleRawSHA256      string `json:"oracle_raw_sha256"`
		FixtureSHA256        string `json:"fixture_sha256"`
		ValidCases           int    `json:"valid_cases"`
		FailureContractCases int    `json:"failure_contract_cases"`
		ExtensionCases       int    `json:"extension_cases"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != 1 || manifest.RustRevision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" ||
		manifest.SourceGitBlob != "0c3d7613d4f1ff69a31c4458bd571074bbd5c618" ||
		manifest.TestsGitBlob != "148d8dde02af1b12433fa87509a742d1e59c3421" ||
		manifest.SourceSHA256 != "090d62952cb4515c9de66489d6ce0e0f3cd5c3becd7e8e2c2bf39d119d4f9890" ||
		manifest.TestsSHA256 != "e4a58e24148ca8120d30d974fc4084ab08e0aa9ea2fd1d4b1101eb21c9155cde" {
		t.Fatal("fixture provenance differs from pinned original source")
	}
	data, err = os.ReadFile(filepath.Join(dir, "oracle.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(data) != manifest.OracleSourceSHA256 {
		t.Fatal("oracle harness checksum mismatch")
	}
	compressed, err := os.ReadFile(filepath.Join(dir, "testdata/oracle.tsv.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.FixtureSHA256 {
		t.Fatal("compressed oracle fixture checksum mismatch")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if digest(data) != manifest.OracleRawSHA256 {
		t.Fatal("uncompressed oracle fixture checksum mismatch")
	}
	var requests bytes.Buffer
	valid, invalid, extension, skipped := 0, 0, 0, 0
	var covered [3][65]bool
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			t.Fatalf("invalid fixture record: %q", line)
		}
		fmt.Fprintln(&requests, strings.Join(fields[:5], "\t"))
		op := fields[0]
		width := parseUint(t, fields[1])
		base := parseUint(t, fields[2])
		value := ds.BaseNU128{Hi: parseUint(t, fields[3]), Lo: parseUint(t, fields[4])}
		if op == "all" {
			valid++
			index := map[uint64]int{32: 0, 64: 1, 128: 2}[width]
			if base < 2 || base > 64 {
				t.Fatal("supported-base record outside base 2..64")
			}
			covered[index][base] = true
		} else if strings.Contains(op, ":") {
			extension++
		} else {
			invalid++
		}
		// Only invalid-base records can exceed a 32-bit Rust/Go usize. The
		// pinned Rust oracle is 64-bit; all valid-base records run on either.
		if uint64(uint(base)) != base {
			skipped++
			continue
		}
		got := execute(op, width, value, uint(base))
		want := strings.Join(fields[5:], "\t")
		if got != want {
			t.Fatalf("original-source mismatch: request %q\nGo:   %q\nRust: %q", strings.Join(fields[:5], "\t"), got, want)
		}
	}
	if valid != manifest.ValidCases || invalid != manifest.FailureContractCases || extension != manifest.ExtensionCases || digest(requests.Bytes()) != manifest.OracleRequestSHA256 {
		t.Fatal("oracle request coverage/checksum mismatch")
	}
	for width, bases := range covered {
		for base := 2; base <= 64; base++ {
			if !bases[base] {
				t.Fatalf("width index %d base %d was not checked", width, base)
			}
		}
	}
	if skipped != 0 {
		t.Logf("%d 64-bit invalid-base records cannot be represented on this host", skipped)
	}
	t.Logf("compared %d supported-base, %d failure-contract and %d custom-trait original Rust records", valid, invalid-skipped, extension)
}

func TestBaseNCustomTrait(t *testing.T) {
	for _, input := range []ds.BaseNValue{ds.BaseNU32(7), ds.BaseNU64(7), ds.BaseNU128{Lo: 7}} {
		if ds.BaseNToBase(input, 10).String() != "7" || len(ds.BaseNToBaseFixedLen(input, 10).String()) != int(input.BaseNEncodedLen(10)) {
			t.Fatal("native adapter does not implement the public extension interface")
		}
	}
	for _, test := range []struct {
		width uint
		value uint64
		want  string
	}{{5, 7, "00007"}, {1, 12345, "5"}, {0, 7, ""}} {
		value := &customValue{value: ds.BaseNU128{Lo: test.value}, width: test.width}
		if got := ds.BaseNToBaseFixedLen(value, 10).String(); got != test.want {
			t.Fatalf("custom width %d value %d: %q != %q", test.width, test.value, got, test.want)
		}
		if strings.Join(value.trace, ",") != "into,len" {
			t.Fatalf("default order/conversion count: %v", value.trace)
		}
	}
	value := &customValue{value: ds.BaseNU128{Lo: 7}, width: 129}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("custom width >128 must panic during the call, before String")
			}
		}()
		_ = ds.BaseNToBaseFixedLen(value, 10)
	}()
	if strings.Join(value.trace, ",") != "into,len" {
		t.Fatalf("invalid custom length evaluation order: %v", value.trace)
	}
	value = &customValue{value: ds.BaseNU128{Lo: 7}, width: 5}
	if got := ds.BaseNToBaseFixedLen(&overrideBase{value}, 10).String(); got != "00042" || strings.Join(value.trace, ",") != "base,len" {
		t.Fatalf("fixed default must dispatch ordinary override: got %q, trace %v", got, value.trace)
	}
	value.trace = nil
	if got := ds.BaseNToBaseFixedLen(&overrideBoth{&overrideBase{value}}, 10).String(); got != "99" || strings.Join(value.trace, ",") != "fixed" {
		t.Fatalf("fixed override must bypass conversion, base and length: got %q, trace %v", got, value.trace)
	}
}

func TestBaseNStringAndAlphabet(t *testing.T) {
	var _ fmt.Stringer = ds.BaseNToBase32(0, 2)
	if ds.BaseNMaxBase != 64 || ds.BaseNAlphanumericOnly != 62 || ds.BaseNCaseInsensitive != 36 {
		t.Fatal("base constants changed")
	}
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@$"
	for i, digit := range alphabet {
		encoded := ds.BaseNToBase32(uint32(i), ds.BaseNMaxBase)
		want := string(digit)
		if encoded.String() != want || encoded.AsRef() != want || fmt.Sprintf("%s", encoded) != want || fmt.Sprint(encoded) != want {
			t.Fatalf("digit %d string/as-ref/display mismatch", i)
		}
	}
	// Returning/copying BaseNString must retain its own buffer and width.
	first := ds.BaseNToBaseFixedLen32(7, 10)
	copy := first
	_ = ds.BaseNToBase128(ds.BaseNU128{Hi: ^uint64(0), Lo: ^uint64(0)}, 2)
	if first.String() != "0000000007" || copy.String() != first.String() {
		t.Fatal("fixed-buffer value changed after another encoding")
	}
}

func TestBaseNFixedLengthOrdering(t *testing.T) {
	values := []ds.BaseNU128{
		{}, {Lo: 1}, {Lo: 35}, {Lo: 36}, {Lo: 37}, {Lo: ^uint64(0)},
		{Hi: 1}, {Hi: 1, Lo: 1}, {Hi: 1 << 63}, {Hi: ^uint64(0), Lo: ^uint64(0)},
	}
	for base := uint(2); base <= ds.BaseNCaseInsensitive; base++ {
		previous := ""
		for _, value := range values {
			current := ds.BaseNToBaseFixedLen128(value, base).String()
			if previous != "" && previous >= current {
				t.Fatalf("base %d fixed-length strings do not preserve numeric order: %q >= %q", base, previous, current)
			}
			previous = current
		}
	}
	// The upstream alphabet intentionally places A after z. ASCII sorting
	// therefore does not preserve numeric ordering once base 37 is used.
	if ds.BaseNToBase32(35, 37).String() != "z" || ds.BaseNToBase32(36, 37).String() != "A" {
		t.Fatal("case-sensitive digit ordering changed")
	}
}

func TestBaseNLiveRustOracle(t *testing.T) {
	compiler, requested := os.LookupEnv("FERRUGO_RUSTC")
	if !requested {
		t.Skip("set FERRUGO_RUSTC to compile the pinned original source and tests")
	}
	if compiler == "" {
		t.Fatal("FERRUGO_RUSTC was explicitly set empty")
	}
	root := repositoryRoot(t)
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "run", "./tools/port-rust-base-n", "-root", root, "-rustc", compiler, "-check")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live original Rust comparison: %v\n%s", err, output)
	}
}

func FuzzBaseN128(f *testing.F) {
	for base := uint8(2); base <= 64; base++ {
		f.Add(uint64(0), uint64(0), base)
		f.Add(^uint64(0), ^uint64(0), base)
		f.Add(uint64(1), uint64(0), base)
	}
	f.Fuzz(func(t *testing.T, hi, lo uint64, rawBase uint8) {
		base := uint(rawBase%63) + 2
		value := ds.BaseNU128{Hi: hi, Lo: lo}
		want := bigValue(value)
		plain := ds.BaseNToBase128(value, base).String()
		fixed := ds.BaseNToBaseFixedLen128(value, base).String()
		if len(plain) > 1 && plain[0] == '0' {
			t.Fatal("nonminimal encoding")
		}
		if len(fixed) != int(ds.BaseNEncodedLen128(base)) || strings.TrimLeft(fixed, "0") != strings.TrimLeft(plain, "0") {
			t.Fatal("fixed encoding does not pad the minimal encoding")
		}
		for _, text := range []string{plain, fixed} {
			parsed := decode(t, text, base)
			if parsed.Cmp(want) != 0 {
				t.Fatalf("128-bit value lost: base %d value %+v text %q", base, value, text)
			}
		}
		custom := &customValue{value: value, width: uint(rawBase)}
		customText, panicked := func() (text string, panicked bool) {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			return ds.BaseNToBaseFixedLen(custom, base).String(), false
		}()
		if custom.width > 128 {
			if !panicked {
				t.Fatal("custom width overflow did not panic")
			}
		} else {
			buffer := strings.Repeat("0", 128-len(plain)) + plain
			if panicked || customText != buffer[128-int(custom.width):] {
				t.Fatal("custom width did not preserve buffer suffix/truncation")
			}
		}
		if strings.Join(custom.trace, ",") != "into,len" {
			t.Fatalf("custom trait operation order changed: %v", custom.trace)
		}
	})
}

func execute(op string, width uint64, value ds.BaseNU128, base uint) (result string) {
	if strings.Contains(op, ":") {
		return executeCustom(op, value, base)
	}
	defer func() {
		if recover() != nil {
			result = "panic"
		}
	}()
	var normal, fixed func() ds.BaseNString
	var length func() uint
	switch width {
	case 32:
		normal = func() ds.BaseNString { return ds.BaseNToBase32(uint32(value.Lo), base) }
		fixed = func() ds.BaseNString { return ds.BaseNToBaseFixedLen32(uint32(value.Lo), base) }
		length = func() uint { return ds.BaseNEncodedLen32(base) }
	case 64:
		normal = func() ds.BaseNString { return ds.BaseNToBase64(value.Lo, base) }
		fixed = func() ds.BaseNString { return ds.BaseNToBaseFixedLen64(value.Lo, base) }
		length = func() uint { return ds.BaseNEncodedLen64(base) }
	case 128:
		normal = func() ds.BaseNString { return ds.BaseNToBase128(value, base) }
		fixed = func() ds.BaseNString { return ds.BaseNToBaseFixedLen128(value, base) }
		length = func() uint { return ds.BaseNEncodedLen128(base) }
	default:
		panic("unknown width")
	}
	switch op {
	case "all":
		return fmt.Sprintf("ok\t%d\t%s\t%s", length(), normal(), fixed())
	case "base":
		return "ok\t" + normal().String()
	case "fixed":
		return "ok\t" + fixed().String()
	case "len":
		return fmt.Sprintf("ok\t%d", length())
	default:
		panic("unknown operation")
	}
}

type customValue struct {
	value ds.BaseNU128
	width uint
	trace []string
}

func (value *customValue) BaseNInto128() ds.BaseNU128 {
	value.trace = append(value.trace, "into")
	return value.value
}

func (value *customValue) BaseNEncodedLen(uint) uint {
	value.trace = append(value.trace, "len")
	return value.width
}

type overrideBase struct{ *customValue }

func (value *overrideBase) BaseNToBase(base uint) ds.BaseNString {
	value.trace = append(value.trace, "base")
	return ds.BaseNToBase32(42, base)
}

type overrideFixed struct{ *customValue }

func (value *overrideFixed) BaseNToBaseFixedLen(base uint) ds.BaseNString {
	value.trace = append(value.trace, "fixed")
	return ds.BaseNToBase32(99, base)
}

type overrideBoth struct{ *overrideBase }

func (value *overrideBoth) BaseNToBaseFixedLen(base uint) ds.BaseNString {
	value.trace = append(value.trace, "fixed")
	return ds.BaseNToBase32(99, base)
}

func executeCustom(op string, n ds.BaseNU128, base uint) (result string) {
	kind, operation, _ := strings.Cut(op, ":")
	value := &customValue{value: n, width: 5}
	defer func() {
		if recover() != nil {
			result = "panic"
		}
		result += "\ttrace:" + strings.Join(value.trace, ",")
	}()
	var input ds.BaseNValue = value
	switch kind {
	case "override-base":
		input = &overrideBase{value}
	case "override-fixed":
		input = &overrideFixed{value}
	case "override-both":
		input = &overrideBoth{&overrideBase{value}}
	case "custommax":
		value.width = ^uint(0)
	default:
		width, err := strconv.ParseUint(strings.TrimPrefix(kind, "custom"), 10, 64)
		if err != nil {
			panic(err)
		}
		value.width = uint(width)
	}
	switch operation {
	case "all":
		return fmt.Sprintf("ok\t%d\t%s\t%s", input.BaseNEncodedLen(base), ds.BaseNToBase(input, base), ds.BaseNToBaseFixedLen(input, base))
	case "base":
		return "ok\t" + ds.BaseNToBase(input, base).String()
	case "fixed":
		return "ok\t" + ds.BaseNToBaseFixedLen(input, base).String()
	case "len":
		return fmt.Sprintf("ok\t%d", input.BaseNEncodedLen(base))
	default:
		panic("unknown custom operation")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

func parseUint(t *testing.T, text string) uint64 {
	t.Helper()
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func bigValue(value ds.BaseNU128) *big.Int {
	high := new(big.Int).Lsh(new(big.Int).SetUint64(value.Hi), 64)
	return high.Or(high, new(big.Int).SetUint64(value.Lo))
}

func decode(t *testing.T, text string, base uint) *big.Int {
	t.Helper()
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ@$"
	value := new(big.Int)
	radix := new(big.Int).SetUint64(uint64(base))
	for _, digit := range []byte(text) {
		index := strings.IndexByte(alphabet, digit)
		if index < 0 || uint(index) >= base {
			t.Fatalf("invalid digit %q in base %d", digit, base)
		}
		value.Mul(value, radix)
		value.Add(value, new(big.Int).SetUint64(uint64(index)))
	}
	return value
}

func digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
