package compiler

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func graphSource(t *testing.T, directory, name, source string) string {
	t.Helper()
	path := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func graphOutput(t *testing.T, executable string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, executable).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("crate graph executable timed out: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("crate graph executable: %v\n%s", err, output)
	}
	return string(output)
}

func checkAndRunGraph(t *testing.T, root *Crate, want string) {
	t.Helper()
	if err := CheckCrate(root, false); err != nil {
		t.Fatalf("CheckCrate: %v", err)
	}
	output := filepath.Join(t.TempDir(), "app")
	if err := BuildCrate(root, output); err != nil {
		t.Fatalf("BuildCrate: %v", err)
	}
	if got := graphOutput(t, output); got != want {
		t.Fatalf("output=%q, want %q", got, want)
	}
	if rustc := os.Getenv("FERRUGO_RUSTC"); rustc != "" {
		t.Run("rustc oracle", func(t *testing.T) {
			if got := rustcGraphOutput(t, rustc, root); got != want {
				t.Fatalf("rustc output=%q, want %q", got, want)
			}
		})
	}
}

// The oracle compiles real rlibs only when explicitly configured. Ferrugo's own
// graph checks/builds above never depend on rustc or a preinstalled Cargo.
func rustcGraphOutput(t *testing.T, rustc string, root *Crate) string {
	t.Helper()
	directory := t.TempDir()
	outputs := map[string]string{}
	next := 0
	canonical := func(path string) string {
		absolute, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			t.Fatal(err)
		}
		return resolved
	}
	compile := func(args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, rustc, args...).CombinedOutput(); err != nil {
			t.Fatalf("rustc graph compilation: %v\n%s", err, output)
		}
	}
	// Identity controls reuse; rustc receives the original logical source path so
	// module lookup beside symlinked crate roots follows Rust's source semantics.
	var library func(*Crate) string
	library = func(crate *Crate) string {
		key := "root:" + canonical(crate.Root)
		if crate.Identity != "" {
			key = "identity:" + crate.Identity
		}
		if output, ok := outputs[key]; ok {
			return output
		}
		next++
		name := fmt.Sprintf("ferrugo_graph_dep%d", next)
		output := filepath.Join(directory, "lib"+name+".rlib")
		args := []string{"--edition=2021", "--crate-type=rlib", "--crate-name", name, crate.Root, "-o", output, "-L", "dependency=" + directory}
		aliases := make([]string, 0, len(crate.Dependencies))
		for alias := range crate.Dependencies {
			aliases = append(aliases, alias)
		}
		sort.Strings(aliases)
		for _, alias := range aliases {
			args = append(args, "--extern", alias+"="+library(crate.Dependencies[alias]))
		}
		compile(args)
		outputs[key] = output
		return output
	}
	output := filepath.Join(directory, "app")
	args := []string{"--edition=2021", "--crate-name", "ferrugo_graph_root", root.Root, "-o", output, "-L", "dependency=" + directory}
	aliases := make([]string, 0, len(root.Dependencies))
	for alias := range root.Dependencies {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		args = append(args, "--extern", alias+"="+library(root.Dependencies[alias]))
	}
	compile(args)
	return graphOutput(t, output)
}

func TestCrateGraphDirectTransitiveAndRenamedDependencies(t *testing.T) {
	directory := t.TempDir()
	units := &Crate{Root: graphSource(t, directory, "units/lib.rs", `pub fn value() -> i32 { 40 }`)}
	math := &Crate{
		Root:         graphSource(t, directory, "math/lib.rs", `pub fn total() -> i32 { renamed_units::value() + 2 }`),
		Dependencies: map[string]*Crate{"renamed_units": units},
	}
	root := &Crate{
		Root:         graphSource(t, directory, "app/main.rs", `use math_alias::total as result; fn main() { println!("{}",result()); }`),
		Dependencies: map[string]*Crate{"math_alias": math},
	}
	checkAndRunGraph(t, root, "42\n")
}

