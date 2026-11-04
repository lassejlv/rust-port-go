// Package cargo implements Ferrugo's initial, local-only Cargo command subset.
package cargo

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/compiler"
)

const Version = "0.1.0"

const help = `ferrugo-cargo 0.1.0 — Ferrugo's bootstrap package builder

Usage: ferrugo-cargo <command> [options]

Commands:
  new <path>      Create a binary package in a new directory
  init [path]     Create a binary package in an existing directory
  build          Compile src/main.rs to target/debug/<name>
  check          Parse and type-check src/main.rs without producing a binary
  run [-- args]   Build and run the binary, forwarding arguments and streams
  clean          Remove only artifacts created by Ferrugo
  version        Print version
  help           Print this help

Options:
  --manifest-path <Cargo.toml>   Select a manifest (build/check/run/clean)
  --release                     Use target/release (build/run; same backend)
  --name <name>                 Set the package name (new/init)
  --bin                         Create a binary package (new/init; default)
  --vcs none                    Skip version control setup (new/init; default)

This bootstrap supports one local binary package, no dependencies, and a strict
Cargo.toml subset. It does not invoke rustc or the upstream Cargo executable.
`

type options struct {
	manifest, name     string
	release            bool
	paths, programArgs []string
}

// Run executes a Cargo-style command and returns the intended process status.
// All streams are supplied explicitly, so embedding callers retain control.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	fail := func(err error) int { fmt.Fprintf(stderr, "error: %v\n", err); return 1 }
	if len(args) == 0 {
		fmt.Fprint(stdout, help)
		return 0
	}
	command := args[0]
	if command == "--version" || command == "-V" {
		command = "version"
	}
	if command == "--help" || command == "-h" {
		command = "help"
	}
	if command == "version" || command == "help" {
		if len(args) > 1 {
			return fail(fmt.Errorf("%s does not accept arguments", command))
		}
		if command == "version" {
			fmt.Fprintf(stdout, "ferrugo-cargo %s\n", Version)
		} else {
			fmt.Fprint(stdout, help)
		}
		return 0
	}
	switch command {
	case "new", "init", "build", "check", "run", "clean":
	default:
		return fail(fmt.Errorf("unknown command %q; use ferrugo-cargo help", command))
	}
	for _, arg := range args[1:] {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "-h" {
			fmt.Fprint(stdout, help)
			return 0
		}
	}
	opts, err := parseOptions(command, args[1:])
	if err != nil {
		return fail(err)
	}
	if command == "new" || command == "init" {
		if err := createPackage(command, opts); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stderr, "Created Ferrugo binary package")
		return 0
	}
	path, err := findManifest(opts.manifest)
	if err != nil {
		return fail(err)
	}
	m, err := readManifest(path)
	if err != nil {
		return fail(err)
	}
	if command == "clean" {
		if err := clean(m); err != nil {
			return fail(err)
		}
		fmt.Fprintln(stderr, "Removed Ferrugo build artifacts")
		return 0
	}
	source := filepath.Join(m.Root, "src", "main.rs")
	if command == "check" {
		if err := compiler.CheckFile(source); err != nil {
			return fail(err)
		}
		fmt.Fprintf(stderr, "Checked %s v%s\n", m.Name, m.Version)
		return 0
	}
	output, err := build(m, source, opts.release, stderr)
	if err != nil {
		return fail(err)
	}
	if command == "build" {
		return 0
	}
	child := exec.Command(output, opts.programArgs...)
	child.Stdin, child.Stdout, child.Stderr = stdin, stdout, stderr
	if err := child.Run(); err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if code := e.ExitCode(); code >= 0 {
				return code
			}
			return 1
		}
		return fail(fmt.Errorf("run %s: %w", m.Name, err))
	}
	return 0
}

func parseOptions(command string, args []string) (options, error) {
	var o options
	creating := command == "new" || command == "init"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := func(flag string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", flag)
			}
			i++
			return args[i], nil
		}
		switch {
		case arg == "--":
			if command == "run" {
				o.programArgs = append(o.programArgs, args[i+1:]...)
				return o, nil
			}
			if creating {
				o.paths = append(o.paths, args[i+1:]...)
				return o, nil
			}
			return o, fmt.Errorf("unexpected -- for %s", command)
		case arg == "--release":
			if command != "build" && command != "run" {
				return o, fmt.Errorf("--release is only supported by build and run")
			}
			o.release = true
		case arg == "--manifest-path" || strings.HasPrefix(arg, "--manifest-path="):
			if creating {
				return o, fmt.Errorf("--manifest-path is not supported by %s", command)
			}
			if o.manifest != "" {
				return o, fmt.Errorf("duplicate --manifest-path")
			}
			var err error
			if strings.HasPrefix(arg, "--manifest-path=") {
				o.manifest = strings.TrimPrefix(arg, "--manifest-path=")
			} else {
				o.manifest, err = value(arg)
			}
			if err != nil {
				return o, err
			}
			if o.manifest == "" {
				return o, fmt.Errorf("--manifest-path requires a value")
			}
		case arg == "--name" || strings.HasPrefix(arg, "--name="):
			if !creating {
				return o, fmt.Errorf("--name is only supported by new and init")
			}
			if o.name != "" {
				return o, fmt.Errorf("duplicate --name")
			}
			var err error
			if strings.HasPrefix(arg, "--name=") {
				o.name = strings.TrimPrefix(arg, "--name=")
			} else {
				o.name, err = value(arg)
			}
			if err != nil {
				return o, err
			}
			if o.name == "" {
				return o, fmt.Errorf("--name requires a value")
			}
		case arg == "--bin":
			if !creating {
				return o, fmt.Errorf("--bin is only supported by new and init")
			}
		case arg == "--vcs":
			if !creating {
				return o, fmt.Errorf("--vcs is only supported by new and init")
			}
			v, err := value(arg)
			if err != nil {
				return o, err
			}
			if v != "none" {
				return o, fmt.Errorf("only --vcs none is supported")
			}
		case strings.HasPrefix(arg, "-"):
			return o, fmt.Errorf("unsupported option %q", arg)
		default:
			if !creating {
				return o, fmt.Errorf("unexpected argument %q; pass program arguments after --", arg)
			}
			o.paths = append(o.paths, arg)
		}
	}
	return o, nil
}

