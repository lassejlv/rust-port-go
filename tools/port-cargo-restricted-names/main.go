// Original Cargo restricted-name source oracle generator. SPDX-License-Identifier: MIT OR Apache-2.0
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
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

const revision = "24de4721844c7a4d839072bab6fb66d41c9bc1f4"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const toolPath = "tools/port-cargo-restricted-names"
const targetPath = "port/cargo/crates/cargo-util-schemas/src"

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
			return filepath.Join(root, fallback), nil
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
	if len(seen) != len(wanted) {
		return fmt.Errorf("metadata selected %d registry packages, expected %d", len(seen), len(wanted))
	}
	return nil
}

type sourcePin struct {
	Path string `json:"path"`
	Blob string `json:"git_blob"`
	Hash string `json:"sha256"`
}

var sourcePins = []sourcePin{
	{"crates/cargo-util-schemas/src/restricted_names.rs", "dfe44bfb92bd1f485091476be02a96c6ee836ce3", "6d3869b64e61f4de029e5ac69f4257c34b3dc6010e4cf39554880b13d15b1950"},
	{"Cargo.lock", "8af3a6d2fbbc40a8c7aa0d97045eebc29c9a6253", "f394b1d1e68c7f7cd11261b1623493c3da407a88f0f50519a403c339b750b5c7"},
}

