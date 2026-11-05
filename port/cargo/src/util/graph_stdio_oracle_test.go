// SPDX-License-Identifier: MIT OR Apache-2.0
package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststdio"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type graphStdioRequest struct {
	Nodes, Repeats, InitialCapacity uint
	Capture, Preheld                bool
	Error                           string
}
type graphStdioSnapshot struct {
	Stage            string
	Length, Capacity uint
	ArcStrong        int
	CaptureHex       string
}
type graphStdioPrintResult struct {
	Failed  bool
	Payload any
}
type graphStdioRecord struct {
	Request               graphStdioRequest
	NativeExit, Exit      int
	StderrHex             string
	Writes                []string
	Snapshots             []graphStdioSnapshot
	CaptureSlotReturned   bool
	Hooks                 []rustfmt.SourcePanicInfo
	HookOtherThreadLocked *bool
	PrintResults          []graphStdioPrintResult
}

func graphStdioTraits() GraphTraits[uint, struct{}] {
	return GraphTraits[uint, struct{}]{
		Node: imrc.Ops[uint]{Clone: func(value rustvec.Ref[uint]) *rustvec.Owned[uint] { return rustvec.Own(*value.Value) }},
		Edge: imrc.Ops[struct{}]{Clone: func(rustvec.Ref[struct{}]) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) }},
		CompareNode: func(a, b rustvec.Ref[uint]) int {
			if *a.Value < *b.Value {
				return -1
			}
			if *a.Value > *b.Value {
				return 1
			}
			return 0
		},
		LessNode:    func(a, b rustvec.Ref[uint]) bool { return *a.Value < *b.Value },
		EqualNode:   func(a, b rustvec.Ref[uint]) bool { return *a.Value == *b.Value },
		DefaultEdge: func(rustfmt.Context) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) },
	}
}

func graphStdioReplay(t *testing.T, want graphStdioRecord) graphStdioRecord {
	t.Helper()
	q := want.Request
	got := graphStdioRecord{Request: q, Writes: []string{}, Snapshots: []graphStdioSnapshot{}, Hooks: []rustfmt.SourcePanicInfo{}, PrintResults: []graphStdioPrintResult{}}
	var stderr strings.Builder
	writeCount := 0
	var current rustfmt.Context
	runtime := ruststdio.NewRuntime(func(bytes []byte, context rustfmt.Context) (int, error) {
		if context != current {
			t.Fatal("Graph stderr did not forward the current operation context")
		}
		got.Writes = append(got.Writes, hex.EncodeToString(bytes))
		writeCount++
		if q.Error == "eio" && writeCount == 1 {
			return 0, syscall.EIO
		}
		return stderr.Write(bytes)
	})
	scope := ruststdio.NewThreadScope()
	operations := &rustfmt.PanicOperations{LocationCells: map[string]*rustfmt.SourceLocation{}, Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
		if !context.ProcessingPanic || !context.Unwinding || context.Runtime.Value != "operation B" {
			t.Fatal("Graph IO hook lost current panic-processing context")
		}
		got.Hooks = append(got.Hooks, info)
		// The source hook can synchronize with another source thread here:
		// Stderr::write_fmt has already released its statement lease.
		done := make(chan struct{})
		go func() {
			lease := runtime.LockStderr(ruststdio.NewThreadScope(), context)
			lease.Close()
			close(done)
		}()
		locked := false
		select {
		case <-done:
			locked = true
		case <-time.After(3 * time.Second):
		}
		got.HookOtherThreadLocked = &locked
	}}
	if len(want.Hooks) != 0 {
		operations.LocationCells["rust.std.stdio.stderr.print_error"] = want.Hooks[0].Location
	}
	current = rustfmt.WithPanicOperations(rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "operation B"}}, operations)
	config := DefaultOwnedGraphConfig()
	config.StderrBinding = runtime.Binding(scope)
	g := NewOwnedGraphWithConfig(graphStdioTraits(), config)
	defer g.CloseWithContext(current)
	construction := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "construction A"}}
	for n := uint(0); n < q.Nodes; n++ {
		g.AddWithContext(rustvec.Own(n), construction)
	}
	for n := uint(1); n < q.Nodes; n++ {
		g.LinkWithContext(rustvec.Own(n-1), rustvec.Own(n), construction).AssignWithContext(rustvec.Own(struct{}{}), construction)
	}
	capture := ruststdio.NewCaptureWithCapacity(q.InitialCapacity)
	strong := 1
	snapshot := func(stage string) {
		state, ok := capture.TrySnapshot()
		if !ok {
			t.Fatal("capture remained locked outside a Graph statement")
		}
		got.Snapshots = append(got.Snapshots, graphStdioSnapshot{Stage: stage, Length: state.Length, Capacity: state.Capacity, ArcStrong: strong, CaptureHex: hex.EncodeToString(state.Bytes)})
	}
	snapshot("before")
	if q.Capture {
		strong++
		// This is the complete used Arc resource witness: no move clones it;
		// actual slot destruction releases precisely this strong reference.
		owner := rustvec.NewOwned(capture, rustvec.ValueOps[*ruststdio.Capture]{Drop: func(_ **ruststdio.Capture, context rustfmt.Context) {
			if context != current {
				t.Fatal("capture Arc Drop lost the current caller")
			}
			strong--
		}})
		old := scope.SetOwnedOutputCapture(ruststdio.NewCaptureOwnerWithContext(owner, capture, current), current)
		if old != nil {
			t.Fatal("initial capture slot was occupied")
		}
	}
	snapshot("set")
	var preheld *ruststdio.StderrLease
	if q.Preheld {
		preheld = runtime.LockStderr(scope, current)
	}
	for repeat := uint(1); repeat <= q.Repeats; repeat++ {
		result := graphStdioPrintResult{}
		func() {
			defer func() {
				if value := recover(); value != nil {
					result.Failed = true
					result.Payload = value
				}
			}()
			g.printOwnedForTest(current)
		}()
		got.PrintResults = append(got.PrintResults, result)
		snapshot(fmt.Sprintf("print%d", repeat))
	}
	if preheld != nil {
		preheld.Close()
	}
	returned := scope.SetOwnedOutputCapture(nil, current)
	if returned != nil {
		got.CaptureSlotReturned = returned.BorrowCaptureWithContext(current) == capture
	}
	snapshot("popped")
	if returned != nil {
		returned.CloseWithContext(current)
	}
	snapshot("dropped")
	scope.CloseWithContext(current)
	got.StderrHex = hex.EncodeToString([]byte(stderr.String()))
	return got
}

func TestGraphOriginalStdioComposition(t *testing.T) {
	data, err := os.ReadFile("testdata/graph_stdio.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Schema int
		Cases  []graphStdioRecord
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || len(fixture.Cases) != 72 {
		t.Fatal("incomplete original Graph stdio corpus")
	}
	for _, want := range fixture.Cases {
		if want.NativeExit != 0 || want.Exit != 0 {
			t.Fatal("unexpected native Graph stdio termination")
		}
		t.Run(fmt.Sprintf("nodes%d/capture%t/preheld%t/repeat%d/cap%d/%s", want.Request.Nodes, want.Request.Capture, want.Request.Preheld, want.Request.Repeats, want.Request.InitialCapacity, want.Request.Error), func(t *testing.T) {
			got := graphStdioReplay(t, want)
			if !reflect.DeepEqual(got, want) {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				t.Fatalf("original Graph stdio differs\nGo: %s\nRust: %s", g, w)
			}
		})
	}
}
