// Mirrors the declaration order of tests/test_autotrait.rs. The exact Rust
// Send+Sync compile witness is retained as a Rust fixture and executed by the
// optional original-test helper; Go has no equivalent trait bound.
package semver_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

func assert_send_sync(t *testing.T, read func() string) {
	t.Helper()
	want := read()
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 10; j++ {
				if got := read(); got != want {
					t.Errorf("concurrent shared read = %q, want %q", got, want)
				}
			}
		}()
	}
	workers.Wait()
}

func TestAutotrait_test(t *testing.T) {
	fixture, err := os.ReadFile("../../../../../tools/port-semver/original-tests/fixtures/tests/test_autotrait.rs")
	if err != nil {
		t.Fatal(err)
	}
	checksum := sha256.Sum256(fixture)
	if hex.EncodeToString(checksum[:]) != "2631b78c9c63f1829c814fa88fcab0c5d77eb4e2d9c0c418d50c33e12a511e90" {
		t.Fatal("original Rust Send+Sync fixture differs")
	}
	build := semver.EmptyBuildMetadata()
	assert_send_sync(t, build.AsStr)
	comparator := comparator(t, "1")
	assert_send_sync(t, comparator.String)
	error := version_err(t, "")
	assert_send_sync(t, error.Error)
	pre := semver.EmptyPrerelease()
	assert_send_sync(t, pre.AsStr)
	version := semver.NewVersion(0, 0, 0)
	assert_send_sync(t, version.String)
	versionReq := semver.Star()
	assert_send_sync(t, versionReq.String)
	op := semver.Exact
	assert_send_sync(t, op.RustDebug)
}
