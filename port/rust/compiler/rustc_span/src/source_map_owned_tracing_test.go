// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual full SourceMap ownership failures while a genuine subscriber is active.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

var sourceMapOwnedTraceSite = rusttracing.NewCallsite(rusttracing.Metadata{Kind: rusttracing.EventKind, Level: rusttracing.Info, Name: "event tools/port-rust-source-map/owned_trace.rs:8", Target: "owned_trace", ModulePath: "owned_trace", File: "tools/port-rust-source-map/owned_trace.rs", Line: 8, Fields: []string{"phase"}})

func sourceMapOwnedTrace(ctx rustfmt.Context, phase string) {
	scope := rusttracing.ScopeFromContext(ctx)
	if scope == nil {
		return
	}
	rusttracing.Event(scope, sourceMapOwnedTraceSite, ctx, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "phase", Value: rusttracing.StrField(phase)}}
	})
}
func sourceMapOwnedCallbackLocation(ctx rustfmt.Context, payload string, line, column uint32) rustfmt.SourceLocation {
	file := "owned_runtime.rs"
	if rusttracing.ScopeFromContext(ctx) != nil {
		file = "owned_trace.rs"
		pos := map[string][2]uint32{"error Debug panic": {9, 18}, "error Drop panic": {10, 18}, "loader Drop panic": {11, 19}, "outer": {12, 18}}[payload]
		line, column = pos[0], pos[1]
	}
	return rustfmt.SourceLocation{File: file, Line: line, Column: column}
}

type sourceMapOwnedTraceVisitor struct{}

func (*sourceMapOwnedTraceVisitor) RecordDebug(field *rusttracing.Field, value rusttracing.DebugValue, c rusttracing.Context) {
	var out strings.Builder
	if err := value.WriteDebugFormatter(&rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions(), Context: c.Panic}); err != nil {
		panic(err)
	}
	fmt.Printf("field:%s:%s:%t\n", field.Name, out.String(), c.Panic.Panicking())
}
func (*sourceMapOwnedTraceVisitor) RecordBool(*rusttracing.Field, bool, rusttracing.Context) {
	panic("unexpected bool")
}
func (*sourceMapOwnedTraceVisitor) RecordI64(*rusttracing.Field, int64, rusttracing.Context) {
	panic("unexpected i64")
}
func (*sourceMapOwnedTraceVisitor) RecordU64(*rusttracing.Field, uint64, rusttracing.Context) {
	panic("unexpected u64")
}
func (*sourceMapOwnedTraceVisitor) RecordStr(field *rusttracing.Field, value string, c rusttracing.Context) {
	fmt.Printf("str:%s:%s:%t\n", field.Name, value, c.Panic.Panicking())
}
func TestSourceMapOwnedTracingChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_OWNED_TRACING") != "1" {
		return
	}
	state := rustvec.NewOwned(struct{}{}, rustvec.ValueOps[struct{}]{Drop: func(_ *struct{}, c rustfmt.Context) { fmt.Printf("subscriber_drop:%t\n", c.Panicking()) }})
	ops := rusttracing.SubscriberOps[struct{}]{Enabled: func(_ *struct{}, m *rusttracing.Metadata, _ rusttracing.Context) bool {
		return m.Kind == rusttracing.EventKind
	}, NewSpan: func(*struct{}, *rusttracing.Attributes, rusttracing.Context) rusttracing.ID {
		return rusttracing.NewID(1)
	}, Event: func(_ *struct{}, e *rusttracing.EventRecord, c rusttracing.Context) {
		m := e.Metadata
		fmt.Printf("event:%s:%s:%d:%t\n", m.Name, m.File, m.Line, c.Panic.Panicking())
		e.Record(&sourceMapOwnedTraceVisitor{}, c)
	}}
	ops.Record = func(*struct{}, rusttracing.ID, []rusttracing.FieldValue, rusttracing.Context) {}
	ops.RecordFollowsFrom = func(*struct{}, rusttracing.ID, rusttracing.ID, rusttracing.Context) {}
	ops.Enter = func(*struct{}, rusttracing.ID, rusttracing.Context) {}
	ops.Exit = func(*struct{}, rusttracing.ID, rusttracing.Context) {}
	dispatch := rusttracing.NewDispatchOwned(rusttracing.DefaultRuntime(), state, ops, rusttracing.Context{})
	scope := rusttracing.NewScope(dispatch)
	sourceMapOwnedChild(os.Getenv("FERRUGO_SOURCE_MAP_OWNED_MODE"), os.Getenv("FERRUGO_SOURCE_MAP_OWNED_CASE"), os.Getenv("FERRUGO_SOURCE_MAP_OWNED_PROFILE"), rusttracing.AttachScope(rustfmt.Context{}, scope))
	scope.Close(rustfmt.Context{})
	dispatch.CloseWithScope(scope, rustfmt.Context{})
	os.Exit(0)
}
func TestSourceMapOwnedTracingNativeOracle(t *testing.T) {
	file, err := os.Open("testdata/source_map_owned_tracing.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(z)
	count, aborts := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 {
			t.Fatal("bad owned tracing fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapOwnedTracingChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_OWNED_TRACING=1", "FERRUGO_SOURCE_MAP_OWNED_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_OWNED_MODE="+p[1], "FERRUGO_SOURCE_MAP_OWNED_CASE="+p[2])
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected owned tracing host failure %s: %v %s", p[:3], err, out)
			}
			kind = "abort"
			aborts++
		}
		if want := sourceMapOracleString(p[4]); kind != p[3] || string(out) != want {
			t.Fatalf("%v got%s/%q want%s/%q", p[:3], kind, out, p[3], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d enabled-tracing owned/runtime scripts, %d isolated aborts", count, aborts)
}
