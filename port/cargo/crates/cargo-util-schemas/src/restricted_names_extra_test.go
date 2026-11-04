package cargoutilschemas

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/cargo/src/util"
	rustclexer "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_lexer/src"
)

// All assertions from restricted_names.rs::tests::valid_feature_names, in the
// same order, including its repeated empty-string assertion.
func TestUpstreamValidFeatureNames(t *testing.T) {
	for _, name := range []string{"c++17", "128bit", "_foo", "feat-name", "feat_name", "foo.bar"} {
		if err := ValidateFeatureName(name); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
	}
	for _, name := range []string{"", "+foo", "-foo", ".foo", "dep:bar", "foo/bar", "foo:bar", "foo?", "?foo", "ⒶⒷⒸ", "a¼", ""} {
		if err := ValidateFeatureName(name); err == nil {
			t.Fatalf("%q unexpectedly valid", name)
		}
	}
}

func TestRestrictedNameSourceEdgeCases(t *testing.T) {
	for _, item := range []struct{ input, lower string }{{"ΟΣ", "ος"}, {"ΟΣΑ", "οσα"}, {"ΟΣ\u0301", "ος\u0301"}, {"İ", "i\u0307"}} {
		if got := util.Lowercase17(item.input); got != item.lower {
			t.Fatalf("Rust Unicode17 lowercase %q: %q, want %q", item.input, got, item.lower)
		}
	}
	if err := ValidateProfileName(""); err != nil {
		t.Fatalf("empty profile must remain allowed: %v", err)
	}
	for _, name := range []string{"CARGOcustom", "pacKage", "BUILD-OVERRIDE"} {
		if err := ValidateProfileName(name); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("%q: got %v, want Unicode17 lowercase reservation", name, err)
		}
	}
	for _, item := range []struct{ name, reason string }{{"1a", "cannot start with a digit"}, {"١a", "first character must be a Unicode XID start"}} {
		err := ValidateName(item.name, "example")
		if err == nil || !strings.Contains(err.Error(), item.reason) {
			t.Fatalf("%q: %v", item.name, err)
		}
	}
	if got := SanitizePackageName("::09a!::", '\x00'); got != "package::a\x00::package" {
		t.Fatalf("sanitized %q", got)
	}
	// Source accepts any char placeholder; it does not guarantee that the
	// resulting name validates when that placeholder is itself invalid.
	if got := SanitizeName("a!", '?'); got != "a?" || ValidatePackageName(got) == nil {
		t.Fatalf("invalid placeholder semantics: %q", got)
	}
	err := ValidateName("a\x00", "custom")
	var typed *NameValidationError
	if !errors.As(err, &typed) || typed.Source() != nil || errors.Unwrap(err) != nil {
		t.Fatalf("transparent source semantics: %v", err)
	}
	if _, exposesInner := any(typed).(interface{ Unwrap() error }); exposesInner {
		t.Fatal("transparent Error must delegate inner.source(), not expose inner")
	}
	wantDebug := "NameValidationError(InvalidCharacter { ch: '\\0', what: \"custom\", name: \"a\\0\", reason: \"characters must be Unicode XID characters (numbers, `-`, `_`, or most letters)\" })"
	if got := typed.RustDebug(); got != wantDebug {
		t.Fatalf("derived Debug differs: %s", got)
	}
}

type nameFixtureReader struct {
	*bytes.Reader
	t *testing.T
}

func (r nameFixtureReader) byte() byte {
	b, err := r.ReadByte()
	if err != nil {
		r.t.Fatal(err)
	}
	return b
}
func (r nameFixtureReader) u32() uint32 {
	var n uint32
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		r.t.Fatal(err)
	}
	return n
}
func (r nameFixtureReader) text() string {
	n := r.u32()
	if uint64(n) > uint64(r.Len()) {
		r.t.Fatal("short fixture string")
	}
	b := make([]byte, int(n))
	if _, err := io.ReadFull(r, b); err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}
