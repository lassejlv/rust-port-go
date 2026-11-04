// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

const (
	oracleRevision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
	oracleLibBlob  = "0aa57e75889fa2d81deb1ec8bf5480b1c930fff1"
	oracleTestBlob = "a7357ba38c8e4484da814ef3c22bf33b75bdbeb2"
	oracleSeed     = uint64(0x2d20b42b883dcfc0)
)

type oracleCase struct {
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Argument  uint32 `json:"argument"`
	Input     string `json:"input"`
	Expected  string `json:"expected"`
}

type oracleCorpus struct {
	Schema      int          `json:"schema"`
	Revision    string       `json:"revision"`
	SourceBlob  string       `json:"source_blob"`
	GeneratedBy string       `json:"generated_by"`
	Cases       []oracleCase `json:"cases"`
}

func readOracleCorpus(t *testing.T) oracleCorpus {
	t.Helper()
	data, err := os.ReadFile("testdata/oracle_corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus oracleCorpus
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Schema != 1 || corpus.Revision != oracleRevision || corpus.SourceBlob != oracleLibBlob || len(corpus.Cases) == 0 {
		t.Fatal("oracle corpus provenance or schema mismatch")
	}
	return corpus
}

// This test requires only Go and committed upstream-generated golden results.
// The bootstrap compiler and its accepted language are not involved.
func TestLexerOracleCorpus(t *testing.T) {
	if os.Getenv("FERRUGO_LEXER_UPDATE") != "" && os.Getenv("FERRUGO_LEXER_ORACLE") != "1" {
		t.Fatal("FERRUGO_LEXER_UPDATE requires FERRUGO_LEXER_ORACLE=1")
	}
	corpus := readOracleCorpus(t)
	names := make(map[string]bool)
	for _, tc := range corpus.Cases {
		if names[tc.Name] || tc.Name == "" {
			t.Fatalf("duplicate or empty corpus name %q", tc.Name)
		}
		names[tc.Name] = true
		t.Run(tc.Name, func(t *testing.T) {
			if !utf8.ValidString(tc.Input) {
				t.Fatal("oracle input violates upstream &str contract")
			}
			if got := evaluateOracleCase(tc); got != tc.Expected {
				t.Fatalf("%s", oracleDifference(tc, got, tc.Expected))
			}
		})
	}
}

func oracleState(cursor *Cursor) string {
	return fmt.Sprintf("%d,%d,%d,%t,%d,%d,%x", cursor.First(), cursor.Second(), cursor.Third(),
		cursor.IsEOF(), cursor.PosWithinToken(), cursor.Prev(), cursor.AsStr())
}

func oracleCursor(input string, argument uint32) string {
	cursor := NewCursor(input, FrontmatterNo)
	out := []string{"initial:" + oracleState(cursor)}
	var actions string
	switch argument {
	case 0:
		actions = strings.Repeat("b", utf8.RuneCountInString(input)+2)
	case 1:
		actions = "briwnqlabriwnqlabe"
	case 2:
		actions = "nrbbiwqalnrbbiwqale"
	default:
		panic("invalid oracle cursor script")
	}
	for _, action := range actions {
		var value string
		switch action {
		case 'b':
			value = "none"
			if r, ok := cursor.Bump(); ok {
				value = strconv.Itoa(int(r))
			}
		case 'r':
			cursor.ResetPosWithinToken()
			value = "reset"
		case 'n':
			_, size := utf8.DecodeRuneInString(cursor.AsStr())
			cursor.BumpBytes(size)
			value = strconv.Itoa(size)
		case 'i':
			cursor.EatWhile(IsIDContinue)
			value = "ident"
		case 'w':
			cursor.EatWhile(IsWhitespace)
			value = "whitespace"
		case 'q':
			cursor.EatUntil('"')
			value = "quote"
		case 'l':
			cursor.EatUntil('\n')
			value = "newline"
		case 'a':
			value = cursor.AdvanceToken().RustDebug()
		case 'e':
			cursor.EatWhile(func(rune) bool { return true })
			value = "end"
		}
		out = append(out, fmt.Sprintf("%c:%s:%s", action, value, oracleState(cursor)))
	}
	return strings.Join(out, "\n")
}

func evaluateOracleCase(tc oracleCase) string {
	switch tc.Operation {
	case "tokens":
		frontmatter := FrontmatterNo
		if tc.Argument == 1 {
			frontmatter = FrontmatterYes
		} else if tc.Argument != 0 {
			panic("invalid oracle frontmatter argument")
		}
		tokens := Tokenize(tc.Input, frontmatter)
		out := make([]string, len(tokens))
		for i, token := range tokens {
			out[i] = token.RustDebug()
		}
		return strings.Join(out, "\n")
	case "shebang":
		if n, ok := StripShebang(tc.Input); ok {
			return strconv.Itoa(n)
		}
		return "none"
	case "raw":
		err := ValidateRawStr(tc.Input, tc.Argument)
		if err == nil {
			return "ok"
		}
		switch err.Tag {
		case InvalidStarter:
			return fmt.Sprintf("invalid_starter:%d", err.BadChar)
		case NoTerminator:
			offset := "none"
			if err.HasPossibleTerminatorOffset {
				offset = strconv.FormatUint(uint64(err.PossibleTerminatorOffset), 10)
			}
			return fmt.Sprintf("no_terminator:%d:%d:%s", err.Expected, err.Found, offset)
		case TooManyDelimiters:
			return fmt.Sprintf("too_many:%d", err.Found)
		}
	case "guarded":
		cursor := NewCursor(tc.Input, FrontmatterNo)
		guard, ok := cursor.GuardedDoubleQuotedString()
		value := "none"
		if ok {
			value = fmt.Sprintf("some:%d:%t:%d", guard.NHashes, guard.Terminated, guard.TokenLen)
		}
		return value + ":" + oracleState(cursor)
	case "properties":
		out := []string{fmt.Sprintf("ident:%t", IsIdent(tc.Input))}
		for _, r := range tc.Input {
			out = append(out, fmt.Sprintf("%d:%t:%t:%t:%t", r, IsWhitespace(r), IsHorizontalWhitespace(r), IsIDStart(r), IsIDContinue(r)))
		}
		return strings.Join(out, "\n")
	case "cursor":
		return oracleCursor(tc.Input, tc.Argument)
	}
	panic("unknown oracle operation: " + tc.Operation)
}

func oracleDifference(tc oracleCase, got, want string) string {
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			return fmt.Sprintf("%s/%s argument=%d input=%q: output line %d\nGo: %s\nRust: %s",
				tc.Name, tc.Operation, tc.Argument, truncateOracle(tc.Input, 512), i+1,
				truncateOracle(gotLines[i], 1500), truncateOracle(wantLines[i], 1500))
		}
	}
	return fmt.Sprintf("%s/%s argument=%d input=%q: Go has %d output lines; Rust has %d",
		tc.Name, tc.Operation, tc.Argument, truncateOracle(tc.Input, 512), len(gotLines), len(wantLines))
}

