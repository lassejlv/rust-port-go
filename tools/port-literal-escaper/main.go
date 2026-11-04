// Command port-literal-escaper verifies the complete registry dependency port
// against its original unmodified implementation and tests.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const crateName = "rustc-literal-escaper"
const crateVersion = "0.0.8"
const crateSHA256 = "bfe6f213fb658c8fb95baabd5420393438cf5a98d707f5dd701d9197c705f71e"
const dependencyPath = "port/dependencies/rustc-literal-escaper/v0_0_8"

//go:embed oracle.rs
var oracleSource []byte

//go:embed oracle/Cargo.toml
var cargoManifest []byte

//go:embed oracle/Cargo.lock
var cargoLock []byte

type options struct {
	root, cargoHome, cargo, rustc string
	check, offline                bool
}
type sourceFile struct {
	Path         string `json:"path"`
	SHA256       string `json:"sha256"`
	Bytes        int    `json:"bytes"`
	Lines        int    `json:"lines"`
	Role         string `json:"role"`
	Target       string `json:"target"`
	Status       string `json:"status"`
	TargetSHA256 string `json:"target_sha256"`
	Evidence     string `json:"evidence"`
}
type manifest struct {
	Format                      int          `json:"format"`
	Name                        string       `json:"name"`
	Version                     string       `json:"version"`
	Registry                    string       `json:"registry"`
	URL                         string       `json:"url"`
	CrateSHA256                 string       `json:"crate_sha256"`
	SourceRevision              string       `json:"source_revision"`
	License                     string       `json:"license"`
	Files                       []sourceFile `json:"files"`
	UpstreamTests               int          `json:"upstream_tests"`
	UpstreamTestsPassed         bool         `json:"upstream_tests_passed"`
	PortedBenchmarkWorkloads    int          `json:"ported_benchmark_workloads"`
	OracleShimSHA256            string       `json:"oracle_shim_sha256"`
	OracleEncoding              string       `json:"oracle_encoding"`
	GoldenCases                 int          `json:"golden_cases"`
	GoldenSHA256                string       `json:"golden_sha256"`
	GoldenGzipSHA256            string       `json:"golden_gzip_sha256"`
	MetadataHex                 string       `json:"metadata_hex"`
	MixedConstructorsHex        string       `json:"mixed_constructors_hex"`
	MixedConstructorValues      []uint16     `json:"mixed_constructor_values"`
	MixedConstructorEncoding    string       `json:"mixed_constructor_encoding"`
	PrivateTraitSpecializations []string     `json:"private_trait_specializations"`
	ScalarValues                int          `json:"scalar_values"`
	ScalarOperations            int          `json:"scalar_operations_per_value"`
	ScalarTraceSHA256           string       `json:"scalar_trace_sha256"`
	ScalarTraceBytes            int64        `json:"scalar_trace_bytes"`
}
type request struct {
	op     byte
	source string
}

