// SPDX-License-Identifier: MIT OR Apache-2.0
package stablehash_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOriginalRustProfiles(t *testing.T) {
	if os.Getenv("FERRUGO_RUN_STABLE_HASH_ORIGINAL_TESTS") != "1" {
		t.Skip("set FERRUGO_RUN_STABLE_HASH_ORIGINAL_TESTS=1 to execute all untouched Rust tests/docs in eight profiles")
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(root, "tools/port-rust-stable-hash/original-tests/run.py"), "--root", root, "--offline")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("unchanged Rust original profiles: %v\n%s", err, output)
	}
}
