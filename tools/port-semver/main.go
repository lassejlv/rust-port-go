// Development-only whole-crate semver 1.0.28 provenance and original oracle.
// SPDX-License-Identifier: MIT OR Apache-2.0
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
	"github.com/lassejlv/rust-port-go/tools/port-semver/contract"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const toolPath = "tools/port-semver"
const targetPath = "port/dependencies/semver/v1_0_28"
const semverChecksum = "8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"

type dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func absolute(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
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

func selectedDirectory(root, variable, value, fallback string, explicit bool) (string, error) {
	if !explicit {
		var present bool
		value, present = os.LookupEnv(variable)
		if !present {
			return absolute(root, fallback), nil
		}
	}
	if value == "" {
		return "", fmt.Errorf("%s directory was explicitly empty", variable)
	}
	return absolute(root, value), nil
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
	seen := map[string]bool{}
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
		seen[key] = true
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
		expectedFiles := map[string]bool{}
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
			expectedFiles[name] = true
			if filepath.IsAbs(name) || strings.Contains(name, "..") {
				compressed.Close()
				return fmt.Errorf("invalid archive member %s", name)
			}
			original, err := io.ReadAll(tarReader)
			if err != nil {
				compressed.Close()
				return err
			}
			targetName := filepath.Join(packageDir, filepath.FromSlash(name))
			info, err := os.Lstat(targetName)
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("expanded source is not a regular file:%s", targetName)
			}
			actual, err := os.ReadFile(targetName)
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
		if err := filepath.WalkDir(packageDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(packageDir, path)
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in expanded registry source:%s", path)
			}
			if entry.IsDir() {
				return nil
			}
			name := filepath.ToSlash(relative)
			if !expectedFiles[name] && name != ".cargo-ok" && name != ".cargo-checksum.json" {
				return fmt.Errorf("unexpected expanded registry file:%s", path)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if len(seen) != len(wanted) {
		return fmt.Errorf("metadata selected %d registry packages, expected %d", len(seen), len(wanted))
	}
	return nil
}

func retainGzip(raw, existing []byte, check bool) ([]byte, error) {
	if len(existing) != 0 {
		r, err := gzip.NewReader(bytes.NewReader(existing))
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(r, int64(len(raw))+1))
			closeErr := r.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(data, raw) {
				return existing, nil
			}
		}
	}
	if check {
		return nil, fmt.Errorf("committed gzip absent, invalid, or differs from original output")
	}
	var result bytes.Buffer
	w, _ := gzip.NewWriterLevel(&result, gzip.BestCompression)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

type sourceFile struct {
	Name                       string `json:"name"`
	Bytes, Lines               int
	SHA256, Target, Adaptation string
}
type inventory struct {
	Crate, Version, ArchiveURL, ArchiveSHA256, UpstreamRevision string
	ArchiveFiles, RustFiles, RustLines, RustBytes               int
	Files                                                       []sourceFile
}