func main() {
	var opts options
	flag.StringVar(&opts.root, "root", ".", "repository root")
	flag.StringVar(&opts.cargoHome, "cargo-home", envDefault("CARGO_HOME", ".toolchain/port-cargo-home"), "Cargo cache")
	flag.StringVar(&opts.cargo, "cargo", os.Getenv("FERRUGO_CARGO"), "Cargo executable")
	flag.StringVar(&opts.rustc, "rustc", os.Getenv("FERRUGO_RUSTC"), "Rust compiler")
	flag.BoolVar(&opts.offline, "offline", false, "require the exact crate to be cached")
	flag.BoolVar(&opts.check, "check", false, "verify outputs without changing tracked files")
	flag.Parse()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "port-literal-escaper:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	cargoHome := absolute(root, opts.cargoHome)
	rustc, err := executable(root, opts.rustc, ".toolchain/rust/bin/rustc")
	if err != nil {
		return err
	}
	oracleDir := filepath.Join(root, ".toolchain/oracles/literal-escaper")
	archives, _ := filepath.Glob(filepath.Join(cargoHome, "registry/cache/*", crateName+"-"+crateVersion+".crate"))
	if len(archives) == 0 {
		cargo, err := executable(root, opts.cargo, ".toolchain/rust/bin/cargo")
		if err != nil {
			return err
		}
		for name, data := range map[string][]byte{"Cargo.toml": cargoManifest, "Cargo.lock": cargoLock, "src/main.rs": []byte("fn main() {}\n")} {
			if err := writeFile(filepath.Join(oracleDir, "fetch", name), data); err != nil {
				return err
			}
		}
		args := []string{"fetch", "--locked", "--manifest-path", filepath.Join(oracleDir, "fetch/Cargo.toml")}
		if opts.offline {
			args = append(args, "--offline")
		}
		cmd := exec.Command(cargo, args...)
		cmd.Env = withEnv(os.Environ(), map[string]string{"CARGO_HOME": cargoHome, "RUSTC": rustc})
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
		archives, _ = filepath.Glob(filepath.Join(cargoHome, "registry/cache/*", crateName+"-"+crateVersion+".crate"))
	}
	if len(archives) != 1 {
		return fmt.Errorf("expected exactly one verified crate archive, got %d", len(archives))
	}
	archive, err := os.ReadFile(archives[0])
	if err != nil {
		return err
	}
	if digest(archive) != crateSHA256 {
		return fmt.Errorf("registry archive checksum mismatch")
	}
	files, err := readArchive(archive)
	if err != nil {
		return err
	}
	if len(files) != 14 {
		return fmt.Errorf("archive has %d files; expected all 14 pinned files", len(files))
	}
	originalDir := filepath.Join(oracleDir, "original")
	for path, data := range files {
		if err := writeFile(filepath.Join(originalDir, path), data); err != nil {
			return err
		}
	}
	rlibrary := filepath.Join(oracleDir, "librustc_literal_escaper.rlib")
	if err := rustCommand(rustc, "--edition=2021", "--crate-type", "rlib", "--crate-name", "rustc_literal_escaper", filepath.Join(originalDir, "src/lib.rs"), "-O", "-o", rlibrary); err != nil {
		return err
	}
	testBinary := filepath.Join(oracleDir, "upstream-tests")
	if err := rustCommand(rustc, "--edition=2021", "--test", "--extern", "rustc_literal_escaper="+rlibrary, filepath.Join(originalDir, "tests/test.rs"), "-o", testBinary); err != nil {
		return err
	}
	if output, err := exec.Command(testBinary, "--test-threads=1").CombinedOutput(); err != nil {
		return fmt.Errorf("unchanged upstream tests: %w\n%s", err, output)
	}
	if err := writeFile(filepath.Join(oracleDir, "oracle.rs"), oracleSource); err != nil {
		return err
	}
	oracleBinary := filepath.Join(oracleDir, "oracle")
	if err := rustCommand(rustc, "--edition=2021", "--extern", "rustc_literal_escaper="+rlibrary, filepath.Join(oracleDir, "oracle.rs"), "-O", "-o", oracleBinary); err != nil {
		return err
	}
	metadata, err := exec.Command(oracleBinary, "metadata").Output()
	if err != nil {
		return err
	}
	constructors, err := exec.Command(oracleBinary, "constructors").Output()
	if err != nil {
		return err
	}
	requests := corpus(files)
	var input bytes.Buffer
	for _, request := range requests {
		input.WriteByte(request.op)
		_ = binary.Write(&input, binary.LittleEndian, uint32(len(request.source)))
		input.WriteString(request.source)
	}
	cmd := exec.Command(oracleBinary)
	cmd.Stdin = &input
	cmd.Stderr = os.Stderr
	results, err := cmd.Output()
	if err != nil {
		return err
	}
	var golden bytes.Buffer
	golden.WriteString("FERRUGOE")
	_ = binary.Write(&golden, binary.LittleEndian, uint32(len(metadata)))
	golden.Write(metadata)
	_ = binary.Write(&golden, binary.LittleEndian, uint32(len(requests)))
	position := 0
	for _, request := range requests {
		if position+4 > len(results) {
			return fmt.Errorf("truncated Rust oracle response")
		}
		length := 4 + 13*int(binary.LittleEndian.Uint32(results[position:]))
		if position+length > len(results) {
			return fmt.Errorf("truncated Rust oracle events")
		}
		golden.WriteByte(request.op)
		_ = binary.Write(&golden, binary.LittleEndian, uint32(len(request.source)))
		golden.WriteString(request.source)
		_ = binary.Write(&golden, binary.LittleEndian, uint32(length))
		golden.Write(results[position : position+length])
		position += length
	}
	if position != len(results) {
		return fmt.Errorf("trailing Rust oracle events")
	}
	packageDir := filepath.Join(root, dependencyPath)
	compressed, err := oracleFixture(filepath.Join(packageDir, "src/testdata/oracle.bin.gz"), golden.Bytes(), opts.check)
	if err != nil {
		return err
	}
	scalarHash := &countingHash{Hash: sha256.New()}
	cmd = exec.Command(oracleBinary, "scalars")
	cmd.Stdout = scalarHash
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	var vcs struct {
		Git struct {
			SHA1 string `json:"sha1"`
		} `json:"git"`
	}
	if err := json.Unmarshal(files[".cargo_vcs_info.json"], &vcs); err != nil {
		return err
	}
	m := manifest{Format: 1, Name: crateName, Version: crateVersion, Registry: "https://crates.io", URL: "https://static.crates.io/crates/" + crateName + "/" + crateName + "-" + crateVersion + ".crate", CrateSHA256: crateSHA256, SourceRevision: vcs.Git.SHA1, License: "Apache-2.0 OR MIT", UpstreamTests: 9, UpstreamTestsPassed: true, PortedBenchmarkWorkloads: 27, OracleShimSHA256: digest(oracleSource), OracleEncoding: "FERRUGOE; metadata length u32 LE + metadata; case count u32 LE; each case: operation u8, UTF8 input length u32 LE + input, response length u32 LE + response; response: event count u32 LE, then {start u32 LE,end u32 LE,kind u8,value u32 LE}; kind0=EscapeError,1=char,2=byte,3=MixedUnit::Char,4=MixedUnit::HighByte,5=NonZero<char>; scalar trace uses these same responses for direct operations0..7, Unicode escapes0..4, continuations2..4 for every valid scalar", GoldenCases: len(requests), GoldenSHA256: digest(golden.Bytes()), GoldenGzipSHA256: digest(compressed), MetadataHex: hex.EncodeToString(metadata), ScalarValues: 1_112_064, ScalarOperations: 16, ScalarTraceSHA256: hex.EncodeToString(scalarHash.Sum(nil)), ScalarTraceBytes: scalarHash.count}
	outputs := map[string][]byte{"src/testdata/oracle.bin.gz": compressed}
	m.MixedConstructorsHex = hex.EncodeToString(constructors)
	m.MixedConstructorValues = []uint16{1, 65, 127, 128, 255}
	m.MixedConstructorEncoding = "for each byte: direct HighByte, normalized From<NonZero<u8>>, direct Char; each encoded as {kind u8,value u32 LE}; then equality flags direct==normalized,direct==Char,normalized==Char,direct==direct"
	for _, trait := range []string{"CheckRaw", "Unescape"} {
		if !bytes.Contains(files["src/lib.rs"], []byte("\ntrait "+trait+" {")) || bytes.Contains(files["src/lib.rs"], []byte("pub trait "+trait)) {
			return fmt.Errorf("trait %s is not private in the pinned source", trait)
		}
		m.PrivateTraitSpecializations = append(m.PrivateTraitSpecializations, trait)
	}
	var paths []string
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		data := files[path]
		entry := sourceFile{Path: path, SHA256: digest(data), Bytes: len(data), Lines: bytes.Count(data, []byte{'\n'})}
		switch path {
		case "src/lib.rs":
			entry.Role = "implementation"
			entry.Target = "src/lib.go"
			entry.Status = "partial"
			entry.Evidence = "Algorithms validated against unchanged Rust for all public modes/errors, complete scalar traces and MixedUnit constructors; full mutable Debug formatter and owned callback destruction contracts remain under completion review"
		case "tests/test.rs":
			entry.Role = "tests"
			entry.Target = "tests/test_test.go"
			entry.Status = "ported"
			entry.Evidence = "All 9 original Rust tests execute unchanged and all 9 mirrored Go tests are retained"
		case "benches/benches.rs":
			entry.Role = "benchmarks"
			entry.Target = "benches/benches_test.go"
			entry.Status = "ported"
			entry.Evidence = "All 27 original benchmark workloads have Go benchmarks and checked single-run tests"
		case "LICENSE-MIT", "LICENSE-APACHE":
			entry.Role = "license"
			entry.Target = path
			entry.Status = "retained"
			entry.Evidence = "Verbatim bytes from checksum-verified official crate archive"
			outputs[entry.Target] = data
		default:
			entry.Role = "upstream-metadata"
			entry.Target = "provenance/" + path
			entry.Status = "retained"
			entry.Evidence = "Verbatim upstream metadata retained; not counted as an implementation port"
			outputs[entry.Target] = data
		}
		target, ok := outputs[entry.Target]
		if !ok {
			target, err = os.ReadFile(filepath.Join(packageDir, entry.Target))
			if err != nil {
				return err
			}
		}
		entry.TargetSHA256 = digest(target)
		m.Files = append(m.Files, entry)
	}
	encoded, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	outputs["source-manifest.json"] = append(encoded, '\n')
	for path, data := range outputs {
		path = filepath.Join(packageDir, path)
		if opts.check {
			existing, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(existing, data) {
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
	fmt.Printf("%s rustc-literal-escaper0.0.8:14 archive files,9 unchanged Rust tests,27 ported benchmark workloads,%d differential cases\n", verb, len(requests))
	fmt.Printf("1112064 scalars x16 operations: %d trace bytes, SHA-256 %s; fixture %d bytes\n", m.ScalarTraceBytes, m.ScalarTraceSHA256, len(compressed))
	return nil
}

func corpus(files map[string][]byte) []request {
	var inputs []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			inputs = append(inputs, s)
		}
	}
	for _, path := range []string{"tests/test.rs", "benches/benches.rs"} {
		for _, literal := range rustStrings(string(files[path])) {
			add(literal)
		}
	}
	for _, s := range []string{"", "\\", "\r", "\n", "\t", "\v", "\f", "\x00", "\"", "'", "\\\r\n", "\\\n\r\n \t\u00a0x", "\\u{1234567z}", "\\u{1234567", "\\u{000000_}", "\\u{_}", "\\u{1__2_}", strings.Repeat("\\u{10ffff}", 1000), "\\\n" + strings.Repeat(" ", 10000) + "\n\u2003x", strings.Repeat("🦀\\xff\x00\\u{D800}", 100)} {
		add(s)
	}
	for value := 0; value <= 255; value++ {
		add(fmt.Sprintf("\\x%02x", value))
		add(fmt.Sprintf("\\x%02Xz", value))
	}
	for _, value := range []uint32{0, 1, 0x7f, 0x80, 0xd7ff, 0xd800, 0xdfff, 0xe000, 0x10ffff, 0x110000, 0xffffff} {
		add(fmt.Sprintf("\\u{%x}", value))
		add(fmt.Sprintf("\\u{%06X}", value))
		add(fmt.Sprintf("\\u{%x_}z", value))
	}
	for value := 0xd800; value <= 0xdfff; value++ {
		add(fmt.Sprintf("\\u{%x}", value))
	}
	alphabet := []rune{'\\', 'x', 'u', '{', '}', '_', '0', 'f', 'G', '\n', '\r', '\t', '"', '\''}
	add("")
	for _, a := range alphabet {
		add(string(a))
		for _, b := range alphabet {
			add(string([]rune{a, b}))
			for _, c := range alphabet {
				add(string([]rune{a, b, c}))
			}
		}
	}
	seed := uint64(0x936fce181f04a23d)
	random := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	alphabet = append(alphabet, 'a', 'n', 'r', 't', '1', '7', '8', '9', 'F', 'z', ' ', 0, '🦀', '\u00a0', '\u2003', '\u2028', '\uFFFD')
	for i := 0; i < 2048; i++ {
		length := int(random() % 65)
		var s strings.Builder
		for j := 0; j < length; j++ {
			s.WriteRune(alphabet[random()%uint64(len(alphabet))])
		}
		add(s.String())
	}
	var requests []request
	for _, source := range inputs {
		for op := byte(0); op < 16; op++ {
			requests = append(requests, request{op, source})
		}
	}
	return requests
}

