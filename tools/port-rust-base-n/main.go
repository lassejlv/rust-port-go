// Command port-rust-base-n validates the complete pinned rustc base_n module
// and regenerates independent original-source reference fixtures.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/bits"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const rustRevision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const sourcePath = "compiler/rustc_data_structures/src/base_n.rs"
const sourceBlob = "0c3d7613d4f1ff69a31c4458bd571074bbd5c618"
const sourceSHA256 = "090d62952cb4515c9de66489d6ce0e0f3cd5c3becd7e8e2c2bf39d119d4f9890"
const testsPath = "compiler/rustc_data_structures/src/base_n/tests.rs"
const testsBlob = "148d8dde02af1b12433fa87509a742d1e59c3421"
const testsSHA256 = "e4a58e24148ca8120d30d974fc4084ab08e0aa9ea2fd1d4b1101eb21c9155cde"

//go:embed oracle.rs
var oracleSource []byte

type manifest struct {
	Format               int    `json:"format"`
	RustRevision         string `json:"rust_revision"`
	SourcePath           string `json:"source_path"`
	SourceGitBlob        string `json:"source_git_blob"`
	SourceSHA256         string `json:"source_sha256"`
	TestsPath            string `json:"tests_path"`
	TestsGitBlob         string `json:"tests_git_blob"`
	TestsSHA256          string `json:"tests_sha256"`
	Rustc                string `json:"rustc"`
	HostPointerBits      int    `json:"host_pointer_bits"`
	UpstreamTests        int    `json:"upstream_tests"`
	ValidCases           int    `json:"valid_cases"`
	FailureContractCases int    `json:"failure_contract_cases"`
	ExtensionCases       int    `json:"extension_cases"`
	OracleSourceSHA256   string `json:"oracle_source_sha256"`
	OracleRequestSHA256  string `json:"oracle_request_sha256"`
	OracleRawSHA256      string `json:"oracle_raw_sha256"`
	FixtureSHA256        string `json:"fixture_sha256"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	compilerDefault := os.Getenv("FERRUGO_RUSTC")
	if compilerDefault == "" {
		compilerDefault = ".toolchain/rust/bin/rustc"
	}
	sourceDefault := os.Getenv("FERRUGO_RUST_SOURCE")
	if sourceDefault == "" {
		sourceDefault = ".toolchain/upstream-rust"
	}
	rustc := flag.String("rustc", compilerDefault, "pinned Rust compiler name/path")
	upstream := flag.String("rust-source", sourceDefault, "pinned Rust source checkout")
	check := flag.Bool("check", false, "compare original Rust oracle with committed fixtures")
	flag.Parse()
	if err := run(*root, *rustc, *upstream, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-base-n:", err)
		os.Exit(1)
	}
}

func run(root, compiler, upstream string, check bool) error {
	var err error
	root, err = filepath.Abs(root)
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
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		return fmt.Errorf("query rustc: %w", err)
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, strings.TrimSpace(string(version)))
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("regeneration requires the pinned 64-bit host profile")
	}
	if !filepath.IsAbs(upstream) {
		upstream = filepath.Join(root, upstream)
	}
	revision, err := gitOutput(upstream, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(revision)) != rustRevision {
		return fmt.Errorf("source checkout HEAD must be %s", rustRevision)
	}
	source, err := readPinnedFile(upstream, sourcePath, sourceBlob, sourceSHA256)
	if err != nil {
		return err
	}
	tests, err := readPinnedFile(upstream, testsPath, testsBlob, testsSHA256)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "ferrugo-base-n-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Join(dir, "base_n"), 0o755); err != nil {
		return err
	}
	for path, data := range map[string][]byte{
		"main.rs": oracleSource, "base_n.rs": source, "base_n/tests.rs": tests,
	} {
		if err := os.WriteFile(filepath.Join(dir, path), data, 0o600); err != nil {
			return err
		}
	}
	for _, test := range []bool{false, true} {
		name := "oracle"
		args := []string{"--edition=2024", "-C", "overflow-checks=yes", "-C", "opt-level=2", "main.rs", "-o"}
		if test {
			name = "upstream-tests"
			args = append(args, name, "--test")
		} else {
			args = append(args, name)
		}
		cmd := exec.Command(compiler, args...)
		cmd.Dir = dir
		// The original source uses the still-unstable std::ascii::Char API.
		// Only this validation process opts into its feature on the pinned rustc.
		cmd.Env = append(os.Environ(), "RUSTC_BOOTSTRAP=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("compile %s: %w\n%s", name, err, output)
		}
	}
	testOutput, err := exec.Command(filepath.Join(dir, "upstream-tests"), "--test-threads=1").CombinedOutput()
	if err != nil || !bytes.Contains(testOutput, []byte("2 passed")) {
		return fmt.Errorf("entire upstream test module: %v\n%s", err, testOutput)
	}
	requests, validCases, invalidCases, extensionCases := makeRequests()
	cmd := exec.Command(filepath.Join(dir, "oracle"))
	cmd.Stdin = bytes.NewReader(requests)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run original Rust oracle: %w\n%s", err, stderr.Bytes())
	}
	if bytes.Count(raw, []byte{'\n'}) != validCases+invalidCases+extensionCases {
		return fmt.Errorf("oracle returned the wrong number of records")
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("all\t")) && bytes.HasSuffix(line, []byte("\tpanic")) {
			return fmt.Errorf("supported-base request unexpectedly panicked: %s", line)
		}
	}
	fixturePath := filepath.Join(root, "tools/port-rust-base-n/testdata/oracle.tsv.gz")
	manifestPath := filepath.Join(root, "tools/port-rust-base-n/manifest.json")
	var compressed []byte
	if check {
		compressed, err = os.ReadFile(fixturePath)
		if err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			return err
		}
		previous, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(previous, raw) {
			return fmt.Errorf("committed fixture differs from original Rust output")
		}
	} else {
		var buf bytes.Buffer
		writer, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return err
		}
		writer.Header.ModTime = time.Time{}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		compressed = buf.Bytes()
	}
	meta := manifest{
		Format: 1, RustRevision: rustRevision, SourcePath: sourcePath, SourceGitBlob: sourceBlob,
		SourceSHA256: sourceSHA256, TestsPath: testsPath, TestsGitBlob: testsBlob, TestsSHA256: testsSHA256,
		Rustc: rustVersion, HostPointerBits: 64, UpstreamTests: 2, ValidCases: validCases,
		FailureContractCases: invalidCases, OracleSourceSHA256: digest(oracleSource),
		ExtensionCases:      extensionCases,
		OracleRequestSHA256: digest(requests), OracleRawSHA256: digest(raw), FixtureSHA256: digest(compressed),
	}
	encodedManifest, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	encodedManifest = append(encodedManifest, '\n')
	if check {
		previous, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, encodedManifest) {
			return fmt.Errorf("manifest differs from verified original-source provenance")
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(fixturePath, compressed, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, encodedManifest, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("base_n: 2 unchanged Rust tests; %d supported-base, %d failure-contract, %d custom-trait cases verified\n", validCases, invalidCases, extensionCases)
	return nil
}

func gitOutput(upstream string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", upstream}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read pinned Git object (fetch it explicitly if absent): %w\n%s", err, stderr.Bytes())
	}
	return output, nil
}

func readPinnedFile(upstream, path, blob, hash string) ([]byte, error) {
	object, err := gitOutput(upstream, "rev-parse", rustRevision+":"+path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(object)) != blob {
		return nil, fmt.Errorf("unexpected Git blob for %s", path)
	}
	source, err := gitOutput(upstream, "show", rustRevision+":"+path)
	if err != nil {
		return nil, err
	}
	if digest(source) != hash {
		return nil, fmt.Errorf("unexpected source SHA-256 for %s", path)
	}
	return source, nil
}

func digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

type value128 struct{ hi, lo uint64 }

func makeRequests() ([]byte, int, int, int) {
	var out bytes.Buffer
	valid, invalid := 0, 0
	for _, width := range []int{32, 64, 128} {
		values := edgeAndRandomValues(width)
		for base := uint64(2); base <= 64; base++ {
			for _, value := range values {
				fmt.Fprintf(&out, "all\t%d\t%d\t%d\t%d\n", width, base, value.hi, value.lo)
				valid++
			}
		}
		max := value128{lo: ^uint64(0)}
		if width == 32 {
			max.lo = uint64(^uint32(0))
		} else if width == 128 {
			max.hi = ^uint64(0)
		}
		for _, base := range []uint64{0, 1, 65, 66, 128, 256, 1 << 32, (1 << 32) + 2, ^uint64(0)} {
			for _, value := range []value128{{}, {lo: 1}, {lo: 63}, {lo: 64}, {lo: 65}, {lo: 128}, max} {
				for _, operation := range []string{"base", "fixed", "len"} {
					// Original encoded_len(1), and fixed(0,1), do not terminate.
					// Do not put infinite computations into the reference stream.
					if base == 1 && (operation == "len" || (operation == "fixed" && value == (value128{}))) {
						continue
					}
					fmt.Fprintf(&out, "%s\t%d\t%d\t%d\t%d\n", operation, width, base, value.hi, value.lo)
					invalid++
				}
			}
		}
	}
	extension := 0
	for _, kind := range []string{"custom0", "custom1", "custom5", "custom128", "custom129", "custommax", "override-base", "override-fixed", "override-both"} {
		for _, base := range []uint64{0, 1, 2, 10, 36, 64, 65} {
			for _, value := range []value128{{}, {lo: 7}, {lo: 12345}, {hi: 1, lo: 7}, {hi: ^uint64(0), lo: ^uint64(0)}} {
				for _, operation := range []string{"all", "base", "fixed", "len"} {
					if kind == "custommax" && operation == "len" {
						// The maximum associated length itself is host-sized.
						// Fixed calls panic on both supported host widths.
						continue
					}
					fmt.Fprintf(&out, "%s:%s\t128\t%d\t%d\t%d\n", kind, operation, base, value.hi, value.lo)
					extension++
				}
			}
		}
	}
	return out.Bytes(), valid, invalid, extension
}

func edgeAndRandomValues(width int) []value128 {
	var values []value128
	seen := make(map[value128]bool)
	add := func(value value128) {
		if width < 128 {
			value.hi = 0
		}
		if width == 32 {
			value.lo &= uint64(^uint32(0))
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	for _, low := range []uint64{0, 1, 35, 36, 37, ^uint64(0)} {
		add(value128{lo: low})
	}
	for bit := 0; bit < width; bit++ {
		var value value128
		if bit < 64 {
			value.lo = 1 << bit
		} else {
			value.hi = 1 << (bit - 64)
		}
		add(value)
		low, borrow := bits.Sub64(value.lo, 1, 0)
		high, _ := bits.Sub64(value.hi, 0, borrow)
		add(value128{high, low})
		low, carry := bits.Add64(value.lo, 1, 0)
		high, _ = bits.Add64(value.hi, 0, carry)
		add(value128{high, low})
	}
	add(value128{hi: ^uint64(0), lo: ^uint64(0)})
	state := uint64(0xa0761d6478bd642f)
	next := func() uint64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return state
	}
	randomCount := 128
	if width == 128 {
		randomCount = 256
	}
	for i := 0; i < randomCount; i++ {
		add(value128{next(), next()})
	}
	return values
}