func createPackage(command string, o options) error {
	if len(o.paths) > 1 || (command == "new" && len(o.paths) != 1) {
		return fmt.Errorf("usage: ferrugo-cargo %s <path> [--name <name>]", command)
	}
	path := "."
	if len(o.paths) == 1 {
		path = o.paths[0]
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	name := o.name
	if name == "" {
		name = filepath.Base(root)
	}
	if !validName(name) {
		return fmt.Errorf("invalid or reserved package name %q; use --name", name)
	}
	if command == "new" {
		if _, err := os.Lstat(root); err == nil {
			return fmt.Errorf("destination %s already exists; use init", root)
		} else if !os.IsNotExist(err) {
			return err
		}
	} else if info, err := os.Stat(root); err != nil {
		return fmt.Errorf("init directory: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("init path is not a directory")
	}
	for _, relative := range []string{"Cargo.toml", "src/main.rs", "src/lib.rs", "build.rs", "src/bin"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			return fmt.Errorf("refusing to overwrite or replace existing %s", relative)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	src := filepath.Join(root, "src")
	if info, err := os.Lstat(src); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("src must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if command == "new" {
		if err := os.Mkdir(root, 0755); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(src, 0755); err != nil {
		return err
	}
	manifest := fmt.Sprintf("[package]\nname = %q\nversion = \"0.1.0\"\nedition = \"2021\"\n", name)
	manifestPath := filepath.Join(root, "Cargo.toml")
	if err := writeExclusive(manifestPath, manifest); err != nil {
		return err
	}
	if err := writeExclusive(filepath.Join(src, "main.rs"), "fn main() {\n    println!(\"Hello, Ferrugo!\");\n}\n"); err != nil {
		// This path was exclusively created by this operation. Existing source files
		// are never removed, even if another process creates one during initialization.
		_ = os.Remove(manifestPath)
		return err
	}
	return nil
}

func writeExclusive(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := io.WriteString(f, content)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func realDir(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing non-directory or symlink build path %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Mkdir(path, 0755)
}

func artifactPaths(m *Manifest, profile string) (string, string) {
	output := filepath.Join(m.Root, "target", profile, m.Name)
	if runtime.GOOS == "windows" {
		output += ".exe"
	}
	return output, output + ".ferrugo-artifact"
}

func marker(m *Manifest) string { return "ferrugo artifact v1\n" + m.Name + "\n" }

func build(m *Manifest, source string, release bool, stderr io.Writer) (string, error) {
	profile := "debug"
	if release {
		profile = "release"
	}
	if err := realDir(filepath.Join(m.Root, "target")); err != nil {
		return "", err
	}
	if err := realDir(filepath.Join(m.Root, "target", profile)); err != nil {
		return "", err
	}
	output, owned := artifactPaths(m, profile)
	for _, p := range []string{output, owned} {
		if info, err := os.Lstat(p); err == nil {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("refusing non-regular artifact path %s", p)
			}
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if data, err := os.ReadFile(owned); err == nil {
		if string(data) != marker(m) {
			return "", fmt.Errorf("invalid Ferrugo artifact marker %s", owned)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	fmt.Fprintf(stderr, "Compiling %s v%s\n", m.Name, m.Version)
	temp, err := os.CreateTemp(filepath.Dir(output), ".ferrugo-build-*")
	if err != nil {
		return "", err
	}
	temporary := temp.Name()
	if err := temp.Close(); err != nil {
		return "", err
	}
	defer os.Remove(temporary)
	if err := compiler.Build(source, temporary); err != nil {
		return "", err
	}
	if err := os.Rename(temporary, output); err != nil {
		return "", err
	}
	if err := os.WriteFile(owned, []byte(marker(m)), 0644); err != nil {
		return "", err
	}
	fmt.Fprintf(stderr, "Finished %s: %s\n", profile, output)
	return output, nil
}

func clean(m *Manifest) error {
	target := filepath.Join(m.Root, "target")
	if info, err := os.Lstat(target); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing non-directory or symlink target path")
	}
	for _, profile := range []string{"debug", "release"} {
		directory := filepath.Join(target, profile)
		if info, err := os.Lstat(directory); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing non-directory or symlink build profile %s", profile)
		}
		output, owned := artifactPaths(m, profile)
		info, err := os.Lstat(owned)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular artifact marker %s", owned)
		}
		data, err := os.ReadFile(owned)
		if err != nil {
			return err
		}
		if string(data) != marker(m) {
			return fmt.Errorf("invalid Ferrugo artifact marker %s", owned)
		}
		if info, err := os.Lstat(output); err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("refusing non-regular build artifact %s", output)
			}
			if err := os.Remove(output); err != nil {
				return err
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Remove(owned); err != nil {
			return err
		}
		_ = os.Remove(directory) // Remove empty directories only; unrelated files survive.
	}
	_ = os.Remove(target)
	return nil
}