func rustStrings(source string) []string {
	var result []string
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "//") {
			if n := strings.IndexByte(source[i:], '\n'); n >= 0 {
				i += n + 1
			} else {
				break
			}
			continue
		}
		if source[i] == '\'' {
			i++
			for i < len(source) {
				if source[i] == '\\' {
					i += 2
					continue
				}
				if source[i] == '\'' {
					i++
					break
				}
				_, n := utf8.DecodeRuneInString(source[i:])
				i += n
			}
			continue
		}
		start := i
		if source[i] == 'r' {
			j := i + 1
			for j < len(source) && source[j] == '#' {
				j++
			}
			if j < len(source) && source[j] == '"' {
				end := strings.Index(source[j+1:], "\""+source[i+1:j])
				if end < 0 {
					panic("unterminated Rust raw string")
				}
				result = append(result, source[j+1:j+1+end])
				hashes := j - i - 1
				i = j + 2 + end + hashes
				continue
			}
		}
		if source[i] != '"' {
			i++
			continue
		}
		i++
		var decoded strings.Builder
		for i < len(source) && source[i] != '"' {
			if source[i] != '\\' {
				c, n := utf8.DecodeRuneInString(source[i:])
				decoded.WriteRune(c)
				i += n
				continue
			}
			i++
			if i >= len(source) {
				panic("unterminated Rust string")
			}
			c := source[i]
			i++
			switch c {
			case 'n':
				decoded.WriteByte('\n')
			case 'r':
				decoded.WriteByte('\r')
			case 't':
				decoded.WriteByte('\t')
			case '0':
				decoded.WriteByte(0)
			case '\\', '"', '\'':
				decoded.WriteByte(c)
			case 'x':
				value, err := strconv.ParseUint(source[i:i+2], 16, 8)
				if err != nil {
					panic(err)
				}
				decoded.WriteByte(byte(value))
				i += 2
			case 'u':
				end := strings.IndexByte(source[i:], '}')
				if end < 0 {
					panic("unclosed Unicode string escape")
				}
				value, err := strconv.ParseUint(strings.ReplaceAll(source[i+1:i+end], "_", ""), 16, 32)
				if err != nil {
					panic(err)
				}
				decoded.WriteRune(rune(value))
				i += end + 1
			case '\n':
				for i < len(source) && strings.ContainsRune(" \t\r\n", rune(source[i])) {
					i++
				}
			default:
				panic(fmt.Sprintf("invalid Rust source escape at %d", start))
			}
		}
		if i == len(source) {
			panic("unterminated Rust quoted string")
		}
		i++
		result = append(result, decoded.String())
	}
	return result
}

