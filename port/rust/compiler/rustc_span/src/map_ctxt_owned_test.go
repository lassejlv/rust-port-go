package rustcspan

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type spanMapperEnvironment struct {
	value int
	token bool
}

func TestMapCtxtOwnedRealCellAndDestructionPoint(t *testing.T) {
	span := NewSpan(nil, 1, 2, RootSyntaxContext(), None[LocalDefId]())
	var events []string
	var calledCell *spanMapperEnvironment
	mapper := NewCtxtMapper(spanMapperEnvironment{1, true}, func(env *spanMapperEnvironment, old SyntaxContext, context rustfmt.Context) SyntaxContext {
		calledCell = env
		env.value = 9
		env.token = false
		events = append(events, fmt.Sprintf("call:%d:%v", old.AsU32(), context.Panicking()))
		return SyntaxContextFromU32(65536)
	}, func(env *spanMapperEnvironment, context rustfmt.Context) {
		if env != calledCell {
			t.Fatal("Drop borrowed a copied environment")
		}
		events = append(events, fmt.Sprintf("drop:%d:%v:%v", env.value, env.token, context.Panicking()))
	}, rustfmt.Context{})
	// Target context needs interning without a session. Environment destruction
	// must complete before that construction panic, with no second destruction.
	panicText := spanTestPanic(func() { MapCtxtOwned(span, nil, mapper) })
	if !strings.Contains(panicText, "cannot access a scoped thread local variable without calling `set` first") || !reflect.DeepEqual(events, []string{"call:0:false", "drop:9:false:false"}) {
		t.Fatalf("destruction/construction order %v panic%q", events, panicText)
	}
	if got := spanTestPanic(func() { MapCtxtOwned(span, nil, mapper) }); !strings.Contains(got, "already consumed") || len(events) != 2 {
		t.Fatal("consumed mapper was reused or destroyed twice")
	}
}

func TestMapCtxtOwnedDecodeAndCallPanicCleanup(t *testing.T) {
	for _, decodeFailure := range []bool{false, true} {
		var events []string
		span := DummySpan()
		session := spanTestSession()
		if decodeFailure {
			span = SpanFromRawSpan(datastructures.RawSpan{Value: uint64(7)<<61 | 100})
		}
		mapper := NewCtxtMapper(spanMapperEnvironment{value: 1, token: true}, func(env *spanMapperEnvironment, _ SyntaxContext, context rustfmt.Context) SyntaxContext {
			env.value = 7
			events = append(events, "call")
			context.Panic("mapper call panic")
			return RootSyntaxContext()
		}, func(env *spanMapperEnvironment, context rustfmt.Context) {
			events = append(events, fmt.Sprintf("drop:%d:%v", env.value, context.Panicking()))
		}, rustfmt.Context{})
		got := spanTestPanic(func() { MapCtxtOwned(span, session, mapper) })
		if decodeFailure {
			if !strings.Contains(got, "index out of range") || !reflect.DeepEqual(events, []string{"drop:1:true"}) {
				t.Fatalf("failed decode cleanup %v panic%q", events, got)
			}
		} else if got != "mapper call panic" || !reflect.DeepEqual(events, []string{"call", "drop:7:true"}) {
			t.Fatalf("failed call cleanup %v panic%q", events, got)
		}
	}
}

func TestMapCtxtOwnedInheritedUnwindAndDropPanic(t *testing.T) {
	var events []string
	mapper := NewCtxtMapper(spanMapperEnvironment{}, func(_ *spanMapperEnvironment, _ SyntaxContext, context rustfmt.Context) SyntaxContext {
		events = append(events, fmt.Sprintf("call:%v", context.Panicking()))
		return RootSyntaxContext()
	}, func(_ *spanMapperEnvironment, context rustfmt.Context) {
		events = append(events, fmt.Sprintf("drop:%v", context.Panicking()))
		context.Panic("drop panic")
	}, rustfmt.Context{Unwinding: true})
	got := spanTestPanic(func() { MapCtxtOwned(DummySpan(), nil, mapper) })
	if got != "drop panic" || !reflect.DeepEqual(events, []string{"call:true", "drop:true"}) {
		t.Fatalf("ambient unwind was mistaken for a local double panic: %v %q", events, got)
	}
}

func TestMapCtxtOwnedAbortProfiles(t *testing.T) {
	if mode := os.Getenv("FERRUGO_SPAN_MAP_ABORT"); mode != "" {
		context := rustfmt.Context{ProcessingPanic: mode == "processing"}
		mapper := NewCtxtMapper(0, func(_ *int, _ SyntaxContext, context rustfmt.Context) SyntaxContext {
			fmt.Fprintln(os.Stdout, "call")
			context.Panic("call panic")
			return RootSyntaxContext()
		}, func(_ *int, context rustfmt.Context) { fmt.Fprintln(os.Stdout, "drop"); context.Panic("drop panic") }, context)
		MapCtxtOwned(DummySpan(), nil, mapper)
		os.Exit(0)
	}
	for _, mode := range []string{"double", "processing"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMapCtxtOwnedAbortProfiles$")
		cmd.Env = append(os.Environ(), "FERRUGO_SPAN_MAP_ABORT="+mode)
		output, err := cmd.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 134 {
			t.Fatalf("%s status%v output%s", mode, err, output)
		}
		want := "call\ndrop\n"
		if mode == "processing" {
			want = "call\n"
		}
		if string(output) != want {
			t.Fatalf("%s cleanup events%q want%q", mode, output, want)
		}
	}
}