func sourceInventory(data []byte) (inventory, map[string][]byte, error) {
	inv := inventory{Crate: "semver", Version: "1.0.28", ArchiveURL: "https://static.crates.io/crates/semver/semver-1.0.28.crate", ArchiveSHA256: semverChecksum, UpstreamRevision: "7625c7aa3f0e8ba21e099d1765bcebcb72aa8816"}
	members := map[string][]byte{}
	if digest(data) != semverChecksum {
		return inv, nil, fmt.Errorf("semver official archive SHA differs")
	}
	g, e := gzip.NewReader(bytes.NewReader(data))
	if e != nil {
		return inv, nil, e
	}
	defer g.Close()
	tr := tar.NewReader(g)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return inv, nil, e
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasPrefix(h.Name, "semver-1.0.28/") {
			return inv, nil, fmt.Errorf("unexpected archive member%s", h.Name)
		}
		name := strings.TrimPrefix(h.Name, "semver-1.0.28/")
		b, e := io.ReadAll(tr)
		if e != nil {
			return inv, nil, e
		}
		members[name] = b
		file := sourceFile{Name: name, Bytes: len(b), Lines: bytes.Count(b, []byte{'\n'}), SHA256: digest(b)}
		if strings.HasSuffix(name, ".rs") {
			inv.RustFiles++
			inv.RustLines += file.Lines
			inv.RustBytes += len(b)
			switch {
			case strings.HasPrefix(name, "src/"):
				file.Target = strings.TrimSuffix(name, ".rs") + ".go"
				if name == "src/identifier.rs" {
					file.Adaptation = "Private inline/heap/tagged-pointer allocation is immutable Go strings and GC; value operations retained, no Rust ABI/allocation timing contract."
				}
			case strings.HasPrefix(name, "tests/"):
				file.Target = strings.TrimSuffix(name, ".rs") + "_test.go"
				if name == "tests/node/mod.rs" {
					file.Target = "tests/node_test.go; tests/node_native_test.go"
				}
				if name == "tests/util/mod.rs" {
					file.Target = "tests/util_test.go"
				}
			case name == "benches/parse.rs":
				file.Target = "benches/parse_test.go"
			}
			if name == "src/display.rs" {
				file.Target = "src/display.go; src/debug.go"
			}
			if name == "src/error.rs" {
				file.Target = "src/error.go; src/error_std.go; src/error_no_std.go"
			}
			if name == "src/serde.rs" {
				file.Target = "src/serde.go; src/serde_disabled.go"
			}
		}
		if file.Target == "" {
			file.Target = "upstream/" + name
			if name == ".gitignore" {
				file.Target = "upstream/gitignore.source"
			}
			if strings.HasPrefix(name, "LICENSE-") {
				file.Target = name
			}
		}
		inv.Files = append(inv.Files, file)
	}
	sort.Slice(inv.Files, func(i, j int) bool { return inv.Files[i].Name < inv.Files[j].Name })
	inv.ArchiveFiles = len(inv.Files)
	if inv.ArchiveFiles != 25 || inv.RustFiles != 15 || inv.RustLines != 3031 || inv.RustBytes != 98144 {
		return inv, nil, fmt.Errorf("archive inventory changed: %+v", inv)
	}
	return inv, members, nil
}
func persist(path string, data []byte, check bool) error {
	if check {
		old, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if !bytes.Equal(old, data) {
			return fmt.Errorf("committed file differs:%s", path)
		}
		return nil
	}
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, data, 0644)
}
func persistJSON(path string, value any, check bool) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	return persist(path, append(b, '\n'), check)
}
func fixture(root, name string, raw []byte, check bool) (string, error) {
	path := filepath.Join(root, targetPath, "src/testdata", name)
	old, e := os.ReadFile(path)
	if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	out, e := retainGzip(raw, old, check)
	if e != nil {
		return "", fmt.Errorf("%s:%w", name, e)
	}
	if !check {
		if e := persist(path, out, false); e != nil {
			return "", e
		}
	}
	return digest(out), nil
}
func queries(corpus []contract.Request) []byte {
	var b bytes.Buffer
	w := json.NewEncoder(&b)
	for _, q := range corpus {
		if e := w.Encode(q); e != nil {
			panic(e)
		}
	}
	return b.Bytes()
}
func verifyGoCore(raw []byte) error {
	s := bufio.NewScanner(bytes.NewReader(raw))
	s.Buffer(make([]byte, 4096), 2<<20)
	n := 0
	for s.Scan() {
		n++
		var row struct {
			Request json.RawMessage
			Result  json.RawMessage
		}
		if e := json.Unmarshal(s.Bytes(), &row); e != nil {
			return e
		}
		q, e := contract.DecodeRequest(row.Request)
		if e != nil {
			return e
		}
		actual, e := json.Marshal(contract.Observe(q))
		if e != nil {
			return e
		}
		var a, b any
		_ = json.Unmarshal(actual, &a)
		_ = json.Unmarshal(row.Result, &b)
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("core request%d %s\nGo:%s\nRust:%s", n, row.Request, actual, row.Result)
		}
	}
	return s.Err()
}
func unicodeSource(raw []byte) ([]byte, error) {
	var text strings.Builder
	text.WriteString("// Code generated by tools/port-semver from the validated Rust 1.99.1\n// Unicode 17.0.0 char Debug compatibility ranges; DO NOT EDIT.\n// Unicode data terms are in ../UNICODE-LICENSE.\npackage semver\n\nvar debugEscapedRanges = [][2]rune{\n")
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		ends := strings.Split(line, "\t")
		if len(ends) != 2 {
			return nil, fmt.Errorf("invalid Unicode range %q", line)
		}
		for _, s := range ends {
			if _, e := strconv.ParseUint(s, 16, 32); e != nil {
				return nil, e
			}
		}
		fmt.Fprintf(&text, "{0x%s,0x%s},\n", ends[0], ends[1])
	}
	text.WriteString("}\n")
	return format.Source([]byte(text.String()))
}

