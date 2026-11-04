// Command port-rust-parse-format builds the unchanged pinned parser and lexer,
// runs their original tests, and produces a structured public-state oracle.
package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const toolPath = "tools/port-rust-parse-format"
const parserPackage = "./port/rust/compiler/rustc_parse_format/src"

type sourcePin struct {
	Path string `json:"path"`
	Blob string `json:"git_blob"`
	Hash string `json:"sha256"`
}

var sources = []sourcePin{
	{"compiler/rustc_parse_format/src/lib.rs", "90e04fe388ad6f592d5785e8c36964ce284a0da6", "451e6c20b1fe8c186fb19c996cc72581220327148154140a307d6d1fafb5b59e"},
	{"compiler/rustc_parse_format/src/tests.rs", "a2e03a514fa5fcc8304933ef59e02aa4400c4181", "8ad525beb126768ecdb887c4b4a2774db4e419aa4c4f7677b8a545c7405fc531"},
	{"compiler/rustc_lexer/src/lib.rs", "0aa57e75889fa2d81deb1ec8bf5480b1c930fff1", "fb7f451aae9d46a171e5599635483b27613a4067d107edd9caf89d79a32ceadf"},
	{"compiler/rustc_lexer/src/tests.rs", "a7357ba38c8e4484da814ef3c22bf33b75bdbeb2", "90516c7444d6102ab1fceae40cf260cf473ec6542bc372d8bf9d6033eaf507da"},
	{"Cargo.lock", "feeba051664e2fb683fdc8914790a81261b3b5d7", "080fd4f801bd66eac0f2e1ec5de21ccd845ec75063922d35efe7a0203e46fcca"},
}

var supportFiles = []string{
	"Cargo.toml", "Cargo.lock", "oracle.rs",
	"upstream/rustc_parse_format/Cargo.toml", "upstream/rustc_lexer/Cargo.toml",
	"upstream/rustc_index/Cargo.toml", "upstream/rustc_index/src/lib.rs",
}

type dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type manifest struct {
	Format                int               `json:"format"`
	Revision              string            `json:"revision"`
	Sources               []sourcePin       `json:"sources"`
	Dependencies          []dependency      `json:"registry_dependencies"`
	Rustc                 string            `json:"rustc"`
	Go                    string            `json:"generation_go"`
	Host                  string            `json:"generation_host"`
	PointerBits           int               `json:"pointer_bits"`
	OriginalTests         map[string]int    `json:"original_tests"`
	Cases                 int               `json:"cases"`
	Profiles              map[string]int    `json:"profiles"`
	Modes                 map[string]int    `json:"modes"`
	Steps                 int               `json:"steps"`
	ConstructorPanics     map[string]int    `json:"constructor_panics"`
	StepPanics            map[string]int    `json:"step_panics"`
	RequestsHash          string            `json:"requests_sha256"`
	OriginalResponsesHash string            `json:"original_responses_sha256"`
	CorpusHash            string            `json:"corpus_raw_sha256"`
	FixtureHash           string            `json:"fixture_sha256"`
	SupportHashes         map[string]string `json:"support_sha256"`
	RequestBuilderHash    string            `json:"request_builder_sha256"`
}

func digest(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func absolute(root, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Join(root, path)
	}
	return path
}