func readArchive(archive []byte) (map[string][]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	files := map[string][]byte{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unexpected crate member type")
		}
		prefix := crateName + "-" + crateVersion + "/"
		if !strings.HasPrefix(header.Name, prefix) {
			return nil, fmt.Errorf("unexpected crate member prefix")
		}
		path := strings.TrimPrefix(header.Name, prefix)
		if filepath.Clean(path) != path || strings.HasPrefix(path, "../") {
			return nil, fmt.Errorf("unsafe crate member")
		}
		data, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		if _, ok := files[path]; ok {
			return nil, fmt.Errorf("duplicate crate member")
		}
		files[path] = data
	}
	return files, nil
}

type countingHash struct {
	hash.Hash
	count int64
}

func (h *countingHash) Write(data []byte) (int, error) {
	n, err := h.Hash.Write(data)
	h.count += int64(n)
	return n, err
}
func rustCommand(rustc string, args ...string) error {
	cmd := exec.Command(rustc, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func absolute(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
func executable(root, name, fallback string) (string, error) {
	if name == "" {
		name = filepath.Join(root, fallback)
		if _, err := os.Stat(name); err != nil {
			name = filepath.Base(fallback)
		}
	}
	if !strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	return absolute(root, name), nil
}
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
func withEnv(env []string, overrides map[string]string) []string {
	var result []string
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if _, ok := overrides[key]; !ok {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

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
			return nil, fmt.Errorf("compressed fixture differs from exact Rust oracle: %s", path)
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