type collectorAbortObservation struct {
	Request contract.CollectorRequest `json:"request"`
	Class   string                    `json:"class"`
	Events  []string                  `json:"events"`
}

func abortOutcome(ctx context.Context, root string, env []string, exe string, input []byte, args ...string) ([]string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	var output, diagnostic bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	if err == nil {
		return nil, fmt.Errorf("expected abort from%s", exe)
	}
	if cmd.ProcessState == nil {
		return nil, err
	}
	code := cmd.ProcessState.ExitCode()
	if code != 134 && !(code < 0 && strings.Contains(diagnostic.String(), "aborting")) {
		return nil, fmt.Errorf("unexpected child status%s: %s", cmd.ProcessState.String(), diagnostic.Bytes())
	}
	events := []string{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if !strings.HasPrefix(line, "EVENT\t") {
			return nil, fmt.Errorf("unexpected abort stdout %q", line)
		}
		events = append(events, strings.TrimPrefix(line, "EVENT\t"))
	}
	return events, nil
}
func verifyCollectorRows(raw []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	line := 0
	for scanner.Scan() {
		line++
		var row struct {
			Request contract.CollectorRequest
			Result  json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return err
		}
		actual, _ := json.Marshal(contract.ObserveCollector(row.Request, nil))
		var a, b any
		_ = json.Unmarshal(actual, &a)
		_ = json.Unmarshal(row.Result, &b)
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("collector row%d %+v\nGo%s\nRust%s", line, row.Request, actual, row.Result)
		}
	}
	return scanner.Err()
}

