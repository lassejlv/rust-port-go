// Command port-lexer-unicode regenerates rustc_lexer's Unicode predicates by
// executing the exact Rust dependencies selected by the pinned Rust lockfile.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const rustRevision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const codePoints = 0x110000
const scalarValues = codePoints - 0x800

type dependency struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	SHA256  string   `json:"crate_sha256"`
	URL     string   `json:"url"`
	License string   `json:"license"`
	Notices []notice `json:"notices"`
}

type notice struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

var dependencies = []dependency{
	{
		Name: "unicode-ident", Version: "1.0.24",
		SHA256:  "e6e4313cd5fcd3dad5cafa179702e2b244f760991f45397d14d4ebf38247da75",
		License: "(MIT OR Apache-2.0) AND Unicode-3.0",
	},
	{
		Name: "unicode-properties", Version: "0.1.4",
		SHA256:  "7df058c713841ad818f1dc5d3fd88063241cc61f49f5fbea4b951e8cf5a8d71d",
		License: "MIT/Apache-2.0",
	},
}

type interval struct{ low, high uint32 }

type class struct {
	Name         string `json:"name"`
	Count        int    `json:"scalar_count"`
	RangeCount   int    `json:"range_count"`
	RangesSHA256 string `json:"ranges_sha256"`
	ranges       []interval
}

type manifest struct {
	Format                 int          `json:"format"`
	RustRevision           string       `json:"rust_revision"`
	UnicodeVersion         [3]uint8     `json:"unicode_version"`
	ScalarValues           int          `json:"scalar_values"`
	CodePoints             int          `json:"code_points"`
	OracleFlags            string       `json:"oracle_flag_bits"`
	OracleSHA256           string       `json:"oracle_flags_sha256"`
	OracleFixtureSHA256    string       `json:"oracle_gzip_sha256"`
	OracleRustSourceSHA256 string       `json:"oracle_rust_source_sha256"`
	OracleCargoLockSHA256  string       `json:"oracle_cargo_lock_sha256"`
	RangeEncoding          string       `json:"range_hash_encoding"`
	Classes                []class      `json:"classes"`
	Dependencies           []dependency `json:"dependencies"`
}

type options struct {
	root, cargo, rustc, cargoHome string
	check, offline                bool
}