func pinnedSource(ctx context.Context, root string, pin sourcePin) ([]byte, error) {
	env := environment(map[string]string{"GIT_NO_LAZY_FETCH": "1"})
	blob, err := command(ctx, root, env, "git", nil, "-C", root, "rev-parse", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(blob)) != pin.Blob {
		return nil, fmt.Errorf("Git blob differs: %s", pin.Path)
	}
	data, err := command(ctx, root, env, "git", nil, "-C", root, "show", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if digest(data) != pin.Hash {
		return nil, fmt.Errorf("source SHA-256 differs: %s", pin.Path)
	}
	return data, nil
}
func write32(w io.Writer, n uint32) {
	if err := binary.Write(w, binary.LittleEndian, n); err != nil {
		panic(err)
	}
}
func text(w io.Writer, s string) {
	write32(w, uint32(len(s)))
	if _, err := io.WriteString(w, s); err != nil {
		panic(err)
	}
}
func requests() ([]byte, int) {
	var records bytes.Buffer
	count := 0
	add := func(op byte, name, what string, placeholder rune) {
		records.WriteByte(op)
		text(&records, name)
		text(&records, what)
		write32(&records, uint32(placeholder))
		count++
	}
	all := func(name string) {
		for op := byte(0); op < 8; op++ {
			add(op, name, "custom `name`\n雪", '_')
		}
	}
	names := []string{"", "a", "_", "-", "0", "9a", "١a", "a١", "a\u0301", "\u0301a", "\u200ca", "a\u200c", "a\u200d", "a\u200b", "a-b", "a_b", "a+b", "a.b", "a:b", "a::b", "::", "::a", "a::", "a::::b", "dep:", "dep:x/y", "Dep:x", "?a/", "a/\x00", "foo?", "ⒶⒷⒸ", "a¼", "¼a", "K", "ΚΣ", "ΟΣ\u0301", "İ", "雪", "😀", "a😀", "\x00", "a\x00", "a\n", "a\r", "a\t", "\"'\\", "\ufeff", "\U00010d50", "\U00001c89", "\U00011f02", "\U0001e5d0", "\U00031350", "\U000323b0", "\U0010ffff", "c++17", "128bit", "feat-name", "feat_name", "foo.bar", "build-override", "BUILD-OVERRIDE", "buİld", "pacKage", "CARGOcustom", "cargó", "cargo\u0301", "cargo-", ""}
	for _, name := range names {
		all(name)
	}
	for _, name := range []string{"build", "check", "clean", "config", "fetch", "fix", "install", "metadata", "package", "publish", "report", "root", "run", "rust", "rustc", "rustdoc", "target", "tmp", "uninstall", "cargo", "build-override"} {
		for _, v := range []string{name, strings.ToUpper(name), "my-" + name, name + "-extra"} {
			all(v)
		}
	}
	alphabet := []rune{'a', 'Z', '0', '_', '-', ':', '/', '+', '.', '?', '\x00', '雪'}
	var enumerate func(string, int)
	enumerate = func(prefix string, depth int) {
		all(prefix)
		if depth == 0 {
			return
		}
		for _, ch := range alphabet {
			enumerate(prefix+string(ch), depth-1)
		}
	}
	enumerate("", 3)
	seed := uint64(0x8b294cc175d09a23)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	scalars := []rune{'a', 'Z', '0', '9', '_', '-', ':', '/', '+', '.', '?', '\x00', '\n', '\t', '\r', '\'', '"', '\\', 'Σ', 'Ο', '\u0301', '\u0345', '\u200c', '\u200d', '\u0085', '١', '¼', 'Ⓐ', 'K', 'İ', '\u00a0', '雪', '😀', 0x10d50, 0x1c89, 0x1e5d0, 0x11f02, 0x323b0, 0x10ffff}
	for i := 0; i < 16000; i++ {
		var name strings.Builder
		n := int(next() % 25)
		for j := 0; j < n; j++ {
			if next()%4 == 0 {
				v := rune(next() % 0x110000)
				if v >= 0xd800 && v <= 0xdfff {
					v = '?'
				}
				name.WriteRune(v)
			} else {
				name.WriteRune(scalars[next()%uint64(len(scalars))])
			}
		}
		p := scalars[next()%uint64(len(scalars))]
		add(byte(i%8), name.String(), "what\x00\"\u0301", p)
	}
	for _, p := range scalars {
		for _, name := range []string{"", "09?!", "09a!雪?", "\u0301_!", "a!::b?", "a\x00\n"} {
			add(6, name, "", p)
			add(7, name, "", p)
		}
	}
	// Arbitrary static context strings exercise every private error payload.
	for _, what := range []string{"", "name", "\x00", "\u0301x", "雪\"\\\n"} {
		add(2, "", what, '_')
		add(2, "?", what, '_')
		add(2, "a?", what, '_')
	}
	var result bytes.Buffer
	result.WriteString("RNR1")
	write32(&result, uint32(count))
	result.Write(records.Bytes())
	return result.Bytes(), count
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
func tables(raw []byte) ([]byte, error) {
	if len(raw) < 4 || string(raw[:4]) != "RNU1" {
		return nil, fmt.Errorf("bad Unicode profile")
	}
	data := raw[4:]
	flags := make([]byte, 0x110000)
	for i := range flags {
		if len(data) < 2 || len(data) < 2+int(data[1]) {
			return nil, fmt.Errorf("short Unicode profile at %x", i)
		}
		flags[i] = data[0]
		data = data[2+int(data[1]):]
	}
	if len(data) != 0 {
		return nil, fmt.Errorf("trailing Unicode profile")
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by tools/port-cargo-restricted-names; DO NOT EDIT.\n// Rust 1.99.1 / Unicode 17.0.0. MIT OR Apache-2.0; see restricted_names_unicode_LICENSE.\npackage cargoutilschemas\n\nconst RestrictedNamesUnicodeVersion = \"17.0.0\"")
	for _, table := range []struct {
		name string
		mask byte
	}{{"nameAlphanumericRanges", 4}, {"nameEscapedRanges", 16}, {"nameNonInitialEscapedRanges", 32}} {
		fmt.Fprintf(&out, "var %s = [][2]rune{\n", table.name)
		for i := 0; i < len(flags); i++ {
			if flags[i]&table.mask == 0 {
				continue
			}
			start := i
			for i+1 < len(flags) && flags[i+1]&table.mask != 0 {
				i++
			}
			fmt.Fprintf(&out, "{0x%x,0x%x},\n", start, i)
		}
		fmt.Fprintln(&out, "}")
	}
	return format.Source(out.Bytes())
}

type manifest struct {
	Format        int               `json:"format"`
	Revision      string            `json:"revision"`
	Sources       []sourcePin       `json:"sources"`
	Dependencies  []dependency      `json:"registry_dependencies"`
	Rustc         string            `json:"rustc"`
	Unicode       string            `json:"stdlib_unicode"`
	Go            string            `json:"generation_go"`
	Host          string            `json:"generation_host"`
	Cases         int               `json:"cases"`
	FormatCases   int               `json:"format_cases"`
	ScalarValues  int               `json:"scalar_values"`
	OriginalTests int               `json:"original_tests"`
	Hashes        map[string]string `json:"sha256"`
}

func run(root, sourceRoot, cargoHome, cargo, rustc string, check bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	version, err := command(ctx, root, nil, rustc, nil, "--version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s, found %s", rustVersion, version)
	}
	source, err := pinnedSource(ctx, sourceRoot, sourcePins[0])
	if err != nil {
		return err
	}
	upstreamLock, err := pinnedSource(ctx, sourceRoot, sourcePins[1])
	if err != nil {
		return err
	}
	lock, err := os.ReadFile(filepath.Join(root, toolPath, "Cargo.lock"))
	if err != nil {
		return err
	}
	deps := lockDependencies(lock)
	if len(deps) != 6 {
		return fmt.Errorf("expected six original locked dependencies, got %d", len(deps))
	}
	known := map[string]string{}
	for _, d := range lockDependencies(upstreamLock) {
		known[d.Name+"@"+d.Version] = d.Checksum
	}
	for _, d := range deps {
		if d.Checksum == "" || known[d.Name+"@"+d.Version] != d.Checksum {
			return fmt.Errorf("dependency differs from original Cargo.lock: %+v", d)
		}
	}
	temp, err := os.MkdirTemp("", "cargo-restricted-names-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	hashes := map[string]string{}
	for _, name := range []string{"Cargo.toml", "Cargo.lock", "oracle.rs", "format_oracle.rs", "main.go", "format.go"} {
		data, err := os.ReadFile(filepath.Join(root, toolPath, name))
		if err != nil {
			return err
		}
		hashes[name] = digest(data)
		if !strings.HasSuffix(name, ".go") {
			if err := os.WriteFile(filepath.Join(temp, name), data, 0600); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(temp, "restricted_names.rs"), source, 0600); err != nil {
		return err
	}
	env := environment(map[string]string{"RUSTC": rustc, "CARGO_HOME": cargoHome, "CARGO_TARGET_DIR": filepath.Join(temp, "target"), "CARGO_NET_OFFLINE": "true", "RUSTUP_TOOLCHAIN": "1.99.1"})
	meta, err := command(ctx, temp, env, cargo, nil, "metadata", "--locked", "--offline", "--format-version=1")
	if err != nil {
		return err
	}
	if err := verifyArchives(meta, cargoHome, deps); err != nil {
		return err
	}
	tests, err := command(ctx, temp, env, cargo, nil, "test", "--locked", "--offline", "--bin", "oracle")
	if err != nil {
		return err
	}
	if !bytes.Contains(tests, []byte("1 passed; 0 failed")) {
		return fmt.Errorf("original inline test missing: %s", tests)
	}
	if _, err := command(ctx, temp, env, cargo, nil, "build", "--locked", "--offline", "--release", "--bin", "oracle"); err != nil {
		return err
	}
	binaryPath := filepath.Join(temp, "target", "release", "oracle")
	request, count := requests()
	answers, err := command(ctx, temp, env, binaryPath, request)
	if err != nil {
		return err
	}
	if len(answers) < 8 || string(answers[:4]) != "RNO1" || binary.LittleEndian.Uint32(answers[4:8]) != uint32(count) {
		return fmt.Errorf("bad original response stream")
	}
	unicode, err := command(ctx, temp, env, binaryPath, nil, "unicode")
	if err != nil {
		return err
	}
	formatCorpus, formatCount, err := nameFormatCorpus(ctx, temp, env, binaryPath)
	if err != nil {
		return err
	}
	table, err := tables(unicode)
	if err != nil {
		return err
	}
	var corpus bytes.Buffer
	corpus.WriteString("RNC1")
	write32(&corpus, uint32(len(request)))
	corpus.Write(request)
	corpus.Write(answers)
	files := map[string][]byte{"restricted_names_unicode.go": table}
	for name, raw := range map[string][]byte{"testdata/restricted_names_cases.bin.gz": corpus.Bytes(), "testdata/restricted_names_unicode.bin.gz": unicode, "testdata/restricted_names_formats.bin.gz": formatCorpus} {
		old, readErr := os.ReadFile(filepath.Join(root, targetPath, name))
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		compressed, err := retainGzip(raw, old, check)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		files[name] = compressed
		hashes[name+".raw"] = digest(raw)
	}
	for name, data := range files {
		hashes[name] = digest(data)
	}
	hashes["requests.raw"] = digest(request)
	hashes["responses.raw"] = digest(answers)
	m := manifest{Format: 1, Revision: revision, Sources: sourcePins, Dependencies: deps, Rustc: rustVersion, Unicode: "17.0.0", Go: runtime.Version(), Host: runtime.GOOS + "/" + runtime.GOARCH, Cases: count, FormatCases: formatCount, ScalarValues: 1112064, OriginalTests: 1, Hashes: hashes}
	manifestPath := filepath.Join(root, toolPath, "manifest.json")
	if check {
		old, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		var previous manifest
		if err := json.Unmarshal(old, &previous); err != nil {
			return err
		}
		m.Go, m.Host = previous.Go, previous.Host
	}
	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	manifestData = append(manifestData, '\n')
	// Check every result before writing any output, so failed provenance and
	// explicit tool-selection errors cannot partly update retained evidence.
	for name, data := range files {
		path := filepath.Join(root, targetPath, name)
		if check {
			old, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(old, data) {
				return fmt.Errorf("generated file differs: %s", path)
			}
		}
	}
	if check {
		old, err := os.ReadFile(manifestPath)
		if err != nil || !bytes.Equal(old, manifestData) {
			return fmt.Errorf("manifest differs")
		}
	} else {
		for name, data := range files {
			path := filepath.Join(root, targetPath, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}
		}
		if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("validated original restricted_names.rs: %d cases, %d formatter scripts, 1112064 Unicode scalars, 1 original test; %s\n", count, formatCount, runtime.Version())
	return nil
}
func main() {
	rootFlag := flag.String("root", ".", "repository root")
	sourceFlag := flag.String("cargo-source", "", "pinned Cargo source Git cache (FERRUGO_CARGO_SOURCE)")
	homeFlag := flag.String("cargo-home", "", "Cargo registry cache (CARGO_HOME)")
	cargoFlag := flag.String("cargo", "", "Cargo executable (FERRUGO_CARGO)")
	rustcFlag := flag.String("rustc", "", "Rust executable (FERRUGO_RUSTC)")
	check := flag.Bool("check", false, "verify original-source fixtures without changing files")
	flag.Parse()
	if *rootFlag == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "root must be nonempty and positional arguments are unsupported")
		os.Exit(1)
	}
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cargo, err := selected(root, "FERRUGO_CARGO", *cargoFlag, filepath.Join(root, ".toolchain/rust/bin/cargo"), "cargo", explicit["cargo"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rustc, err := selected(root, "FERRUGO_RUSTC", *rustcFlag, filepath.Join(root, ".toolchain/rust/bin/rustc"), "rustc", explicit["rustc"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	source, err := selectedDirectory(root, "FERRUGO_CARGO_SOURCE", *sourceFlag, ".toolchain/upstream-cargo", explicit["cargo-source"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home, err := selectedDirectory(root, "CARGO_HOME", *homeFlag, ".toolchain/port-cargo-home", explicit["cargo-home"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run(root, source, home, cargo, rustc, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
