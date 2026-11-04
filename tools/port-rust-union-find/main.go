// Command port-rust-union-find compiles the complete pinned upstream source and
// regenerates differential cases for both debug and release assertion profiles.
// SPDX-License-Identifier: MIT OR Apache-2.0
package main

import (
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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-union-find/validation"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"

//go:embed oracle.rs
var oracleSource []byte

type source struct {
	Path    string `json:"path"`
	GitBlob string `json:"git_blob"`
	SHA256  string `json:"sha256"`
	Lines   int    `json:"lines"`
	Bytes   int    `json:"bytes"`
}

type manifest struct {
	Format             int      `json:"format"`
	Revision           string   `json:"rust_revision"`
	Rustc              string   `json:"rustc"`
	HostPointerBits    int      `json:"host_pointer_bits"`
	Sources            []source `json:"sources"`
	UpstreamTests      int      `json:"upstream_tests"`
	RequestsPerProfile int      `json:"requests_per_profile"`
	Profiles           []string `json:"profiles"`
	Records            int      `json:"records"`
	OracleSourceSHA256 string   `json:"oracle_source_sha256"`
	RequestsSHA256     string   `json:"requests_sha256"`
	ResponsesSHA256    string   `json:"responses_sha256"`
	FixtureSHA256      string   `json:"fixture_sha256"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	rustcDefault := os.Getenv("FERRUGO_RUSTC")
	if rustcDefault == "" {
		rustcDefault = ".toolchain/rust/bin/rustc"
	}
	sourceDefault := os.Getenv("FERRUGO_RUST_SOURCE")
	if sourceDefault == "" {
		sourceDefault = ".toolchain/upstream-rust"
	}
	rustc := flag.String("rustc", rustcDefault, "pinned Rust compiler path")
	checkout := flag.String("rust-source", sourceDefault, "pinned Rust source checkout")
	check := flag.Bool("check", false, "verify existing golden fixture instead of writing it")
	flag.Parse()
	if err := run(*root, *rustc, *checkout, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-union-find:", err)
		os.Exit(1)
	}
}

func run(root, rustc, checkout string, check bool) error {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(rustc) {
		if strings.ContainsAny(rustc, `/\`) {
			rustc = filepath.Join(root, rustc)
		} else {
			rustc, err = exec.LookPath(rustc)
			if err != nil {
				return err
			}
		}
	}
	if !filepath.IsAbs(checkout) {
		checkout = filepath.Join(root, checkout)
	}
	version, err := exec.Command(rustc, "--version").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s; found %s", rustVersion, bytes.TrimSpace(version))
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("oracle regeneration requires the pinned 64-bit host")
	}
	head, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("Rust source HEAD differs from %s", revision)
	}
	sources := []source{
		{Path: "compiler/rustc_data_structures/src/union_find.rs", GitBlob: "ef73cd7ab40abb07d1281315625d312632c5bbae"},
		{Path: "compiler/rustc_data_structures/src/union_find/tests.rs", GitBlob: "34a4e4f8e6edc36719eb5ad996252420957cfc22"},
		{Path: "compiler/rustc_index/src/idx.rs", GitBlob: "60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8"},
		{Path: "compiler/rustc_index/src/vec.rs", GitBlob: "97aad8e6e8c048812f27d1b4df46ed26604be2fa"},
		{Path: "compiler/rustc_index/src/slice.rs", GitBlob: "415fe370b702c4dea8f511f1faacb128af4cc6be"},
	}
	temp, err := os.MkdirTemp("", "ferrugo-union-find-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := os.MkdirAll(filepath.Join(temp, "upstream/union_find"), 0755); err != nil {
		return err
	}
	for i := range sources {
		item := &sources[i]
		data, err := exec.Command("git", "-C", checkout, "show", revision+":"+item.Path).Output()
		if err != nil {
			return fmt.Errorf("read %s: %w", item.Path, err)
		}
		if blob(data) != item.GitBlob {
			return fmt.Errorf("source git blob differs for %s", item.Path)
		}
		item.SHA256 = digest(data)
		item.Lines = bytes.Count(data, []byte{'\n'})
		item.Bytes = len(data)
		var path string
		switch i {
		case 0:
			path = "upstream/union_find.rs"
		case 1:
			path = "upstream/union_find/tests.rs"
		case 2:
			path = "upstream/idx.rs"
		}
		if path != "" {
			if err := os.WriteFile(filepath.Join(temp, path), data, 0600); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(temp, "main.rs"), oracleSource, 0600); err != nil {
		return err
	}
	compile := func(name string, debug, test bool) error {
		profile := "no"
		if debug {
			profile = "yes"
		}
		args := []string{"--edition=2024", "-C", "opt-level=2", "-C", "overflow-checks=yes", "-C", "debug-assertions=" + profile, "main.rs", "-o", name}
		if test {
			args = append(args, "--test")
		}
		command := exec.Command(rustc, args...)
		command.Dir = temp
		if output, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("compile %s: %w\n%s", name, err, output)
		}
		return nil
	}
	if err := compile("upstream-tests", true, true); err != nil {
		return err
	}
	if output, err := exec.Command(filepath.Join(temp, "upstream-tests"), "--test-threads=1").CombinedOutput(); err != nil || !bytes.Contains(output, []byte("2 passed")) {
		return fmt.Errorf("original Rust tests: %v\n%s", err, output)
	}
	requests := oracle.Requests()
	var protocol strings.Builder
	for _, request := range requests {
		protocol.WriteString(request.Protocol())
	}
	var records bytes.Buffer
	for _, debug := range []bool{false, true} {
		name := "oracle-release"
		if debug {
			name = "oracle-debug"
		}
		if err := compile(name, debug, false); err != nil {
			return err
		}
		command := exec.Command(filepath.Join(temp, name))
		command.Stdin = strings.NewReader(protocol.String())
		var stderr bytes.Buffer
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			return fmt.Errorf("run %s: %w\n%s", name, err, stderr.Bytes())
		}
		lines := bytes.Split(bytes.TrimSuffix(output, []byte{'\n'}), []byte{'\n'})
		if len(lines) != len(requests) {
			return fmt.Errorf("%s returned %d records, want %d", name, len(lines), len(requests))
		}
		encoder := json.NewEncoder(&records)
		for i, line := range lines {
			response := string(line)
			if got := oracle.Execute(requests[i], debug); got != response {
				return fmt.Errorf("Go differs at %s case %d: request=%+v\nGo=%s\nRust=%s", name, i, requests[i], got, response)
			}
			if err := encoder.Encode(oracle.Record{Request: requests[i], DebugAssertions: debug, Response: response}); err != nil {
				return err
			}
		}
	}
	fixturePath := filepath.Join(root, "port/rust/compiler/rustc_data_structures/src/union_find/testdata/oracle.jsonl.gz")
	manifestPath := filepath.Join(root, "tools/port-rust-union-find/manifest.json")
	var fixture []byte
	if check {
		fixture, err = os.ReadFile(fixturePath)
		if err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(fixture))
		if err != nil {
			return err
		}
		raw, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			return err
		}
		if !bytes.Equal(raw, records.Bytes()) {
			return fmt.Errorf("golden oracle records differ")
		}
	} else {
		var compressed bytes.Buffer
		writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		if err != nil {
			return err
		}
		if _, err := writer.Write(records.Bytes()); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		fixture = compressed.Bytes()
	}
	meta := manifest{Format: 1, Revision: revision, Rustc: rustVersion, HostPointerBits: strconv.IntSize, Sources: sources, UpstreamTests: 2, RequestsPerProfile: len(requests), Profiles: []string{"release: debug_assertions=false", "debug: debug_assertions=true"}, Records: len(requests) * 2, OracleSourceSHA256: digest(oracleSource), RequestsSHA256: digest([]byte(protocol.String())), ResponsesSHA256: digest(records.Bytes()), FixtureSHA256: digest(fixture)}
	metadata, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metadata = append(metadata, '\n')
	for path, data := range map[string][]byte{fixturePath: fixture, manifestPath: metadata} {
		if check {
			existing, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("manifest differs: %s", path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}
		}
	}
	action := "generated"
	if check {
		action = "verified"
	}
	fmt.Printf("%s %d original-source records in two assertion profiles; 2 unchanged upstream tests pass\n", action, len(requests)*2)
	return nil
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func blob(data []byte) string {
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d%c", len(data), 0)
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}
