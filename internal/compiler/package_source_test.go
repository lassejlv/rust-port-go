package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrateRootFileSymlinkPreservesModuleDirectory(t *testing.T) {
	directory := t.TempDir()
	actual := graphSource(t, directory, "actual/main.rs", `mod values; fn main(){println!("{}",values::get());}`)
	graphSource(t, directory, "actual/values.rs", `pub fn get()->i32{1}`)
	graphSource(t, directory, "logical/values.rs", `pub fn get()->i32{2}`)
	logical := filepath.Join(directory, "logical", "main.rs")
	if err := os.Symlink(actual, logical); err != nil {
		t.Skip(err)
	}
	checkAndRunGraph(t, &Crate{Root: logical}, "2\n")
}

func TestPackageIdentitySeparatesSharedPhysicalSource(t *testing.T) {
	directory := t.TempDir()
	shared := graphSource(t, directory, "shared.rs", `#[derive(Copy,Clone)]pub struct Item{pub n:i32}pub fn make()->Item{Item{n:7}}pub fn read(item:Item)->i32{item.n}`)
	left := &Crate{Identity: "package-left", Root: shared}
	right := &Crate{Identity: "package-right", Root: shared}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{}",right::read(left::make()));}`), Dependencies: map[string]*Crate{"left": left, "right": right}}
	if err := CheckCrate(root, false); err == nil {
		t.Fatal("records from distinct package identities were unified")
	}
	// Keeping the two identities separate also works when values never cross.
	root.Root = graphSource(t, directory, "ok.rs", `fn main(){println!("{} {}",left::read(left::make()),right::read(right::make()));}`)
	checkAndRunGraph(t, root, "7 7\n")
}

func TestConflictingLogicalModuleBasesDoNotSilentlyUnify(t *testing.T) {
	directory := t.TempDir()
	source := graphSource(t, directory, "first/lib.rs", `mod values;pub fn get()->i32{values::get()}`)
	graphSource(t, directory, "first/values.rs", `pub fn get()->i32{1}`)
	graphSource(t, directory, "second/values.rs", `pub fn get()->i32{2}`)
	alias := filepath.Join(directory, "second", "lib.rs")
	if err := os.Symlink(source, alias); err != nil {
		t.Skip(err)
	}
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{} {}",a::get(),b::get());}`), Dependencies: map[string]*Crate{"a": {Root: source}, "b": {Root: alias}}}
	if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("ambiguous source contexts accepted: %v", err)
	}
}

func TestStdDependencyPreludeShadowIsExplicitlyUnsupported(t *testing.T) {
	directory := t.TempDir()
	root := &Crate{Root: graphSource(t, directory, "main.rs", `fn main(){println!("{}",std::answer());}`), Dependencies: map[string]*Crate{"std": {Root: graphSource(t, directory, "std/lib.rs", `pub fn answer()->i32{42}`)}}}
	if err := CheckCrate(root, false); err == nil || !strings.Contains(err.Error(), "implicit Rust prelude") {
		t.Fatalf("std replacement silently accepted: %v", err)
	}
}
