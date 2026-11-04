package compiler

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

// Crate describes one local crate and its explicit extern dependency bindings.
// Libraries shared by aliases or a dependency diamond retain one type identity.
type Crate struct {
	// Identity optionally distinguishes packages with a shared physical source root.
	Identity     string
	Root         string
	Dependencies map[string]*Crate
}

// CheckCrate checks a local crate graph. library selects a root without an entrypoint.
func CheckCrate(root *Crate, library bool) error {
	_, _, err := analyzeCrate(root, !library)
	return err
}

// CheckLibraryFile checks a library source tree without requiring main.
func CheckLibraryFile(sourcePath string) error { return CheckCrate(&Crate{Root: sourcePath}, true) }

// BuildCrate compiles all local crates into one native bootstrap executable.
// It does not generate Rust rlibs or claim Rust ABI compatibility.
func BuildCrate(root *Crate, outputPath string) error {
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	c, sources, err := analyzeCrate(root, true)
	if err != nil {
		return err
	}
	if err = protectSources(outputAbs, sources); err != nil {
		return err
	}
	return buildNative(emitChecked(c), outputAbs)
}

func analyzeCrate(root *Crate, requireMain bool) (*checked, []string, error) {
	loader := &crateLoader{loaded: make(map[string]*loadedCrate)}
	program, err := loader.load(root, true)
	if err != nil {
		return nil, nil, err
	}
	c, err := analyzeProgramMode(program, requireMain)
	if err != nil {
		return nil, nil, fmt.Errorf("crate %s: %w", root.Root, err)
	}
	return c, loader.sources, nil
}

type loadedCrate struct {
	config  *Crate
	program *syntax.Program
	active  bool
}
type crateLoader struct {
	loaded  map[string]*loadedCrate
	sources []string
	nextID  int
}

func canonicalCrateRoot(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("crate root source path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("crate root %s: %w", path, err)
	}
	return canonical, nil
}

func (l *crateLoader) load(config *Crate, root bool) (*syntax.Program, error) {
	if config == nil {
		return nil, fmt.Errorf("nil crate dependency")
	}
	canonical, err := canonicalCrateRoot(config.Root)
	if err != nil {
		return nil, err
	}
	key := canonical
	if config.Identity != "" {
		key = "package:" + config.Identity
	}
	if prior := l.loaded[key]; prior != nil {
		if prior.active {
			return nil, fmt.Errorf("crate dependency cycle involving %s", canonical)
		}
		same, err := sameCrateGraph(prior.config, config, make(map[cratePair]bool))
		if err != nil {
			return nil, err
		}
		if !same {
			return nil, fmt.Errorf("conflicting dependency graphs for crate %s", canonical)
		}
		return prior.program, nil
	}
	aliases := make([]string, 0, len(config.Dependencies))
	for alias := range config.Dependencies {
		if alias == "std" {
			return nil, fmt.Errorf("dependency alias std shadows the implicit Rust prelude and is unsupported")
		}
		if !validExternName(alias) {
			return nil, fmt.Errorf("invalid Rust extern identifier %q", alias)
		}
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	program, sources, err := loadCrate(config.Root)
	if err != nil {
		return nil, err
	}
	if !root {
		l.nextID++
		program.CrateID = fmt.Sprintf("@crate%d", l.nextID)
	}
	program.Externs = make(map[string]*syntax.Program, len(aliases))
	entry := &loadedCrate{config: config, program: program, active: true}
	l.loaded[key] = entry
	l.sources = append(l.sources, sources...)
	for _, alias := range aliases {
		dependency, err := l.load(config.Dependencies[alias], false)
		if err != nil {
			return nil, fmt.Errorf("dependency %s of %s: %w", alias, canonical, err)
		}
		program.Externs[alias] = dependency
	}
	entry.active = false
	return program, nil
}

type cratePair struct{ a, b *Crate }

func sameCrateGraph(a, b *Crate, seen map[cratePair]bool) (bool, error) {
	if a == b {
		return true, nil
	}
	if a == nil || b == nil {
		return false, nil
	}
	pair := cratePair{a, b}
	if seen[pair] {
		return true, nil
	}
	seen[pair] = true
	aa, err := canonicalCrateRoot(a.Root)
	if err != nil {
		return false, err
	}
	bb, err := canonicalCrateRoot(b.Root)
	if err != nil {
		return false, err
	}
	ab, err := crateModuleBase(a.Root)
	if err != nil {
		return false, err
	}
	bbasedir, err := crateModuleBase(b.Root)
	if err != nil {
		return false, err
	}
	if aa != bb || ab != bbasedir || a.Identity != b.Identity || len(a.Dependencies) != len(b.Dependencies) {
		return false, nil
	}
	for alias, dependency := range a.Dependencies {
		other, exists := b.Dependencies[alias]
		if !exists {
			return false, nil
		}
		same, err := sameCrateGraph(dependency, other, seen)
		if err != nil || !same {
			return false, err
		}
	}
	return true, nil
}

func crateModuleBase(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Dir(absolute))
}

var externIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validExternName(name string) bool {
	if !externIdentifier.MatchString(name) || name == "_" {
		return false
	}
	switch name {
	case "as", "async", "await", "break", "const", "continue", "crate", "dyn", "else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop", "match", "mod", "move", "mut", "pub", "ref", "return", "self", "Self", "static", "struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while", "abstract", "become", "box", "do", "final", "macro", "override", "priv", "typeof", "unsized", "virtual", "yield", "try", "gen":
		return false
	}
	return true
}
