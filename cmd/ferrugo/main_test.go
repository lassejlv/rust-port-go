package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDriver(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "hello.rs")
	program := `fn main(){println!("hello");}`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name     string
		args     []string
		status   int
		out, err string
	}{
		{"version", []string{"--version"}, 0, "ferrugo 0.1.0", ""},
		{"help", []string{"--help"}, 0, "Usage:", ""},
		{"check", []string{"--check", source}, 0, "", ""},
		{"emit", []string{source, "--emit=c", "-o", "-"}, 0, "int main(void)", ""},
		{"unknown", []string{"--wat", source}, 1, "", "unsupported option"},
		{"missing", []string{"--check"}, 1, "", "source file required"},
		{"conflict", []string{"--check", "--emit=c", source}, 1, "", "cannot be combined"},
		{"overwrite", []string{"--emit=c", source, "-o", source}, 1, "", "overwrite source"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			status := run(tt.args, &out, &errs)
			if status != tt.status || !strings.Contains(out.String(), tt.out) || !strings.Contains(errs.String(), tt.err) {
				t.Fatalf("status %d, stdout=%q stderr=%q", status, out.String(), errs.String())
			}
		})
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != program {
		t.Fatal("source changed", err)
	}
}

func TestDriverLoadsModulesForCheckAndEmission(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.rs")
	module := filepath.Join(dir, "math.rs")
	if err := os.WriteFile(source, []byte(`mod math; fn main(){println!("{}",math::answer());}`), 0600); err != nil {
		t.Fatal(err)
	}
	original := `pub fn answer()->i32{42}`
	if err := os.WriteFile(module, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		args   []string
		status int
		want   string
	}{
		{"check", []string{"--check", source}, 0, ""},
		{"emit stdout", []string{"--emit=c", source, "-o", "-"}, 0, "int main(void)"},
		{"protect module", []string{"--emit=c", source, "-o", module}, 1, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			status := run(tt.args, &out, &errs)
			if status != tt.status || !strings.Contains(out.String(), tt.want) {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, out.String(), errs.String())
			}
		})
	}
	data, err := os.ReadFile(module)
	if err != nil || string(data) != original {
		t.Fatal("module source changed", err)
	}
}

func TestDriverLibraryChecking(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "lib.rs")
	if err := os.WriteFile(source, []byte(`pub fn add(a:i32,b:i32)->i32{a+b} fn main(n:i32)->i32{n}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		args   []string
		status int
		want   string
	}{
		{"check lib", []string{"--check", "--crate-type=lib", source}, 0, ""},
		{"check binary entrypoint", []string{"--check", source}, 1, "main must"},
		{"library artifacts unsupported", []string{"--crate-type=lib", source}, 1, "library output artifacts"},
		{"wrong kind", []string{"--check", "--crate-type", "rlib", source}, 1, "only bin or lib"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			status := run(tt.args, &out, &errs)
			if status != tt.status || !strings.Contains(errs.String(), tt.want) {
				t.Fatalf("status=%d out=%q errors=%q", status, out.String(), errs.String())
			}
		})
	}
}
