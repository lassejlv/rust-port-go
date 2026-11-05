// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func TestProcessingSubscriberPanicChild(t *testing.T) {
	mode := os.Getenv("FERRUGO_TRACING_PROCESSING_CASE")
	if mode == "" {
		return
	}
	rt := NewRuntime()
	ctx := rustfmt.Context{ProcessingPanic: true, Unwinding: true}
	ops := SubscriberOps[struct{}]{
		Enabled:           func(*struct{}, *Metadata, Context) bool { return true },
		RegisterCallsite:  func(*struct{}, *Metadata, Context) Interest { return Always },
		NewSpan:           func(*struct{}, *Attributes, Context) ID { fmt.Println("new-span"); panic("raw subscriber") },
		Record:            func(*struct{}, ID, []FieldValue, Context) {},
		RecordFollowsFrom: func(*struct{}, ID, ID, Context) {},
		Event:             func(*struct{}, *EventRecord, Context) { fmt.Println("event"); panic("raw subscriber") },
		Enter:             func(*struct{}, ID, Context) {}, Exit: func(*struct{}, ID, Context) {},
	}
	state := rustvec.NewOwned(struct{}{}, rustvec.ValueOps[struct{}]{Drop: func(*struct{}, rustfmt.Context) { fmt.Println("subscriber-drop") }})
	if mode == "dispatch" {
		ops.OnRegisterDispatch = func(*struct{}, *Dispatch, Context) { fmt.Println("on-register"); panic("raw subscriber") }
		NewDispatchOwned(rt, state, ops, Context{Panic: ctx})
		return
	}
	d := NewDispatchOwned(rt, state, ops, Context{})
	scope := NewScope(d)
	kind := EventKind
	if mode == "span" {
		kind = SpanKind
	}
	site := NewCallsiteIn(rt, Metadata{Kind: kind, Name: "probe", Level: Info, Fields: []string{"value"}})
	fields := func(rustfmt.Context) OwnedFieldValues[struct{}] {
		return OwnedFieldValues[struct{}]{Env: rustvec.NewOwned(struct{}{}, rustvec.ValueOps[struct{}]{Drop: func(*struct{}, rustfmt.Context) { fmt.Println("field-drop") }})}
	}
	if mode == "event" {
		EventOwnedFields(scope, site, ctx, fields)
	} else {
		NewSpanOwnedFields(scope, site, ctx, fields)
	}
}

func TestProcessingSubscriberPanicsNativeOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/processing.tsv")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		p := strings.Split(row, "\t")
		if len(p) != 3 || p[1] != "abort" {
			t.Fatal("invalid processing-panic fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestProcessingSubscriberPanicChild$")
		command.Env = append(os.Environ(), "FERRUGO_TRACING_PROCESSING_CASE="+p[0])
		var stdout bytes.Buffer
		command.Stdout = &stdout
		err := command.Run()
		cancel()
		var exit *exec.ExitError
		if value, ok := err.(*exec.ExitError); ok {
			exit = value
		}
		if exit == nil || exit.ExitCode() != 134 {
			t.Fatalf("%s: expected explicit abort exit134, got %v", p[0], err)
		}
		if got := hex.EncodeToString(stdout.Bytes()); got != p[2] {
			t.Fatalf("%s: events%q, want hex%s", p[0], stdout.String(), p[2])
		}
		count++
	}
	if count != 3 {
		t.Fatalf("processing-panic record count%d", count)
	}
}
