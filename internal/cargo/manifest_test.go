package cargo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manifestFixture(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte("fn main() {}"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Cargo.toml")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestManifestStringsAndComments(t *testing.T) {
	path := manifestFixture(t, `# a comment
[package] # another
name = 'a-project'
version = "0.1.0"
edition = "2024"
description = "a # within a string and \"quote\"" # outside
`)
	m, err := readManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "a-project" || m.Version != "0.1.0" || m.Edition != "2024" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestManifestRejectsUnsupportedOrInvalidInput(t *testing.T) {
	base := "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"
	cases := map[string]string{
		"dependencies":    base + "[dependencies]\n",
		"workspace":       base + "[workspace]\n",
		"build script":    base + "build = \"custom.rs\"\n",
		"duplicate key":   base + "name = \"other\"\n",
		"duplicate table": base + "[package]\n",
		"unknown setting": base + "autobins = false\n",
		"custom binary":   base + "[[bin]]\n",
		"array metadata":  base + "authors = [\"A\"]\n",
		"edition":         base + "edition = \"2099\"\n",
		"bad escape":      base + "description = \"bad\\x41\"\n",
		"multiline":       base + "description = \"\"\"multiple\nlines\"\"\"\n",
		"missing version": "[package]\nname = \"demo\"\n",
		"bad name":        "[package]\nname = \"../../escape\"\nversion = \"0.1.0\"\n",
		"reserved name":   "[package]\nname = \"fn\"\nversion = \"0.1.0\"\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := readManifest(manifestFixture(t, source)); err == nil {
				t.Fatal("expected manifest rejection")
			}
		})
	}
}

func TestManifestRejectsAutomaticUnsupportedTargets(t *testing.T) {
	for _, target := range []string{"build.rs", "src/lib.rs", "src/bin"} {
		t.Run(target, func(t *testing.T) {
			path := manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")
			if err := os.WriteFile(filepath.Join(filepath.Dir(path), filepath.FromSlash(target)), []byte(""), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := readManifest(path); err == nil || !strings.Contains(err.Error(), "unsupported target") {
				t.Fatalf("expected unsupported target, got %v", err)
			}
		})
	}
}

func TestManifestDiscoverySearchesParents(t *testing.T) {
	root := filepath.Dir(manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"))
	child := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(child); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	actual, err := findManifest("")
	if err != nil {
		t.Fatal(err)
	}
	actualInfo, err := os.Stat(actual)
	if err != nil {
		t.Fatal(err)
	}
	expectedInfo, err := os.Stat(filepath.Join(root, "Cargo.toml"))
	if err != nil {
		t.Fatal(err)
	}
	// macOS may canonicalize /var to /private/var in os.Getwd. Compare
	// filesystem identity, since both paths must identify the same manifest.
	if !os.SameFile(actualInfo, expectedInfo) {
		t.Fatalf("wrong manifest %s", actual)
	}
}

func TestSemanticVersions(t *testing.T) {
	for _, version := range []string{"0.1.0", "1.2.3-alpha.1", "0.0.0+build.01", "1.2.3-rc-1+build.hash"} {
		if !validVersion(version) {
			t.Errorf("valid version rejected: %s", version)
		}
	}
	for _, version := range []string{"01.2.3", "1.02.3", "1.2.03", "1.2", "v1.2.3", "1.2.3-01", "1.2.3-alpha..1", "1.2.3+", "1.2.3-"} {
		if validVersion(version) {
			t.Errorf("invalid version accepted: %s", version)
		}
	}
}

// These syntax cases were cross-checked with Python 3 tomllib. Keep this test
// dependency-free: the oracle is used during development, not at test runtime.
func TestManifestRejectsInvalidTOMLText(t *testing.T) {
	base := "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"
	reject := func(name, source string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if _, err := readManifest(manifestFixture(t, source)); err == nil {
				t.Fatal("accepted TOML-invalid source")
			}
		})
	}
	controls := make([]byte, 0, 33)
	for c := byte(0); c < 32; c++ {
		controls = append(controls, c)
	}
	controls = append(controls, 0x7f)
	for _, c := range controls {
		if c != '\t' {
			reject(fmt.Sprintf("literal/control_U%04X", c), base+"description = 'a"+string(c)+"b'\n")
			reject(fmt.Sprintf("basic/control_U%04X", c), base+"description = \"a"+string(c)+"b\"\n")
		}
		if c != '\t' && c != '\n' {
			reject(fmt.Sprintf("comment/control_U%04X", c), base+"# before"+string(c)+"after\n")
		}
	}
	for _, space := range []rune{'\u00a0', '\u0085', '\u1680', '\u2003', '\u2028', '\u2029', '\u202f', '\u205f', '\u3000'} {
		reject(fmt.Sprintf("non_TOML_whitespace/U%04X_before_table", space), string(space)+base)
		reject(fmt.Sprintf("non_TOML_whitespace/U%04X_in_key", space), base+"description"+string(space)+"=\"x\"\n")
		reject(fmt.Sprintf("non_TOML_whitespace/U%04X_after_value", space), base+"description=\"x\""+string(space)+"\n")
		reject(fmt.Sprintf("non_TOML_whitespace/U%04X_blank_line", space), base+string(space)+"\n")
	}
	reject("lone_CR_at_EOF", base+"# comment\r")
	reject("lone_CR_line_endings", strings.ReplaceAll(base, "\n", "\r"))
	for name, invalid := range map[string]string{
		"invalid_leading_byte": "\xff",
		"overlong_sequence":    "\xc0\xaf",
		"surrogate_encoding":   "\xed\xa0\x80",
		"truncated_sequence":   "\xc3",
		"invalid_continuation": "\xe2\x28\xa1",
	} {
		reject("invalid_UTF8/literal/"+name, base+"description='a"+invalid+"b'\n")
		reject("invalid_UTF8/basic/"+name, base+"description=\"a"+invalid+"b\"\n")
		reject("invalid_UTF8/comment/"+name, base+"# a"+invalid+"b\n")
	}
}

func TestManifestAcceptsValidTOMLText(t *testing.T) {
	base := "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"
	cases := map[string]string{
		"literal_tab":     base + "description='a\tb'\n",
		"basic_tab":       base + "description=\"a\tb\"\n",
		"escaped_NUL":     base + "description=\"a\\u0000b\"\n",
		"escaped_DEL":     base + "description=\"a\\u007Fb\"\n",
		"CRLF":            strings.ReplaceAll(base, "\n", "\r\n"),
		"basic_Unicode":   base + "description=\"hé🦀\u2028\"\n",
		"literal_Unicode": base + "description='hé🦀\u2028'\n",
		"comment_Unicode": base + "# hé🦀\u0085\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := readManifest(manifestFixture(t, source)); err != nil {
				t.Fatalf("rejected supported valid TOML: %v", err)
			}
		})
	}
}