func selected(root, variable, flagValue, local, fallback string, explicit bool) (string, error) {
	name := flagValue
	if !explicit {
		var found bool
		name, found = os.LookupEnv(variable)
		if !found {
			name = local
			if info, err := os.Stat(name); err != nil || info.IsDir() {
				name = fallback
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("%s executable was explicitly empty", variable)
	}
	if strings.ContainsAny(name, `/\`) {
		name = absolute(root, name)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s executable %q: %w", variable, name, err)
	}
	return filepath.Abs(path)
}

func environment(overrides map[string]string) []string {
	drop := map[string]bool{"RUSTFLAGS": true, "CARGO_ENCODED_RUSTFLAGS": true, "RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true, "RUSTC_BOOTSTRAP": true, "UPDATE_EXPECT": true, "CARGO_BUILD_TARGET": true}
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, changed := overrides[key]; !changed && !drop[key] {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func command(ctx context.Context, dir string, env []string, exe string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", exe, args, err, stderr.Bytes())
	}
	return output, nil
}

func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	return command(ctx, root, environment(map[string]string{"GIT_NO_LAZY_FETCH": "1"}), "git", nil, append([]string{"-C", root}, args...)...)
}

func pinnedSource(ctx context.Context, root string, pin sourcePin) ([]byte, error) {
	object, err := git(ctx, root, "rev-parse", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(object)) != pin.Blob {
		return nil, fmt.Errorf("source Git blob differs: %s", pin.Path)
	}
	data, err := git(ctx, root, "show", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if digest(data) != pin.Hash {
		return nil, fmt.Errorf("source SHA-256 differs: %s", pin.Path)
	}
	return data, nil
}

// Both files are Cargo-generated lockfiles, not general-purpose TOML input.
func lockDependencies(data []byte) []dependency {
	var result []dependency
	for _, block := range strings.Split(string(data), "[[package]]") {
		var dep dependency
		registry := false
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			text, err := strconv.Unquote(value)
			if err != nil {
				continue
			}
			switch key {
			case "name":
				dep.Name = text
			case "version":
				dep.Version = text
			case "checksum":
				dep.Checksum = text
			case "source":
				registry = text == "registry+https://github.com/rust-lang/crates.io-index"
			}
		}
		if registry {
			result = append(result, dep)
		}
	}
	sort.Slice(result, func(a, b int) bool {
		return result[a].Name < result[b].Name || result[a].Name == result[b].Name && result[a].Version < result[b].Version
	})
	return result
}

func verifyDependencies(selected, upstream []byte) ([]dependency, error) {
	deps := lockDependencies(selected)
	known := map[string]string{}
	for _, dep := range lockDependencies(upstream) {
		known[dep.Name+"@"+dep.Version] = dep.Checksum
	}
	if len(deps) != 16 {
		return nil, fmt.Errorf("expected 16 pinned registry packages, got %d", len(deps))
	}
	for _, dep := range deps {
		if dep.Checksum == "" || known[dep.Name+"@"+dep.Version] != dep.Checksum {
			return nil, fmt.Errorf("registry package differs from pinned upstream lock: %+v", dep)
		}
	}
	return deps, nil
}

// Verify the archive checksums and every original member in the actual expanded
// package selected by Cargo metadata, rather than trusting an editable cache.
func verifyArchives(metadata []byte, cargoHome string, deps []dependency) error {
	var decoded struct {
		Packages []struct {
			Name     string  `json:"name"`
			Version  string  `json:"version"`
			Source   *string `json:"source"`
			Manifest string  `json:"manifest_path"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		return err
	}
	wanted := map[string]string{}
	for _, dep := range deps {
		wanted[dep.Name+"@"+dep.Version] = dep.Checksum
	}
	for _, pkg := range decoded.Packages {
		if pkg.Source == nil {
			continue
		}
		if *pkg.Source != "registry+https://github.com/rust-lang/crates.io-index" {
			return fmt.Errorf("unexpected registry source: %s", *pkg.Source)
		}
		key := pkg.Name + "@" + pkg.Version
		expected, ok := wanted[key]
		if !ok {
			return fmt.Errorf("unlocked registry package %s", key)
		}
		packageDir := filepath.Dir(pkg.Manifest)
		registry := filepath.Base(filepath.Dir(packageDir))
		archive := filepath.Join(cargoHome, "registry/cache", registry, pkg.Name+"-"+pkg.Version+".crate")
		data, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		if digest(data) != expected {
			return fmt.Errorf("registry archive checksum mismatch: %s", archive)
		}
		compressed, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		tarReader := tar.NewReader(compressed)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				compressed.Close()
				return err
			}
			if header.Typeflag != tar.TypeReg {
				continue
			}
			prefix := pkg.Name + "-" + pkg.Version + "/"
			if !strings.HasPrefix(header.Name, prefix) {
				compressed.Close()
				return fmt.Errorf("unexpected archive path %s", header.Name)
			}
			name := strings.TrimPrefix(header.Name, prefix)
			if filepath.IsAbs(name) || strings.Contains(name, "..") {
				compressed.Close()
				return fmt.Errorf("invalid archive member %s", name)
			}
			original, err := io.ReadAll(tarReader)
			if err != nil {
				compressed.Close()
				return err
			}
			actual, err := os.ReadFile(filepath.Join(packageDir, filepath.FromSlash(name)))
			if err != nil {
				compressed.Close()
				return err
			}
			if !bytes.Equal(actual, original) {
				compressed.Close()
				return fmt.Errorf("expanded registry source changed: %s/%s", key, name)
			}
		}
		if err := compressed.Close(); err != nil {
			return err
		}
	}
	return nil
}

