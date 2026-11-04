// Command port-rust-thousands regenerates and checks reference outputs from
// the entire pinned upstream thousands module and its unchanged test module.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const sourcePath = "compiler/rustc_data_structures/src/thousands/mod.rs"
const testsPath = "compiler/rustc_data_structures/src/thousands/tests.rs"
const sourceBlob = "b251ebe58f6b9942537c650fe5b65e0315c41f3b"
const testsBlob = "0f9a648802b85ae6731512c13cabe4fddd02d8b2"
const sourceSHA = "adcd6db15c34f2fbf4aa9804427cd2f86c8e9e63be5a07316b1ab5e6240d21fb"
const testsSHA = "9596a03a2af2075b20952c1483b31d43a6b9d76918b9184148a96bb60b6de56f"
const destination = "port/rust/compiler/rustc_data_structures/src/thousands"

//go:embed oracle.rs
var oracleSource []byte

type manifest struct {
	Format        int            `json:"format"`
	Revision      string         `json:"rust_revision"`
	SourcePath    string         `json:"source_path"`
	SourceBlob    string         `json:"source_git_blob"`
	SourceSHA     string         `json:"source_sha256"`
	TestsPath     string         `json:"tests_path"`
	TestsBlob     string         `json:"tests_git_blob"`
	TestsSHA      string         `json:"tests_sha256"`
	Rustc         string         `json:"rustc"`
	PointerBits   int            `json:"host_pointer_bits"`
	UpstreamTests int            `json:"upstream_tests"`
	Cases         map[string]int `json:"cases"`
	OracleSHA     string         `json:"oracle_source_sha256"`
	RequestSHA    string         `json:"request_sha256"`
	RawSHA        string         `json:"oracle_raw_sha256"`
	CompressedSHA string         `json:"fixture_sha256"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	rustc := flag.String("rustc", environmentDefault("FERRUGO_RUSTC", ".toolchain/rust/bin/rustc"), "pinned compiler path or PATH name")
	source := flag.String("rust-source", environmentDefault("FERRUGO_RUST_SOURCE", ".toolchain/upstream-rust"), "pinned Rust checkout")
	check := flag.Bool("check", false, "check original outputs and provenance without rewriting fixtures")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(1)
	}
	if err := run(*root, *rustc, *source, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-thousands:", err)
		os.Exit(1)
	}
}

func environmentDefault(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value // Explicit empty/invalid inputs fail instead of falling back.
	}
	return fallback
}

func run(root, compiler, upstream string, check bool) error {
	if compiler == "" || upstream == "" {
		return fmt.Errorf("compiler and source checkout must not be empty")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(compiler) {
		if strings.ContainsAny(compiler, `/\`) {
			compiler = filepath.Join(root, compiler)
		} else {
			compiler, err = exec.LookPath(compiler)
			if err != nil {
				return err
			}
		}
	}
	if !filepath.IsAbs(upstream) {
		upstream = filepath.Join(root, upstream)
	}
	version, err := command(10*time.Second, "", nil, compiler, "--version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, strings.TrimSpace(string(version)))
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("reference regeneration requires the pinned 64-bit profile; the Go port also supports 32-bit hosts")
	}
	head, err := git(upstream, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("upstream HEAD differs from %s", revision)
	}
	source, err := pinnedFile(upstream, sourcePath, sourceBlob, sourceSHA)
	if err != nil {
		return err
	}
	tests, err := pinnedFile(upstream, testsPath, testsBlob, testsSHA)
	if err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "ferrugo-thousands-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.Mkdir(filepath.Join(temporary, "thousands"), 0755); err != nil {
		return err
	}
	wrapped := bytes.Replace(oracleSource, []byte("// PINNED_THOUSANDS_MODULE"), source, 1)
	if bytes.Count(wrapped, source) != 1 {
		return fmt.Errorf("complete original module must occur once verbatim in the wrapper")
	}
	for name, contents := range map[string][]byte{"main.rs": wrapped, "thousands/tests.rs": tests} {
		if err := os.WriteFile(filepath.Join(temporary, name), contents, 0600); err != nil {
			return err
		}
	}
	for _, test := range []bool{false, true} {
		name := "oracle"
		args := []string{"--edition=2024", "-C", "opt-level=2", "main.rs", "-o"}
		if test {
			name = "upstream-tests"
			args = append(args, name, "--test")
		} else {
			args = append(args, name)
		}
		if _, err := command(30*time.Second, temporary, nil, compiler, args...); err != nil {
			return err
		}
	}
	output, err := command(10*time.Second, temporary, nil, filepath.Join(temporary, "upstream-tests"), "--test-threads=1")
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("4 passed")) {
		return fmt.Errorf("expected all four unchanged tests to pass: %s", output)
	}
	requests, counts := makeRequests()
	raw, err := command(time.Minute, temporary, requests, filepath.Join(temporary, "oracle"))
	if err != nil {
		return err
	}
	total := 0
	for _, count := range counts {
		total += count
	}
	if bytes.Count(raw, []byte{'\n'}) != total {
		return fmt.Errorf("expected %d reference records", total)
	}
	fixturePath := filepath.Join(root, destination, "testdata/oracle.tsv.gz")
	compressed, err := fixtureEncoding(fixturePath, raw, check)
	if err != nil {
		return err
	}
	meta := manifest{1, revision, sourcePath, sourceBlob, sourceSHA, testsPath, testsBlob, testsSHA,
		rustVersion, 64, 4, counts, digest(oracleSource), digest(requests), digest(raw), digest(compressed)}
	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	manifestPath := filepath.Join(root, destination, "oracle_manifest.json")
	if check {
		previous, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, encoded) {
			return fmt.Errorf("manifest differs from verified original-source provenance")
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fixturePath, compressed, 0644); err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, encoded, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("thousands: four unchanged Rust tests; %d reference cases (%d floats), source hashes and fixture verified\n", total, counts["f"])
	return nil
}

func command(timeout time.Duration, dir string, input []byte, executable string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", executable, args, err, stderr.Bytes())
	}
	return output, nil
}

func git(upstream string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", upstream}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read pinned Git object; hydrate it explicitly if absent: %w\n%s", err, stderr.Bytes())
	}
	return output, nil
}

func pinnedFile(upstream, path, blob, hash string) ([]byte, error) {
	object, err := git(upstream, "rev-parse", revision+":"+path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(object)) != blob {
		return nil, fmt.Errorf("Git blob differs for %s", path)
	}
	data, err := git(upstream, "show", revision+":"+path)
	if err != nil {
		return nil, err
	}
	if digest(data) != hash {
		return nil, fmt.Errorf("source SHA-256 differs for %s", path)
	}
	return data, nil
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// Preserve any valid existing gzip representation when its decoded original
// output matches, even if this Go version would choose different gzip bytes.
func fixtureEncoding(path string, raw []byte, check bool) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		reader, decodeErr := gzip.NewReader(bytes.NewReader(existing))
		if decodeErr == nil {
			previous, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(previous, raw) {
				return existing, nil
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if check {
		return nil, fmt.Errorf("committed fixture is absent, invalid or differs from original output")
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}

func makeRequests() ([]byte, map[string]int) {
	var out bytes.Buffer
	counts := map[string]int{"u": 0, "i": 0, "f": 0, "s": 0}
	add := func(kind, value string) { fmt.Fprintf(&out, "%s\t%s\n", kind, value); counts[kind]++ }
	unsigned := map[uint64]bool{}
	addInteger := func(value uint64) {
		if unsigned[value] {
			return
		}
		unsigned[value] = true
		add("u", strconv.FormatUint(value, 10))
		add("i", strconv.FormatInt(int64(value), 10))
	}
	for _, value := range []uint64{0, 1, 99, 345, 999, 1000, 12001, 999999, 1000000, 12345678, 1<<31 - 1, 1 << 31, 1<<32 - 1, 1 << 32, 1<<63 - 1, 1 << 63, ^uint64(0)} {
		addInteger(value)
	}
	for bit := 0; bit < 64; bit++ {
		v := uint64(1) << bit
		addInteger(v - 1)
		addInteger(v)
		addInteger(v + 1)
	}
	for value := uint64(10); value <= 10_000_000_000_000_000_000; {
		addInteger(value - 1)
		addInteger(value)
		addInteger(value + 1)
		if value > ^uint64(0)/10 {
			break
		}
		value *= 10
	}
	seed := uint64(0x58f2930ad7e19b65)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	for i := 0; i < 2048; i++ {
		addInteger(next())
	}
	floats := map[uint64]bool{}
	addFloat := func(bits uint64) {
		if !floats[bits] {
			floats[bits] = true
			add("f", fmt.Sprintf("%016x", bits))
		}
	}
	for exponent := uint64(0); exponent < 2048; exponent++ {
		for _, mantissa := range []uint64{0, 1, 1<<51 - 1, 1 << 51, 1<<52 - 2, 1<<52 - 1} {
			for _, sign := range []uint64{0, 1 << 63} {
				addFloat(sign | exponent<<52 | mantissa)
			}
		}
	}
	for _, value := range []float64{0, 0.00000001, -0.00000001, 0.9999999, -0.9999999, 345.4999999, -100000, 123456789.123456789, -123456789.123456789} {
		addFloat(math.Float64bits(value))
	}
	for integer := -1000; integer <= 1000; integer += 7 {
		for _, fraction := range []float64{0.25, 0.75} {
			value := float64(integer) + fraction
			for _, candidate := range []float64{value, math.Nextafter(value, math.Inf(1)), math.Nextafter(value, math.Inf(-1))} {
				addFloat(math.Float64bits(candidate))
			}
		}
	}
	for exponent := -323; exponent <= 308; exponent++ {
		value := math.Pow10(exponent)
		for _, candidate := range []float64{value, math.Nextafter(value, math.Inf(1)), math.Nextafter(value, math.Inf(-1))} {
			addFloat(math.Float64bits(candidate))
			addFloat(math.Float64bits(-candidate))
		}
	}
	for bit := 0; bit < 52; bit++ {
		for _, sign := range []uint64{0, 1 << 63} {
			addFloat(sign | 0x7ff0000000000000 | 1<<bit)
		}
	}
	for i := 0; i < 12000; i++ {
		addFloat(next())
	}
	for _, text := range []string{"", "0", "12345.67e14", "-1234.5678e10", "------", "abcdefgh", "-1b", "-3456xyz", "+12345", "١٢٣٤٥", "１２３４５", "123٤5678"} {
		add("s", hex.EncodeToString([]byte(text)))
	}
	for digits := 0; digits <= 64; digits++ {
		text := strings.Repeat("1234567890", (digits+9)/10)[:digits]
		for _, prefix := range []string{"", "-"} {
			for _, suffix := range []string{"", ".123456", "e12345", "_1234", "🦀", "é", "٤", "\x00\t\n"} {
				add("s", hex.EncodeToString([]byte(prefix+text+suffix)))
			}
		}
	}
	return out.Bytes(), counts
}