func TestCrateGraphSharedDiamondPreservesStructIdentity(t *testing.T) {
	for _, sharing := range []string{"shared pointer", "same canonical root", "symlink canonical root"} {
		t.Run(sharing, func(t *testing.T) {
			directory := t.TempDir()
			common := &Crate{Root: graphSource(t, directory, "common/lib.rs", `
#[derive(Copy,Clone)] pub struct Number { pub value:i32 }
pub fn make(value:i32) -> Number { Number {value} }
`)}
			other := common
			if sharing == "same canonical root" {
				other = &Crate{Root: filepath.Join(directory, "common", "..", "common", "lib.rs")}
			}
			if sharing == "symlink canonical root" {
				alias := filepath.Join(directory, "alias")
				if err := os.Symlink(filepath.Dir(common.Root), alias); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				other = &Crate{Root: filepath.Join(alias, "lib.rs")}
			}
			left := &Crate{Root: graphSource(t, directory, "left/lib.rs", `pub fn make() -> data::Number { data::make(7) }`), Dependencies: map[string]*Crate{"data": common}}
			right := &Crate{Root: graphSource(t, directory, "right/lib.rs", `pub fn read(number:model::Number) -> i32 { number.value + 1 }`), Dependencies: map[string]*Crate{"model": other}}
			root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main() { println!("{}",right::read(left::make())); }`), Dependencies: map[string]*Crate{"left": left, "right": right}}
			checkAndRunGraph(t, root, "8\n")
		})
	}
}

func TestCrateGraphOwnRootsPrivateHelpersAndOrdinaryLibraryMain(t *testing.T) {
	directory := t.TempDir()
	dependency := &Crate{Root: graphSource(t, directory, "dependency/lib.rs", `
mod inner;
fn hidden() -> i32 { 4 }
pub fn main(n:i32) -> i32 { crate::hidden() + inner::value() + n }
`)}
	graphSource(t, directory, "dependency/inner.rs", `pub fn value() -> i32 { super::hidden() + crate::hidden() }`)
	root := &Crate{Root: graphSource(t, directory, "main.rs", `
fn hidden() -> i32 { 1000 }
fn main() { println!("{} {}",dependency::main(2),crate::hidden()); }
`), Dependencies: map[string]*Crate{"dependency": dependency}}
	checkAndRunGraph(t, root, "14 1000\n")
}

func TestCrateGraphSameBasenamesStaySeparate(t *testing.T) {
	directory := t.TempDir()
	first := &Crate{Root: graphSource(t, directory, "first/lib.rs", `mod inner; pub fn value()->i32{inner::value()}`)}
	second := &Crate{Root: graphSource(t, directory, "second/lib.rs", `mod inner; pub fn value()->i32{inner::value()}`)}
	graphSource(t, directory, "first/inner.rs", `pub fn value()->i32{17}`)
	graphSource(t, directory, "second/inner.rs", `pub fn value()->i32{23}`)
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{} {}",first::value(),second::value());}`), Dependencies: map[string]*Crate{"first": first, "second": second}}
	checkAndRunGraph(t, root, "17 23\n")
}

func TestCrateGraphLocalModuleShadowsExternPrelude(t *testing.T) {
	directory := t.TempDir()
	dependency := &Crate{Root: graphSource(t, directory, "dependency/lib.rs", `pub fn value()->i32{2}`)}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `mod dependency {pub fn value()->i32{1}} fn main(){println!("{}",dependency::value());}`), Dependencies: map[string]*Crate{"dependency": dependency}}
	checkAndRunGraph(t, root, "1\n")
}

func TestLibraryChecksDoNotRequireAnEntrypoint(t *testing.T) {
	directory := t.TempDir()
	path := graphSource(t, directory, "lib.rs", `pub fn value()->i32{3}`)
	if err := CheckLibraryFile(path); err != nil {
		t.Fatalf("CheckLibraryFile: %v", err)
	}
	if err := CheckCrate(&Crate{Root: path}, true); err != nil {
		t.Fatalf("CheckCrate library: %v", err)
	}
	if err := CheckFile(path); err == nil || !strings.Contains(err.Error(), "main") {
		t.Fatalf("binary check without main=%v", err)
	}
	ordinaryMain := graphSource(t, directory, "ordinary.rs", `pub fn main(n:i32)->i32{n+1}`)
	if err := CheckLibraryFile(ordinaryMain); err != nil {
		t.Fatalf("ordinary library main function rejected: %v", err)
	}
}

