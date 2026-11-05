package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type libCustomPos struct {
	value uint
	id    uint32
	seen  *libCustomPos
}

func libPosRun(fail int, ctx rustfmt.Context, emit func(string)) {
	next := uint32(0)
	check := func(which int, current rustfmt.Context) {
		if fail == which {
			current.PanicAt(rustfmt.SourcePanicInfo{Payload: "pos", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}
	}
	same := func(value *libCustomPos) bool {
		old := value.seen
		value.seen = value
		return old == nil || old == value
	}
	ops := NewPosOperations(PosMethods[libCustomPos]{
		FromUsize: func(n uint, current rustfmt.Context) libCustomPos {
			emit(fmt.Sprintf("from_usize:%d:%t", n, current.Panicking()))
			check(1, current)
			next++
			return libCustomPos{value: n, id: next}
		},
		ToUsize: func(value *libCustomPos, current rustfmt.Context) uint {
			emit(fmt.Sprintf("to_usize:%d:%d:%t:%t", value.id, value.value, current.Panicking(), same(value)))
			value.value++
			check(2, current)
			return value.value
		},
		FromU32: func(n uint32, current rustfmt.Context) libCustomPos {
			emit(fmt.Sprintf("from_u32:%d:%t", n, current.Panicking()))
			check(3, current)
			next++
			return libCustomPos{value: uint(n), id: next}
		},
		ToU32: func(value *libCustomPos, current rustfmt.Context) uint32 {
			emit(fmt.Sprintf("to_u32:%d:%d:%t:%t", value.id, value.value, current.Panicking(), same(value)))
			value.value++
			check(4, current)
			return uint32(value.value)
		},
	})
	drop := func(value *libCustomPos, current rustfmt.Context) {
		emit(fmt.Sprintf("drop:%d:%d:%t", value.id, value.value, current.Panicking()))
	}
	view := ops.Bind(ctx)
	a := ops.FromUsize(10, ctx)
	defer spanCloseOwnedCallback(ctx, func(current rustfmt.Context) { drop(&a, current) })
	emit(fmt.Sprintf("result_usize:%d", view.ToUsize(&a)))
	emit(fmt.Sprintf("result_u32:%d", ops.ToU32(&a, ctx)))
	b := view.FromU32(20)
	defer spanCloseOwnedCallback(ctx, func(current rustfmt.Context) { drop(&b, current) })
	emit(fmt.Sprintf("result_second:%d", view.ToU32(&b)))
}
func libPosAmbient(mode string, fail int, emit func(string)) {
	attempt := func(ctx rustfmt.Context) {
		caught := false
		func() { defer func() { caught = recover() != nil }(); libPosRun(fail, ctx, emit) }()
		emit(fmt.Sprintf("caught:%t", caught))
	}
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(rustfmt.SourcePanicInfo, rustfmt.Context) { emit("hook") }})
	if mode == "normal" {
		attempt(ctx)
		return
	}
	if mode == "unwinding" {
		func() {
			defer func() { emit(fmt.Sprintf("ambient_caught:%t", recover() != nil)) }()
			defer spanCloseOwnedCallback(ctx, func(current rustfmt.Context) { emit("ambient_drop"); attempt(current) })
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "ambient", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}()
		return
	}
	if mode == "processing" {
		ctx = rustfmt.WithPanicOperations(ctx, &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, current rustfmt.Context) { emit("ambient_hook"); attempt(current) }})
		func() {
			defer func() { emit(fmt.Sprintf("ambient_caught:%t", recover() != nil)) }()
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "ambient", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}()
		return
	}
	panic(mode)
}
func TestLibPosAbortChild(t *testing.T) {
	query := os.Getenv("FERRUGO_LIB_POS_CHILD")
	if query == "" {
		return
	}
	p := strings.Split(query, ":")
	if len(p) != 2 {
		t.Fatal("Pos child request")
	}
	fail, _ := strconv.Atoi(p[1])
	libPosAmbient(p[0], fail, func(event string) { _, _ = fmt.Fprintln(os.Stderr, "EVENT:"+event) })
}
func TestLibOriginalPosProvider(t *testing.T) {
	f, err := os.Open("testdata/lib-pos.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	scan := bufio.NewScanner(g)
	count, aborts := 0, 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 {
			t.Fatal("Pos fixture shape")
		}
		events := []string{}
		if p[3] == "true" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestLibPosAbortChild$")
			command.Env = append(os.Environ(), "FERRUGO_LIB_POS_CHILD="+strings.Join(p[:2], ":"))
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("Pos abort%v: %v %s", p[:2], err, output)
			}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
		} else {
			fail, err := strconv.Atoi(p[1])
			if err != nil {
				t.Fatal(err)
			}
			libPosAmbient(p[0], fail, func(event string) { events = append(events, event) })
		}
		if strings.Join(events, ",") != p[4] {
			t.Fatalf("Pos%v\nGo: %s\nRust: %s", p[:4], strings.Join(events, ","), p[4])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 15 || aborts != 4 {
		t.Fatal("Pos count", count, aborts)
	}
}
