package cargo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func packageFixture(t *testing.T, base, relative, name string, library bool, source, dependencies string) string {
	t.Helper()
	root := filepath.Join(base, relative)
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "Cargo.toml")
	contents := "[package]\nname = \"" + name + "\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"
	if dependencies != "" {
		contents += "\n[dependencies]\n" + dependencies + "\n"
	}
	if err := os.WriteFile(manifest, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	sourceName := "main.rs"
	if library {
		sourceName = "lib.rs"
	}
	if err := os.WriteFile(filepath.Join(root, "src", sourceName), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func diamondFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	packageFixture(t, root, "shared", "shared-base", true, "#[derive(Copy, Clone)]\npub struct Number { pub value: i32 }\npub fn make(value: i32) -> Number { Number { value } }\n", "")
	packageFixture(t, root, "left", "left", true, "pub fn make(value: i32) -> shared_base::Number { shared_base::make(value) }\n", "shared-base = { path = \"../shared\" }")
	packageFixture(t, root, "right", "right", true, "pub fn read(value: core::Number) -> i32 { value.value + 1 }\n", "core = { path = \"../shared\", package = \"shared-base\" }")
	manifest := packageFixture(t, root, "app", "app", false, "fn main() { let value = left::make(41); println!(\"{}\", right::read(value)); }\n", "left = { path = \"../left\" }\nright = { path = \"../right\" }")
	return manifest, root
}

func TestDependencyGraphInternsSharedLibrary(t *testing.T) {
	manifest, _ := diamondFixture(t)
	graph, err := loadPackageGraph(manifest)
	if err != nil {
		t.Fatal(err)
	}
	left := graph.Dependencies["left"].Dependencies["shared_base"]
	right := graph.Dependencies["right"].Dependencies["core"]
	if left != right || left.Crate != right.Crate {
		t.Fatal("diamond graph gave shared library distinct identities")
	}
	if graph.Crate.Dependencies["left"].Dependencies["shared_base"] != graph.Crate.Dependencies["right"].Dependencies["core"] {
		t.Fatal("compiler graph did not preserve interning")
	}
}

func TestDependencyGraphCanonicalizesSymlinkPaths(t *testing.T) {
	root := t.TempDir()
	packageFixture(t, root, "shared", "shared", true, "pub fn value() -> i32 { 1 }", "")
	if err := os.Symlink(filepath.Join(root, "shared"), filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	packageFixture(t, root, "left", "left", true, "pub fn value() -> i32 { shared::value() }", "shared = { path = \"../shared\" }")
	packageFixture(t, root, "right", "right", true, "pub fn value() -> i32 { shared::value() }", "shared = { path = \"../linked\" }")
	manifest := packageFixture(t, root, "app", "app", false, "fn main() {}", "left = { path = \"../left\" }\nright = { path = \"../right\" }")
	graph, err := loadPackageGraph(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Dependencies["left"].Dependencies["shared"] != graph.Dependencies["right"].Dependencies["shared"] {
		t.Fatal("symlink paths created distinct package nodes")
	}
}

func TestDependencyGraphRejectsCyclesAndInvalidEdges(t *testing.T) {
	for _, name := range []string{"cycle", "missing", "name mismatch", "binary dependency"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			dependencies := "dep = { path = \"../dep\" }"
			switch name {
			case "cycle":
				packageFixture(t, root, "dep", "dep", true, "pub fn value() -> i32 { 1 }", "app = { path = \"../app\" }")
			case "name mismatch":
				packageFixture(t, root, "dep", "other", true, "pub fn value() -> i32 { 1 }", "")
			case "binary dependency":
				packageFixture(t, root, "dep", "dep", false, "fn main() {}", "")
			}
			manifest := packageFixture(t, root, "app", "app", name == "cycle", "pub fn value() -> i32 { 1 }", dependencies)
			if _, err := loadPackageGraph(manifest); err == nil {
				t.Fatal("invalid package graph accepted")
			}
		})
	}
}

func TestLocalDependencyCheckAndNativeRun(t *testing.T) {
	manifest, root := diamondFixture(t)
	if code, stdout, stderr := runCommand("check", "--manifest-path", manifest); code != 0 || stdout != "" {
		t.Fatalf("check dependency graph: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	for _, name := range []string{"app", "left", "right", "shared"} {
		if _, err := os.Stat(filepath.Join(root, name, "target")); !os.IsNotExist(err) {
			t.Fatalf("check created target for %s", name)
		}
	}
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("native portion requires C compiler")
	}
	if code, stdout, stderr := runCommand("run", "--manifest-path", manifest); code != 0 || stdout != "42\n" {
		t.Fatalf("run dependency graph: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "shared", "target")); !os.IsNotExist(err) {
		t.Fatal("dependency emitted an unsupported standalone artifact")
	}
	graph, err := loadPackageGraph(manifest)
	if err != nil {
		t.Fatal(err)
	}
	output, _ := artifactPaths(graph.Manifest, "debug")
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	invalid := filepath.Join(root, "right", "src", "lib.rs")
	if err := os.WriteFile(invalid, []byte("pub fn read(value: core::Number) -> i32 { true }"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCommand("build", "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "lib.rs") {
		t.Fatalf("invalid library accepted: %d %s", code, stderr)
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed dependency build changed existing binary")
	}
}

func TestDependencyPrivacyAndRenamedAliases(t *testing.T) {
	root := t.TempDir()
	packageFixture(t, root, "math", "math-lib", true, "pub fn add(a: i32,b: i32) -> i32 { a+b }\nfn secret() -> i32 { 9 }", "")
	manifest := packageFixture(t, root, "app", "app", false, "fn main() { println!(\"{}\", numbers::add(20,22)); }", "numbers = { path = \"../math\", package = \"math-lib\" }")
	if code, _, stderr := runCommand("check", "--manifest-path", manifest); code != 0 {
		t.Fatalf("renamed dependency check: %s", stderr)
	}
	source := filepath.Join(root, "app", "src", "main.rs")
	if err := os.WriteFile(source, []byte("fn main() { println!(\"{}\", numbers::secret()); }"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCommand("check", "--manifest-path", manifest); code == 0 {
		t.Fatal("private dependency item was accessible")
	}
	if err := os.WriteFile(source, []byte("fn main() { println!(\"{}\", math_lib::add(20,22)); }"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCommand("check", "--manifest-path", manifest); code == 0 {
		t.Fatal("original crate name remained available after rename")
	}
}

func TestNewLibraryAndUnsupportedArtifacts(t *testing.T) {
	root := filepath.Join(t.TempDir(), "addition")
	if code, _, stderr := runCommand("new", root, "--lib"); code != 0 {
		t.Fatalf("new --lib: %s", stderr)
	}
	manifest := filepath.Join(root, "Cargo.toml")
	m, err := readManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Library || filepath.Base(m.Source) != "lib.rs" {
		t.Fatalf("wrong library manifest: %+v", m)
	}
	if code, _, stderr := runCommand("check", "--manifest-path", manifest); code != 0 {
		t.Fatalf("check new library: %s", stderr)
	}
	for _, command := range []string{"build", "run"} {
		if code, _, stderr := runCommand(command, "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "library-only") {
			t.Fatalf("%s library: %d %s", command, code, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
		t.Fatal("library-only commands created fake artifacts")
	}
	existing := t.TempDir()
	if code, _, stderr := runCommand("init", existing, "--lib", "--name", "initialized"); code != 0 {
		t.Fatalf("init --lib: %s", stderr)
	}
	if code, _, _ := runCommand("init", existing, "--lib", "--name", "initialized"); code == 0 {
		t.Fatal("init --lib overwrote existing library")
	}
}

func TestDependencyGraphRejectsDuplicatePackageNames(t *testing.T) {
	for _, test := range []string{"same package aliases", "same coordinate paths"} {
		t.Run(test, func(t *testing.T) {
			root := t.TempDir()
			packageFixture(t, root, "one", "shared", true, "pub fn value() -> i32 { 1 }", "")
			secondPath := "../one"
			if test == "same coordinate paths" {
				packageFixture(t, root, "two", "shared", true, "pub fn value() -> i32 { 2 }", "")
				secondPath = "../two"
			}
			dependencies := "first = { path = \"../one\", package = \"shared\" }\nsecond = { path = \"" + secondPath + "\", package = \"shared\" }"
			manifest := packageFixture(t, root, "app", "app", false, "fn main() {}", dependencies)
			if _, err := loadPackageGraph(manifest); err == nil {
				t.Fatal("duplicate package identity accepted")
			}
		})
	}
}

func TestDependencyGraphDistinctManifestsRetainIdentity(t *testing.T) {
	root := t.TempDir()
	packageFixture(t, root, "one", "first", true, "pub fn value() -> i32 { 1 }", "")
	packageFixture(t, root, "two", "second", true, "pub fn value() -> i32 { 1 }", "")
	source := filepath.Join(root, "two", "src", "lib.rs")
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "one", "src", "lib.rs"), source); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manifest := packageFixture(t, root, "app", "app", false, "fn main() {}", "first={path=\"../one\"}\nsecond={path=\"../two\"}")
	graph, err := loadPackageGraph(manifest)
	if err != nil {
		t.Fatal(err)
	}
	first, second := graph.Dependencies["first"].Crate, graph.Dependencies["second"].Crate
	if first == second || first.Identity == second.Identity || first.Identity == "" || second.Identity == "" {
		t.Fatal("distinct package manifests lost nominal crate identities")
	}
}

func TestCargoBuildProtectsAllGraphSources(t *testing.T) {
	for _, origin := range []string{"binary", "dependency"} {
		t.Run(origin, func(t *testing.T) {
			root := t.TempDir()
			manifest := packageFixture(t, root, "app", "app", false, "fn main() {}", "")
			source := filepath.Join(root, "app", "src", "main.rs")
			if origin == "dependency" {
				packageFixture(t, root, "dep", "dep", true, "pub fn value() -> i32 { 7 }", "")
				manifest = packageFixture(t, root, "app", "app", false, "fn main() { println!(\"{}\",dep::value()); }", "dep={path=\"../dep\"}")
				source = filepath.Join(root, "dep", "src", "lib.rs")
			}
			graph, err := loadPackageGraph(manifest)
			if err != nil {
				t.Fatal(err)
			}
			output, _ := artifactPaths(graph.Manifest, "debug")
			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Link(source, output); err != nil {
				t.Skipf("hardlinks unavailable: %v", err)
			}
			before, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if code, _, stderr := runCommand("build", "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "source file") {
				t.Fatalf("source alias build: code=%d stderr=%s", code, stderr)
			}
			after, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			outputContent, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !bytes.Equal(before, outputContent) {
				t.Fatal("source alias build changed existing files")
			}
		})
	}
}

func TestManifestFileSymlinkPreservesPackageContext(t *testing.T) {
	root := t.TempDir()
	actual := packageFixture(t, root, "actual", "app", false, "fn main() { println!(\"1\"); }", "")
	logicalRoot := filepath.Join(root, "logical")
	if err := os.MkdirAll(filepath.Join(logicalRoot, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	logicalSource := filepath.Join(logicalRoot, "src", "main.rs")
	source := []byte("fn main() { println!(\"2\"); }")
	if err := os.WriteFile(logicalSource, source, 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(logicalRoot, "Cargo.toml")
	if err := os.Symlink(actual, manifest); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	graph, err := loadPackageGraph(manifest)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.Stat(graph.Crate.Root)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(logicalSource)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(got, want) {
		t.Fatal("manifest file symlink selected source in physical manifest directory")
	}
	if code, stdout, stderr := runCommand("check", "--manifest-path", manifest); code != 0 || stdout != "" {
		t.Fatalf("check logical package: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(logicalRoot, "target")); !os.IsNotExist(err) {
		t.Fatal("logical package check created artifacts")
	}
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("native portion requires C compiler")
	}
	actualManifest, err := readManifest(actual)
	if err != nil {
		t.Fatal(err)
	}
	actualOutput, actualMarker := artifactPaths(actualManifest, "debug")
	if err := os.MkdirAll(filepath.Dir(actualOutput), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actualOutput, []byte("KEEP"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actualMarker, []byte(marker(actualManifest)), 0644); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := runCommand("run", "--manifest-path", manifest); code != 0 || stdout != "2\n" {
		t.Fatalf("run logical package: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	output, owned := artifactPaths(graph.Manifest, "debug")
	if _, err := os.Stat(output); err != nil {
		t.Fatal("binary missing from selected package target directory")
	}
	if code, _, stderr := runCommand("clean", "--manifest-path", manifest); code != 0 {
		t.Fatalf("clean logical package: %s", stderr)
	}
	for _, path := range []string{output, owned} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("clean left logical artifact %s", path)
		}
	}
	kept, err := os.ReadFile(actualOutput)
	if err != nil || string(kept) != "KEEP" {
		t.Fatal("logical build/clean modified physical manifest package artifacts")
	}
	if _, err := os.Stat(actualMarker); err != nil {
		t.Fatal("logical clean removed physical manifest package marker")
	}
	// The selected source context also feeds the compiler's final-output guard.
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(logicalSource, output); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	if code, _, stderr := runCommand("build", "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "source file") {
		t.Fatalf("logical source alias accepted: code=%d stderr=%s", code, stderr)
	}
	after, err := os.ReadFile(logicalSource)
	if err != nil || !bytes.Equal(after, source) {
		t.Fatal("logical source alias guard changed source")
	}
}