func TestCrateGraphBinaryNeedsItsOwnMain(t *testing.T) {
	directory := t.TempDir()
	dependency := &Crate{Root: graphSource(t, directory, "dependency/lib.rs", `pub fn main(){}`)}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn helper(){}`), Dependencies: map[string]*Crate{"dependency": dependency}}
	if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), "main") {
		t.Fatalf("dependency main satisfied binary root: %v", err)
	}
}

func TestCrateGraphRejectsVisibilityAndRootLeaks(t *testing.T) {
	cases := []struct{ name, library, binary, want string }{
		{"private function", `fn hidden()->i32{1}`, `fn main(){println!("{}",dep::hidden());}`, "private"},
		{"private module", `mod hidden {pub fn value()->i32{1}}`, `fn main(){println!("{}",dep::hidden::value());}`, "private"},
		{"private field", `#[derive(Copy,Clone)] pub struct Value{secret:i32} pub fn make()->Value{Value{secret:1}}`, `fn main(){let value=dep::make();println!("{}",value.secret);}`, "private"},
		{"private returned type", `#[derive(Copy,Clone)] struct Value{pub value:i32} pub fn make()->Value{Value{value:1}}`, `fn main(){let value=dep::make();}`, "private"},
		{"super cannot escape dependency", `pub fn value()->i32{super::hidden()}`, `fn hidden()->i32{1} fn main(){println!("{}",dep::value());}`, "super"},
		{"crate path cannot enter extern prelude", `pub fn value()->i32{1}`, `fn main(){println!("{}",crate::dep::value());}`, "dep"},
		{"self path cannot enter extern prelude", `pub fn value()->i32{1}`, `fn main(){println!("{}",self::dep::value());}`, "dep"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			dependency := &Crate{Root: graphSource(t, directory, "dep/lib.rs", tc.library)}
			root := &Crate{Root: graphSource(t, directory, "main.rs", tc.binary), Dependencies: map[string]*Crate{"dep": dependency}}
			if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want rejection containing %q", err, tc.want)
			}
		})
	}
}

func TestCrateGraphTransitiveExternsDoNotLeak(t *testing.T) {
	directory := t.TempDir()
	secret := &Crate{Root: graphSource(t, directory, "secret/lib.rs", `pub fn value()->i32{1}`)}
	outer := &Crate{Root: graphSource(t, directory, "outer/lib.rs", `pub fn value()->i32{secret::value()}`), Dependencies: map[string]*Crate{"secret": secret}}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{}",secret::value());}`), Dependencies: map[string]*Crate{"outer": outer}}
	if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("transitive extern leaked into root: %v", err)
	}
}

func TestCrateGraphSameShapeTypesHaveDistinctIdentity(t *testing.T) {
	directory := t.TempDir()
	left := &Crate{Root: graphSource(t, directory, "left/lib.rs", `#[derive(Copy,Clone)] pub struct Value{pub n:i32} pub fn make()->Value{Value{n:1}}`)}
	right := &Crate{Root: graphSource(t, directory, "right/lib.rs", `#[derive(Copy,Clone)] pub struct Value{pub n:i32} pub fn read(value:Value)->i32{value.n}`)}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{}",right::read(left::make()));}`), Dependencies: map[string]*Crate{"left": left, "right": right}}
	if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("distinct crate record types were unified: %v", err)
	}
}

