// Package cargoconformance_test exercises local package graphs through Ferrugo
// and, when requested, an installed upstream Cargo without network access.
package cargoconformance_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/internal/cargo"
)

type result struct {
	stdout, stderr []byte
	status         int
}
type referenceTools struct{ cargo, rustc string }
type fixture struct{ name, category, path string }

func executable(t *testing.T, value, label string) string {
	t.Helper()
	if strings.TrimSpace(value) == "" {
		t.Fatalf("%s is set but empty; provide an executable or unset the variable", label)
	}
	path, err := exec.LookPath(value)
	if err != nil {
		t.Fatalf("%s=%q is unavailable: %v", label, value, err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return absolute
}
func version(t *testing.T, path, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version failed: %v\n%s", path, err, out)
	}
	if !bytes.HasPrefix(out, []byte(prefix+" ")) {
		t.Fatalf("%s does not report a %s version: %q", path, prefix, out)
	}
	t.Logf("package reference: %s", strings.TrimSpace(string(out)))
}
func configuredReference(t *testing.T) *referenceTools {
	t.Helper()
	requested, enabled := os.LookupEnv("FERRUGO_CARGO")
	if !enabled {
		return nil
	}
	tools := &referenceTools{cargo: executable(t, requested, "FERRUGO_CARGO")}
	version(t, tools.cargo, "cargo")
	rustc, explicit := os.LookupEnv("FERRUGO_RUSTC")
	if !explicit {
		extension := ""
		if strings.EqualFold(filepath.Ext(tools.cargo), ".exe") {
			extension = ".exe"
		}
		rustc = filepath.Join(filepath.Dir(tools.cargo), "rustc"+extension)
	}
	label := "rustc beside FERRUGO_CARGO (or set FERRUGO_RUSTC)"
	if explicit {
		label = "FERRUGO_RUSTC"
	}
	tools.rustc = executable(t, rustc, label)
	version(t, tools.rustc, "rustc")
	return tools
}
func fixtures(t *testing.T) []fixture {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", "testdata", "packages"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []fixture
	for _, category := range []string{"run", "check", "reject"} {
		entries, err := os.ReadDir(filepath.Join(base, category))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			t.Fatalf("no %s package fixtures", category)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				t.Fatalf("unexpected fixture entry %s/%s", category, entry.Name())
			}
			cases = append(cases, fixture{name: entry.Name(), category: category, path: filepath.Join(base, category, entry.Name())})
		}
	}
	return cases
}
func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Each implementation receives its own full copy. In particular Cargo.lock,
// target trees and marker files can never be written into committed fixtures.
func copyFixture(t *testing.T, source string) string {
	t.Helper()
	destination := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("fixture contains non-regular file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return destination
}

func ferrugo(t *testing.T, args ...string) result {
	t.Helper()
	completed := make(chan result, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		status := cargo.Run(args, strings.NewReader(""), &stdout, &stderr)
		completed <- result{stdout: stdout.Bytes(), stderr: stderr.Bytes(), status: status}
	}()
	select {
	case got := <-completed:
		return got
	case <-time.After(30 * time.Second):
		// cargo.Run has no cancellation API. This is a bounded test wait for trusted,
		// finite fixtures, not a claim that the child work has been hard-cancelled.
		t.Fatal("Ferrugo package fixture exceeded 30-second test wait")
	}
	return result{}
}
func referenceEnv(home, target, rustc string) []string {
	replaced := map[string]bool{"CARGO_HOME": true, "CARGO_TARGET_DIR": true, "CARGO_NET_OFFLINE": true, "RUSTC": true, "RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true, "RUSTFLAGS": true, "CARGO_ENCODED_RUSTFLAGS": true}
	env := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !replaced[key] {
			env = append(env, entry)
		}
	}
	return append(env, "CARGO_HOME="+home, "CARGO_TARGET_DIR="+target, "CARGO_NET_OFFLINE=true", "RUSTC="+rustc)
}
func upstream(t *testing.T, tools *referenceTools, manifest, command, home, target string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tools.cargo, command, "--quiet", "--offline", "--jobs", "1", "--manifest-path", manifest)
	cmd.Env = referenceEnv(home, target, tools.rustc)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("upstream Cargo exceeded timeout: %v", ctx.Err())
	}
	got := result{stdout: stdout.Bytes(), stderr: stderr.Bytes()}
	if err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatalf("execute upstream Cargo: %v", err)
		}
		got.status = exited.ExitCode()
	}
	return got
}
func expectSuccess(t *testing.T, label string, got result, want []byte) {
	t.Helper()
	if got.status != 0 || !bytes.Equal(got.stdout, want) {
		t.Fatalf("%s: status=%d stdout=%q, want status0 stdout=%q\nstderr: %s", label, got.status, got.stdout, want, got.stderr)
	}
}
func expectRejection(t *testing.T, label string, got result) {
	t.Helper()
	if got.status == 0 {
		t.Fatalf("%s accepted invalid package fixture; stdout=%q stderr=%q", label, got.stdout, got.stderr)
	}
	if len(got.stderr) == 0 {
		t.Fatalf("%s rejected without a diagnostic", label)
	}
}

func TestPackageConformance(t *testing.T) {
	tools := configuredReference(t)
	if tools == nil {
		t.Log("FERRUGO_CARGO unset: Ferrugo package fixtures only; upstream Cargo comparison disabled")
	}
	slots := make(chan struct{}, 3)
	for _, f := range fixtures(t) {
		t.Run(f.category+"/"+f.name, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			ownRoot := copyFixture(t, f.path)
			manifest := filepath.Join(ownRoot, "app", "Cargo.toml")
			checked := ferrugo(t, "check", "--manifest-path", manifest)
			if f.category == "reject" {
				expectRejection(t, "Ferrugo check", checked)
			} else {
				expectSuccess(t, "Ferrugo check", checked, nil)
				if _, err := os.Stat(filepath.Join(ownRoot, "app", "target")); !os.IsNotExist(err) {
					t.Fatalf("Ferrugo check produced target artifacts or unexpected stat failure: %v", err)
				}
			}
			var want []byte
			if f.category == "run" {
				want = read(t, filepath.Join(f.path, "output.stdout"))
				expectSuccess(t, "Ferrugo run", ferrugo(t, "run", "--manifest-path", manifest), want)
			}
			if tools != nil {
				referenceRoot := copyFixture(t, f.path)
				home, target := t.TempDir(), t.TempDir()
				referenceManifest := filepath.Join(referenceRoot, "app", "Cargo.toml")
				original := upstream(t, tools, referenceManifest, "check", home, target)
				if f.category == "reject" {
					expectRejection(t, "upstream Cargo check", original)
				} else {
					expectSuccess(t, "upstream Cargo check", original, nil)
					if f.category == "run" {
						expectSuccess(t, "upstream Cargo run", upstream(t, tools, referenceManifest, "run", home, target), want)
					}
				}
			}
		})
	}
}
