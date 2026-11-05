// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestIndexMapRawProcessingWorker(t *testing.T) {
	if os.Getenv("FERRUGO_INDEXMAP_RAW_PROCESSING") == "" {
		return
	}
	config := im.Config[uint64, uint64]{BucketLayout: im.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: im.KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ im.Context) bool { return *a == *b }, Drop: func(_ *uint64, _ im.Context) {
		fmt.Println("key")
		defer fmt.Println("raw-local")
		panic("raw host callback")
	}}}, Values: im.ValueOps[uint64]{ValueOps: rusthashmap.ValueOps[uint64]{Drop: func(_ *uint64, _ im.Context) { fmt.Println("value") }}}}
	m := im.WithHasher(config, rusthashmap.FxBuilder{PointerBits: 64})
	m.Insert(1, 2)
	m.Insert(3, 4)
	m.Close(im.Context{ProcessingPanic: true, Unwinding: true})
	fmt.Println("after")
}
func TestIndexMapRawProcessingStopsBeforeLibraryCleanup(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestIndexMapRawProcessingWorker$")
	cmd.Env = append(os.Environ(), "FERRUGO_INDEXMAP_RAW_PROCESSING=1")
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 134 || strings.TrimSpace(string(output)) != "key\nraw-local" {
		t.Fatalf("processing escape ran library cleanup: %v /%s", err, output)
	}
}
