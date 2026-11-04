// Command port-cargo-graph validates the complete pinned Cargo graph.rs port
// against an unchanged-source Rust oracle. It never participates in production.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const cargoRevision = "24de4721844c7a4d839072bab6fb66d41c9bc1f4"
const graphSourceBlob = "74cdc51a0c36f21ce64f291540be571c05b12d63"
const graphSourceHash = "238123cc4c8300921008db5bcec1bbdf0946a023e0acafe9997e6917b5fada51"
const pinnedRustc = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"

type dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Checksum string `json:"crate_sha256"`
}
type outputEvidence struct {
	SHA256 string         `json:"sha256"`
	Bytes  int            `json:"bytes"`
	Panics map[string]int `json:"panic_results,omitempty"`
}
type report struct {
	Format        int                       `json:"format"`
	CargoRevision string                    `json:"cargo_revision"`
	SourcePath    string                    `json:"source_path"`
	SourceGitBlob string                    `json:"source_git_blob"`
	SourceSHA256  string                    `json:"source_sha256"`
	SourceLines   int                       `json:"source_lines"`
	SourceBytes   int                       `json:"source_bytes"`
	Rustc         string                    `json:"rustc"`
	Go            string                    `json:"go"`
	Host          string                    `json:"host"`
	Dependencies  []dependency              `json:"oracle_dependencies"`
	UpstreamTests int                       `json:"upstream_tests_per_profile"`
	Cases         int                       `json:"cases_per_profile"`
	InputSHA256   string                    `json:"case_input_sha256"`
	Profiles      []string                  `json:"profiles"`
	Outputs       map[string]outputEvidence `json:"original_source_outputs"`
	Files         map[string]string         `json:"evidence_sha256"`
	Commands      []string                  `json:"commands"`
	Storage       string                    `json:"storage_adapter"`
	Skips         []string                  `json:"skips"`
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func environment(base []string, override map[string]string) []string {
	out := make([]string, 0, len(base)+len(override))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if _, exists := override[key]; !exists {
			out = append(out, entry)
		}
	}
	for key, value := range override {
		out = append(out, key+"="+value)
	}
	return out
}
func execute(root, executable string, args []string, env []string) ([]byte, error) {
	cmd := exec.Command(executable, args...)
	cmd.Dir, cmd.Env = root, env
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w\n%s", filepath.Base(executable), strings.Join(args, " "), err, output)
	}
	return output, nil
}
func lockEntries(data []byte) map[string]dependency {
	out := make(map[string]dependency)
	for _, block := range strings.Split(string(data), "[[package]]")[1:] {
		entry := dependency{}
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			decoded, err := strconv.Unquote(value)
			if err != nil {
				continue
			}
			switch key {
			case "name":
				entry.Name = decoded
			case "version":
				entry.Version = decoded
			case "checksum":
				entry.Checksum = decoded
			}
		}
		if entry.Name != "" {
			out[entry.Name+"@"+entry.Version] = entry
		}
	}
	return out
}
func environmentDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// An explicit executable never falls back. Relative paths resolve from root;
// bare names use PATH. Unspecified tools prefer the optional local toolchain.
func resolveExecutable(root, name, localDefault string) (string, error) {
	if name == "" {
		local := filepath.Join(root, localDefault)
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			name = local
		} else {
			name = filepath.Base(localDefault)
		}
	}
	if strings.ContainsAny(name, `/\\`) && !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(resolved)
}

