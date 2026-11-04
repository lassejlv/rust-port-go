package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCrate(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, source := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestModuleFileLayoutsAndOriginalSpans(t *testing.T) {
	root := writeCrate(t, map[string]string{
		"entry.rs":                `mod flat; mod directory; mod inline {mod nested;} fn main(){}`,
		"flat.rs":                 `mod nested; fn flat(){}`,
		"flat/nested.rs":          `fn leaf(){}`,
		"directory/mod.rs":        `mod nested;`,
		"directory/nested/mod.rs": `fn leaf(){}`,
		"inline/nested.rs":        `fn inline_leaf(){}`,
	})
	program, sources, err := loadCrate(filepath.Join(root, "entry.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 6 {
		t.Fatalf("sources=%v", sources)
	}
	leaf := program.Modules[0].Program.Modules[0].Program.Functions[0]
	if leaf.Span.File != filepath.Join(root, "flat", "nested.rs") {
		t.Fatalf("wrong source location %+v", leaf.Span)
	}
	if program.Modules[2].Program.Modules[0].Program.Functions[0].Span.File != filepath.Join(root, "inline", "nested.rs") {
		t.Fatal("inline module used wrong external base")
	}
}

func TestModuleFileErrors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"missing", map[string]string{"main.rs": `mod absent; fn main(){}`}, "missing module"},
		{"ambiguous", map[string]string{"main.rs": `mod both; fn main(){}`, "both.rs": "", "both/mod.rs": ""}, "ambiguous module"},
		{"syntax location", map[string]string{"main.rs": `mod child; fn main(){}`, "child.rs": "\nfn invalid( {"}, "child.rs:2:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := writeCrate(t, tt.files)
			_, _, err := loadCrate(filepath.Join(root, "main.rs"))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
	t.Run("directory source", func(t *testing.T) {
		root := writeCrate(t, map[string]string{"main.rs": `mod child; fn main(){}`})
		if err := os.Mkdir(filepath.Join(root, "child.rs"), 0755); err != nil {
			t.Fatal(err)
		}
		_, _, err := loadCrate(filepath.Join(root, "main.rs"))
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatal(err)
		}
	})
}

func TestPhysicalModuleCycle(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root := writeCrate(t, map[string]string{"main.rs": `mod again; fn main(){}`})
			main := filepath.Join(root, "main.rs")
			again := filepath.Join(root, "again.rs")
			var err error
			if kind == "symlink" {
				err = os.Symlink(main, again)
			} else {
				err = os.Link(main, again)
			}
			if err != nil {
				t.Skip(err)
			}
			_, _, err = loadCrate(main)
			if err == nil || !strings.Contains(err.Error(), "module file cycle") {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestFileAPIsAndAllSourceProtection(t *testing.T) {
	root := writeCrate(t, map[string]string{
		"main.rs": `mod math; fn main(){println!("{}",math::answer());}`,
		"math.rs": `pub fn answer()->i32{42}`,
	})
	main := filepath.Join(root, "main.rs")
	module := filepath.Join(root, "math.rs")
	if err := CheckFile(main); err != nil {
		t.Fatal(err)
	}
	code, err := EmitCFile(main)
	if err != nil || !strings.Contains(code, "int main(void)") {
		t.Fatalf("emit: %v", err)
	}
	if err := WriteC(main, filepath.Join(root, "main.c")); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(module)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{main, module} {
		if err := Build(main, output); err == nil {
			t.Fatalf("Build overwrote source %s", output)
		}
		if err := WriteC(main, output); err == nil {
			t.Fatalf("WriteC overwrote source %s", output)
		}
	}
	alias := filepath.Join(root, "source-alias")
	if err := os.Link(module, alias); err == nil {
		if err := Build(main, alias); err == nil {
			t.Fatal("Build overwrote source alias")
		}
		if err := WriteC(main, alias); err == nil {
			t.Fatal("WriteC overwrote source alias")
		}
	}
	after, err := os.ReadFile(module)
	if err != nil || string(after) != string(original) {
		t.Fatal("module source changed", err)
	}
	if err := Check(`mod math; fn main(){}`); err == nil {
		t.Fatal("source-only API silently loaded external module")
	}
}

func TestWriteCFailurePreservesPriorOutput(t *testing.T) {
	root := writeCrate(t, map[string]string{"main.rs": `mod math; fn main(){}`, "math.rs": `fn bad()->i32{true}`})
	output := filepath.Join(root, "crate.c")
	if err := os.WriteFile(output, []byte("prior output"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteC(filepath.Join(root, "main.rs"), output); err == nil {
		t.Fatal("invalid module accepted")
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "prior output" {
		t.Fatal("failed emission replaced output", err)
	}
	entries, err := filepath.Glob(filepath.Join(root, ".ferrugo-emit-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files %v: %v", entries, err)
	}
}

func TestWriteCUsesSameNormalizedOutputForGuardAndPublication(t *testing.T) {
	root := writeCrate(t, map[string]string{
		"crate/main.rs": `mod a; fn main(){a::run();}`,
		"crate/a.rs":    `pub fn run(){}`,
	})
	crate := filepath.Join(root, "crate")
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(filepath.Join(crate, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(other, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(other, "link")
	if err := os.Symlink(filepath.Join(crate, "subdir"), link); err != nil {
		t.Skip(err)
	}
	// Keep .. in the actual argument; filepath.Join would erase the regression.
	output := link + string(filepath.Separator) + ".." + string(filepath.Separator) + "a.rs"
	module := filepath.Join(crate, "a.rs")
	before, err := os.ReadFile(module)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteC(filepath.Join(crate, "main.rs"), output); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(module)
	if err != nil || string(after) != string(before) {
		t.Fatal("module source replaced through symlink/.. output", err)
	}
	normalized := filepath.Join(other, "a.rs")
	code, err := os.ReadFile(normalized)
	if err != nil || !strings.Contains(string(code), "Generated by Ferrugo") {
		t.Fatalf("normalized output was not used: %v", err)
	}
}
