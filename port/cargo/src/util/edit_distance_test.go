package util

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// Port of upstream test_edit_distance, including its exclusive char::MAX bound.
func TestEditDistanceUpstream(t *testing.T) {
	for c := rune(0); c < utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		value := string(c)
		if distance, ok := EditDistance(value, value, ^uint(0)); !ok || distance != 0 {
			t.Fatalf("U+%04X: (%d,%v), want Some(0)", c, distance, ok)
		}
	}
	a := "\nMäry häd ä little lämb\n\nLittle lämb\n"
	b := "\nMary häd ä little lämb\n\nLittle lämb\n"
	c := "Mary häd ä little lämb\n\nLittle lämb\n"
	for _, test := range []struct {
		a, b string
		want uint
	}{{a, b, 1}, {b, a, 1}, {a, c, 2}, {c, a, 2}, {b, c, 1}, {c, b, 1}} {
		distance, ok := EditDistance(test.a, test.b, ^uint(0))
		if !ok || distance != test.want {
			t.Fatalf("EditDistance(%q,%q)=(%d,%v), want %d", test.a, test.b, distance, ok, test.want)
		}
	}
}

func editDistanceFixture(t *testing.T, name string) ([]byte, []byte) {
	t.Helper()
	compressed, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return raw, compressed
}
func editDistanceU32(t *testing.T, reader *bytes.Reader) uint32 {
	t.Helper()
	var value uint32
	if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func editDistanceU64(t *testing.T, reader *bytes.Reader) uint64 {
	t.Helper()
	var value uint64
	if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func editDistanceString(t *testing.T, reader *bytes.Reader) string {
	t.Helper()
	length := editDistanceU32(t, reader)
	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(data) {
		t.Fatal("oracle string is not UTF-8")
	}
	return string(data)
}
func editDistanceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestEditDistancePinnedUnicodeExhaustive(t *testing.T) {
	raw, _ := editDistanceFixture(t, "edit_distance_unicode.bin.gz")
	reader := bytes.NewReader(raw)
	header := make([]byte, 9)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(header, []byte{'C', 'E', 'D', 'U', '1', 0, 17, 0, 0}) {
		t.Fatalf("bad Unicode fixture header %v", header)
	}
	flags := make([]byte, 0x110000)
	if _, err := io.ReadFull(reader, flags); err != nil {
		t.Fatal(err)
	}
	exceptions := map[rune][]rune{}
	count := editDistanceU32(t, reader)
	for range count {
		c := rune(editDistanceU32(t, reader))
		length, err := reader.ReadByte()
		if err != nil {
			t.Fatal(err)
		}
		lower := make([]rune, length)
		for i := range lower {
			lower[i] = rune(editDistanceU32(t, reader))
		}
		exceptions[c] = lower
	}
	if reader.Len() != 0 {
		t.Fatal("trailing Unicode fixture bytes")
	}
	scalars := 0
	for c := rune(0); c <= utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		scalars++
		expected := exceptions[c]
		if expected == nil {
			expected = []rune{c}
		}
		single := string(c)
		if got := editDistanceLower(single); !slices.Equal(got, expected) {
			t.Fatalf("lower U+%04X=%U, want %U", c, got, expected)
		}
		cased := flags[c]&1 != 0
		ignorable := flags[c]&2 != 0
		if editDistanceInRanges(c, editDistanceSigmaCased) != cased || editDistanceInRanges(c, editDistanceCaseIgnorable) != ignorable {
			t.Fatalf("context property mismatch U+%04X", c)
		}
		// The fixture's properties were obtained from these exact Rust context
		// probes. Test the full Go lowering, including neighboring scalar expansion.
		target := 'ς'
		if cased {
			target = 'σ'
		}
		following := expected
		if c == 'Σ' {
			following = []rune{'ς'}
		}
		after := append([]rune{'a', target}, following...)
		if got := editDistanceLower("AΣ" + single); !slices.Equal(got, after) {
			t.Fatalf("AΣ U+%04X: %U, want %U", c, got, after)
		}
		target = 'σ'
		if cased || ignorable {
			target = 'ς'
		}
		before := append(append([]rune{'a'}, expected...), target)
		if got := editDistanceLower("A" + single + "Σ"); !slices.Equal(got, before) {
			t.Fatalf("A U+%04X Σ: %U, want %U", c, got, before)
		}
	}
	if scalars != 1112064 {
		t.Fatalf("checked %d scalars", scalars)
	}
}

func TestEditDistanceRustOracleCases(t *testing.T) {
	raw, _ := editDistanceFixture(t, "edit_distance_cases.bin.gz")
	reader := bytes.NewReader(raw)
	header := make([]byte, 6)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatal(err)
	}
	if string(header) != "CEDC1\x00" {
		t.Fatal("bad oracle case header")
	}
	distances := editDistanceU32(t, reader)
	for index := range distances {
		a, b := editDistanceString(t, reader), editDistanceString(t, reader)
		limit64 := editDistanceU64(t, reader)
		limit := uint(limit64)
		if uint64(limit) != limit64 {
			limit = ^uint(0)
		} // usize::MAX reference on 32-bit Go.
		present, err := reader.ReadByte()
		if err != nil {
			t.Fatal(err)
		}
		expected := uint(0)
		if present != 0 {
			expected = uint(editDistanceU32(t, reader))
		}
		distance, found := EditDistance(a, b, limit)
		if found != (present != 0) || (found && distance != expected) {
			t.Fatalf("oracle distance case %d: %q/%q limit %d=(%d,%v), want (%d,%v)", index, a, b, limit, distance, found, expected, present != 0)
		}
	}
	suggestions := editDistanceU32(t, reader)
	type candidate struct {
		index int
		name  string
	}
	key := func(value *candidate) string { return value.name }
	for index := range suggestions {
		choice, kind := editDistanceString(t, reader), editDistanceString(t, reader)
		count := editDistanceU32(t, reader)
		candidates := make([]candidate, count)
		for i := range candidates {
			candidates[i] = candidate{i, editDistanceString(t, reader)}
		}
		wanted := int32(editDistanceU32(t, reader))
		message := editDistanceString(t, reader)
		best, found := Closest(choice, slices.Values(candidates), key)
		if found != (wanted >= 0) || (found && best.index != int(wanted)) {
			t.Fatalf("oracle suggestion %d: got %+v/%v, want index %d", index, best, found, wanted)
		}
		if got := ClosestMsg(choice, slices.Values(candidates), key, kind); got != message {
			t.Fatalf("oracle message %d: %q, want %q", index, got, message)
		}
	}
	if reader.Len() != 0 {
		t.Fatal("trailing case fixture bytes")
	}
	t.Logf("matched %d distance cases and %d suggestion/message cases", distances, suggestions)
}

