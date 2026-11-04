package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Build compiles a Rust source file to a native executable using the C11 backend.
// CC may select one compiler executable; shell command strings are never evaluated.
func Build(sourcePath, outputPath string) error {
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if sourceAbs == outputAbs {
		return fmt.Errorf("output would overwrite source file %s", sourcePath)
	}
	if a, err := os.Stat(sourceAbs); err == nil {
		if b, err := os.Stat(outputAbs); err == nil && os.SameFile(a, b) {
			return fmt.Errorf("output refers to source file %s", sourcePath)
		}
	}
	source, err := os.ReadFile(sourceAbs)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	code, err := EmitC(string(source))
	if err != nil {
		return fmt.Errorf("%s:%w", sourcePath, err)
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	ccPath, err := exec.LookPath(cc)
	if err != nil {
		return fmt.Errorf("C11 compiler %q unavailable (set CC to an executable path): %w", cc, err)
	}
	dir, err := os.MkdirTemp("", "ferrugo-c-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	cPath := filepath.Join(dir, "crate.c")
	if err = os.WriteFile(cPath, []byte(code), 0600); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(outputAbs), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	// Create the executable beside its destination so publication is an atomic rename.
	target, err := os.CreateTemp(filepath.Dir(outputAbs), ".ferrugo-build-*")
	if err != nil {
		return err
	}
	temporary := target.Name()
	target.Close()
	defer os.Remove(temporary)
	command := exec.Command(ccPath, "-std=c11", "-O0", cPath, "-o", temporary)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("C backend failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	if err = os.Chmod(temporary, 0755); err != nil {
		return err
	}
	if err = os.Rename(temporary, outputAbs); err != nil {
		return fmt.Errorf("publish executable: %w", err)
	}
	return nil
}