func main() {
	var opts options
	flag.StringVar(&opts.root, "root", ".", "repository root")
	flag.StringVar(&opts.cargo, "cargo", os.Getenv("FERRUGO_CARGO"), "Cargo executable (FERRUGO_CARGO, local toolchain, then PATH)")
	flag.StringVar(&opts.rustc, "rustc", os.Getenv("FERRUGO_RUSTC"), "Rust compiler (FERRUGO_RUSTC, local toolchain, then PATH)")
	flag.StringVar(&opts.cargoHome, "cargo-home", environmentDefault("CARGO_HOME", ".toolchain/port-cargo-home"), "Cargo cache (CARGO_HOME or isolated local cache), relative to root or absolute")
	flag.BoolVar(&opts.check, "check", false, "verify generated files without changing them")
	flag.BoolVar(&opts.offline, "offline", false, "require dependencies to be cached")
	flag.Parse()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "port-lexer-unicode:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	absolute := func(path string) string {
		if filepath.IsAbs(path) {
			return path
		}
		return filepath.Join(root, path)
	}
	cargo, err := resolveExecutable(root, opts.cargo, ".toolchain/rust/bin/cargo")
	if err != nil {
		return fmt.Errorf("select Cargo executable: %w", err)
	}
	rustc, err := resolveExecutable(root, opts.rustc, ".toolchain/rust/bin/rustc")
	if err != nil {
		return fmt.Errorf("select Rust compiler: %w", err)
	}
	cargoHome := absolute(opts.cargoHome)
	upstream := filepath.Join(root, ".toolchain/upstream-rust")
	cmd := exec.Command("git", "-C", upstream, "rev-parse", "HEAD")
	revision, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("read upstream Rust revision: %w", err)
	}
	if strings.TrimSpace(string(revision)) != rustRevision {
		return fmt.Errorf("upstream Rust revision is %q; expected %s", bytes.TrimSpace(revision), rustRevision)
	}
	sourceDir := filepath.Join(root, "tools/port-lexer-unicode/oracle")
	for _, lockPath := range []string{filepath.Join(upstream, "Cargo.lock"), filepath.Join(sourceDir, "Cargo.lock")} {
		if err := checkLockfile(lockPath); err != nil {
			return err
		}
	}
	oracleDir := filepath.Join(root, ".toolchain/oracles/lexer-unicode")
	for _, name := range []string{"Cargo.toml", "Cargo.lock", "src/main.rs"} {
		data, err := os.ReadFile(filepath.Join(sourceDir, name))
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(oracleDir, name), data); err != nil {
			return err
		}
	}
	args := []string{"run", "--release", "--locked", "--manifest-path", filepath.Join(oracleDir, "Cargo.toml")}
	if opts.offline {
		args = append(args, "--offline")
	}
	cmd = exec.Command(cargo, args...)
	cmd.Dir = root
	cmd.Env = withEnv(os.Environ(), map[string]string{"CARGO_HOME": cargoHome, "RUSTC": rustc})
	cmd.Stderr = os.Stderr
	oracle, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("execute pinned Rust Unicode oracle: %w", err)
	}
	if len(oracle) != 12+codePoints || !bytes.Equal(oracle[:8], []byte("FERRUGOU")) || oracle[11] != 0 {
		return errors.New("invalid Rust Unicode oracle output")
	}
	version := [3]uint8{oracle[8], oracle[9], oracle[10]}
	if version != [3]uint8{17, 0, 0} {
		return fmt.Errorf("Unicode version %v does not match pinned 17.0.0", version)
	}
	flags := oracle[12:]
	for cp, value := range flags {
		if value&^byte(7) != 0 || (cp >= 0xd800 && cp <= 0xdfff && value != 0) {
			return fmt.Errorf("invalid oracle flags %d at U+%04X", value, cp)
		}
	}
	classes := makeClasses(flags)
	packageDir := filepath.Join(root, "port/rust/compiler/rustc_lexer")
	fixturePath := filepath.Join(packageDir, "src/testdata/unicode_oracle.bin.gz")
	compressed, err := oracleFixture(fixturePath, flags, opts.check)
	if err != nil {
		return err
	}
	outputs := map[string][]byte{}
	for i := range dependencies {
		dep := &dependencies[i]
		dep.URL = fmt.Sprintf("https://static.crates.io/crates/%s/%s-%s.crate", dep.Name, dep.Name, dep.Version)
		archive, err := oneMatch(filepath.Join(cargoHome, "registry/cache/*", dep.Name+"-"+dep.Version+".crate"))
		if err != nil {
			return err
		}
		archiveBytes, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		if got := digest(archiveBytes); got != dep.SHA256 {
			return fmt.Errorf("crate %s checksum %s; expected %s", dep.Name, got, dep.SHA256)
		}
		crateDir, err := oneMatch(filepath.Join(cargoHome, "registry/src/*", dep.Name+"-"+dep.Version))
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(crateDir)
		if err != nil {
			return err
		}
		dep.Notices = nil
		for _, entry := range entries {
			if entry.IsDir() || !(strings.HasPrefix(entry.Name(), "LICENSE") || entry.Name() == "COPYRIGHT" || entry.Name() == "NOTICE") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(crateDir, entry.Name()))
			if err != nil {
				return err
			}
			dep.Notices = append(dep.Notices, notice{entry.Name(), digest(data)})
			outputs[filepath.Join(packageDir, "dependencies", dep.Name, entry.Name())] = data
		}
		if len(dep.Notices) == 0 {
			return fmt.Errorf("crate %s has no license notices", dep.Name)
		}
	}
	rustSource, err := os.ReadFile(filepath.Join(sourceDir, "src/main.rs"))
	if err != nil {
		return err
	}
	cargoLock, err := os.ReadFile(filepath.Join(sourceDir, "Cargo.lock"))
	if err != nil {
		return err
	}
	m := manifest{
		Format: 1, RustRevision: rustRevision, UnicodeVersion: version,
		ScalarValues: scalarValues, CodePoints: codePoints,
		OracleFlags:  "bit 0 = XID_Start; bit 1 = XID_Continue; bit 2 = UnicodeEmoji::is_emoji_char; one byte per code point; surrogates = 0",
		OracleSHA256: digest(flags), OracleFixtureSHA256: digest(compressed),
		OracleRustSourceSHA256: digest(rustSource), OracleCargoLockSHA256: digest(cargoLock),
		RangeEncoding: "consecutive inclusive ranges, each lower then upper bound as little-endian uint32",
		Classes:       classes, Dependencies: dependencies,
	}
	manifestBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tables, err := goTables(version, classes)
	if err != nil {
		return err
	}
	outputs[filepath.Join(packageDir, "src/unicode_tables.go")] = tables
	outputs[fixturePath] = compressed
	outputs[filepath.Join(packageDir, "unicode_manifest.json")] = append(manifestBytes, '\n')
	for path, data := range outputs {
		if opts.check {
			actual, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, data) {
				return fmt.Errorf("generated output differs: %s", path)
			}
		} else if err := writeFile(path, data); err != nil {
			return err
		}
	}
	verb := "generated"
	if opts.check {
		verb = "verified"
	}
	fmt.Printf("%s Unicode %d.%d.%d: %d scalar values, flags SHA-256 %s\n", verb, version[0], version[1], version[2], scalarValues, m.OracleSHA256)
	for _, c := range classes {
		fmt.Printf("  %s: %d scalars in %d ranges\n", c.Name, c.Count, c.RangeCount)
	}
	return nil
}

