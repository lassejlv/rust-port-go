// Whole-crate original rustc-stable-hash0.1.2 oracle/provenance checker.
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
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/tools/port-rust-stable-hash/contract"
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

const toolPath = "tools/port-rust-stable-hash"
const targetPath = "port/dependencies/rustc-stable-hash/v0_1_2"
const checksum = "781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08"
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

type sourceFile struct {
	Path       string `json:"path"`
	Bytes      int    `json:"bytes"`
	Lines      int    `json:"lines"`
	SHA256     string `json:"sha256"`
	Target     string `json:"target"`
	Adaptation string `json:"adaptation,omitempty"`
}
type sourceInventory struct {
	Crate         string       `json:"crate"`
	Version       string       `json:"version"`
	ArchiveSHA256 string       `json:"archive_sha256"`
	Revision      string       `json:"revision"`
	ArchiveFiles  int          `json:"archive_files"`
	RustFiles     int          `json:"rust_files"`
	RustLines     int          `json:"rust_lines"`
	RustBytes     int          `json:"rust_bytes"`
	Files         []sourceFile `json:"files"`
}

func inventory(data []byte) (sourceInventory, map[string][]byte, error) {
	inv := sourceInventory{Crate: "rustc-stable-hash", Version: "0.1.2", ArchiveSHA256: checksum, Revision: "bda19e8681a381715fc3f32ee95aa5fda0a7f5d0"}
	members := map[string][]byte{}
	if digest(data) != checksum {
		return inv, nil, fmt.Errorf("official archive checksum differs")
	}
	g, e := gzip.NewReader(bytes.NewReader(data))
	if e != nil {
		return inv, nil, e
	}
	defer g.Close()
	t := tar.NewReader(g)
	for {
		h, e := t.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return inv, nil, e
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasPrefix(h.Name, "rustc-stable-hash-0.1.2/") {
			return inv, nil, fmt.Errorf("invalid archive member%s", h.Name)
		}
		name := strings.TrimPrefix(h.Name, "rustc-stable-hash-0.1.2/")
		b, e := io.ReadAll(t)
		if e != nil {
			return inv, nil, e
		}
		members[name] = b
		f := sourceFile{Path: name, Bytes: len(b), Lines: bytes.Count(b, []byte{'\n'}), SHA256: digest(b)}
		if strings.HasSuffix(name, ".rs") {
			inv.RustFiles++
			inv.RustLines += f.Lines
			inv.RustBytes += f.Bytes
			switch name {
			case "src/sip128/tests.rs":
				f.Target = "tests/sip128_test.go; tests/sip128_vector_test.go; tests/sip128_arch_amd64_test.go; tests/sip128_arch_386_test.go; tests/sip128_arch_arm_test.go"
			case "src/stable_hasher/tests.rs":
				f.Target = "tests/stable_hasher_test.go"
			default:
				f.Target = strings.TrimSuffix(name, ".rs") + ".go"
			}
			if name == "src/sip128.rs" {
				f.Adaptation = "MaybeUninit buffer bytes use initialized Go storage; the source never exposes their contents through Debug. No Rust ABI/inline-layout or pointer-copy UB is claimed."
			}
			if name == "src/stable_hasher.rs" {
				f.Target += "; src/ownership.go"
				f.Adaptation = "Rust associated types/generic owned parameters use explicit trait dictionaries and live ownership witnesses, retaining actual state cells and deterministic Drop/cleanup."
			}
		}
		if f.Target == "" {
			f.Target = "upstream/" + name
			if name == ".gitignore" {
				f.Target = "upstream/gitignore.source"
			}
			if strings.HasPrefix(name, "LICENSE-") {
				f.Target = name
			}
		}
		inv.Files = append(inv.Files, f)
	}
	sort.Slice(inv.Files, func(i, j int) bool { return inv.Files[i].Path < inv.Files[j].Path })
	inv.ArchiveFiles = len(inv.Files)
	if inv.ArchiveFiles != 17 || inv.RustFiles != 6 || inv.RustLines != 1441 || inv.RustBytes != 50695 {
		return inv, nil, fmt.Errorf("archive inventory changed:%+v", inv)
	}
	return inv, members, nil
}
func encodeQueries[T any](rows []T) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, row := range rows {
		if e := enc.Encode(row); e != nil {
			panic(e)
		}
	}
	return b.Bytes()
}
func compareRows[T any](raw []byte, observe func(T) any) error {
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 4<<20)
	n := 0
	for scan.Scan() {
		n++
		var row struct {
			Request T
			Result  json.RawMessage
		}
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			return e
		}
		actual, e := json.Marshal(observe(row.Request))
		if e != nil {
			return e
		}
		var a, b any
		_ = json.Unmarshal(actual, &a)
		_ = json.Unmarshal(row.Result, &b)
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("row%d %+v\nGo%s\nRust%s", n, row.Request, actual, row.Result)
		}
	}
	return scan.Err()
}
func compareFormat(raw []byte) (int, error) {
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 4<<20)
	n := 0
	for scan.Scan() {
		n++
		var row contract.FormatRow
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			return n, e
		}
		actual := contract.ObserveFormat(row)
		if !reflect.DeepEqual(actual, row) {
			return n, fmt.Errorf("format row%d %+v\nGo%+v", n, row, actual)
		}
	}
	return n, scan.Err()
}