func TestEditDistanceOracleIntegrity(t *testing.T) {
	data, err := os.ReadFile("edit_distance_unicode_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		CargoRevision  string `json:"cargo_revision"`
		SourceSHA256   string `json:"source_sha256"`
		SourceGitBlob  string `json:"source_git_blob"`
		UnicodeOracle  string `json:"unicode_oracle_sha256"`
		UnicodeFixture string `json:"unicode_fixture_sha256"`
		CaseOracle     string `json:"case_oracle_sha256"`
		CaseFixture    string `json:"case_fixture_sha256"`
		Tables         string `json:"tables_sha256"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CargoRevision != "24de4721844c7a4d839072bab6fb66d41c9bc1f4" || manifest.SourceSHA256 != "073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407" || manifest.SourceGitBlob != "e67259956fc26a53b1be30efb371161249dfff8a" {
		t.Fatal("upstream source pin changed")
	}
	for _, fixture := range []struct{ name, raw, gz string }{{"edit_distance_unicode.bin.gz", manifest.UnicodeOracle, manifest.UnicodeFixture}, {"edit_distance_cases.bin.gz", manifest.CaseOracle, manifest.CaseFixture}} {
		raw, gz := editDistanceFixture(t, fixture.name)
		if editDistanceHash(raw) != fixture.raw || editDistanceHash(gz) != fixture.gz {
			t.Fatalf("fixture checksum mismatch: %s", fixture.name)
		}
	}
	tables, err := os.ReadFile("edit_distance_unicode.go")
	if err != nil {
		t.Fatal(err)
	}
	if editDistanceHash(tables) != manifest.Tables {
		t.Fatal("generated Unicode table checksum mismatch")
	}
}

func TestEditDistanceRestrictedDamerauAndLimits(t *testing.T) {
	for _, test := range []struct {
		a, b            string
		limit, distance uint
		found           bool
	}{
		{"CA", "ABC", 2, 0, false}, {"CA", "ABC", 3, 3, true}, // OSA differs from unrestricted Damerau.
		{"ab", "ba", 1, 1, true}, {"ab", "ba", 0, 0, false},
		{"CHECK", "check", 0, 0, true}, {"İ", "i\u0307", 0, 0, true},
		{"prefixXsuffix", "prefixYsuffix", 1, 1, true}, {"", "abcd", 3, 0, false},
		{"\U00001c89", "\U00001c8a", 0, 0, true}, // Unicode 16; absent from Go 1.23's Unicode tables.
	} {
		got, found := EditDistance(test.a, test.b, test.limit)
		if got != test.distance || found != test.found {
			t.Fatalf("%q/%q limit %d=(%d,%v), want (%d,%v)", test.a, test.b, test.limit, got, found, test.distance, test.found)
		}
	}
}

func TestEditDistanceClosestTieAndKeyEvaluation(t *testing.T) {
	values := []string{"a", "b"}
	calls := []string{}
	key := func(value *string) string { calls = append(calls, *value); return *value }
	value, found := Closest("x", slices.Values(values), key)
	if !found || value != "a" || !slices.Equal(calls, []string{"a", "b"}) {
		t.Fatalf("first-min tie/key calls: %q %v %q", value, found, calls)
	}
	calls = nil
	message := ClosestMsg("x", slices.Values(values), key, "command")
	if message != "\n\nhelp: a command with a similar name exists: `a`" || !slices.Equal(calls, []string{"a", "b", "a"}) {
		t.Fatalf("message/key calls: %q %q", message, calls)
	}
	calls = nil
	if got := ClosestMsg("unrelated-long-name", slices.Values(values), key, "command"); got != "" || len(calls) != 2 {
		t.Fatalf("missing candidate/key calls: %q %q", got, calls)
	}
	empty, found := Closest("", slices.Values([]string{""}), func(value *string) string { return *value })
	if !found || empty != "" {
		t.Fatal("zero-valued candidate lost Option presence")
	}
}

func TestEditDistanceRejectsInvalidUTF8(t *testing.T) {
	for _, test := range []struct {
		name string
		call func()
	}{
		{"left", func() { EditDistance("\xff", "ok", 3) }},
		{"right", func() { EditDistance("ok", "\xc0\xaf", 3) }},
		{"empty iterator choice", func() { Closest("\xff", slices.Values([]string{}), func(value *string) string { return *value }) }},
		{"key", func() { Closest("x", slices.Values([]string{"\xff"}), func(value *string) string { return *value }) }},
		{"kind", func() {
			ClosestMsg("x", slices.Values([]string{}), func(value *string) string { return *value }, "\xff")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				value := recover()
				if value == nil || !strings.Contains(value.(string), "UTF-8") {
					t.Fatalf("invalid UTF-8 did not panic explicitly: %v", value)
				}
			}()
			test.call()
		})
	}
}

func FuzzEditDistance(f *testing.F) {
	for _, pair := range [][2]string{{"ab", "ba"}, {"ΟΣ", "ος"}, {"İ", "i\u0307"}, {"CA", "ABC"}, {"\U00001c89", "\U00001c8a"}, {"", "🦀"}} {
		f.Add(pair[0], pair[1], uint8(3))
	}
	f.Fuzz(func(t *testing.T, a, b string, limitByte uint8) {
		if !utf8.ValidString(a) || !utf8.ValidString(b) || len(a) > 256 || len(b) > 256 {
			t.Skip()
		}
		limit := uint(limitByte)
		distance, found := EditDistance(a, b, limit)
		reverse, reverseFound := EditDistance(b, a, limit)
		if distance != reverse || found != reverseFound {
			t.Fatal("distance lost symmetry")
		}
		unlimited, ok := EditDistance(a, b, ^uint(0))
		if !ok || found != (unlimited <= limit) || (found && distance != unlimited) {
			t.Fatal("limit changed the computed distance")
		}
	})
}

// FERRUGO_RUSTC opts in to executing the original Rust source. Fixed committed
// oracle fixtures keep ordinary Go tests independent of Rust and source checkouts.
func TestEditDistanceLiveRustOracle(t *testing.T) {
	requested, enabled := os.LookupEnv("FERRUGO_RUSTC")
	if !enabled {
		t.Skip("set FERRUGO_RUSTC to run the pinned original Rust source")
	}
	if strings.TrimSpace(requested) == "" {
		t.Fatal("FERRUGO_RUSTC is set but empty")
	}
	compiler, err := exec.LookPath(requested)
	if err != nil {
		t.Fatalf("FERRUGO_RUSTC unavailable: %v", err)
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		t.Fatalf("read original Rust version: %v", err)
	}
	if strings.TrimSpace(string(version)) != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" {
		t.Fatalf("live Unicode oracle requires pinned Rust 1.99.1, found %s", version)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	upstream := os.Getenv("FERRUGO_CARGO_SOURCE")
	if upstream == "" {
		upstream = filepath.Join(root, ".toolchain", "upstream-cargo")
	} else if !filepath.IsAbs(upstream) {
		upstream = filepath.Join(root, upstream)
	}
	source := filepath.Join(upstream, "src", "util", "edit_distance.rs")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read pinned Cargo source: %v", err)
	}
	if editDistanceHash(original) != "073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407" {
		t.Fatal("original Cargo source hash differs from the pinned file")
	}
	executable := filepath.Join(t.TempDir(), "edit-distance-original")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, compiler, "--edition=2024", "-C", "opt-level=2", filepath.Join(root, "tools", "port-cargo-edit-distance", "oracle.rs"), "-o", executable)
	command.Env = append(os.Environ(), "CARGO_EDIT_DISTANCE_SOURCE="+source)
	if diagnostics, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile original Rust oracle: %v\n%s", err, diagnostics)
	}
	upstreamTests := filepath.Join(t.TempDir(), "upstream-tests")
	originalTestCommand := exec.CommandContext(ctx, compiler, "--edition=2024", "--test", "-C", "opt-level=2", source, "-o", upstreamTests)
	if diagnostics, err := originalTestCommand.CombinedOutput(); err != nil {
		t.Fatalf("compile original upstream test: %v\n%s", err, diagnostics)
	}
	if diagnostics, err := exec.CommandContext(ctx, upstreamTests).CombinedOutput(); err != nil {
		t.Fatalf("run original upstream test: %v\n%s", err, diagnostics)
	}
	for _, test := range []struct{ mode, fixture string }{{"unicode", "edit_distance_unicode.bin.gz"}, {"cases", "edit_distance_cases.bin.gz"}} {
		actual, err := exec.CommandContext(ctx, executable, test.mode).Output()
		if err != nil {
			t.Fatalf("execute original Rust %s oracle: %v", test.mode, err)
		}
		expected, _ := editDistanceFixture(t, test.fixture)
		if !bytes.Equal(actual, expected) {
			t.Fatalf("original Rust %s oracle differs from committed fixtures", test.mode)
		}
	}
}