func (r nameFixtureReader) magic(value string) {
	b := make([]byte, len(value))
	if _, err := io.ReadFull(r, b); err != nil || string(b) != value {
		r.t.Fatalf("bad fixture magic: %q (%v)", b, err)
	}
}
func nameFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("../../../../../tools/port-cargo-restricted-names/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Hashes map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != manifest.Hashes[path] {
		t.Fatalf("fixture provenance hash differs: %s", path)
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 64<<20 {
		t.Fatal("oversized fixture")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != manifest.Hashes[path+".raw"] {
		t.Fatalf("raw provenance hash differs: %s", path)
	}
	return raw
}

func TestOriginalRestrictedNamesOracle(t *testing.T) {
	data := nameFixture(t, "testdata/restricted_names_cases.bin.gz")
	r := nameFixtureReader{bytes.NewReader(data), t}
	r.magic("RNC1")
	requestSize := r.u32()
	if uint64(requestSize) > uint64(r.Len()) {
		t.Fatal("short request stream")
	}
	request := make([]byte, int(requestSize))
	if _, err := io.ReadFull(r, request); err != nil {
		t.Fatal(err)
	}
	q := nameFixtureReader{bytes.NewReader(request), t}
	q.magic("RNR1")
	count := q.u32()
	r.magic("RNO1")
	if r.u32() != count {
		t.Fatal("request/response counts differ")
	}
	for i := uint32(0); i < count; i++ {
		op, name, what, placeholder := q.byte(), q.text(), q.text(), rune(q.u32())
		kind, display, debug, source := r.byte(), r.text(), r.text(), r.byte()
		var err error
		actual := ""
		switch op {
		case 0:
			err = ValidatePackageName(name)
		case 1:
			err = ValidateRegistryName(name)
		case 2:
			err = ValidateName(name, what)
		case 3:
			err = ValidateProfileName(name)
		case 4:
			err = ValidateFeatureName(name)
		case 5:
			err = ValidatePathBaseName(name)
		case 6:
			actual = SanitizeName(name, placeholder)
		case 7:
			actual = SanitizePackageName(name, placeholder)
		default:
			t.Fatalf("bad fixture operation %d", op)
		}
		fail := func() {
			t.Fatalf("case %d op=%d name=%q what=%q placeholder=%U: got result=%q error=%v; original kind=%d display=%q debug=%q source=%d", i, op, name, what, placeholder, actual, err, kind, display, debug, source)
		}
		if op >= 6 {
			if kind != 2 || actual != display || debug != "" || source != 0 {
				fail()
			}
			continue
		}
		if err == nil {
			if kind != 0 || display != "" || debug != "" || source != 0 {
				fail()
			}
			continue
		}
		var typed *NameValidationError
		if kind != 1 || !errors.As(err, &typed) || typed.Error() != display || typed.RustDebug() != debug || typed.Source() != nil || source != 0 || errors.Unwrap(err) != nil {
			fail()
		}
	}
	if r.Len() != 0 || q.Len() != 0 {
		t.Fatal("trailing oracle bytes")
	}
	t.Logf("%d original-source requests", count)
}

func TestOriginalRestrictedNameUnicodeProfile(t *testing.T) {
	r := nameFixtureReader{bytes.NewReader(nameFixture(t, "testdata/restricted_names_unicode.bin.gz")), t}
	r.magic("RNU1")
	scalars := 0
	for value := 0; value < 0x110000; value++ {
		flags, size := r.byte(), int(r.byte())
		lower := make([]byte, size)
		if _, err := io.ReadFull(r, lower); err != nil {
			t.Fatal(err)
		}
		ch := rune(value)
		if !utf8.ValidRune(ch) {
			if flags != 0 || size != 0 {
				t.Fatalf("surrogate profile %x", value)
			}
			continue
		}
		scalars++
		if rustclexer.IsIDStart(ch) != (flags&1 != 0 || ch == '_') || rustclexer.IsIDContinue(ch) != (flags&2 != 0) || nameInRanges(ch, nameAlphanumericRanges) != (flags&4 != 0) || nameDigit(ch) != (flags&8 != 0) || nameInRanges(ch, nameEscapedRanges) != (flags&16 != 0) || nameInRanges(ch, nameNonInitialEscapedRanges) != (flags&32 != 0) || util.Lowercase17(string(ch)) != string(lower) {
			t.Fatalf("Unicode17 predicate or lowercase mismatch at %U", ch)
		}
	}
	if r.Len() != 0 || scalars != 1112064 {
		t.Fatalf("Unicode fixture: %d scalars, %d trailing bytes", scalars, r.Len())
	}
}

func TestRestrictedNamePreconditions(t *testing.T) {
	operations := []func(){func() { ValidatePackageName("\xff") }, func() { ValidateRegistryName("\xff") }, func() { ValidateName("\xff", "name") }, func() { ValidateName("name", "\xff") }, func() { ValidateProfileName("\xff") }, func() { ValidateFeatureName("\xff") }, func() { ValidatePathBaseName("\xff") }, func() { SanitizeName("\xff", '_') }, func() { SanitizePackageName("\xff", '_') }, func() { util.Lowercase17("\xff") }}
	for _, p := range []rune{-1, 0xd800, 0x110000} {
		p := p
		operations = append(operations, func() { SanitizeName("", p) }, func() { SanitizePackageName("", p) })
	}
	for i, operation := range operations {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected explicit Rust input precondition panic")
				}
			}()
			operation()
		})
	}
}

func FuzzRestrictedNameValidation(f *testing.F) {
	for _, name := range []string{"", "::09a!::", "١a", "a\x00", "c++17", "dep:x/y", "pacKage", "\U00010d50", "a\u200d", "ΟΣ\u0301"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		if !utf8.ValidString(name) || len(name) > 4096 {
			t.Skip()
		}
		ValidatePackageName(name)
		ValidateRegistryName(name)
		ValidateName(name, "fuzz name")
		ValidateProfileName(name)
		ValidateFeatureName(name)
		ValidatePathBaseName(name)
		if sanitized := SanitizeName(name, '_'); ValidateName(sanitized, "sanitized") != nil {
			t.Fatalf("invalid sanitized name %q from %q", sanitized, name)
		}
		if sanitized := SanitizePackageName(name, '_'); ValidatePackageName(sanitized) != nil {
			t.Fatalf("invalid sanitized package %q from %q", sanitized, name)
		}
	})
}