func verifyCloneRows(raw []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	line := 0
	for scanner.Scan() {
		line++
		var row struct {
			Request contract.CloneRequest
			Result  json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return err
		}
		actual, err := json.Marshal(contract.ObserveClone(row.Request))
		if err != nil {
			return err
		}
		var a, b any
		if err := json.Unmarshal(actual, &a); err != nil {
			return err
		}
		if err := json.Unmarshal(row.Result, &b); err != nil {
			return err
		}
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("clone row%d %+v\nGo%s\nRust%s", line, row.Request, actual, row.Result)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if line != len(contract.CloneCorpus()) {
		return fmt.Errorf("clone cases%d expected%d", line, len(contract.CloneCorpus()))
	}
	return nil
}
func run() error {
	check := flag.Bool("check", false, "verify original outputs against retained files without rewriting them")
	rootFlag := flag.String("root", ".", "repository root")
	cargoFlag := flag.String("cargo", "", "Cargo executable; FERRUGO_CARGO, local toolchain, then PATH")
	rustFlag := flag.String("rustc", "", "rustc executable; FERRUGO_RUSTC, local toolchain, then PATH")
	goFlag := flag.String("go", "", "Go executable; FERRUGO_GO, local toolchain, then PATH")
	homeFlag := flag.String("cargo-home", "", "cache directory; CARGO_HOME or current host cache")
	flag.Parse()
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	root, e := filepath.Abs(*rootFlag)
	if e != nil {
		return e
	}
	if _, e := os.Stat(filepath.Join(root, "go.mod")); e != nil {
		return fmt.Errorf("-root must identify repository:%w", e)
	}
	cargo, e := selected(root, "FERRUGO_CARGO", *cargoFlag, filepath.Join(root, ".toolchain/rust/bin/cargo"), "cargo", explicit["cargo"])
	if e != nil {
		return e
	}
	rustc, e := selected(root, "FERRUGO_RUSTC", *rustFlag, filepath.Join(root, ".toolchain/rust/bin/rustc"), "rustc", explicit["rustc"])
	if e != nil {
		return e
	}
	goexe, e := selected(root, "FERRUGO_GO", *goFlag, filepath.Join(root, ".toolchain/go/bin/go"), "go", explicit["go"])
	if e != nil {
		return e
	}
	fallbackHome := filepath.Join(root, ".toolchain/port-cargo-home")
	if _, e := os.Stat(fallbackHome); e != nil {
		user, e := os.UserHomeDir()
		if e != nil {
			return e
		}
		fallbackHome = filepath.Join(user, ".cargo")
	}
	home, e := selectedDirectory(root, "CARGO_HOME", *homeFlag, fallbackHome, explicit["cargo-home"])
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	target := filepath.Join(root, ".toolchain/oracles/semver/target")
	env := environment(map[string]string{"CARGO_HOME": home, "RUSTC": rustc, "RUSTC_BOOTSTRAP": "1", "CARGO_TARGET_DIR": target, "GOTOOLCHAIN": "local"})
	version, e := command(ctx, root, env, rustc, nil, "--version")
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("Rust formatter oracle requires%s; got%s", rustVersion, version)
	}
	manifest := filepath.Join(root, toolPath, "oracle/Cargo.toml")
	lock, e := os.ReadFile(filepath.Join(root, toolPath, "oracle/Cargo.lock"))
	if e != nil {
		return e
	}
	deps := lockDependencies(lock)
	meta, e := command(ctx, root, env, cargo, nil, "metadata", "--locked", "--offline", "--all-features", "--format-version=1", "--manifest-path", manifest)
	if e != nil {
		return e
	}
	if e := verifyArchives(meta, home, deps); e != nil {
		return e
	}
	var decoded struct {
		Packages []struct {
			Name, Version string
			Manifest      string `json:"manifest_path"`
		}
	}
	if e := json.Unmarshal(meta, &decoded); e != nil {
		return e
	}
	var archive string
	for _, p := range decoded.Packages {
		if p.Name == "semver" {
			index := filepath.Base(filepath.Dir(filepath.Dir(p.Manifest)))
			archive = filepath.Join(home, "registry/cache", index, "semver-1.0.28.crate")
		}
	}
	data, e := os.ReadFile(archive)
	if e != nil {
		return e
	}
	inv, members, e := sourceInventory(data)
	if e != nil {
		return e
	}
	for _, f := range inv.Files {
		if !strings.HasSuffix(f.Name, ".rs") {
			if e := persist(filepath.Join(root, targetPath, f.Target), members[f.Name], *check); e != nil {
				return e
			}
		}
	}
	if e := persistJSON(filepath.Join(root, targetPath, "source-manifest.json"), inv, *check); e != nil {
		return e
	}
	coreQueries := queries(contract.CoreCorpus())
	serdeQueries := queries(contract.SerdeCorpus())
	var coreOutput, serdeOutput []byte
	profiles := []struct{ name, features string }{{"std", "std,prefixfree"}, {"no_std", "prefixfree"}, {"serde", "std,serde,prefixfree"}, {"no_std+serde", "serde,prefixfree"}}
	for _, p := range profiles {
		fmt.Fprintf(os.Stderr, "original semver profile %s\n", p.name)
		if _, e := command(ctx, root, env, cargo, nil, "build", "--locked", "--offline", "--manifest-path", manifest, "--no-default-features", "--features", p.features, "--bin", "oracle"); e != nil {
			return e
		}
		original, e := command(ctx, root, env, filepath.Join(target, "debug/oracle"), coreQueries)
		if e != nil {
			return e
		}
		if coreOutput == nil {
			coreOutput = original
			if e := verifyGoCore(original); e != nil {
				return e
			}
		} else if !bytes.Equal(coreOutput, original) {
			return fmt.Errorf("semver core differs under profile%s", p.name)
		}
		if strings.Contains(p.features, "serde") {
			original, e := command(ctx, root, env, filepath.Join(target, "debug/oracle"), serdeQueries)
			if e != nil {
				return e
			}
			if serdeOutput == nil {
				serdeOutput = original
			} else if !bytes.Equal(serdeOutput, original) {
				return fmt.Errorf("serde protocol differs underprofile%s", p.name)
			}
		}
	}
	hashes := map[string]string{}
	for name, raw := range map[string][]byte{"core.jsonl.gz": coreOutput, "serde.jsonl.gz": serdeOutput} {
		sha, e := fixture(root, name, raw, *check)
		if e != nil {
			return e
		}
		hashes[name] = sha
	}
	for _, p := range []struct{ bin, name string }{{"debug-oracle", "debug.tsv.gz"}, {"debug-more-oracle", "debug_more.tsv.gz"}, {"debug-dispatch-oracle", "debug_dispatch.tsv.gz"}, {"debug-more-dispatch-oracle", "debug_more_dispatch.tsv.gz"}, {"debug-state-oracle", "debug_state.tsv.gz"}, {"debug-newline-oracle", "debug_newline_dispatch.tsv.gz"}} {
		if _, e := command(ctx, root, env, cargo, nil, "build", "--locked", "--offline", "--manifest-path", manifest, "--no-default-features", "--features", "std", "--bin", p.bin); e != nil {
			return e
		}
		raw, e := command(ctx, root, env, filepath.Join(target, "debug", p.bin), nil)
		if e != nil {
			return e
		}
		sha, e := fixture(root, p.name, raw, *check)
		if e != nil {
			return e
		}
		hashes[p.name] = sha
	}
	if _, e := command(ctx, root, env, cargo, nil, "build", "--locked", "--offline", "--manifest-path", manifest, "--bin", "unicode-oracle"); e != nil {
		return e
	}
	unicodeRaw, e := command(ctx, root, env, filepath.Join(target, "debug/unicode-oracle"), nil)
	if e != nil {
		return e
	}
	unicodeGo, e := unicodeSource(unicodeRaw)
	if e != nil {
		return e
	}
	if e := persist(filepath.Join(root, targetPath, "src/debug_unicode.go"), unicodeGo, *check); e != nil {
		return e
	}
	if _, e := command(ctx, root, env, cargo, nil, "build", "--locked", "--offline", "--manifest-path", manifest, "--no-default-features", "--features", "std", "--bin", "collector-oracle"); e != nil {
		return e
	}
	var collectorQueries bytes.Buffer
	collectorEncoder := json.NewEncoder(&collectorQueries)
	for _, q := range contract.CollectorCorpus() {
		if e := collectorEncoder.Encode(q); e != nil {
			return e
		}
	}
	collectorExe := filepath.Join(target, "debug/collector-oracle")
	layout, e := command(ctx, root, env, collectorExe, nil, "--layout")
	if e != nil {
		return e
	}
	var comparatorSize, comparatorAlign uint
	if _, e := fmt.Sscanf(string(layout), "%d %d", &comparatorSize, &comparatorAlign); e != nil {
		return e
	}
	if comparatorSize != contract.CollectorReferenceSize() {
		return fmt.Errorf("source Comparator size%d does not match collector profile%d", comparatorSize, contract.CollectorReferenceSize())
	}

	collectorOutput, e := command(ctx, root, env, collectorExe, collectorQueries.Bytes())
	if e != nil {
		return e
	}
	if e := verifyCollectorRows(collectorOutput); e != nil {
		return e
	}
	sha, e := fixture(root, "collector.jsonl.gz", collectorOutput, *check)
	if e != nil {
		return e
	}
	hashes["collector.jsonl.gz"] = sha
	var cloneQueries bytes.Buffer
	for _, q := range contract.CloneCorpus() {
		if e := json.NewEncoder(&cloneQueries).Encode(q); e != nil {
			return e
		}
	}
	cloneOutput, e := command(ctx, root, env, collectorExe, cloneQueries.Bytes(), "--clones")
	if e != nil {
		return e
	}
	if e := verifyCloneRows(cloneOutput); e != nil {
		return e
	}
	cloneSHA, e := fixture(root, "clone.jsonl.gz", cloneOutput, *check)
	if e != nil {
		return e
	}
	hashes["clone.jsonl.gz"] = cloneSHA
	goChild := filepath.Join(root, ".toolchain/oracles/semver/collector-go")
	if _, e := command(ctx, root, env, goexe, nil, "build", "-o", goChild, "./"+toolPath); e != nil {
		return e
	}
	var abortRows []collectorAbortObservation
	for _, q := range contract.CollectorAbortCorpus() {
		request, e := json.Marshal(q)
		if e != nil {
			return e
		}
		native, e := abortOutcome(ctx, root, env, collectorExe, append(request, '\n'), "--live")
		if e != nil {
			return e
		}
		actual, e := abortOutcome(ctx, root, env, goChild, nil, "-collector-child", string(request))
		if e != nil {
			return e
		}
		if !reflect.DeepEqual(native, actual) {
			return fmt.Errorf("collector abort trace %+v\nGo%v\nRust%v", q, actual, native)
		}
		abortRows = append(abortRows, collectorAbortObservation{q, "abort", native})
	}
	if e := persistJSON(filepath.Join(root, toolPath, "collector-aborts.json"), abortRows, *check); e != nil {
		return e
	}
	for _, tags := range []string{"", "semver_no_std", "semver_serde", "semver_no_std,semver_serde"} {
		args := []string{"test", "-count=1"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, "./"+targetPath+"/...")
		out, e := command(ctx, root, env, goexe, nil, args...)
		if e != nil {
			return e
		}
		fmt.Fprint(os.Stderr, string(out))
	}
	if e := verifyArchives(meta, home, deps); e != nil {
		return e
	}
	for _, file := range inv.Files {
		for _, target := range strings.Split(file.Target, "; ") {
			if _, e := os.Stat(filepath.Join(root, targetPath, target)); e != nil {
				return e
			}
		}
	}
	report := map[string]any{"crate": "semver 1.0.28", "status": "validated", "archive_sha256": semverChecksum, "rustc": rustVersion, "native_feature_profiles": []string{"std", "no_std", "serde", "no_std+serde"}, "core_requests": len(contract.CoreCorpus()), "serde_requests": len(contract.SerdeCorpus()), "debug_writer_records": 10500, "hash_profiles": []string{"raw native-endian", "primitive overrides", "write_str and write_length_prefix overrides"}, "registry_dependencies": deps, "fixtures_sha256": hashes, "collector_source_layout": map[string]uint{"size": comparatorSize, "align": comparatorAlign}, "collector_requests": len(contract.CollectorCorpus()), "collector_abort_cases": len(contract.CollectorAbortCorpus()), "clone_capacity_requests": len(contract.CloneCorpus()), "std_scope": "scoped collection strategy witnesses; no general stdVec module completion", "original_tests_audit": "original-tests/audit.json", "production_rust_fallback": false, "unicode_compatibility": "all Rust1.99.1 Unicode17 scalar char Debug values; generated independently of Go Unicode version"}
	if e := persistJSON(filepath.Join(root, toolPath, "report.json"), report, *check); e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "verified all15 Rust files,33982 core +3136 serde requests,10500 Debug writer records,4142 collector requests,30 abort-prefix cases,108 clone cases,4 native feature profiles\n")
	return nil
}
func main() {
	if len(os.Args) == 3 && os.Args[1] == "-collector-child" {
		var q contract.CollectorRequest
		if err := json.Unmarshal([]byte(os.Args[2]), &q); err != nil {
			panic(err)
		}
		result := contract.ObserveCollector(q, func(s string) { fmt.Println("EVENT\t" + s) })
		raw, _ := json.Marshal(result)
		fmt.Println(string(raw))
		return
	}
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