func main() {
	root := flag.String("root", ".", "repository root")
	cargo := flag.String("cargo", os.Getenv("FERRUGO_CARGO"), "Cargo executable (FERRUGO_CARGO, local toolchain, then PATH)")
	rustc := flag.String("rustc", os.Getenv("FERRUGO_RUSTC"), "Rust compiler (FERRUGO_RUSTC, local toolchain, then PATH)")
	goBinary := flag.String("go", os.Getenv("FERRUGO_GO"), "Go executable (FERRUGO_GO, local toolchain, then PATH)")
	cargoHome := flag.String("cargo-home", environmentDefault("CARGO_HOME", ".toolchain/port-cargo-home"), "Cargo cache (CARGO_HOME or local isolated cache)")
	check := flag.Bool("check", false, "verify retained fixture and report without changing them")
	offline := flag.Bool("offline", true, "require pinned dependencies already cached")
	flag.Parse()
	explicitFlags := make(map[string]bool)
	flag.Visit(func(option *flag.Flag) { explicitFlags[option.Name] = true })
	for _, tool := range []struct{ flag, environment, value string }{{"cargo", "FERRUGO_CARGO", *cargo}, {"rustc", "FERRUGO_RUSTC", *rustc}, {"go", "FERRUGO_GO", *goBinary}} {
		_, explicitEnvironment := os.LookupEnv(tool.environment)
		if tool.value == "" && (explicitFlags[tool.flag] || explicitEnvironment) {
			fmt.Fprintf(os.Stderr, "port-cargo-graph: -%s/%s executable is explicitly empty\n", tool.flag, tool.environment)
			os.Exit(1)
		}
	}
	if err := run(*root, *cargo, *rustc, *goBinary, *cargoHome, *check, *offline); err != nil {
		fmt.Fprintln(os.Stderr, "port-cargo-graph:", err)
		os.Exit(1)
	}
}
func run(root, cargo, rustc, goBinary, cargoHome string, check, offline bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absolute := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	cargo, err = resolveExecutable(root, cargo, ".toolchain/rust/bin/cargo")
	if err != nil {
		return fmt.Errorf("select Cargo executable: %w", err)
	}
	rustc, err = resolveExecutable(root, rustc, ".toolchain/rust/bin/rustc")
	if err != nil {
		return fmt.Errorf("select Rust compiler: %w", err)
	}
	goBinary, err = resolveExecutable(root, goBinary, ".toolchain/go/bin/go")
	if err != nil {
		return fmt.Errorf("select Go executable: %w", err)
	}
	if cargoHome == "" {
		return fmt.Errorf("Cargo cache path is explicitly empty")
	}
	cargoHome = absolute(cargoHome)
	env := os.Environ()
	version, err := execute(root, rustc, []string{"--version"}, env)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != pinnedRustc {
		return fmt.Errorf("expected %s, found %s", pinnedRustc, version)
	}
	goVersion, err := execute(root, goBinary, []string{"version"}, env)
	if err != nil {
		return err
	}
	head, err := execute(root, "git", []string{"-C", absolute(".toolchain/upstream-cargo"), "rev-parse", "HEAD"}, env)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != cargoRevision {
		return fmt.Errorf("Cargo reference cache is not at pinned revision")
	}
	source := absolute(".toolchain/upstream-cargo/src/util/graph.rs")
	original, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if hash(original) != graphSourceHash {
		return fmt.Errorf("pinned graph.rs source hash mismatch")
	}
	blob, err := execute(root, "git", []string{"-C", absolute(".toolchain/upstream-cargo"), "rev-parse", "HEAD:src/util/graph.rs"}, env)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(blob)) != graphSourceBlob {
		return fmt.Errorf("pinned graph.rs blob mismatch")
	}
	upstreamLock, err := os.ReadFile(absolute(".toolchain/upstream-cargo/Cargo.lock"))
	if err != nil {
		return err
	}
	oracleLock, err := os.ReadFile(absolute("tools/port-cargo-graph/Cargo.lock"))
	if err != nil {
		return err
	}
	upstreamEntries, oracleEntries := lockEntries(upstreamLock), lockEntries(oracleLock)
	var dependencies []dependency
	for _, key := range []string{"bitmaps@2.1.0", "im-rc@15.1.0", "rand_core@0.6.4", "rand_xoshiro@0.6.0", "sized-chunks@0.6.5", "typenum@1.20.1", "version_check@0.9.5"} {
		entry, exists := oracleEntries[key]
		if !exists || entry.Checksum == "" || entry != upstreamEntries[key] {
			return fmt.Errorf("oracle dependency %s does not match pinned Cargo.lock", key)
		}
		dependencies = append(dependencies, entry)
	}
	if len(oracleEntries) != len(dependencies)+1 {
		return fmt.Errorf("unexpected oracle dependency set")
	}
	target := absolute(".toolchain/oracles/cargo-graph/target")
	env = environment(env, map[string]string{"CARGO_HOME": cargoHome, "RUSTC": rustc, "CARGO_GRAPH_SOURCE": source, "CARGO_TARGET_DIR": target})
	manifest := absolute("tools/port-cargo-graph/Cargo.toml")
	for _, release := range []bool{false, true} {
		for _, action := range []string{"test", "build"} {
			args := []string{action, "--locked", "--manifest-path", manifest}
			if offline {
				args = append(args, "--offline")
			}
			if release {
				args = append(args, "--release")
			}
			output, err := execute(root, cargo, args, env)
			if err != nil {
				return err
			}
			if action == "test" && !bytes.Contains(output, []byte("test result: ok. 3 passed;")) {
				return fmt.Errorf("expected all three original graph tests to execute:\n%s", output)
			}
		}
	}
	debug := filepath.Join(target, "debug/ferrugo-cargo-graph-oracle")
	release := filepath.Join(target, "release/ferrugo-cargo-graph-oracle")
	write := "1"
	if check {
		write = ""
	}
	goEnv := environment(os.Environ(), map[string]string{"FERRUGO_GRAPH_ORACLE_DEBUG": debug, "FERRUGO_GRAPH_ORACLE_RELEASE": release, "FERRUGO_GRAPH_WRITE": write})
	output, err := execute(root, goBinary, []string{"test", "./port/cargo/src/util", "-run", "^TestGraph", "-count=1", "-v"}, goEnv)
	if err != nil {
		return err
	}
	if !bytes.Contains(output, []byte("matched 1416 original-source graph cases")) {
		return fmt.Errorf("unexpected Go graph test result:\n%s", output)
	}
	fixturePath := "port/cargo/src/util/testdata/graph_cases.json.gz"
	fixtureData, err := os.ReadFile(absolute(fixturePath))
	if err != nil {
		return err
	}
	reader, err := gzip.NewReader(bytes.NewReader(fixtureData))
	if err != nil {
		return err
	}
	defer reader.Close()
	var fixture struct {
		Version       int    `json:"format"`
		Cases         int    `json:"cases"`
		InputSHA256   string `json:"input_sha256"`
		DebugOutput   string `json:"debug_stdout"`
		DebugError    string `json:"debug_stderr"`
		ReleaseOutput string `json:"release_stdout"`
		ReleaseError  string `json:"release_stderr"`
	}
	if err := json.NewDecoder(reader).Decode(&fixture); err != nil {
		return err
	}
	files := make(map[string]string)
	for _, path := range []string{"port/cargo/src/util/graph.go", "port/cargo/src/util/graph_test.go", fixturePath, "tools/port-cargo-graph/main.go", "tools/port-cargo-graph/oracle.rs", "tools/port-cargo-graph/Cargo.toml", "tools/port-cargo-graph/Cargo.lock"} {
		data, err := os.ReadFile(absolute(path))
		if err != nil {
			return err
		}
		files[path] = hash(data)
	}
	outputs := make(map[string]outputEvidence)
	for name, value := range map[string]string{"debug_stdout": fixture.DebugOutput, "debug_stderr": fixture.DebugError, "release_stdout": fixture.ReleaseOutput, "release_stderr": fixture.ReleaseError} {
		item := outputEvidence{SHA256: hash([]byte(value)), Bytes: len(value)}
		if strings.HasSuffix(name, "stdout") {
			item.Panics = make(map[string]int)
			for _, kind := range []string{"missing_node", "path_edge", "path_end", "other"} {
				item.Panics[kind] = strings.Count(value, "panic "+kind+"\n")
			}
		}
		outputs[name] = item
	}
	metadata := report{Format: 1, CargoRevision: cargoRevision, SourcePath: "src/util/graph.rs", SourceGitBlob: graphSourceBlob, SourceSHA256: graphSourceHash, SourceLines: bytes.Count(original, []byte{'\n'}), SourceBytes: len(original), Rustc: pinnedRustc, Go: strings.TrimSpace(string(goVersion)), Host: runtime.GOOS + "/" + runtime.GOARCH, Dependencies: dependencies, UpstreamTests: 3, Cases: fixture.Cases, InputSHA256: fixture.InputSHA256, Profiles: []string{"debug (debug_assertions=true)", "release (debug_assertions=false)"}, Outputs: outputs, Files: files, Commands: []string{"GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-graph", "GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go run ./tools/port-cargo-graph -check", "GOCACHE=/tmp/ferrugo-go-cache .toolchain/go/bin/go test ./port/cargo/src/util -run '^TestGraph' -count=1"}, Storage: "im_rc::OrdMap is replaced by Go maps traversed through comparator-sorted keys; Clone eagerly creates independent adjacency/edge storage with supplied Clone adapters. Debug assertions are enabled by default; DisableDebugAssertions selects release behavior.", Skips: []string{}}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	reportPath := absolute("tools/port-cargo-graph/validation.json")
	if check {
		retained, err := os.ReadFile(reportPath)
		if err != nil {
			return err
		}
		// The retained Go/host identify evidence generation, not a restriction
		// on machines rerunning this portable source comparison. All source,
		// harness, dependency, input and result evidence remains checked.
		var generatedReport report
		if err := json.Unmarshal(retained, &generatedReport); err != nil {
			return err
		}
		metadata.Go, metadata.Host = generatedReport.Go, generatedReport.Host
		data, err = json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if !bytes.Equal(retained, data) {
			return fmt.Errorf("graph validation report differs; regenerate after reviewing the change")
		}
	} else if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return err
	}
	action := "generated"
	if check {
		action = "verified"
	}
	fmt.Printf("%s graph validation: 3 upstream tests per profile; %d corpus cases per debug/release profile; exact stdout, panic categories and stderr/test-printer parity (%s; %s/%s)\n", action, fixture.Cases, strings.TrimSpace(string(goVersion)), runtime.GOOS, runtime.GOARCH)
	return nil
}