func truncateOracle(s string, limit int) string {
	if len(s) > limit {
		return s[:limit] + "..."
	}
	return s
}

type originalLexerOracle struct {
	cmd    *exec.Cmd
	input  io.WriteCloser
	writer *bufio.Writer
	reader *bufio.Reader
	stderr bytes.Buffer
}

func oracleRoot(t *testing.T) string {
	t.Helper()
	path, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if data, err := os.ReadFile(filepath.Join(path, "go.mod")); err == nil && strings.HasPrefix(string(data), "module github.com/lassejlv/rust-port-go\n") {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			t.Fatal("cannot locate Ferrugo repository go.mod")
		}
		path = parent
	}
}

func oraclePath(t *testing.T, variable, fallback string) string {
	t.Helper()
	if path, exists := os.LookupEnv(variable); exists {
		if path == "" {
			t.Fatalf("%s must be an explicit nonempty path", variable)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(oracleRoot(t), path)
		}
		return filepath.Clean(path)
	}
	return fallback
}

func oracleExecutable(t *testing.T, variable, fallback string) string {
	t.Helper()
	name, exists := os.LookupEnv(variable)
	if !exists {
		name = fallback
	}
	if name == "" {
		t.Fatalf("%s must be an explicit nonempty path or executable name", variable)
	}
	if !filepath.IsAbs(name) && strings.ContainsAny(name, `/\`) {
		name = filepath.Join(oracleRoot(t), name)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s executable %q is unavailable: %v", variable, name, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}

func TestOracleExecutableAndSourceSelection(t *testing.T) {
	// Go runs this package's tests below the repository root. A bare tool must
	// still resolve through PATH, as it does in the upstream-ports CI job.
	directory := t.TempDir()
	name := "ferrugo-oracle-path-probe"
	probe := filepath.Join(directory, name)
	if err := os.WriteFile(probe, nil, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	t.Setenv("FERRUGO_CARGO", name)
	if got := oracleExecutable(t, "FERRUGO_CARGO", "/unused"); got != probe {
		t.Fatalf("bare Cargo selection = %s; want %s", got, probe)
	}
	t.Setenv("FERRUGO_RUST_SOURCE", ".toolchain/upstream-rust")
	want := filepath.Join(oracleRoot(t), ".toolchain/upstream-rust")
	if got := oraclePath(t, "FERRUGO_RUST_SOURCE", "/unused"); got != want {
		t.Fatalf("relative source selection = %s; want %s", got, want)
	}
}

func oracleCommandEnv(rustc, cargoHome string) []string {
	drop := map[string]bool{
		"RUSTC": true, "CARGO_HOME": true, "CARGO_TARGET_DIR": true,
		"RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true,
		"RUSTFLAGS": true, "CARGO_ENCODED_RUSTFLAGS": true,
		"RUSTDOCFLAGS": true, "CARGO_ENCODED_RUSTDOCFLAGS": true,
		"UPDATE_EXPECT": true,
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if !drop[key] {
			env = append(env, value)
		}
	}
	return append(env, "RUSTC="+rustc, "CARGO_HOME="+cargoHome)
}

func verifyOracleSource(t *testing.T, root, relative, blob string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hydrate pinned upstream source %s: %v", path, err)
	}
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d%c", len(data), byte(0))
	hash.Write(data)
	if got := hex.EncodeToString(hash.Sum(nil)); got != blob {
		t.Fatalf("upstream source hash mismatch for %s: got %s, require %s at %s", path, got, blob, oracleRevision)
	}
	return path
}

func startOriginalLexerOracle(t *testing.T, sourceRoot string) (*originalLexerOracle, string) {
	t.Helper()
	root := oracleRoot(t)
	cargo := oracleExecutable(t, "FERRUGO_CARGO", filepath.Join(root, ".toolchain/rust/bin/cargo"))
	rustc := oracleExecutable(t, "FERRUGO_RUSTC", filepath.Join(root, ".toolchain/rust/bin/rustc"))
	cargoHome := oraclePath(t, "CARGO_HOME", filepath.Join(root, ".toolchain/port-cargo-home"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	version := func(binary, prefix string) string {
		command := exec.CommandContext(ctx, binary, "--version")
		out, err := command.CombinedOutput()
		if err != nil || !strings.HasPrefix(string(out), prefix+" ") {
			t.Fatalf("oracle requires original %s executable %s: %v: %s", prefix, binary, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	t.Log(version(cargo, "cargo"))
	rustVersion := version(rustc, "rustc")
	t.Log(rustVersion)
	head := exec.CommandContext(ctx, "git", "-C", sourceRoot, "rev-parse", "HEAD")
	out, err := head.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != oracleRevision {
		t.Fatalf("FERRUGO_RUST_SOURCE must be a checkout at %s: %v: %s", oracleRevision, err, out)
	}
	lib := verifyOracleSource(t, sourceRoot, "compiler/rustc_lexer/src/lib.rs", oracleLibBlob)
	verifyOracleSource(t, sourceRoot, "compiler/rustc_lexer/src/tests.rs", oracleTestBlob)
	directory := t.TempDir()
	toolDir := filepath.Join(root, "tools/port-lexer-oracle")
	for _, file := range []string{"Cargo.toml", "Cargo.lock", "main.rs"} {
		data, err := os.ReadFile(filepath.Join(toolDir, file))
		if err != nil {
			t.Fatal(err)
		}
		if file == "main.rs" {
			hashes := "#"
			for strings.Contains(lib, "\""+hashes) {
				hashes += "#"
			}
			data = append([]byte(fmt.Sprintf("#[path = r%s\"%s\"%s]\nmod lexer;\n", hashes, lib, hashes)), data...)
		}
		if err := os.WriteFile(filepath.Join(directory, file), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := oracleCommandEnv(rustc, cargoHome)
	for _, verb := range []string{"test", "build"} {
		command := exec.CommandContext(ctx, cargo, verb, "--locked", "--offline", "--manifest-path", filepath.Join(directory, "Cargo.toml"), "--bin", "ferrugo-rustc-lexer-oracle")
		command.Env = env
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("original lexer %s failed (fetch dependencies with the command in tools/port-lexer-oracle/README.md): %v\n%s", verb, err, out)
		}
		if verb == "test" {
			t.Log(strings.TrimSpace(string(out)))
		}
	}
	command := exec.CommandContext(ctx, filepath.Join(directory, "target/debug/ferrugo-rustc-lexer-oracle"))
	oracle := &originalLexerOracle{cmd: command}
	command.Stderr = &oracle.stderr
	oracle.input, err = command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	oracle.writer, oracle.reader = bufio.NewWriter(oracle.input), bufio.NewReader(stdout)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		oracle.input.Close()
		if err := command.Wait(); err != nil && !t.Failed() {
			t.Errorf("original lexer oracle exited: %v: %s", err, oracle.stderr.String())
		}
	})
	return oracle, rustVersion
}

func (oracle *originalLexerOracle) evaluate(t *testing.T, tc oracleCase) string {
	t.Helper()
	if _, err := fmt.Fprintf(oracle.writer, "%s\t%d\t%x\n", tc.Operation, tc.Argument, tc.Input); err != nil {
		t.Fatal(err)
	}
	if err := oracle.writer.Flush(); err != nil {
		t.Fatal(err)
	}
	line, err := oracle.reader.ReadString('\n')
	if err != nil {
		t.Fatalf("original lexer oracle request %s/%s failed: %v", tc.Name, tc.Operation, err)
	}
	response, err := hex.DecodeString(strings.TrimSuffix(line, "\n"))
	if err != nil {
		t.Fatalf("invalid original oracle response: %v", err)
	}
	return string(response)
}

// Opt-in means the original source is actually compiled. No installed lexer,
// bootstrap token stream, or Go implementation can substitute for this oracle.
func TestOriginalLexerOracle(t *testing.T) {
	if value, set := os.LookupEnv("FERRUGO_LEXER_ORACLE"); !set {
		t.Skip("set FERRUGO_LEXER_ORACLE=1 to build the pinned upstream lexer")
	} else if value != "1" {
		t.Fatal("FERRUGO_LEXER_ORACLE must be 1 when set")
	}
	root := oracleRoot(t)
	sourceRoot := oraclePath(t, "FERRUGO_RUST_SOURCE", filepath.Join(root, ".toolchain/upstream-rust"))
	oracle, version := startOriginalLexerOracle(t, sourceRoot)
	corpus := readOracleCorpus(t)
	update := os.Getenv("FERRUGO_LEXER_UPDATE") == "1"
	compare := func(tc oracleCase) string {
		want := oracle.evaluate(t, tc)
		if got := evaluateOracleCase(tc); got != want {
			t.Fatalf("%s", oracleDifference(tc, got, want))
		}
		return want
	}
	for i, tc := range corpus.Cases {
		want := compare(tc)
		if update {
			corpus.Cases[i].Expected = want
		} else if tc.Expected != want {
			t.Fatalf("committed golden differs from original lexer: %s", oracleDifference(tc, tc.Expected, want))
		}
	}
	if update {
		corpus.GeneratedBy = version
		data, err := json.MarshalIndent(corpus, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile("testdata/oracle_corpus.json", append(data, '\n'), 0644); err != nil {
			t.Fatal(err)
		}
	}
	random := oracleSeed
	next := func(n int) int {
		random ^= random << 13
		random ^= random >> 7
		random ^= random << 17
		return int(random % uint64(n))
	}
	fragments := []string{"a", "r", "b", "c", "k", "_", "0", "1", "9", "x", "e", "E", "f64", "u128", "#", "\"", "'", "\\", "/", "*", "-", "+", ".", "!", "=", "[", "]", "(", ")", "{", "}", " ", "\t", "\r", "\n", "\x00", "\u0085", "\u200e", "\u2028", "\u2029", "é", "中", "λ", "\u0301", "🦀", "😀", "\u200d", "\ufe0f", "\U0001e6c0", "\U00010ffff"}
	for i := 0; i < 6000; i++ {
		var input strings.Builder
		for n := next(96); n > 0; n-- {
			if next(8) == 0 {
				r := rune(next(0x110000))
				if r < 0xd800 || r > 0xdfff {
					input.WriteRune(r)
				}
			} else {
				input.WriteString(fragments[next(len(fragments))])
			}
		}
		source := input.String()
		compare(oracleCase{Name: fmt.Sprintf("random/%04d", i), Operation: "tokens", Argument: uint32(i % 2), Input: source})
		if i%16 == 0 {
			compare(oracleCase{Name: fmt.Sprintf("random-shebang/%04d", i), Operation: "shebang", Input: "#!" + source})
		}
		if i%32 == 0 {
			compare(oracleCase{Name: fmt.Sprintf("random-cursor/%04d", i), Operation: "cursor", Argument: uint32(i % 3), Input: source})
			compare(oracleCase{Name: fmt.Sprintf("random-properties/%04d", i), Operation: "properties", Input: source})
		}
	}
	for i := 0; i < 1024; i++ {
		hashes := strings.Repeat("#", next(300))
		closer := strings.Repeat("#", next(300))
		prefix := []string{"r", "br", "cr"}[i%3]
		contents := fragments[next(len(fragments))] + "\"" + closer + fragments[next(len(fragments))]
		source := prefix + hashes + "\"" + contents
		if i%3 == 0 {
			source += "\"" + hashes + "suffix"
		}
		compare(oracleCase{Name: fmt.Sprintf("random-raw/%04d", i), Operation: "raw", Argument: uint32(len(prefix)), Input: source})
		compare(oracleCase{Name: fmt.Sprintf("random-guarded/%04d", i), Operation: "guarded", Input: hashes + "\"" + contents})
		leading := []string{"", " ", "\n", "\r\n", "\u200e", "\u2028\t"}[i%6]
		fence := strings.Repeat("-", 3+next(6))
		info := []string{"cargo", "cargo.toml", "cargo-test", "", "!!!", "é", "a b"}[i%7]
		closing := []string{fence, "---", "\t---", "fn main(){}", "use x;", "#![a]", "//! doc"}[i%7]
		compare(oracleCase{Name: fmt.Sprintf("random-frontmatter/%04d", i), Operation: "tokens", Argument: 1,
			Input: leading + fence + info + "\n" + contents + "\n" + closing + "\nfn main(){}"})
	}
	counts, invalidUTF8, totalBytes := make(map[string]int), 0, int64(0)
	for _, tree := range []struct{ name, path string }{
		{"rust", sourceRoot}, {"cargo", filepath.Join(root, ".toolchain/upstream-cargo")}, {"repository", root},
	} {
		err := filepath.WalkDir(tree.path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if path != tree.path && (entry.Name() == ".git" || entry.Name() == ".toolchain" || entry.Name() == "target" || entry.Name() == "bin") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".rs" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !utf8.Valid(data) {
				invalidUTF8++ // Upstream takes &str, not arbitrary source bytes.
				return nil
			}
			relative, _ := filepath.Rel(tree.path, path)
			name := tree.name + "/" + filepath.ToSlash(relative)
			source := string(data)
			compare(oracleCase{Name: name, Operation: "shebang", Input: source})
			compare(oracleCase{Name: name + "/slice", Operation: "tokens", Input: source})
			if n, ok := StripShebang(source); ok {
				source = source[n:]
			}
			compare(oracleCase{Name: name + "/document", Operation: "tokens", Argument: 1, Input: source})
			counts[tree.name]++
			totalBytes += int64(len(data))
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s sources: %v", tree.name, err)
		}
	}
	t.Logf("original lexer matched: %d committed cases, 6000 seeded UTF-8 cases, 1024 each raw/guarded/frontmatter cases; %v hydrated .rs files (%d bytes); skipped %d non-UTF-8 files; seed=%#x",
		len(corpus.Cases), counts, totalBytes, invalidUTF8, oracleSeed)
}
