// Command port-rust-binary-search regenerates original-source search results,
// callback traces and panic outcomes for the complete binary_search_util port.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const sourcePrefix = "compiler/rustc_data_structures/src/binary_search_util/"

type source struct {
	Path    string `json:"path"`
	GitBlob string `json:"git_blob"`
	SHA256  string `json:"sha256"`
}
type manifest struct {
	Format             int      `json:"format"`
	RustRevision       string   `json:"rust_revision"`
	Rustc              string   `json:"rustc"`
	Sources            []source `json:"sources"`
	Cases              uint32   `json:"cases"`
	OracleSourceSHA256 string   `json:"oracle_source_sha256"`
	OracleSHA256       string   `json:"oracle_sha256"`
	FixtureSHA256      string   `json:"fixture_sha256"`
}

func main() {
	compilerDefault := os.Getenv("FERRUGO_RUSTC")
	if compilerDefault == "" {
		compilerDefault = ".toolchain/rust/bin/rustc"
	}
	sourceDefault := os.Getenv("FERRUGO_RUST_SOURCE")
	if sourceDefault == "" {
		sourceDefault = ".toolchain/upstream-rust"
	}
	root := flag.String("root", ".", "repository root")
	compiler := flag.String("rustc", compilerDefault, "Rust compiler name/path, defaults to FERRUGO_RUSTC")
	checkout := flag.String("rust-source", sourceDefault, "pinned Rust checkout, defaults to FERRUGO_RUST_SOURCE")
	check := flag.Bool("check", false, "verify reference files without changing them")
	flag.Parse()
	if err := run(*root, *compiler, *checkout, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-binary-search:", err)
		os.Exit(1)
	}
}
func run(root, compiler, checkout string, check bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(compiler) {
		if strings.ContainsAny(compiler, "/\\") {
			compiler = filepath.Join(root, compiler)
		} else {
			compiler, err = exec.LookPath(compiler)
			if err != nil {
				return err
			}
		}
	}
	if !filepath.IsAbs(checkout) {
		checkout = filepath.Join(root, checkout)
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, strings.TrimSpace(string(version)))
	}
	head, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("Rust source pin differs: %s", strings.TrimSpace(string(head)))
	}
	temp, err := os.MkdirTemp("", "rust-binary-search-original-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	upstream := filepath.Join(temp, "upstream")
	if err := os.Mkdir(upstream, 0755); err != nil {
		return err
	}
	sources := []source{{Path: sourcePrefix + "mod.rs", GitBlob: "1c6e227cec40870c0288f60270f609aeb4ebdc8e"}, {Path: sourcePrefix + "tests.rs", GitBlob: "d74febb5c0fc4a13e2530de71982520e3a944fcf"}}
	for i := range sources {
		item := &sources[i]
		data, err := exec.Command("git", "-C", checkout, "show", revision+":"+item.Path).Output()
		if err != nil {
			return fmt.Errorf("read pinned %s: %w", item.Path, err)
		}
		if blob(data) != item.GitBlob {
			return fmt.Errorf("source blob mismatch: %s", item.Path)
		}
		item.SHA256 = digest(data)
		if err := os.WriteFile(filepath.Join(upstream, filepath.Base(item.Path)), data, 0644); err != nil {
			return err
		}
	}
	source := filepath.Join(upstream, "mod.rs")
	originalTests := filepath.Join(temp, "original-tests")
	command := exec.Command(compiler, "--edition=2024", "--crate-name", "binary_search_original", "--test", "-C", "opt-level=2", source, "-o", originalTests)
	if diagnostics, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("compile upstream tests: %w\n%s", err, diagnostics)
	}
	if diagnostics, err := exec.Command(originalTests).CombinedOutput(); err != nil {
		return fmt.Errorf("run upstream tests: %w\n%s", err, diagnostics)
	}
	oraclePath := filepath.Join(root, "tools", "port-rust-binary-search", "oracle.rs")
	oracleSource, err := os.ReadFile(oraclePath)
	if err != nil {
		return err
	}
	executable := filepath.Join(temp, "oracle")
	command = exec.Command(compiler, "--edition=2024", "-C", "opt-level=2", oraclePath, "-o", executable)
	command.Env = append(os.Environ(), "BINARY_SEARCH_SOURCE="+source)
	if diagnostics, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("compile original-source oracle: %w\n%s", err, diagnostics)
	}
	raw, err := exec.Command(executable).Output()
	if err != nil {
		return fmt.Errorf("execute original-source oracle: %w", err)
	}
	if len(raw) < 10 || string(raw[:6]) != "RBSU2\x00" {
		return fmt.Errorf("invalid oracle stream")
	}
	cases := binary.LittleEndian.Uint32(raw[6:10])
	fixturePath := filepath.Join(root, "port", "rust", "compiler", "rustc_data_structures", "src", "binary_search_util", "testdata", "oracle.bin.gz")
	fixture, err := oracleFixture(fixturePath, raw, check)
	if err != nil {
		return err
	}
	meta := manifest{Format: 2, RustRevision: revision, Rustc: rustVersion, Sources: sources, Cases: cases, OracleSourceSHA256: digest(oracleSource), OracleSHA256: digest(raw), FixtureSHA256: digest(fixture)}
	metadata, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metadata = append(metadata, '\n')
	target := "port/rust/compiler/rustc_data_structures/src/binary_search_util/"
	files := map[string][]byte{target + "oracle_manifest.json": metadata, target + "testdata/oracle.bin.gz": fixture}
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
	fmt.Printf("%s %d original Rust binary-search cases with key/comparator traces and panic outcomes\n", action, cases)
	return nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func blob(data []byte) string {
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d\x00", len(data))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// Go versions can produce different valid DEFLATE streams for identical bytes.
// Reuse an existing stream only after bounded decompression reaches a valid
// checksum/EOF and yields the exact original Rust oracle. The manifest still
// binds the exact compressed bytes, so check mode also detects changed streams.
func oracleFixture(path string, oracle []byte, check bool) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		reader, openErr := gzip.NewReader(bytes.NewReader(existing))
		if openErr == nil {
			decoded, readErr := io.ReadAll(io.LimitReader(reader, int64(len(oracle))+1))
			closeErr := reader.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(decoded, oracle) {
				return existing, nil
			}
		}
		if check {
			return nil, fmt.Errorf("compressed fixture does not contain the exact Rust oracle: %s", path)
		}
	} else if !os.IsNotExist(err) || check {
		return nil, err
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(oracle); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}
