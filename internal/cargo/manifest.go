package cargo

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Manifest is the deliberately small Cargo manifest understood by the bootstrap.
// All other tables and compilation settings produce an error.
type Manifest struct {
	Path         string
	Root         string
	Name         string
	Version      string
	Edition      string
	Source       string
	Library      bool
	Dependencies []Dependency
}

// Dependency describes a local path entry before graph resolution.
type Dependency struct {
	Name    string
	Alias   string
	Path    string
	Package string
}

var packageName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
var packageVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func validVersion(v string) bool {
	parts := packageVersion.FindStringSubmatch(v)
	if parts == nil {
		return false
	}
	for _, identifier := range strings.Split(parts[4], ".") {
		numeric := true
		for _, c := range identifier {
			if c < '0' || c > '9' {
				numeric = false
				break
			}
		}
		if numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func validName(name string) bool {
	if !packageName.MatchString(name) {
		return false
	}
	switch name {
	case "as", "break", "const", "continue", "crate", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "Self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while", "async", "await", "dyn", "abstract", "become", "box", "do", "final", "macro", "override", "priv", "typeof", "unsized", "virtual", "yield", "try", "gen":
		return false
	}
	return true
}

func findManifest(explicit string) (string, error) {
	if explicit != "" {
		p, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if filepath.Base(p) != "Cargo.toml" {
			return "", fmt.Errorf("--manifest-path must name a Cargo.toml file")
		}
		return p, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "Cargo.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find Cargo.toml in the current directory or any parent directory")
		}
		dir = parent
	}
}

// stripComment recognizes comments outside TOML's single-line quoted strings.
func stripComment(s string) (string, error) {
	var quote byte
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
		} else {
			if c == '#' {
				return strings.Trim(s[:i], " \t"), nil
			}
			if c == '"' || c == '\'' {
				quote = c
			}
		}
	}
	if quote != 0 {
		return "", fmt.Errorf("unterminated or multiline string (only single-line strings are supported)")
	}
	return strings.Trim(s, " \t"), nil
}

func tomlString(s string) (string, error) {
	s = strings.Trim(s, " \t")
	if len(s) < 2 {
		return "", fmt.Errorf("expected a quoted string")
	}
	if s[0] == '\'' && s[len(s)-1] == '\'' && !strings.Contains(s[1:len(s)-1], "'") {
		return s[1 : len(s)-1], nil
	}
	if s[0] != '"' || s[len(s)-1] != '"' {
		return "", fmt.Errorf("expected a quoted string")
	}
	// strconv.Unquote handles the common escape subset; reject Go-specific
	// escapes that TOML does not define so invalid TOML never passes as valid.
	for i := 1; i < len(s)-1; i++ {
		if s[i] == '\\' {
			i++
			if i >= len(s)-1 || !strings.ContainsRune("\"\\btnfruU", rune(s[i])) {
				return "", fmt.Errorf("unsupported TOML escape")
			}
		}
	}
	v, err := strconv.Unquote(s)
	if err != nil {
		return "", fmt.Errorf("invalid quoted string: %w", err)
	}
	return v, nil
}

// manifestLine preserves physical line endings. bufio.ScanLines drops a trailing
// CR even at EOF, which would hide a TOML-invalid standalone carriage return.
func manifestLine(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// Validate raw source before stripping comments or decoding escapes. Escaped
// controls such as \u0000 are valid TOML string content and remain allowed.
func validateManifestLine(line string) error {
	if !utf8.ValidString(line) {
		return fmt.Errorf("manifest must contain valid UTF-8")
	}
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\r' {
			if i+1 >= len(line) || line[i+1] != '\n' {
				return fmt.Errorf("carriage return must be followed by line feed")
			}
		} else if c != '\t' && c != '\n' && (c < 0x20 || c == 0x7f) {
			return fmt.Errorf("raw control character U+%04X is not allowed in TOML", c)
		}
	}
	return nil
}

