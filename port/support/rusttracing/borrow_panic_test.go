// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func TestBorrowPanicChild(t *testing.T) {
	mode := os.Getenv("FERRUGO_TRACING_BORROW_MODE")
	if mode == "" {
		return
	}
	caseName := os.Getenv("FERRUGO_TRACING_BORROW_CASE")
	data, err := os.ReadFile("../../../tools/port-rust-tracing/processing/BORROW_VALIDATION.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile struct {
		Locations map[string]*rustfmt.SourceLocation `json:"locations"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	var operation func(rustfmt.Context)
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{LocationCells: profile.Locations, Hook: func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
		if info.Location == nil {
			t.Fatal("missing explicit source profile cell")
		}
		fmt.Printf("hook:%s:%x:%x:%d:%d:%t\n", info.Kind, fmt.Sprint(info.Payload), info.Location.File, info.Location.Line, info.Location.Column, processing.Panicking())
		if mode == "processing" && info.Payload == "outer" {
			operation(processing)
		}
	}})
	operation = func(current rustfmt.Context) {
		if caseName == "id" {
			NewIDWithContext(0, current)
			return
		}
		rt := DefaultRuntime()
		ops := SubscriberOps[struct{}]{Enabled: func(*struct{}, *Metadata, Context) bool { return true }, NewSpan: func(*struct{}, *Attributes, Context) ID { return NewID(1) }, Record: func(*struct{}, ID, []FieldValue, Context) {}, RecordFollowsFrom: func(*struct{}, ID, ID, Context) {}, Event: func(*struct{}, *EventRecord, Context) {}, Enter: func(*struct{}, ID, Context) {}, Exit: func(*struct{}, ID, Context) {}}
		d := NewDispatchOwned(rt, rustvec.Own(struct{}{}), ops, Context{Panic: current})
		defer dropOnReturn(d.Close, current)
		scope := NewScope(nil)
		defer dropOnReturn(scope.Close, current)
		guard := scope.SetDefault(d, current)
		defer dropOnReturn(guard.Close, current)
		GetDefault(scope, current, func(*Dispatch, Context) {
			if caseName == "set" {
				scope.SetDefault(d, current)
			} else {
				guard.Close(current)
			}
		})
	}
	attempt := func(current rustfmt.Context) {
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			operation(current)
		}()
		fmt.Printf("caught:%t\n", caught)
	}
	outer := func() {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("outer"), CanUnwind: true})
	}
	switch mode {
	case "normal":
		attempt(ctx)
	case "processing":
		outer()
	case "inherited":
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			defer func() {
				payload := recover()
				if payload != nil {
					unwind := ctx
					unwind.Unwinding = true
					fmt.Println("outer-drop:true")
					attempt(unwind)
					fmt.Println("outer-end:true")
					ctx.ResumeUnwind(payload)
				}
			}()
			outer()
		}()
		fmt.Printf("outer-caught:%t\n", caught)
	default:
		t.Fatal("invalid source mode")
	}
	os.Exit(0)
}

func TestBorrowAndZeroIDNativeOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/borrow_panic.tsv")
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
		if len(p) != 4 {
			t.Fatal("invalid borrow/ID fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestBorrowPanicChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_TRACING_BORROW_MODE="+p[0], "FERRUGO_TRACING_BORROW_CASE="+p[1])
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		cancel()
		if p[2] == "abort" {
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 134 {
				t.Fatalf("%s/%s exit%v", p[0], p[1], err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(out.Bytes()); got != p[3] {
			want, _ := hex.DecodeString(p[3])
			t.Fatalf("%s/%s events%q want%q", p[0], p[1], out.String(), want)
		}
		count++
	}
	if count != 9 {
		t.Fatalf("borrow/ID record count%d", count)
	}
}
