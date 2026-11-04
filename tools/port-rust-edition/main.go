// Command port-rust-edition validates the complete original edition module,
// its actual derives, and the Go trait-operation adapters against real codecs
// and hashers. Reference dependency facades are scoped and audited, not ports.
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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const editionRevision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const editionSourcePath = "compiler/rustc_span/src/edition.rs"
const editionSourceBlob = "d4f3f0e208e34401cf6d02af5a8606a3980a0171"
const editionSourceSHA = "a1dd82ce1a5b22b27e3360bcf8bd606fd4fa6669f6de6305536d6436d63ac7ae"
const editionPackage = "port/rust/compiler/rustc_span/src"

//go:embed oracle.rs
var editionOracle []byte

type editionBuildProfile struct {
	Revision            string          `json:"rust_revision"`
	SourceVersion       string          `json:"pinned_source_version"`
	Compiler            string          `json:"compiler"`
	Rustflags           string          `json:"rustflags"`
	SourceProvenanceSHA string          `json:"source_provenance_sha256"`
	Registry            json.RawMessage `json:"registry_dependencies"`
	Audit               json.RawMessage `json:"used_method_audit"`
	Library             string          `json:"library"`
	LinkArguments       []string        `json:"link_arguments"`
}

func main() {
	root := flag.String("root", ".", "repository root")
	rustc := flag.String("rustc", editionEnv("FERRUGO_RUSTC", ".toolchain/rust/bin/rustc"), "installed reference rustc path or PATH name")
	cargo := flag.String("cargo", editionEnv("FERRUGO_CARGO", ".toolchain/rust/bin/cargo"), "installed reference Cargo path or PATH name")
	check := flag.Bool("check", false, "check live original output and evidence without rewriting retained files")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(1)
	}
	if err := editionRun(*root, *rustc, *cargo, *check); err != nil {
		fmt.Fprintln(os.Stderr, "port-rust-edition:", err)
		os.Exit(1)
	}
}

func editionEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func editionExecutable(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("explicit executable override is empty")
	}
	if !filepath.IsAbs(name) {
		if strings.ContainsAny(name, `/\`) {
			name = filepath.Join(root, name)
		} else {
			resolved, err := exec.LookPath(name)
			if err != nil {
				return "", err
			}
			name = resolved
		}
	}
	return name, nil
}

func editionRun(root, rustc, cargo string, check bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	rustc, err = editionExecutable(root, rustc)
	if err != nil {
		return err
	}
	cargo, err = editionExecutable(root, cargo)
	if err != nil {
		return err
	}
	if editionHostBits != 64 {
		return fmt.Errorf("reference generation requires the recorded 64-bit host profile")
	}
	// --offline covers dependencies; preparation also rejects missing pinned
	// source objects rather than mutating original source or inventing derives.
	args := []string{filepath.Join(root, "tools/port-rust-edition/reference-build/prepare.py"), "--root", root, "--rustc", rustc, "--cargo", cargo, "--offline", "--validate"}
	if _, err := editionCommand(3*time.Minute, root, nil, nil, "python3", args...); err != nil {
		return err
	}
	cache := filepath.Join(root, ".toolchain/oracles/edition-reference")
	profileBytes, err := os.ReadFile(filepath.Join(cache, "build-profile.json"))
	if err != nil {
		return err
	}
	var profile editionBuildProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		return err
	}
	if profile.Revision != editionRevision || profile.Library == "" || len(profile.LinkArguments) == 0 {
		return fmt.Errorf("incomplete or unpinned reference build profile")
	}
	source, err := os.ReadFile(filepath.Join(cache, "original", editionSourcePath))
	if err != nil {
		return err
	}
	if editionDigest(source) != editionSourceSHA {
		return fmt.Errorf("complete original Edition source hash differs")
	}
	provenance, err := os.ReadFile(filepath.Join(cache, "source-provenance.json"))
	if err != nil {
		return err
	}
	if editionDigest(provenance) != profile.SourceProvenanceSHA {
		return fmt.Errorf("reference source provenance hash differs")
	}
	expanded, err := os.ReadFile(filepath.Join(cache, "expanded-edition.rs"))
	if err != nil {
		return err
	}
	helpers, err := editionHelperHashes(filepath.Join(root, "tools/port-rust-edition/reference-build"))
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "ferrugo-edition-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	sourceFile := filepath.Join(dir, "oracle.rs")
	if err := os.WriteFile(sourceFile, editionOracle, 0600); err != nil {
		return err
	}
	binary := filepath.Join(dir, "oracle")
	compile := []string{"--edition=2024", sourceFile, "-o", binary}
	compile = append(compile, strings.Fields(profile.Rustflags)...)
	compile = append(compile, profile.LinkArguments...)
	if _, err := editionCommand(time.Minute, root, nil, []string{"RUSTC_BOOTSTRAP=1"}, rustc, compile...); err != nil {
		return err
	}
	requests, err := editionRequests()
	if err != nil {
		return err
	}
	raw, err := editionCommand(time.Minute, root, requests, nil, binary)
	if err != nil {
		return err
	}
	if !bytes.HasSuffix(raw, []byte("unused-interface-calls\t0\n")) {
		return fmt.Errorf("reference did not prove zero unused-interface calls")
	}
	counts := countEditionRecords(raw)
	if counts["variant"] != 5 || counts["compare"] != 25 || counts["format"] != 75 || counts["hash"] != 645 {
		return fmt.Errorf("reference coverage count differs: %v", counts)
	}
	fixturePath := filepath.Join(root, editionPackage, "edition_testdata/oracle.tsv.gz")
	compressed, err := editionFixture(fixturePath, raw, check)
	if err != nil {
		return err
	}
	meta := map[string]any{
		"format": 1, "rust_revision": editionRevision, "source_path": editionSourcePath, "source_git_blob": editionSourceBlob, "source_sha256": editionSourceSHA,
		"pinned_source_version": profile.SourceVersion, "compiler": profile.Compiler, "rustc_bootstrap": "1", "rustflags": profile.Rustflags,
		"development_reference_only": true, "reference_dependency_port_credit": false,
		"reference_scope":                    "Complete unchanged Edition and actual macros/codec/hash algorithms; exact relevant traits/primitive implementation; only unused interface types are opaque fail-fast sentinels. Not a full rustc_span/rustc_data_structures or compiler ABI build.",
		"reference_source_provenance_sha256": profile.SourceProvenanceSHA, "reference_helper_sha256": helpers, "reference_registry_dependencies": profile.Registry, "used_method_audit": profile.Audit,
		"expanded_source_sha256": editionDigest(expanded), "oracle_source_sha256": editionDigest(editionOracle), "request_sha256": editionDigest(requests), "oracle_raw_sha256": editionDigest(raw), "fixture_sha256": editionDigest(compressed), "case_counts": counts,
		"hash_validation": "Each request replays operations emitted by the actual Go Hash/StableHash adapters into real Rust hashers and compares their stateful digests with the original derived impls; the production Go code never invokes Rust or hardcodes digests.",
	}
	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	manifestPath := filepath.Join(root, editionPackage, "edition_oracle_manifest.json")
	if check {
		previous, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(previous, encoded) {
			return fmt.Errorf("retained manifest differs from verified original-source/runtime provenance")
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
	fmt.Printf("edition: all 5 variants/derives, %d stateful hash comparisons, all byte tags and %d parse cases; unused interface calls=0\n", counts["hash"], counts["parse"])
	return nil
}

func editionCommand(timeout time.Duration, dir string, input []byte, extraEnv []string, executable string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = dir
	command.Stdin = bytes.NewReader(input)
	if extraEnv != nil {
		command.Env = append(os.Environ(), extraEnv...)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", executable, args, err, stderr.Bytes())
	}
	return output, nil
}

func editionDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func editionHelperHashes(directory string) (map[string]string, error) {
	values := make(map[string]string)
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".rs" && ext != ".py" && ext != ".toml" && filepath.Base(path) != "Cargo.lock" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		values[filepath.ToSlash(relative)] = editionDigest(data)
		return nil
	})
	return values, err
}

func editionFixture(path string, raw []byte, check bool) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		reader, err := gzip.NewReader(bytes.NewReader(existing))
		if err == nil {
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
		return nil, fmt.Errorf("retained fixture is missing, invalid or differs from original output")
	}
	var encoded bytes.Buffer
	writer, err := gzip.NewWriterLevel(&encoded, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(raw); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}