func checkLockfile(path string) error {
	lock, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for _, dep := range dependencies {
		found := false
		for _, section := range strings.Split(string(lock), "[[package]]") {
			if !strings.Contains(section, "\nname = \""+dep.Name+"\"\n") {
				continue
			}
			if !strings.Contains(section, "\nversion = \""+dep.Version+"\"\n") ||
				!strings.Contains(section, "\nchecksum = \""+dep.SHA256+"\"\n") ||
				!strings.Contains(section, "\nsource = \"registry+https://github.com/rust-lang/crates.io-index\"\n") {
				return fmt.Errorf("%s has unexpected %s version, checksum, or source", path, dep.Name)
			}
			found = true
		}
		if !found {
			return fmt.Errorf("%s is missing %s", path, dep.Name)
		}
	}
	return nil
}

func makeClasses(flags []byte) []class {
	classes := []class{{Name: "XID_Start"}, {Name: "XID_Continue"}, {Name: "Emoji"}}
	for i := range classes {
		c := &classes[i]
		for cp, f := range flags {
			if f&(1<<i) == 0 {
				continue
			}
			c.Count++
			if len(c.ranges) > 0 && c.ranges[len(c.ranges)-1].high+1 == uint32(cp) {
				c.ranges[len(c.ranges)-1].high = uint32(cp)
			} else {
				c.ranges = append(c.ranges, interval{uint32(cp), uint32(cp)})
			}
		}
		var encoded bytes.Buffer
		for _, r := range c.ranges {
			_ = binary.Write(&encoded, binary.LittleEndian, r.low)
			_ = binary.Write(&encoded, binary.LittleEndian, r.high)
		}
		c.RangeCount = len(c.ranges)
		c.RangesSHA256 = digest(encoded.Bytes())
	}
	return classes
}

func goTables(version [3]uint8, classes []class) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprint(&b, "// Code generated by tools/port-lexer-unicode; DO NOT EDIT.\n")
	fmt.Fprint(&b, "// Derived from unicode-ident 1.0.24 and unicode-properties 0.1.4.\n")
	fmt.Fprint(&b, "// Dependency licenses and notices are retained in ../dependencies/.\n\n")
	fmt.Fprint(&b, "package rustclexer\n\n")
	fmt.Fprint(&b, "//go:generate go run ../../../../../tools/port-lexer-unicode -root ../../../../..\n\n")
	fmt.Fprint(&b, "// UnicodeVersion matches the Unicode version used by the pinned Rust lexer.\n")
	fmt.Fprintf(&b, "var UnicodeVersion = [3]uint8{%d, %d, %d}\n\n", version[0], version[1], version[2])
	fmt.Fprint(&b, "type unicodeRange struct { low, high rune }\n\n")
	names := []string{"xidStartRanges", "xidContinueRanges", "emojiRanges"}
	for i, c := range classes {
		fmt.Fprintf(&b, "// %s contains %d scalars in %d ranges; SHA-256 %s.\n", c.Name, c.Count, c.RangeCount, c.RangesSHA256)
		fmt.Fprintf(&b, "var %s = [...]unicodeRange{\n", names[i])
		for _, r := range c.ranges {
			fmt.Fprintf(&b, "{0x%X, 0x%X},\n", r.low, r.high)
		}
		fmt.Fprint(&b, "}\n\n")
	}
	fmt.Fprint(&b, `func isXIDStart(c rune) bool { return inUnicodeRanges(c, xidStartRanges[:]) }

func isXIDContinue(c rune) bool { return inUnicodeRanges(c, xidContinueRanges[:]) }

func isEmojiChar(c rune) bool { return inUnicodeRanges(c, emojiRanges[:]) }

func inUnicodeRanges(c rune, ranges []unicodeRange) bool {
	lo, hi := 0, len(ranges)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if c < ranges[mid].low {
			hi = mid
		} else if c > ranges[mid].high {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}
`)
	return format.Source(b.Bytes())
}

func oneMatch(pattern string) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("expected one path matching %s, found %d", pattern, len(matches))
	}
	return matches[0], nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func withEnv(env []string, overrides map[string]string) []string {
	out := make([]string, 0, len(env)+len(overrides))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if _, replaced := overrides[key]; !replaced {
			out = append(out, entry)
		}
	}
	for key, value := range overrides {
		out = append(out, key+"="+value)
	}
	return out
}

func environmentDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func resolveExecutable(root, name, localDefault string) (string, error) {
	if name == "" {
		local := filepath.Join(root, localDefault)
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			return local, nil
		}
		name = filepath.Base(localDefault)
	}
	if !strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	return name, nil
}

// Go versions can emit different DEFLATE streams for the same input. Preserve
// the existing stream when its checked payload matches; the manifest separately
// verifies the SHA-256 of these exact compressed bytes.
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
