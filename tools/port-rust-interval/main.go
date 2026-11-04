// Command port-rust-interval validates the complete pinned interval source
// against Go and writes checksummed, offline original-source fixtures.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-interval/validation"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const smallvecHash = "67b1b7a3b5fe4f1376887184045fcf45c69e92af734b7aaddc05fb777b6fbd03"

var sourceBlobs = map[string]string{
	"idx.rs":            "60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8",
	"slice.rs":          "415fe370b702c4dea8f511f1faacb128af4cc6be",
	"vec.rs":            "97aad8e6e8c048812f27d1b4df46ed26604be2fa",
	"interval.rs":       "475c81e66ab09229137350775db615e39df24eee",
	"interval/tests.rs": "cf3222e6c6572303939f620858a4d98a90ba2bbe",
}

//go:embed oracle.rs
var oracleSource []byte

const storageSource = "mod idx; pub use idx::{Idx,IntoSliceIdx}; mod slice; pub use slice::IndexSlice; mod vec; pub use vec::IndexVec;\n"

type source struct {
	Path    string `json:"path"`
	GitBlob string `json:"git_blob"`
	SHA256  string `json:"sha256"`
}
type profile struct {
	OverflowChecks, DebugAssertions bool
	Records                         int
	RequestsSHA256, ResponsesSHA256 string
}
type manifest struct {
	Format                int       `json:"format"`
	Revision              string    `json:"rust_revision"`
	Rustc                 string    `json:"rustc"`
	HostPointerBits       int       `json:"host_pointer_bits"`
	Sources               []source  `json:"sources"`
	SmallvecVersion       string    `json:"smallvec_version"`
	SmallvecArchiveSHA256 string    `json:"smallvec_archive_sha256"`
	SmallvecSourceSHA256  string    `json:"smallvec_source_sha256"`
	SmallvecFeatures      []string  `json:"smallvec_features"`
	OriginalTests         int       `json:"original_tests"`
	OracleSHA256          string    `json:"oracle_sha256"`
	StorageWrapperSHA256  string    `json:"storage_wrapper_sha256"`
	Profiles              []profile `json:"profiles"`
	Records               int       `json:"records"`
	FixtureSHA256         string    `json:"fixture_sha256"`
	RawFixtureSHA256      string    `json:"raw_fixture_sha256"`
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
	compiler := flag.String("rustc", compilerDefault, "pinned Rust compiler name/path")
	upstream := flag.String("rust-source", sourceDefault, "pinned original Rust checkout")
	archive := flag.String("smallvec-archive", ".toolchain/port-rust-interval-deps/smallvec-1.15.1.crate", "checksummed Cargo.lock dependency archive")
	check := flag.Bool("check", false, "verify original Rust answers, Go behavior and committed artifacts")
	fetch := flag.Bool("fetch-dependency", false, "explicitly permit downloading the pinned SmallVec archive if missing")
	flag.Parse()
	if err := run(*root, *compiler, *upstream, *archive, *check, *fetch); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-interval:", err)
		os.Exit(1)
	}
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func gitBlob(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(data), 0)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
func gitRead(upstream, path string) ([]byte, error) {
	cmd := exec.Command("git", "-C", upstream, "show", revision+":"+path)
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read original %s (fetch absent objects explicitly): %w\n%s", path, err, stderr.Bytes())
	}
	return data, nil
}
func resolvePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
func command(dir, compiler string, args ...string) error {
	cmd := exec.Command(compiler, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "RUSTC_BOOTSTRAP=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compile original source: %w\n%s", err, out)
	}
	return nil
}
func unpack(archive []byte, destination string) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || !strings.HasPrefix(clean, "smallvec-1.15.1"+string(filepath.Separator)) {
			if header.Typeflag == tar.TypeDir && clean == "smallvec-1.15.1" {
				continue
			}
			return fmt.Errorf("unexpected dependency archive member %q", header.Name)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("unexpected dependency archive link %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
	}
	return nil
}
func run(root, compiler, upstream, archivePath string, check, fetch bool) error {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("oracle regeneration requires the pinned 64-bit host")
	}
	if !filepath.IsAbs(compiler) {
		if strings.ContainsAny(compiler, `/\`) {
			compiler = resolvePath(root, compiler)
		} else {
			compiler, err = exec.LookPath(compiler)
			if err != nil {
				return err
			}
		}
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, strings.TrimSpace(string(version)))
	}
	upstream = resolvePath(root, upstream)
	head, err := exec.Command("git", "-C", upstream, "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("source checkout HEAD differs from %s", revision)
	}
	lock, err := gitRead(upstream, "Cargo.lock")
	if err != nil {
		return err
	}
	if !bytes.Contains(lock, []byte("name = \"smallvec\"\nversion = \"1.15.1\"\nsource = \"registry+https://github.com/rust-lang/crates.io-index\"\nchecksum = \""+smallvecHash+"\"")) {
		return fmt.Errorf("pinned Cargo.lock SmallVec entry differs")
	}
	archivePath = resolvePath(root, archivePath)
	archive, err := os.ReadFile(archivePath)
	if os.IsNotExist(err) && fetch {
		client := http.Client{Timeout: 30 * time.Second}
		response, downloadErr := client.Get("https://static.crates.io/crates/smallvec/smallvec-1.15.1.crate")
		if downloadErr != nil {
			return downloadErr
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("dependency download: %s", response.Status)
		}
		archive, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err == nil && digest(archive) == smallvecHash {
			if err = os.MkdirAll(filepath.Dir(archivePath), 0755); err == nil {
				err = os.WriteFile(archivePath, archive, 0644)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("read SmallVec archive (use -fetch-dependency to download explicitly): %w", err)
	}
	if digest(archive) != smallvecHash {
		return fmt.Errorf("SmallVec archive checksum differs from pinned Cargo.lock")
	}
	dir, err := os.MkdirTemp("", "ferrugo-interval-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := unpack(archive, dir); err != nil {
		return err
	}
	meta := manifest{Format: 1, Revision: revision, Rustc: rustVersion, HostPointerBits: 64, SmallvecVersion: "1.15.1", SmallvecArchiveSHA256: smallvecHash, SmallvecFeatures: []string{}, OriginalTests: 7, OracleSHA256: digest(oracleSource), StorageWrapperSHA256: digest([]byte(storageSource))}
	for _, path := range []string{"idx.rs", "slice.rs", "vec.rs", "interval.rs", "interval/tests.rs"} {
		fullPath := "compiler/rustc_index/src/" + path
		data, err := gitRead(upstream, fullPath)
		if err != nil {
			return err
		}
		if gitBlob(data) != sourceBlobs[path] {
			return fmt.Errorf("original source Git blob differs: %s", path)
		}
		meta.Sources = append(meta.Sources, source{Path: fullPath, GitBlob: sourceBlobs[path], SHA256: digest(data)})
		target := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil {
			return err
		}
	}
	smallvec := filepath.Join(dir, "smallvec-1.15.1/src/lib.rs")
	smallvecSource, err := os.ReadFile(smallvec)
	if err != nil {
		return err
	}
	meta.SmallvecSourceSHA256 = digest(smallvecSource)
	if err := os.WriteFile(filepath.Join(dir, "storage.rs"), []byte(storageSource), 0600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.rs"), oracleSource, 0600); err != nil {
		return err
	}
	if err := command(dir, compiler, "--edition=2018", "--crate-name=smallvec", "--crate-type=rlib", smallvec, "-o", "libsmallvec.rlib"); err != nil {
		return err
	}
	requests := oracle.Requests()
	var fixtureRaw bytes.Buffer
	encoder := json.NewEncoder(&fixtureRaw)
	for _, flags := range [][2]bool{{true, true}, {false, false}, {true, false}, {false, true}} {
		overflow, debug := flags[0], flags[1]
		checkedText, debugText := "no", "no"
		if overflow {
			checkedText = "yes"
		}
		if debug {
			debugText = "yes"
		}
		options := []string{"--edition=2024", "-C", "opt-level=2", "-C", "overflow-checks=" + checkedText, "-C", "debug-assertions=" + debugText}
		args := append(append([]string{}, options...), "--crate-name=original_index", "--crate-type=rlib", "storage.rs", "-o", "liboriginal_index.rlib")
		if err := command(dir, compiler, args...); err != nil {
			return err
		}
		args = append(append([]string{}, options...), "--crate-name=oracle", "main.rs", "--extern", "smallvec=libsmallvec.rlib", "--extern", "original_index=liboriginal_index.rlib", "-o", "oracle")
		if err := command(dir, compiler, args...); err != nil {
			return err
		}
		if overflow && debug {
			testArgs := append(append([]string{}, args...), "--test")
			for i := range testArgs {
				if testArgs[i] == "oracle" && i > 0 && testArgs[i-1] == "-o" {
					testArgs[i] = "upstream-tests"
				}
			}
			if err := command(dir, compiler, testArgs...); err != nil {
				return err
			}
			output, err := exec.Command(filepath.Join(dir, "upstream-tests"), "--test-threads=1").CombinedOutput()
			if err != nil || !bytes.Contains(output, []byte("7 passed")) {
				return fmt.Errorf("whole unchanged original tests failed: %v\n%s", err, output)
			}
		}
		var input bytes.Buffer
		for _, r := range requests {
			input.WriteString(r.Protocol())
		}
		cmd := exec.Command(filepath.Join(dir, "oracle"))
		cmd.Stdin = bytes.NewReader(input.Bytes())
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("original Rust oracle: %w\n%s", err, stderr.Bytes())
		}
		lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
		if len(lines) != len(requests) {
			return fmt.Errorf("oracle returned %d records for %d requests", len(lines), len(requests))
		}
		for i, response := range lines {
			actual := oracle.Execute(requests[i], overflow, debug)
			if actual != response {
				return fmt.Errorf("Go differs at profile overflow=%v/debug=%v case %d\nrequest: %s\nGo:   %s\nRust: %s", overflow, debug, i, requests[i].Protocol(), actual, response)
			}
			if err := encoder.Encode(oracle.Record{Request: requests[i], OverflowChecks: overflow, DebugAssertions: debug, Response: response}); err != nil {
				return err
			}
		}
		meta.Profiles = append(meta.Profiles, profile{OverflowChecks: overflow, DebugAssertions: debug, Records: len(requests), RequestsSHA256: digest(input.Bytes()), ResponsesSHA256: digest(output)})
		meta.Records += len(requests)
	}
	fixturePath := filepath.Join(root, "tools/port-rust-interval/testdata/oracle.jsonl.gz")
	manifestPath := filepath.Join(root, "tools/port-rust-interval/manifest.json")
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
		if !bytes.Equal(previous, fixtureRaw.Bytes()) {
			return fmt.Errorf("committed fixture differs from original Rust output")
		}
	} else {
		var buf bytes.Buffer
		writer, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return err
		}
		if _, err := writer.Write(fixtureRaw.Bytes()); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		compressed = buf.Bytes()
	}
	meta.FixtureSHA256 = digest(compressed)
	meta.RawFixtureSHA256 = digest(fixtureRaw.Bytes())
	metadata, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metadata = append(metadata, '\n')
	if check {
		previous, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, metadata) {
			return fmt.Errorf("manifest differs from verified provenance")
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fixturePath, compressed, 0644); err != nil {
			return err
		}
		if err := os.WriteFile(manifestPath, metadata, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("interval: 7 unchanged upstream tests; %d Go/Rust records across four overflow/debug profiles verified\n", meta.Records)
	return nil
}
