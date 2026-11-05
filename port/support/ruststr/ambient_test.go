package ruststr

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func strAmbientProbe(ambient, op string, emit func(string)) {
	run := func(ctx rustfmt.Context) {
		caught := false
		func() {
			defer func() {
				p := recover()
				emit(fmt.Sprintf("scope_drop:%t", ctx.Panicking() || p != nil))
				caught = p != nil
			}()
			emit(fmt.Sprintf("before:%t", ctx.Panicking()))
			text := "aé🦀z"
			switch op {
			case "valid":
				Slice(text, 1, 7, ctx, "probe")
			case "boundary":
				Slice(text, 2, 3, ctx, "probe")
			case "backward":
				Slice(text, 7, 1, ctx, "probe")
			case "begin_oob":
				Slice(text, 9, 10, ctx, "probe")
			case "end_oob":
				Slice(text, 0, 9, ctx, "probe")
			case "inclusive_boundary":
				SliceInclusive(text, Inclusive(1, 1), ctx, "probe")
			case "inclusive_len":
				SliceInclusive(text, Inclusive(0, 8), ctx, "probe")
			case "from_boundary":
				SliceFrom(text, 2, ctx, "probe")
			case "to_boundary":
				SliceTo(text, 2, ctx, "probe")
			default:
				panic(op)
			}
			emit("done")
		}()
		emit(fmt.Sprintf("caught:%t", caught))
	}
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, _ rustfmt.Context) { emit("hook") }})
	if ambient == "normal" {
		run(ctx)
		return
	}
	if ambient == "unwinding" {
		func() {
			defer func() {
				if recover() != nil {
					emit("ambient_caught:true")
				}
			}()
			defer func() { emit("ambient_drop"); current := ctx; current.Unwinding = true; run(current) }()
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "ambient", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}()
		return
	}
	if ambient == "processing" {
		ctx = rustfmt.WithPanicOperations(ctx, &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, current rustfmt.Context) { emit("ambient_hook"); run(current) }})
		func() {
			defer func() {
				if recover() != nil {
					emit("ambient_caught:true")
				}
			}()
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "ambient", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}()
		return
	}
	panic(ambient)
}

func TestSDKStrAmbientChild(t *testing.T) {
	query := os.Getenv("FERRUGO_STR_AMBIENT_CHILD")
	if query == "" {
		return
	}
	fields := strings.Split(query, ":")
	if len(fields) != 2 {
		t.Fatal("child request")
	}
	strAmbientProbe(fields[0], fields[1], func(event string) { _, _ = fmt.Fprintln(os.Stderr, "EVENT:"+event) })
}

func TestOriginalSDKStrAmbient(t *testing.T) {
	raw, err := os.ReadFile("testdata/ambient.tsv")
	if err != nil {
		t.Fatal(err)
	}
	count, aborts := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		count++
		p := strings.Split(line, "\t")
		if len(p) != 4 {
			t.Fatal("ambient fixture shape")
		}
		events := []string{}
		if p[2] == "true" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestSDKStrAmbientChild$")
			command.Env = append(os.Environ(), "FERRUGO_STR_AMBIENT_CHILD="+p[0]+":"+p[1])
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%v: expected abort134, got%v: %s", p[:2], err, output)
			}
			for _, event := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(event, "EVENT:") {
					events = append(events, strings.TrimPrefix(event, "EVENT:"))
				}
			}
		} else {
			strAmbientProbe(p[0], p[1], func(event string) { events = append(events, event) })
		}
		if strings.Join(events, ",") != p[3] {
			t.Fatalf("%v\nGo: %s\nRust: %s", p[:2], strings.Join(events, ","), p[3])
		}
	}
	if count != 27 || aborts != 8 {
		t.Fatal("ambient count", count, aborts)
	}
}
