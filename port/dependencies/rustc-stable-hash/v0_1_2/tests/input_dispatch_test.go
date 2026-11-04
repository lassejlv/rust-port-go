// SPDX-License-Identifier: MIT OR Apache-2.0
// Supplemental compiler-witness check for the original tests' Hash inputs.
package stablehash_test

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
)

//go:embed testdata/input_dispatch.json
var compilerHashInputFixture []byte

type inputTrace []string

func (trace *inputTrace) Write(bytes []byte) {
	*trace = append(*trace, "bytes:"+hex.EncodeToString(bytes))
}
func (trace *inputTrace) Finish64() uint64    { return 0 }
func (trace *inputTrace) WriteU8(value uint8) { *trace = append(*trace, fmt.Sprintf("u8:%d", value)) }
func (trace *inputTrace) WriteUsize(value uint) {
	*trace = append(*trace, fmt.Sprintf("usize:%d", value))
}
func (trace *inputTrace) WriteIsize(value int) {
	*trace = append(*trace, fmt.Sprintf("isize:%d", value))
}
func TestCompilerHashInputDispatch(t *testing.T) {
	var fixture struct {
		TargetArch string `json:"target_arch"`
		Events     map[string][]string
	}
	if err := json.Unmarshal(compilerHashInputFixture, &fixture); err != nil {
		t.Fatal(err)
	}
	rustArch := map[string]string{"amd64": "x86_64", "386": "x86", "arm": "arm"}[runtime.GOARCH]
	if rustArch != fixture.TargetArch {
		t.Skip("compiler std Hash input witness is specific to " + fixture.TargetArch + "; original source tests remain active")
	}
	u := [4]int32{1, 0, 0, 0}
	cases := map[string]any{
		"strings":    [2]string{"aa", "bb"},
		"slices":     [3][]int32{u[:1], u[1:3], u[3:]},
		"wide_slice": [3][]int32{u[:], u[4:4], u[4:4]},
		"isizes":     [2]int{-1, 1},
	}
	for name, value := range cases {
		var trace inputTrace
		writeHashInput(&trace, value)
		if got, want := fmt.Sprint([]string(trace)), fmt.Sprint(fixture.Events[name]); got != want {
			t.Fatalf("compiler Hash input dispatch %s: got%s want%s", name, got, want)
		}
	}
}
