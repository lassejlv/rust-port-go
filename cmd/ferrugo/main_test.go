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