type arithRequest struct {
	Op string `json:"op"`
	A  uint64 `json:"a"`
	B  uint64 `json:"b"`
}

func arithQueries() []byte {
	rows := []arithRequest{}
	values := []uint64{0, 1, 2, 3, 7, 8, 63, 64, 127, 128, 254, 255, 256, 0x7fffffff, 0x80000000, 0xffffffff, 0x100000000, 0x7fffffffffffffff, 0x8000000000000000, 0xfffffffffffffffe, 0xffffffffffffffff}
	for _, op := range []string{"add", "sub"} {
		for _, a := range values {
			for _, b := range values {
				rows = append(rows, arithRequest{op, a, b})
			}
		}
	}
	return encodeQueries(rows)
}

type processOutcome struct {
	Code       int
	Output     []byte
	Diagnostic []byte
}

func outcome(ctx context.Context, root string, env []string, exe string, args ...string) (processOutcome, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = root
	cmd.Env = env
	var out, diagnostic bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &diagnostic
	err := cmd.Run()
	result := processOutcome{Output: out.Bytes(), Diagnostic: diagnostic.Bytes()}
	if err == nil {
		return result, nil
	}
	if cmd.ProcessState == nil {
		return result, err
	}
	code := cmd.ProcessState.ExitCode()
	if code < 0 && strings.Contains(diagnostic.String(), "aborting") {
		code = 134
	}
	result.Code = code
	if code != 134 {
		return result, fmt.Errorf("unexpected child%s:%v/%s", exe, err, diagnostic.Bytes())
	}
	return result, nil
}
func events(raw []byte) ([]string, error) {
	result := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if !strings.HasPrefix(line, "EVENT\t") {
			return nil, fmt.Errorf("invalid event%q", line)
		}
		result = append(result, strings.TrimPrefix(line, "EVENT\t"))
	}
	return result, nil
}

type ownedRow struct {
	Request contract.OwnershipCase `json:"request"`
	Code    int                    `json:"code"`
	Events  []string               `json:"events"`
}

