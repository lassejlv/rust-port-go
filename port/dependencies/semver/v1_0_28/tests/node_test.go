//go:build semver_node

// Mirrors semver 1.0.28 tests/node/mod.rs. Requesting this profile requires
// node and its semver package; missing prerequisites fail explicitly.
package semver_test

import (
	"bytes"
	"fmt"
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"os/exec"
	"strings"
)

func requirement_matches(req semver.VersionReq, version semver.Version) bool {
	script := strings.Replace("console.log(require('semver').satisfies('{}', '{}'))", "{}", version.String(), 1)
	script = strings.Replace(script, "{}", strings.ReplaceAll(req.String(), ",", ""), 1)
	command := exec.Command("node", "-e", script)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		panic(fmt.Sprintf("requested semver_node profile requires node and require('semver'): %v; %s", err, stderr.String()))
	}
	if stdout.String() == "true\n" {
		return true
	}
	if stdout.String() == "false\n" {
		return false
	}
	panic(strings.Replace("unexpected output: {}", "{}", stdout.String()+stderr.String(), 1))
}
