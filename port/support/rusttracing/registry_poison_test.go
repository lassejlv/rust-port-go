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

func TestRegistryPoisonChild(t *testing.T) {
	mode := os.Getenv("FERRUGO_TRACING_POISON_MODE")
	if mode == "" {
		return
	}
	caseName := os.Getenv("FERRUGO_TRACING_POISON_CASE")
	profile, _, _, err := tracingProducerLoad("registry")
	if err != nil {
		t.Fatal(err)
	}
	rt := DefaultRuntime()
	type state struct {
		name string
		fail bool
	}
	var create func(string, bool, rustfmt.Context) *Dispatch
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{LocationCells: profile.Locations, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		if info.Location == nil {
			t.Fatal("missing source location cell")
		}
		// The source Result::unwrap still owns its acquired poisoned guard.
		if info.Location == profile.Locations[PanicSiteRegistryWritePoison] {
			if rt.dispatchMu.TryRLock() {
				rt.dispatchMu.RUnlock()
				t.Fatal("write guard released before poison hook")
			}
		}
		if info.Location == profile.Locations[PanicSiteRegistryReadPoison] {
			if rt.dispatchMu.TryLock() {
				rt.dispatchMu.Unlock()
				t.Fatal("read guard released before poison hook")
			}
		}
		fmt.Printf("hook:%s:%x:%x:%d:%d:%t\n", info.Kind, fmt.Sprint(info.Payload), info.Location.File, info.Location.Line, info.Location.Column, current.Panicking())
		if strings.HasPrefix(mode, "processing") && info.Payload == "outer" {
			if mode == "processing_write" {
				create("after", false, current)
			} else {
				rt.RebuildInterest(nil, current)
			}
		}
	}})
	raise := func(current rustfmt.Context, key, payload string) {
		current.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicStaticStr, Payload: payload, Location: current.SourcePanicLocation(key), CanUnwind: true})
	}
	failRead := false
	create = func(name string, fail bool, current rustfmt.Context) *Dispatch {
		ops := SubscriberOps[state]{
			OnRegisterDispatch: func(s *state, _ *Dispatch, c Context) {
				fmt.Printf("on-register:%s\n", s.name)
				if s.fail {
					raise(c.Panic, "register", "register")
				}
			},
			RegisterCallsite: func(s *state, _ *Metadata, c Context) Interest {
				fmt.Printf("callsite:%s\n", s.name)
				if failRead {
					raise(c.Panic, "callsite", "callsite")
				}
				return Always
			},
			Enabled: func(*state, *Metadata, Context) bool { return true }, NewSpan: func(*state, *Attributes, Context) ID { return NewID(1) }, Record: func(*state, ID, []FieldValue, Context) {}, RecordFollowsFrom: func(*state, ID, ID, Context) {}, Event: func(*state, *EventRecord, Context) {}, Enter: func(*state, ID, Context) {}, Exit: func(*state, ID, Context) {},
		}
		return NewDispatchOwned(rt, rustvec.NewOwned(state{name, fail}, rustvec.ValueOps[state]{Drop: func(s *state, c rustfmt.Context) { fmt.Printf("drop:%s:%t\n", s.name, c.Panicking()) }}), ops, Context{Panic: current})
	}
	attempt := func(label string, body func() *Dispatch) (out *Dispatch) {
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			out = body()
		}()
		fmt.Printf("caught:%s:%t\n", label, caught)
		return
	}
	script := func(current rustfmt.Context, caseName string, processing bool) {
		var bases []*Dispatch
		if caseName != "write0" {
			bases = append(bases, create("base0", false, current), create("base1", false, current))
		}
		if caseName == "read2" {
			failRead = true
			attempt("first", func() *Dispatch {
				site := NewCallsiteIn(rt, Metadata{Kind: EventKind, Name: "registry", Level: Info})
				Event(nil, site, current, func() []FieldValue { return nil })
				return nil
			})
			failRead = false
		} else {
			attempt("first", func() *Dispatch { return create("failed", true, current) })
		}
		if processing {
			raise(current, "outer", "outer")
		}
		after := attempt("after", func() *Dispatch { return create("after", false, current) })
		attempt("rebuild", func() *Dispatch { rt.RebuildInterest(nil, current); return nil })
		after.Close(current)
		for _, base := range bases {
			base.Close(current)
		}
	}
	switch mode {
	case "normal":
		script(ctx, caseName, false)
	case "inherited":
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			defer func() {
				p := recover()
				if p != nil {
					unwind := ctx
					unwind.Unwinding = true
					fmt.Println("outer-drop:true")
					script(unwind, caseName, false)
					fmt.Println("outer-end:true")
					ctx.ResumeUnwind(p)
				}
			}()
			raise(ctx, "outer", "outer")
		}()
		fmt.Printf("outer-caught:%t\n", caught)
	default:
		script(ctx, "write2", true)
	}
	os.Exit(0)
}

func TestRegistryPoisonNativeOracle(t *testing.T) {
	profile, data, directory, err := tracingProducerLoad("registry")
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
			t.Fatal("invalid registry fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, binary, "-test.run=^TestRegistryPoisonChild$")
		cmd.Env = append(tracingProducerChildEnv(profile, directory), "FERRUGO_TRACING_POISON_MODE="+p[0], "FERRUGO_TRACING_POISON_CASE="+p[1])
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		err := cmd.Run()
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
	if count != 8 {
		t.Fatalf("registry count%d", count)
	}
}