func retainGzip(raw, existing []byte, check bool) ([]byte, error) {
	if len(existing) != 0 {
		reader, err := gzip.NewReader(bytes.NewReader(existing))
		if err == nil {
			decoded, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(decoded, raw) {
				return existing, nil
			}
		}
	}
	if check {
		return nil, fmt.Errorf("committed gzip fixture is absent, invalid, or differs from original output")
	}
	var result bytes.Buffer
	writer, err := gzip.NewWriterLevel(&result, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err = writer.Write(raw); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func lines(data []byte) ([]json.RawMessage, error) {
	reader := bufio.NewReader(bytes.NewReader(data))
	var result []json.RawMessage
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) != 0 {
			if !json.Valid(line) {
				return nil, fmt.Errorf("invalid JSON record")
			}
			result = append(result, append(json.RawMessage(nil), line...))
		}
		if err == io.EOF {
			break
		}
	}
	return result, nil
}

func run(root, cargo, rustc, goTool, cargoHome, sourceRoot string, check bool) error {
	if strconv.IntSize != 64 {
		return fmt.Errorf("original regeneration requires a 64-bit host")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	version, err := command(ctx, root, nil, rustc, nil, "--version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("require %s, found %s", rustVersion, version)
	}
	goVersion, err := command(ctx, root, nil, goTool, nil, "version")
	if err != nil || !bytes.HasPrefix(goVersion, []byte("go version ")) {
		return fmt.Errorf("selected Go executable is unavailable/incompatible: %v: %s", err, goVersion)
	}
	cargoVersion, err := command(ctx, root, nil, cargo, nil, "--version")
	if err != nil || !bytes.HasPrefix(cargoVersion, []byte("cargo ")) {
		return fmt.Errorf("selected Cargo executable is unavailable/incompatible: %v: %s", err, cargoVersion)
	}
	head, err := git(ctx, sourceRoot, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != revision {
		return fmt.Errorf("upstream Rust checkout must be at %s", revision)
	}
	workspace, err := os.MkdirTemp("", "ferrugo-parse-format-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	supportHashes := map[string]string{}
	for _, name := range supportFiles {
		data, err := os.ReadFile(filepath.Join(root, toolPath, name))
		if err != nil {
			return err
		}
		supportHashes[name] = digest(data)
		path := filepath.Join(workspace, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			return err
		}
	}
	var upstreamLock []byte
	for _, pin := range sources {
		data, err := pinnedSource(ctx, sourceRoot, pin)
		if err != nil {
			return err
		}
		if pin.Path == "Cargo.lock" {
			upstreamLock = data
			continue
		}
		name := strings.TrimPrefix(pin.Path, "compiler/")
		path := filepath.Join(workspace, "upstream", filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err = os.WriteFile(path, data, 0600); err != nil {
			return err
		}
	}
	lock, err := os.ReadFile(filepath.Join(workspace, "Cargo.lock"))
	if err != nil {
		return err
	}
	deps, err := verifyDependencies(lock, upstreamLock)
	if err != nil {
		return err
	}
	env := environment(map[string]string{"RUSTC": rustc, "CARGO_HOME": cargoHome, "CARGO_TARGET_DIR": filepath.Join(workspace, "target"), "CARGO_PROFILE_DEV_DEBUG_ASSERTIONS": "true", "CARGO_PROFILE_DEV_OVERFLOW_CHECKS": "true", "CARGO_PROFILE_TEST_OVERFLOW_CHECKS": "true"})
	metadata, err := command(ctx, workspace, env, cargo, nil, "metadata", "--locked", "--offline", "--format-version=1", "--manifest-path", filepath.Join(workspace, "Cargo.toml"))
	if err != nil {
		return err
	}
	if err = verifyArchives(metadata, cargoHome, deps); err != nil {
		return err
	}
	testOutput, err := command(ctx, workspace, env, cargo, nil, "test", "--locked", "--offline", "--workspace", "--lib", "--manifest-path", filepath.Join(workspace, "Cargo.toml"))
	if err != nil {
		return err
	}
	if !bytes.Contains(testOutput, []byte("28 passed")) || !bytes.Contains(testOutput, []byte("17 passed")) {
		return fmt.Errorf("original tests did not report all 28 parser and 17 lexer tests: %s", testOutput)
	}
	requestsPath := filepath.Join(workspace, "requests.jsonl")
	dumpEnv := environment(map[string]string{"FERRUGO_PARSE_FORMAT_DUMP_REQUESTS": requestsPath})
	if output, err := command(ctx, root, dumpEnv, goTool, nil, "test", parserPackage, "-run", "^TestDumpOracleRequests$", "-count=1"); err != nil {
		return fmt.Errorf("dump deterministic Go requests: %w\n%s", err, output)
	}
	requests, err := os.ReadFile(requestsPath)
	if err != nil {
		return err
	}
	records, err := lines(requests)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("no oracle requests")
	}
	responses := make([]json.RawMessage, len(records))
	profiles := map[string]int{"checked": 0, "wrapping": 0}
	modes := map[string]int{}
	for _, checked := range []bool{true, false} {
		label := map[bool]string{true: "checked", false: "wrapping"}[checked]
		var input bytes.Buffer
		var indices []int
		for i, record := range records {
			var info struct {
				Checked *bool  `json:"checked"`
				Mode    string `json:"mode"`
			}
			if err = json.Unmarshal(record, &info); err != nil {
				return err
			}
			if info.Checked == nil {
				return fmt.Errorf("request %d lacks explicit overflow profile", i)
			}
			if *info.Checked == checked {
				input.Write(record)
				input.WriteByte('\n')
				indices = append(indices, i)
				modes[info.Mode]++
			}
		}
		profiles[label] = len(indices)
		if len(indices) == 0 {
			return fmt.Errorf("missing %s requests", label)
		}
		profileEnv := environment(map[string]string{"RUSTC": rustc, "CARGO_HOME": cargoHome, "CARGO_TARGET_DIR": filepath.Join(workspace, "target"), "CARGO_PROFILE_DEV_DEBUG_ASSERTIONS": "true", "CARGO_PROFILE_DEV_OVERFLOW_CHECKS": strconv.FormatBool(checked)})
		if _, err = command(ctx, workspace, profileEnv, cargo, nil, "build", "--locked", "--offline", "--bin", "ferrugo-parse-format-oracle", "--manifest-path", filepath.Join(workspace, "Cargo.toml")); err != nil {
			return err
		}
		output, err := command(ctx, workspace, profileEnv, filepath.Join(workspace, "target/debug/ferrugo-parse-format-oracle"), input.Bytes())
		if err != nil {
			return err
		}
		answers, err := lines(output)
		if err != nil {
			return err
		}
		if len(answers) != len(indices) {
			return fmt.Errorf("%s oracle returned %d records, require %d", label, len(answers), len(indices))
		}
		for i, index := range indices {
			responses[index] = answers[i]
		}
	}
	var corpus, originalResponses bytes.Buffer
	constructorPanics, stepPanics := map[string]int{}, map[string]int{}
	steps := 0
	for i, request := range records {
		var record = struct {
			Request  json.RawMessage `json:"request"`
			Response json.RawMessage `json:"response"`
		}{request, responses[i]}
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		corpus.Write(data)
		corpus.WriteByte('\n')
		originalResponses.Write(responses[i])
		originalResponses.WriteByte('\n')
		var count struct {
			ConstructorPanic *string `json:"constructor_panic"`
			Steps            []struct {
				Panic *string `json:"panic"`
			} `json:"steps"`
		}
		if err = json.Unmarshal(responses[i], &count); err != nil {
			return err
		}
		if count.ConstructorPanic != nil {
			constructorPanics[*count.ConstructorPanic]++
		}
		steps += len(count.Steps)
		for _, step := range count.Steps {
			if step.Panic != nil {
				stepPanics[*step.Panic]++
			}
		}
	}
	fixturePath := filepath.Join(root, toolPath, "cases.jsonl.gz")
	existing, err := os.ReadFile(fixturePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	compressed, err := retainGzip(corpus.Bytes(), existing, check)
	if err != nil {
		return err
	}
	builder, err := os.ReadFile(filepath.Join(root, "port/rust/compiler/rustc_parse_format/src/oracle_test.go"))
	if err != nil {
		return err
	}
	meta := manifest{Format: 1, Revision: revision, Sources: sources, Dependencies: deps, Rustc: rustVersion, Go: strings.TrimSpace(string(goVersion)), Host: runtime.GOOS + "/" + runtime.GOARCH, PointerBits: 64, OriginalTests: map[string]int{"rustc_parse_format": 28, "rustc_lexer": 17}, Cases: len(records), Profiles: profiles, Modes: modes, Steps: steps, ConstructorPanics: constructorPanics, StepPanics: stepPanics, RequestsHash: digest(requests), OriginalResponsesHash: digest(originalResponses.Bytes()), CorpusHash: digest(corpus.Bytes()), FixtureHash: digest(compressed), SupportHashes: supportHashes, RequestBuilderHash: digest(builder)}
	manifestPath := filepath.Join(root, toolPath, "manifest.json")
	if check {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		var prior manifest
		if err = json.Unmarshal(data, &prior); err != nil {
			return err
		}
		meta.Go, meta.Host = prior.Go, prior.Host
	}
	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if check {
		prior, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		if !bytes.Equal(prior, encoded) {
			return fmt.Errorf("manifest differs from current pinned source, requests, wrapper, or original results")
		}
	} else {
		if err = os.WriteFile(fixturePath, compressed, 0644); err != nil {
			return err
		}
		if err = os.WriteFile(manifestPath, encoded, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("parse_format: original tests 28+17; %d requests, %d steps; profiles=%v modes=%v; constructor panics=%v step panics=%v; Go=%s host=%s/%s\n", len(records), steps, profiles, modes, constructorPanics, stepPanics, strings.TrimSpace(string(goVersion)), runtime.GOOS, runtime.GOARCH)
	return nil
}

func main() {
	rootFlag := flag.String("root", ".", "repository root")
	cargoFlag := flag.String("cargo", "", "Cargo executable")
	rustcFlag := flag.String("rustc", "", "pinned Rust executable")
	goFlag := flag.String("go", "", "Go executable used for deterministic request builder")
	sourceFlag := flag.String("rust-source", "", "pinned Rust checkout")
	homeFlag := flag.String("cargo-home", "", "registry cache")
	check := flag.Bool("check", false, "verify without rewriting retained evidence")
	flag.Parse()
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	fail := func(err error) { fmt.Fprintln(os.Stderr, "port-rust-parse-format:", err); os.Exit(1) }
	if flag.NArg() != 0 {
		fail(fmt.Errorf("unexpected positional arguments: %v", flag.Args()))
	}
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fail(err)
	}
	cargo, err := selected(root, "FERRUGO_CARGO", *cargoFlag, filepath.Join(root, ".toolchain/rust/bin/cargo"), "cargo", explicit["cargo"])
	if err != nil {
		fail(err)
	}
	rustc, err := selected(root, "FERRUGO_RUSTC", *rustcFlag, filepath.Join(root, ".toolchain/rust/bin/rustc"), "rustc", explicit["rustc"])
	if err != nil {
		fail(err)
	}
	goTool, err := selected(root, "FERRUGO_GO", *goFlag, filepath.Join(runtime.GOROOT(), "bin/go"), "go", explicit["go"])
	if err != nil {
		fail(err)
	}
	path := func(name, variable, value, fallback string) string {
		if !explicit[name] {
			var exists bool
			value, exists = os.LookupEnv(variable)
			if !exists {
				value = fallback
			}
		}
		if value == "" {
			fail(fmt.Errorf("%s path was explicitly empty", variable))
		}
		return absolute(root, value)
	}
	sourceRoot := path("rust-source", "FERRUGO_RUST_SOURCE", *sourceFlag, ".toolchain/upstream-rust")
	cargoHome := path("cargo-home", "CARGO_HOME", *homeFlag, ".toolchain/port-cargo-home")
	if err = run(root, cargo, rustc, goTool, cargoHome, sourceRoot, *check); err != nil {
		fail(err)
	}
}
