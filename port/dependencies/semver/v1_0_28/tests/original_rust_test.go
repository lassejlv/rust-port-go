package semver_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOriginalRustProfiles(t *testing.T) {
	if os.Getenv("FERRUGO_RUN_SEMVER_ORIGINAL_TESTS") != "1" {
		t.Skip("set FERRUGO_RUN_SEMVER_ORIGINAL_TESTS=1 to compile all original Rust test bodies unchanged")
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(root, "tools/port-semver/original-tests/run.py"), "--root", root, "--offline")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("original Rust test profiles: %v\n%s", err, output)
	}
}