// dependencyTable parses the supported single-line inline-table subset. Commas
// inside quoted paths are data; nested tables, dotted keys and non-string values
// are deliberately outside this bootstrap contract.
func dependencyTable(name, text string) (Dependency, error) {
	d := Dependency{Name: name, Alias: strings.ReplaceAll(name, "-", "_")}
	if !validName(d.Alias) || d.Alias == "_" {
		return d, fmt.Errorf("invalid Rust dependency alias %q", d.Alias)
	}
	text = strings.Trim(text, " \t")
	if len(text) < 2 || text[0] != '{' || text[len(text)-1] != '}' {
		return d, fmt.Errorf("dependency %q requires an inline table with path and optional package", name)
	}
	body := text[1 : len(text)-1]
	var fields []string
	start := 0
	var quote byte
	escaped := false
	for i := 0; i < len(body); i++ {
		c := body[i]
		if quote != 0 {
			if quote == '"' && escaped {
				escaped = false
				continue
			}
			if quote == '"' && c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
		} else if c == '"' || c == '\'' {
			quote = c
		} else if c == ',' {
			fields = append(fields, body[start:i])
			start = i + 1
		}
	}
	if quote != 0 {
		return d, fmt.Errorf("unterminated dependency string")
	}
	fields = append(fields, body[start:])
	seen := map[string]bool{}
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		key = strings.Trim(key, " \t")
		if !ok || key == "" {
			return d, fmt.Errorf("dependency inline table requires key = value entries without trailing comma")
		}
		if seen[key] {
			return d, fmt.Errorf("duplicate dependency key %q", key)
		}
		seen[key] = true
		if key != "path" && key != "package" {
			return d, fmt.Errorf("unsupported dependency key %q; only path and package are supported", key)
		}
		decoded, err := tomlString(value)
		if err != nil {
			return d, fmt.Errorf("dependency %s: %w", key, err)
		}
		if key == "path" {
			d.Path = decoded
		} else {
			d.Package = decoded
		}
	}
	if d.Path == "" {
		return d, fmt.Errorf("dependency %q requires a nonempty path", name)
	}
	if seen["package"] && !validName(d.Package) {
		return d, fmt.Errorf("invalid dependency package name %q", d.Package)
	}
	return d, nil
}

func readManifest(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	defer f.Close()
	m := &Manifest{Path: path, Root: filepath.Dir(path), Edition: "2015"}
	scan := bufio.NewScanner(f)
	scan.Split(manifestLine)
	seen := map[string]bool{}
	tables := map[string]bool{}
	aliases := map[string]bool{}
	table := ""
	line := 0
	for scan.Scan() {
		line++
		fail := func(format string, args ...any) (*Manifest, error) {
			return nil, fmt.Errorf("%s:%d: %s", path, line, fmt.Sprintf(format, args...))
		}
		text := scan.Text()
		if err := validateManifestLine(text); err != nil {
			return fail("%v", err)
		}
		text = strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
		raw, err := stripComment(text)
		if err != nil {
			return fail("%v", err)
		}
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "[") {
			switch raw {
			case "[package]":
				table = "package"
			case "[dependencies]":
				table = "dependencies"
			default:
				return fail("unsupported table %q; only package and local path dependencies are supported", raw)
			}
			if tables[table] {
				return fail("duplicate [%s] table", table)
			}
			tables[table] = true
			continue
		}
		if table == "" {
			return fail("expected [package] or [dependencies] table")
		}
		key, value, ok := strings.Cut(raw, "=")
		if !ok {
			return fail("expected key = value")
		}
		key = strings.Trim(key, " \t")
		value = strings.Trim(value, " \t")
		scoped := table + "." + key
		if seen[scoped] {
			return fail("duplicate %s key %q", table, key)
		}
		seen[scoped] = true
		if table == "dependencies" {
			d, err := dependencyTable(key, value)
			if err != nil {
				return fail("%v", err)
			}
			if aliases[d.Alias] {
				return fail("duplicate normalized dependency alias %q", d.Alias)
			}
			aliases[d.Alias] = true
			m.Dependencies = append(m.Dependencies, d)
			continue
		}
		switch key {
		case "name", "version", "edition", "description", "license", "repository", "homepage", "documentation", "readme":
			v, err := tomlString(value)
			if err != nil {
				return fail("%s: %v", key, err)
			}
			switch key {
			case "name":
				m.Name = v
			case "version":
				m.Version = v
			case "edition":
				m.Edition = v
			}
		default:
			return fail("unsupported package key %q", key)
		}
	}
	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	if !tables["package"] {
		return nil, fmt.Errorf("%s: missing [package] table", path)
	}
	if !validName(m.Name) {
		return nil, fmt.Errorf("%s: invalid or reserved package name %q", path, m.Name)
	}
	if !validVersion(m.Version) {
		return nil, fmt.Errorf("%s: package.version must be a semantic version such as 0.1.0", path)
	}
	switch m.Edition {
	case "2015", "2018", "2021", "2024":
	default:
		return nil, fmt.Errorf("%s: unsupported Rust edition %q", path, m.Edition)
	}
	for _, name := range []string{"build.rs", "src/bin"} {
		if _, err := os.Lstat(filepath.Join(m.Root, filepath.FromSlash(name))); err == nil {
			return nil, fmt.Errorf("%s: unsupported target %s", path, name)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	for _, name := range []string{"main.rs", "lib.rs"} {
		source := filepath.Join(m.Root, "src", name)
		info, err := os.Stat(source)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("crate source %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s must be a regular file", source)
		}
		if m.Source != "" {
			return nil, fmt.Errorf("%s: unsupported dual binary/library targets; choose src/main.rs or src/lib.rs", path)
		}
		m.Source = source
		m.Library = name == "lib.rs"
	}
	if m.Source == "" {
		return nil, fmt.Errorf("%s: expected one crate root at src/main.rs or src/lib.rs", path)
	}
	return m, nil
}
