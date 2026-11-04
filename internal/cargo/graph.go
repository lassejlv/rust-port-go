package cargo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/compiler"
)

// packageGraph interns packages by canonical manifest path, so a library shared
// by several dependents retains one compiler crate identity.
type packageGraph struct {
	Manifest     *Manifest
	Crate        *compiler.Crate
	Dependencies map[string]*packageGraph
}

type graphLoader struct {
	memo     map[string]*packageGraph
	active   map[string]bool
	stack    []string
	packages map[string]string
}

func loadPackageGraph(path string) (*packageGraph, error) {
	loader := graphLoader{memo: map[string]*packageGraph{}, active: map[string]bool{}, packages: map[string]string{}}
	return loader.load(path)
}

func canonicalManifest(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// A Cargo.toml file symlink keeps the directory where it was selected as
	// its package/source context. Directory symlink aliases still unify.
	directory, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", fmt.Errorf("resolve manifest %s: %w", path, err)
	}
	canonical := filepath.Join(directory, filepath.Base(absolute))
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("manifest %s must be a regular file", path)
	}
	return canonical, nil
}

func (l *graphLoader) load(path string) (*packageGraph, error) {
	canonical, err := canonicalManifest(path)
	if err != nil {
		return nil, err
	}
	if l.active[canonical] {
		cycle := append(append([]string{}, l.stack...), canonical)
		return nil, fmt.Errorf("cyclic local package dependency: %s", strings.Join(cycle, " -> "))
	}
	if existing := l.memo[canonical]; existing != nil {
		return existing, nil
	}
	manifest, err := readManifest(canonical)
	if err != nil {
		return nil, err
	}
	coordinate := manifest.Name + "@" + manifest.Version
	if previous := l.packages[coordinate]; previous != "" && previous != canonical {
		return nil, fmt.Errorf("package collision: %s is provided by both %s and %s", coordinate, previous, canonical)
	}
	l.packages[coordinate] = canonical
	node := &packageGraph{Manifest: manifest, Crate: &compiler.Crate{Root: manifest.Source, Identity: canonical, Dependencies: map[string]*compiler.Crate{}}, Dependencies: map[string]*packageGraph{}}
	l.memo[canonical] = node
	l.active[canonical] = true
	l.stack = append(l.stack, canonical)
	defer func() { delete(l.active, canonical); l.stack = l.stack[:len(l.stack)-1] }()
	directPackages := map[string]string{}
	for _, dependency := range manifest.Dependencies {
		directory := dependency.Path
		if !filepath.IsAbs(directory) {
			directory = filepath.Join(manifest.Root, directory)
		}
		child, err := l.load(filepath.Join(directory, "Cargo.toml"))
		if err != nil {
			return nil, fmt.Errorf("%s: dependency %q: %w", manifest.Path, dependency.Name, err)
		}
		expected := dependency.Package
		if expected == "" {
			expected = dependency.Name
		}
		if child.Manifest.Name != expected {
			return nil, fmt.Errorf("%s: dependency %q expects package %q, found %q", manifest.Path, dependency.Name, expected, child.Manifest.Name)
		}
		if !child.Manifest.Library {
			return nil, fmt.Errorf("%s: dependency %q must be a library-only package with src/lib.rs", manifest.Path, dependency.Name)
		}
		if previous := directPackages[child.Manifest.Path]; previous != "" {
			return nil, fmt.Errorf("%s: package %q is depended on multiple times as %q and %q", manifest.Path, child.Manifest.Name, previous, dependency.Alias)
		}
		directPackages[child.Manifest.Path] = dependency.Alias
		node.Dependencies[dependency.Alias] = child
		node.Crate.Dependencies[dependency.Alias] = child.Crate
	}
	return node, nil
}
