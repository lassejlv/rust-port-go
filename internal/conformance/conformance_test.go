// Package conformance_test exercises complete Ferrugo builds against committed
// language fixtures. Optional rustc differential runs use the identical sources.
package conformance_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/internal/compiler"
)

type result struct {
	stdout []byte
	stderr []byte
	status int
}

// Requested differential validation must fail when rustc is unavailable, rather
// than quietly turning a differential run into a Ferrugo-only run.
func configuredRustc(t *testing.T) string {
	t.Helper()
	requested, enabled := os.LookupEnv("FERRUGO_RUSTC")
	if !enabled {
		return ""
	}
	if strings.TrimSpace(requested) == "" {
		t.Fatal("FERRUGO_RUSTC is set but empty; provide a rustc executable or unset the variable")
	}
	path, err := exec.LookPath(requested)
	if err != nil {
		t.Fatalf("FERRUGO_RUSTC=%q is unavailable: %v", requested, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, absolute, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("FERRUGO_RUSTC=%q failed --version: %v: %s", requested, err, out)
	}
	if !bytes.HasPrefix(out, []byte("rustc ")) {
		t.Fatalf("FERRUGO_RUSTC=%q does not report a rustc version: %q", requested, out)
	}
	t.Logf("differential compiler: %s", strings.TrimSpace(string(out)))
	return absolute
}

func fixtures(t *testing.T, category string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", "conformance", category, "*.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatalf("no %s fixtures found", category)
	}
	for i, path := range paths {
		paths[i], err = filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func run(t *testing.T, path string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("executable exceeded time limit: %v", ctx.Err())
	}
	got := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatalf("execute %s: %v", path, err)
		}
		got.status = exited.ExitCode()
	}
	return got
}

func compileRust(t *testing.T, rustc, source, output string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// Explicit overflow checks keep rustc's runtime profile aligned with Ferrugo.
	command := exec.CommandContext(ctx, rustc, "--edition=2021", "--crate-name", "ferrugo_conformance", "-A", "warnings", "-C", "overflow-checks=yes", source, "-o", output)
	diagnostics, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("rustc exceeded time limit: %v", ctx.Err())
	}
	return diagnostics, err
}

func expect(t *testing.T, label string, got result, want []byte, status int) {
	t.Helper()
	if !bytes.Equal(got.stdout, want) || got.status != status {
		t.Fatalf("%s: stdout=%q status=%d; want stdout=%q status=%d\nstderr: %s", label, got.stdout, got.status, want, status, got.stderr)
	}
}

func TestConformance(t *testing.T) {
	rustc := configuredRustc(t)
	if rustc == "" {
		t.Log("FERRUGO_RUSTC unset: running Ferrugo goldens; rustc differential comparison is disabled")
	}
	// Bound subprocess concurrency independently of go test -parallel. This keeps
	// the optional rustc run modest on small development and CI machines.
	slots := make(chan struct{}, 4)
	for _, source := range fixtures(t, "run") {
		name := strings.TrimSuffix(filepath.Base(source), ".rs")
		t.Run("run/"+name, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			prefix := strings.TrimSuffix(source, ".rs")
			want := read(t, prefix+".stdout")
			status := 0
			if data, err := os.ReadFile(prefix + ".status"); err == nil {
				value, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
				if parseErr != nil || value < 0 || value > 255 {
					t.Fatalf("invalid status golden %q", data)
				}
				status = value
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}
			dir := t.TempDir()
			ferrugoOutput := filepath.Join(dir, "ferrugo-program")
			if err := compiler.Build(source, ferrugoOutput); err != nil {
				t.Fatalf("Ferrugo compile: %v", err)
			}
			actual := run(t, ferrugoOutput)
			expect(t, "Ferrugo", actual, want, status)
			if rustc != "" {
				rustOutput := filepath.Join(dir, "rustc-program")
				if diagnostics, err := compileRust(t, rustc, source, rustOutput); err != nil {
					t.Fatalf("rustc rejected run fixture: %v\n%s", err, diagnostics)
				}
				reference := run(t, rustOutput)
				expect(t, "rustc", reference, want, status)
				expect(t, "Ferrugo versus rustc", actual, reference.stdout, reference.status)
			}
		})
	}
	for _, category := range []string{"compile-fail", "unsupported"} {
		for _, source := range fixtures(t, category) {
			name := strings.TrimSuffix(filepath.Base(source), ".rs")
			t.Run(category+"/"+name, func(t *testing.T) {
				t.Parallel()
				slots <- struct{}{}
				defer func() { <-slots }()
				if err := compiler.Check(string(read(t, source))); err == nil {
					t.Fatalf("Ferrugo accepted %s fixture", category)
				}
				if rustc != "" {
					diagnostics, err := compileRust(t, rustc, source, filepath.Join(t.TempDir(), "rustc-program"))
					if category == "compile-fail" {
						if err == nil {
							t.Fatal("rustc accepted invalid-Rust fixture")
						}
						var exited *exec.ExitError
						if !errors.As(err, &exited) {
							t.Fatalf("rustc could not execute: %v\n%s", err, diagnostics)
						}
					} else if err != nil {
						t.Fatalf("rustc rejected valid but unsupported Rust: %v\n%s", err, diagnostics)
					}
				}
			})
		}
	}
}
