// Command port-rust-index regenerates original-source Idx/IntoSliceIdx fixtures.
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

	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-index/validation"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const sourcePath = "compiler/rustc_index/src/idx.rs"
const sourceBlob = "60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8"

//go:embed oracle.rs
var oracleSource []byte

type profile struct {
	Nightly        bool `json:"nightly"`
	OverflowChecks bool `json:"overflow_checks"`
	Records        int  `json:"records"`
}
type manifest struct {
	Format                   int    `json:"format"`
	Revision                 string `json:"rust_revision"`
	SourcePath               string `json:"source_path"`
	SourceBlob               string `json:"source_git_blob"`
	SourceSHA256             string `json:"source_sha256"`
	SourceLines, SourceBytes int
	Rustc                    string    `json:"rustc"`
	PointerBits              int       `json:"host_pointer_bits"`
	Profiles                 []profile `json:"profiles"`
	TotalRecords             int       `json:"total_records"`
	OracleSourceSHA256       string    `json:"oracle_source_sha256"`
	RequestsSHA256           string    `json:"requests_sha256"`
	ResponsesSHA256          string    `json:"responses_sha256"`
	FixtureSHA256            string    `json:"fixture_sha256"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	compiler := os.Getenv("FERRUGO_RUSTC")
	if compiler == "" {
		compiler = ".toolchain/rust/bin/rustc"
	}
	checkout := os.Getenv("FERRUGO_RUST_SOURCE")
	if checkout == "" {
		checkout = ".toolchain/upstream-rust"
	}
	rustc := flag.String("rustc", compiler, "pinned Rust compiler path")
	source := flag.String("rust-source", checkout, "pinned Rust source checkout")
	check := flag.Bool("check", false, "verify existing fixture and manifest")
	flag.Parse()
	if err := run(*root, *rustc, *source, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-index:", err)
		os.Exit(1)
	}
}
func run(root, rustc, checkout string, check bool) error {
	if !oracle.NightlyEnabled {
		return fmt.Errorf("run with Go build tag ferrugo_rustc_nightly to validate all four profiles")
	}
	if strconv.IntSize != 64 {
		return fmt.Errorf("regeneration requires the pinned 64-bit host")
	}
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
		return fmt.Errorf("expected %s; got %s", rustVersion, bytes.TrimSpace(version))
	}
	head, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("source HEAD differs from pin")
	}
	source, err := exec.Command("git", "-C", checkout, "show", revision+":"+sourcePath).Output()
	if err != nil {
		return err
	}
	if blob(source) != sourceBlob {
		return fmt.Errorf("source Git blob differs")
	}
	temp, err := os.MkdirTemp("", "ferrugo-index-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	for path, data := range map[string][]byte{"idx.rs": source, "main.rs": oracleSource} {
		if err := os.WriteFile(filepath.Join(temp, path), data, 0600); err != nil {
			return err
		}
	}
	meta := manifest{Format: 1, Revision: revision, SourcePath: sourcePath, SourceBlob: sourceBlob, SourceSHA256: digest(source), SourceLines: bytes.Count(source, []byte{'\n'}), SourceBytes: len(source), Rustc: rustVersion, PointerBits: strconv.IntSize, OracleSourceSHA256: digest(oracleSource)}
	var records, allRequests bytes.Buffer
	for _, nightly := range []bool{false, true} {
		for _, checked := range []bool{false, true} {
			name := fmt.Sprintf("oracle-%t-%t", nightly, checked)
			overflow := "no"
			if checked {
				overflow = "yes"
			}
			args := []string{"--edition=2024", "-C", "opt-level=2", "-C", "overflow-checks=" + overflow, "main.rs", "-o", name}
			if nightly {
				args = append(args, "--cfg", `feature="nightly"`)
			}
			command := exec.Command(rustc, args...)
			command.Dir = temp
			if output, err := command.CombinedOutput(); err != nil {
				return fmt.Errorf("compile %s: %w\n%s", name, err, output)
			}
			requests := oracle.Requests(nightly)
			var input strings.Builder
			for _, request := range requests {
				input.WriteString(request.Protocol())
			}
			fmt.Fprintf(&allRequests, "nightly=%t checked=%t\n", nightly, checked)
			allRequests.WriteString(input.String())
			command = exec.Command(filepath.Join(temp, name))
			command.Stdin = strings.NewReader(input.String())
			var stderr bytes.Buffer
			command.Stderr = &stderr
			output, err := command.Output()
			if err != nil {
				return fmt.Errorf("run %s: %w\n%s", name, err, stderr.Bytes())
			}
			lines := bytes.Split(bytes.TrimSuffix(output, []byte{'\n'}), []byte{'\n'})
			if len(lines) != len(requests) {
				return fmt.Errorf("record count differs for %s", name)
			}
			encoder := json.NewEncoder(&records)
			for i, line := range lines {
				response := string(line)
				if err := oracle.CaptureRangeState(&requests[i], response); err != nil {
					return err
				}
				if got := oracle.Execute(requests[i], checked); got != response {
					return fmt.Errorf("%s case %d request=%+v\nGo=%s\nRust=%s", name, i, requests[i], got, response)
				}
				if err := encoder.Encode(oracle.Record{Request: requests[i], Nightly: nightly, OverflowChecks: checked, Response: response}); err != nil {
					return err
				}
			}
			meta.Profiles = append(meta.Profiles, profile{Nightly: nightly, OverflowChecks: checked, Records: len(requests)})
			meta.TotalRecords += len(requests)
		}
	}
	fixturePath := filepath.Join(root, "port/rust/compiler/rustc_index/src/idx_testdata/oracle.jsonl.gz")
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
			return fmt.Errorf("original-source fixture differs")
		}
	} else {
		var buffer bytes.Buffer
		writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
		if err != nil {
			return err
		}
		if _, err := writer.Write(records.Bytes()); err != nil {
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		fixture = buffer.Bytes()
	}
	meta.RequestsSHA256 = digest(allRequests.Bytes())
	meta.ResponsesSHA256 = digest(records.Bytes())
	meta.FixtureSHA256 = digest(fixture)
	metadata, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	metadata = append(metadata, '\n')
	for path, data := range map[string][]byte{fixturePath: fixture, filepath.Join(root, "tools/port-rust-index/manifest.json"): metadata} {
		if check {
			existing, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, data) {
				return fmt.Errorf("generated metadata differs: %s", path)
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
	fmt.Printf("%s %d complete original-source Idx records across four profiles\n", action, meta.TotalRecords)
	return nil
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func blob(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(data), 0)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}