func ownershipCases() []contract.OwnershipCase {
	rows := []contract.OwnershipCase{}
	for _, context := range []string{"normal", "inherited", "processing"} {
		for _, operation := range []string{"raw-clone", "raw-new"} {
			rows = append(rows, contract.OwnershipCase{Operation: operation, Mode: "normal", Context: context})
		}
		for _, mode := range []string{"normal", "drop", "clone"} {
			rows = append(rows, contract.OwnershipCase{Operation: "clone", Mode: mode, Context: context})
		}
		for _, mode := range []string{"normal", "finish", "from", "h-drop", "r-drop", "finish-h-drop", "from-r-drop"} {
			rows = append(rows, contract.OwnershipCase{Operation: "finish", Mode: mode, Context: context})
		}
	}
	rows = append(rows, contract.OwnershipCase{Operation: "borrow", Mode: "normal", Context: "normal"})
	return rows
}
func selectedHome(root, value string, explicit bool) (string, error) {
	fallback := filepath.Join(root, ".toolchain/port-cargo-home")
	if _, e := os.Stat(fallback); e != nil {
		user, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		fallback = filepath.Join(user, ".cargo")
	}
	return selectedDirectory(root, "CARGO_HOME", value, fallback, explicit)
}
func run() error {
	check := flag.Bool("check", false, "verify retained original outputs without writing")
	rootFlag := flag.String("root", ".", "repository root")
	cargoFlag := flag.String("cargo", "", "Cargo executable")
	rustFlag := flag.String("rustc", "", "rustc executable")
	goFlag := flag.String("go", "", "Go executable")
	homeFlag := flag.String("cargo-home", "", "Cargo registry cache")
	flag.Parse()
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	root, e := filepath.Abs(*rootFlag)
	if e != nil {
		return e
	}
	cargo, e := selected(root, "FERRUGO_CARGO", *cargoFlag, filepath.Join(root, ".toolchain/rust/bin/cargo"), "cargo", set["cargo"])
	if e != nil {
		return e
	}
	rustc, e := selected(root, "FERRUGO_RUSTC", *rustFlag, filepath.Join(root, ".toolchain/rust/bin/rustc"), "rustc", set["rustc"])
	if e != nil {
		return e
	}
	goexe, e := selected(root, "FERRUGO_GO", *goFlag, filepath.Join(root, ".toolchain/go/bin/go"), "go", set["go"])
	if e != nil {
		return e
	}
	home, e := selectedHome(root, *homeFlag, set["cargo-home"])
	if e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	target := filepath.Join(root, ".toolchain/oracles/rust-stable-hash/target")
	env := environment(map[string]string{"CARGO_HOME": home, "RUSTC": rustc, "RUSTC_BOOTSTRAP": "1", "CARGO_TARGET_DIR": target, "GOTOOLCHAIN": "local"})
	v, e := command(ctx, root, env, rustc, nil, "--version")
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(v)) != rustVersion {
		return fmt.Errorf("reference compiler%s required, got%s", rustVersion, v)
	}
	manifest := filepath.Join(root, toolPath, "oracle/Cargo.toml")
	lock, e := os.ReadFile(filepath.Join(root, toolPath, "oracle/Cargo.lock"))
	if e != nil {
		return e
	}
	deps := lockDependencies(lock)
	metadata, e := command(ctx, root, env, cargo, nil, "metadata", "--locked", "--offline", "--all-features", "--format-version=1", "--manifest-path", manifest)
	if e != nil {
		return e
	}
	if e := verifyArchives(metadata, home, deps); e != nil {
		return e
	}
	var meta struct {
		Packages []struct {
			Name     string
			Manifest string `json:"manifest_path"`
		}
	}
	if e := json.Unmarshal(metadata, &meta); e != nil {
		return e
	}
	var archive string
	for _, p := range meta.Packages {
		if p.Name == "rustc-stable-hash" {
			index := filepath.Base(filepath.Dir(filepath.Dir(p.Manifest)))
			archive = filepath.Join(home, "registry/cache", index, "rustc-stable-hash-0.1.2.crate")
		}
	}
	data, e := os.ReadFile(archive)
	if e != nil {
		return e
	}
	inv, members, e := inventory(data)
	if e != nil {
		return e
	}
	for _, f := range inv.Files {
		if !strings.HasSuffix(f.Path, ".rs") {
			if e := persist(filepath.Join(root, targetPath, f.Target), members[f.Path], *check); e != nil {
				return e
			}
		}
	}
	if e := persistJSON(filepath.Join(root, targetPath, "source-manifest.json"), inv, *check); e != nil {
		return e
	}
	// Development-only exact private source module, covered by the verified archive.
	if e := os.WriteFile(filepath.Join(root, toolPath, "oracle/arith_source.rs"), members["src/int_overflow.rs"], 0644); e != nil {
		return e
	}
	hashes := map[string]string{}
	coreOutputs := map[bool][]byte{}
	traceOutputs := map[bool][]byte{}
	arithInput := arithQueries()
	profiles := []map[string]any{}
	for _, nightly := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, overflow := range []bool{false, true} {
				name := fmt.Sprintf("nightly%t-debug%t-overflow%t", nightly, debug, overflow)
				fmt.Fprintln(os.Stderr, "original profile", name)
				flags := fmt.Sprintf("-C debug-assertions=%t -C overflow-checks=%t", debug, overflow)
				profileEnv := environment(map[string]string{"CARGO_HOME": home, "RUSTC": rustc, "RUSTC_BOOTSTRAP": "1", "CARGO_TARGET_DIR": target, "GOTOOLCHAIN": "local", "RUSTFLAGS": flags})
				args := []string{"build", "--locked", "--offline", "--manifest-path", manifest, "--bin", "oracle", "--bin", "trace-oracle", "--bin", "arith-oracle", "--bin", "format-oracle", "--bin", "ownership-probe"}
				if nightly {
					args = append(args, "--features", "nightly")
				}
				if _, e := command(ctx, root, profileEnv, cargo, nil, args...); e != nil {
					return e
				}
				original, e := command(ctx, root, profileEnv, filepath.Join(target, "debug/oracle"), encodeQueries(contract.Corpus()))
				if e != nil {
					return e
				}
				if old := coreOutputs[debug]; old != nil && !bytes.Equal(old, original) {
					return fmt.Errorf("stream output changes across irrelevant profile%s", name)
				}
				coreOutputs[debug] = original
				if !nightly {
					if e := compareRows[contract.Request](original, func(q contract.Request) any {
						return contract.Observe(q, hash.ArithmeticProfile{DebugAssertions: debug, OverflowChecks: overflow})
					}); e != nil {
						return e
					}
				}
				trace, e := command(ctx, root, profileEnv, filepath.Join(target, "debug/trace-oracle"), encodeQueries(contract.TraceCorpus()))
				if e != nil {
					return e
				}
				if old := traceOutputs[nightly]; old != nil && !bytes.Equal(old, trace) {
					return fmt.Errorf("trait trace changes across arithmetic profile%s", name)
				}
				traceOutputs[nightly] = trace
				if !nightly {
					if e := compareRows[contract.TraceRequest](trace, func(q contract.TraceRequest) any { return contract.ObserveTrace(q) }); e != nil {
						return e
					}
				}
				arithmetic, e := command(ctx, root, profileEnv, filepath.Join(target, "debug/arith-oracle"), arithInput)
				if e != nil {
					return e
				}
				file := fmt.Sprintf("arith_debug%t_overflow%t.jsonl.gz", debug, overflow)
				sha, e := fixture(root, file, arithmetic, *check)
				if e != nil {
					return e
				}
				hashes[file] = sha
				profiles = append(profiles, map[string]any{"nightly": nightly, "debug_assertions": debug, "overflow_checks": overflow})
			}
		}
	}
	for debug, raw := range coreOutputs {
		name := fmt.Sprintf("stream_debug%t.jsonl.gz", debug)
		sha, e := fixture(root, name, raw, *check)
		if e != nil {
			return e
		}
		hashes[name] = sha
	}
	for nightly, raw := range traceOutputs {
		name := fmt.Sprintf("trace_nightly%t.jsonl.gz", nightly)
		sha, e := fixture(root, name, raw, *check)
		if e != nil {
			return e
		}
		hashes[name] = sha
	}
	formatOutput, e := command(ctx, root, env, filepath.Join(target, "debug/format-oracle"), nil)
	if e != nil {
		return e
	}
	formatRows, e := compareFormat(formatOutput)
	if e != nil {
		return e
	}
	sha, e := fixture(root, "format.jsonl.gz", formatOutput, *check)
	if e != nil {
		return e
	}
	hashes["format.jsonl.gz"] = sha
	goChild := filepath.Join(root, ".toolchain/oracles/rust-stable-hash/go-child")
	if _, e := command(ctx, root, env, goexe, nil, "build", "-o", goChild, "./"+toolPath); e != nil {
		return e
	}
	ownedRows := []ownedRow{}
	for _, q := range ownershipCases() {
		native, e := outcome(ctx, root, env, filepath.Join(target, "debug/ownership-probe"), q.Operation, q.Mode, q.Context)
		if e != nil {
			return e
		}
		expected, e := events(native.Output)
		if e != nil {
			return e
		}
		request, _ := json.Marshal(q)
		actual, e := outcome(ctx, root, env, goChild, "-ownership-child", string(request))
		if e != nil {
			return e
		}
		observed, e := events(actual.Output)
		if e != nil {
			return e
		}
		if native.Code != actual.Code || !reflect.DeepEqual(expected, observed) {
			return fmt.Errorf("owned case%+v\nGo%d/%v\nRust%d/%v", q, actual.Code, observed, native.Code, expected)
		}
		ownedRows = append(ownedRows, ownedRow{q, native.Code, expected})
	}
	if e := persistJSON(filepath.Join(root, toolPath, "ownership.json"), ownedRows, *check); e != nil {
		return e
	}
	for _, nightly := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, overflow := range []bool{false, true} {
				goEnv := environment(map[string]string{"CARGO_HOME": home, "RUSTC": rustc, "RUSTC_BOOTSTRAP": "1", "CARGO_TARGET_DIR": target, "GOTOOLCHAIN": "local", "FERRUGO_STABLE_HASH_TEST_DEBUG": strconv.FormatBool(debug), "FERRUGO_STABLE_HASH_TEST_CHECKED": strconv.FormatBool(overflow)})
				args := []string{"test", "-count=1"}
				if nightly {
					args = append(args, "-tags", "rustc_stable_hash_nightly")
				}
				args = append(args, "./"+targetPath+"/...")
				out, e := command(ctx, root, goEnv, goexe, nil, args...)
				if e != nil {
					return e
				}
				fmt.Fprint(os.Stderr, string(out))
			}
		}
	}
	if e := verifyArchives(metadata, home, deps); e != nil {
		return e
	}
	for _, f := range inv.Files {
		for _, target := range strings.Split(f.Target, "; ") {
			if _, e := os.Stat(filepath.Join(root, targetPath, target)); e != nil {
				return e
			}
		}
	}
	report := map[string]any{"crate": "rustc-stable-hash 0.1.2", "status": "validated", "archive_sha256": checksum, "rustc": rustVersion, "source_files": 6, "source_lines": 1441, "production_rust_fallback": false, "native_profiles": profiles, "stream_requests": len(contract.Corpus()), "generic_trace_streams": len(contract.TraceCorpus()), "format_records": formatRows, "ownership_cases": len(ownedRows), "fixtures_sha256": hashes, "dependencies": deps, "original_tests_audit": "original-tests/audit.json", "scope": "complete crate API through explicit trait/ownership witnesses; no Rust ABI or general standard-library source credit"}
	if e := persistJSON(filepath.Join(root, toolPath, "report.json"), report, *check); e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "verified6 Rust files,8 profiles,%d streams,%d format records,%d owned cases\n", len(contract.Corpus()), formatRows, len(ownedRows))
	return nil
}
func main() {
	if len(os.Args) == 3 && os.Args[1] == "-ownership-child" {
		var q contract.OwnershipCase
		if e := json.Unmarshal([]byte(os.Args[2]), &q); e != nil {
			panic(e)
		}
		contract.ObserveOwnership(q, func(s string) { fmt.Println("EVENT\t" + s) })
		return
	}
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