func TestCrateGraphInvalidDescriptions(t *testing.T) {
	directory := t.TempDir()
	rootPath := graphSource(t, directory, "main.rs", `fn main(){}`)
	libraryPath := graphSource(t, directory, "lib.rs", `pub fn value()->i32{1}`)
	for _, alias := range []string{"", "some-name", "123name", "crate", "self", "super", "fn", "_", "foo::bar", "two words", "é"} {
		t.Run("extern "+alias, func(t *testing.T) {
			root := &Crate{Root: rootPath, Dependencies: map[string]*Crate{alias: {Root: libraryPath}}}
			if err := CheckCrate(root, false); err == nil {
				t.Fatalf("invalid extern identifier %q accepted", alias)
			}
		})
	}
	for _, tc := range []struct {
		name string
		root *Crate
	}{
		{"nil root", nil},
		{"empty root", &Crate{}},
		{"missing root", &Crate{Root: filepath.Join(directory, "missing.rs")}},
		{"directory root", &Crate{Root: directory}},
		{"nil dependency", &Crate{Root: rootPath, Dependencies: map[string]*Crate{"dep": nil}}},
		{"missing dependency", &Crate{Root: rootPath, Dependencies: map[string]*Crate{"dep": {Root: filepath.Join(directory, "missing.rs")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckCrate(tc.root, false); err == nil {
				t.Fatal("invalid crate description accepted")
			}
		})
	}
}

func TestCrateGraphRejectsPointerAndCanonicalPathCycles(t *testing.T) {
	for _, cycle := range []string{"pointer", "canonical path", "symlink canonical path"} {
		t.Run(cycle, func(t *testing.T) {
			directory := t.TempDir()
			first := &Crate{Root: graphSource(t, directory, "first/lib.rs", `pub fn value()->i32{1}`)}
			second := &Crate{Root: graphSource(t, directory, "second/lib.rs", `pub fn value()->i32{2}`)}
			first.Dependencies = map[string]*Crate{"second": second}
			back := first
			if cycle == "canonical path" {
				back = &Crate{Root: first.Root}
			}
			if cycle == "symlink canonical path" {
				alias := filepath.Join(directory, "alias.rs")
				if err := os.Symlink(first.Root, alias); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				back = &Crate{Root: alias}
			}
			second.Dependencies = map[string]*Crate{"first": back}
			if err := CheckCrate(first, true); err == nil || !strings.Contains(err.Error(), "cycle") {
				t.Fatalf("crate graph cycle not diagnosed: %v", err)
			}
		})
	}
}

func TestBuildCrateProtectsEveryDependencySource(t *testing.T) {
	directory := t.TempDir()
	leafPath := graphSource(t, directory, "leaf/lib.rs", `mod inner; pub fn value()->i32{inner::value()}`)
	leafModule := graphSource(t, directory, "leaf/inner.rs", `pub fn value()->i32{1}`)
	dependencyPath := graphSource(t, directory, "dependency/lib.rs", `mod inner; pub fn value()->i32{inner::value()+leaf::value()}`)
	dependencyModule := graphSource(t, directory, "dependency/inner.rs", `pub fn value()->i32{2}`)
	rootPath := graphSource(t, directory, "main.rs", `fn main(){println!("{}",dependency::value());}`)
	leaf := &Crate{Root: leafPath}
	dependency := &Crate{Root: dependencyPath, Dependencies: map[string]*Crate{"leaf": leaf}}
	root := &Crate{Root: rootPath, Dependencies: map[string]*Crate{"dependency": dependency}}
	original := map[string][]byte{}
	for _, path := range []string{rootPath, dependencyPath, dependencyModule, leafPath, leafModule} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		original[path] = contents
	}
	for _, path := range []string{rootPath, dependencyPath, dependencyModule, leafPath, leafModule} {
		t.Run(filepath.ToSlash(strings.TrimPrefix(path, directory+string(filepath.Separator))), func(t *testing.T) {
			if err := BuildCrate(root, path); err == nil {
				t.Fatal("dependency source overwrite accepted")
			}
			for source, want := range original {
				got, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("build changed source %s", source)
				}
			}
		})
	}
	for _, kind := range []string{"hardlink", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			alias := filepath.Join(directory, kind)
			var err error
			if kind == "hardlink" {
				err = os.Link(leafModule, alias)
			} else {
				err = os.Symlink(leafModule, alias)
			}
			if err != nil {
				t.Skipf("link unavailable: %v", err)
			}
			if err := BuildCrate(root, alias); err == nil {
				t.Fatal("source alias overwrite accepted")
			}
			got, err := os.ReadFile(leafModule)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, original[leafModule]) {
				t.Fatal("source alias build changed dependency module")
			}
		})
	}
}

func TestBuildCrateFailurePreservesExistingExecutableAndDependency(t *testing.T) {
	directory := t.TempDir()
	invalid := []byte(`pub fn value()->i32{absent()}`)
	dependencyPath := graphSource(t, directory, "dependency/lib.rs", string(invalid))
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{}",dependency::value());}`), Dependencies: map[string]*Crate{"dependency": {Root: dependencyPath}}}
	output := filepath.Join(directory, "app")
	original := []byte("existing executable")
	if err := os.WriteFile(output, original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := BuildCrate(root, output); err == nil {
		t.Fatal("invalid dependency built")
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("failed graph build replaced existing executable")
	}
	got, err = os.ReadFile(dependencyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, invalid) {
		t.Fatal("failed graph build changed dependency source")
	}
}
