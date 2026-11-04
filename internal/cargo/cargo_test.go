package cargo

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runCommand(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestNewAndInitPreserveExistingFiles(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "hello")
	if code, _, stderr := runCommand("new", root); code != 0 {
		t.Fatalf("new: %d %s", code, stderr)
	}
	manifest := filepath.Join(root, "Cargo.toml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"new", "init"} {
		if code, _, _ := runCommand(cmd, root); code == 0 {
			t.Fatalf("%s unexpectedly replaced existing package", cmd)
		}
	}
	after, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing manifest was modified")
	}
	existing := t.TempDir()
	if err := os.Mkdir(filepath.Join(existing, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(existing, "src", "main.rs")
	if err := os.WriteFile(source, []byte("KEEP"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCommand("init", existing, "--name", "example"); code == 0 {
		t.Fatal("init overwrote existing source")
	}
	content, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "KEEP" {
		t.Fatal("source overwritten")
	}
	if _, err := os.Stat(filepath.Join(existing, "Cargo.toml")); !os.IsNotExist(err) {
		t.Fatal("init partially created manifest over existing source")
	}
}

func TestCLIRejectsUnknownArguments(t *testing.T) {
	for _, args := range [][]string{{"publish"}, {"build", "--locked"}, {"new"}, {"new", "a", "b"}, {"init", "--lib"}, {"run", "arg"}, {"build", "--manifest-path"}, {"new", "x", "--name", "fn"}, {"new", "x", "--vcs", "git"}} {
		if code, _, stderr := runCommand(args...); code == 0 || !strings.Contains(stderr, "error:") {
			t.Fatalf("%v: expected CLI error, got %d %s", args, code, stderr)
		}
	}
}

func TestRunArgumentsAfterSeparator(t *testing.T) {
	opts, err := parseOptions("run", []string{"--release", "--", "--help", "arg with spaces"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.release || len(opts.programArgs) != 2 || opts.programArgs[0] != "--help" || opts.programArgs[1] != "arg with spaces" {
		t.Fatalf("wrong options: %+v", opts)
	}
}

func TestBuildCheckRunClean(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler unavailable")
	}
	root := filepath.Join(t.TempDir(), "greeting")
	if code, _, stderr := runCommand("new", root); code != 0 {
		t.Fatalf("new: %s", stderr)
	}
	manifest := filepath.Join(root, "Cargo.toml")
	if code, _, stderr := runCommand("check", "--manifest-path", manifest); code != 0 {
		t.Fatalf("check: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
		t.Fatal("check created artifacts")
	}
	if code, stdout, stderr := runCommand("run", "--manifest-path", manifest, "--", "--help"); code != 0 || stdout != "Hello, Ferrugo!\n" {
		t.Fatalf("run: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if code, _, stderr := runCommand("build", "--release", "--manifest-path", manifest); code != 0 {
		t.Fatalf("release: %s", stderr)
	}
	unrelated := filepath.Join(root, "target", "debug", "keep.txt")
	if err := os.WriteFile(unrelated, []byte("KEEP"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCommand("clean", "--manifest-path", manifest); code != 0 {
		t.Fatalf("clean: %s", stderr)
	}
	m, err := readManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"debug", "release"} {
		output, owned := artifactPaths(m, profile)
		for _, p := range []string{output, owned} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatalf("artifact remains after clean: %s", p)
			}
		}
	}
	content, err := os.ReadFile(unrelated)
	if err != nil || string(content) != "KEEP" {
		t.Fatal("clean removed unrelated target file")
	}
}

func TestCheckDoesNotAcceptInvalidProgram(t *testing.T) {
	manifest := manifestFixture(t, "[package]\nname = \"bad\"\nversion = \"0.1.0\"\n")
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifest), "src", "main.rs"), []byte("fn main() { let n: bool = 42; }"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCommand("check", "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "error:") {
		t.Fatalf("invalid source accepted: %d %s", code, stderr)
	}
}

func TestCleanPreservesUnownedOutputs(t *testing.T) {
	manifest := manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	m, err := readManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	output, _ := artifactPaths(m, "debug")
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("UNOWNED"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := clean(m); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(output)
	if err != nil || string(content) != "UNOWNED" {
		t.Fatal("clean removed unowned binary")
	}
}

func TestRejectSymlinkTarget(t *testing.T) {
	manifest := manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	root := filepath.Dir(manifest)
	external := t.TempDir()
	if err := os.Symlink(external, filepath.Join(root, "target")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if code, _, _ := runCommand("build", "--manifest-path", manifest); code == 0 {
		t.Fatal("build followed target symlink")
	}
	if code, _, _ := runCommand("clean", "--manifest-path", manifest); code == 0 {
		t.Fatal("clean followed target symlink")
	}
}

func TestFailedRebuildPreservesWorkingBinary(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler unavailable")
	}
	manifest := manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	root := filepath.Dir(manifest)
	if code, _, stderr := runCommand("build", "--manifest-path", manifest); code != 0 {
		t.Fatal(stderr)
	}
	m, err := readManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	output, _ := artifactPaths(m, "debug")
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "main.rs"), []byte("fn main() { let x: bool = 1; }"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runCommand("build", "--manifest-path", manifest); code == 0 {
		t.Fatal("invalid rebuild succeeded")
	}
	after, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed rebuild changed the working executable")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(output), ".ferrugo-build-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) > 0 {
		t.Fatalf("temporary build files leaked: %v", leftovers)
	}
}

func TestCleanRefusesSymlinkArtifactAndInvalidMarker(t *testing.T) {
	for _, test := range []string{"artifact symlink", "marker symlink", "invalid marker"} {
		t.Run(test, func(t *testing.T) {
			manifest := manifestFixture(t, "[package]\nname = \"demo\"\nversion = \"0.1.0\"\n")
			m, err := readManifest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			output, owned := artifactPaths(m, "debug")
			if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
				t.Fatal(err)
			}
			external := filepath.Join(t.TempDir(), "keep")
			if err := os.WriteFile(external, []byte("KEEP"), 0644); err != nil {
				t.Fatal(err)
			}
			switch test {
			case "artifact symlink":
				if err := os.Symlink(external, output); err != nil {
					t.Skip(err)
				}
				if err := os.WriteFile(owned, []byte(marker(m)), 0644); err != nil {
					t.Fatal(err)
				}
			case "marker symlink":
				if err := os.WriteFile(output, []byte("KEEP"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, owned); err != nil {
					t.Skip(err)
				}
			default:
				if err := os.WriteFile(output, []byte("KEEP"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(owned, []byte("FOREIGN"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := clean(m); err == nil {
				t.Fatal("expected refusal to clean unsafe artifact")
			}
			content, err := os.ReadFile(external)
			if err != nil || string(content) != "KEEP" {
				t.Fatal("external file was modified")
			}
			if _, err := os.Lstat(output); err != nil {
				t.Fatal("unsafe artifact was removed")
			}
		})
	}
}

func TestExternalModuleCheckAndRun(t *testing.T) {
	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("C compiler unavailable")
	}
	root := filepath.Join(t.TempDir(), "modular")
	if code, _, stderr := runCommand("new", root); code != 0 {
		t.Fatalf("new: %s", stderr)
	}
	mainPath := filepath.Join(root, "src", "main.rs")
	if err := os.WriteFile(mainPath, []byte("mod arithmetic;\nuse crate::arithmetic::double as twice;\nfn main() { println!(\"{}\", twice(21)); }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	modulePath := filepath.Join(root, "src", "arithmetic.rs")
	if err := os.WriteFile(modulePath, []byte("pub fn double(n: i32) -> i32 { n * 2 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "Cargo.toml")
	if code, stdout, stderr := runCommand("check", "--manifest-path", manifest); code != 0 || stdout != "" {
		t.Fatalf("check module package: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
		t.Fatal("module check created build artifacts")
	}
	if code, stdout, stderr := runCommand("run", "--manifest-path", manifest); code != 0 || stdout != "42\n" {
		t.Fatalf("run module package: code=%d stdout=%q stderr=%s", code, stdout, stderr)
	}
	if err := os.WriteFile(modulePath, []byte("pub fn double(n: i32) -> i32 { true }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := runCommand("check", "--manifest-path", manifest); code == 0 || !strings.Contains(stderr, "arithmetic.rs") {
		t.Fatalf("check did not report imported-file error: code=%d stderr=%s", code, stderr)
	}
}
