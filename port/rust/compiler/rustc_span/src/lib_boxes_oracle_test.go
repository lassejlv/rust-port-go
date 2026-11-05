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
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func libBoxValue(seed uint32) DistinctSources {
	return DistinctSources{Begin: FileNameAndBytePos{CustomFileName(fmt.Sprintf("begin-%d", seed)), BytePos(seed)}, End: FileNameAndBytePos{CustomFileName("end-é"), BytePos(seed)}}
}
func libBoxStringAddress(value *DistinctSources) *byte {
	text, _ := value.Begin.Name.BorrowCustom()
	return unsafe.StringData(*text)
}
func libBoxIdentities(seed uint32, emit func(string)) {
	box := new(DistinctSources)
	*box = libBoxValue(seed)
	original := box
	text := libBoxStringAddress(box)
	line := DistinctSourcesBoxSpanLinesError(box)
	emit(fmt.Sprintf("line:%t", line.BorrowDistinctSources() == original))
	clonedLine := line.Clone()
	emit(fmt.Sprintf("clone_line:%t:%t", clonedLine.BorrowDistinctSources() != original, libBoxStringAddress(clonedLine.BorrowDistinctSources()) != text))
	emit(fmt.Sprintf("line_eq:%t", line.Equal(clonedLine)))
	box = line.TakeDistinctSources()
	emit(fmt.Sprintf("take:%t", box == original))
	snippet := DistinctSourcesBoxSpanSnippetError(box)
	p, _ := snippet.BorrowDistinctSources()
	emit(fmt.Sprintf("snippet:%t", p == original))
	cloned := snippet.Clone()
	c, _ := cloned.BorrowDistinctSources()
	emit(fmt.Sprintf("clone_snippet:%t", c != original))
	p.Begin.Pos = BytePos(seed ^ 1)
	emit(fmt.Sprintf("mut:%t:%t", p.Begin.Pos == BytePos(seed^1), snippet.Equal(cloned)))
	target := DistinctSourcesSpanSnippetError(libBoxValue(9))
	old, _ := target.BorrowDistinctSources()
	target.CloneFrom(snippet)
	fresh, _ := target.BorrowDistinctSources()
	emit(fmt.Sprintf("clone_from:%t:%t", fresh != old, fresh != original))
	box, ok := snippet.TakeDistinctSources()
	if !ok {
		panic("expected source Box")
	}
	line = DistinctSourcesBoxSpanLinesError(box)
	emit(fmt.Sprintf("take_back:%t", line.BorrowDistinctSources() == original))
}

type libBoxSink struct {
	calls, fail int
	panic       bool
	ctx         rustfmt.Context
	emit        func(string)
}

func (sink *libBoxSink) WriteString(text string) (int, error) {
	n := sink.calls
	sink.calls++
	sink.emit(fmt.Sprintf("write:%d:%t", n, sink.ctx.Panicking()))
	if n == sink.fail {
		if sink.panic {
			sink.ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "sink", Kind: rustfmt.PanicStaticStr, CanUnwind: true})
		}
		return 0, libFormatFailure
	}
	return len(text), nil
}
func (sink *libBoxSink) Write(bytes []byte) (int, error) { return sink.WriteString(string(bytes)) }

func libBoxOwnedFormat(fail int, panicSink bool, ctx rustfmt.Context, emit func(string)) {
	value := DistinctSourcesSpanSnippetError(libBoxValue(31))
	pointer, _ := value.BorrowDistinctSources()
	defer spanCloseOwnedCallback(ctx, func(drop rustfmt.Context) {
		actual, _ := value.BorrowDistinctSources()
		emit(fmt.Sprintf("drop:%t:%t", actual == pointer, drop.Panicking()))
		value = nil
	})
	sink := &libBoxSink{fail: fail, panic: panicSink, ctx: ctx, emit: emit}
	err := value.WriteDebugFormatter(nil, &rustfmt.Formatter{Writer: sink, Options: rustfmt.DefaultOptions(), Context: ctx})
	actual, _ := value.BorrowDistinctSources()
	emit(fmt.Sprintf("return:%t:%t", err == nil, actual == pointer))
}
func libBoxAmbient(mode string, fail int, panicSink bool, emit func(string)) {
	attempt := func(ctx rustfmt.Context) {
		caught := false
		func() { defer func() { caught = recover() != nil }(); libBoxOwnedFormat(fail, panicSink, ctx, emit) }()
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
func TestLibBoxesAbortChild(t *testing.T) {
	query := os.Getenv("FERRUGO_LIB_BOX_CHILD")
	if query == "" {
		return
	}
	p := strings.Split(query, ":")
	if len(p) != 3 {
		t.Fatal("box child request")
	}
	fail, _ := strconv.Atoi(p[1])
	panicSink, _ := strconv.ParseBool(p[2])
	libBoxAmbient(p[0], fail, panicSink, func(event string) { _, _ = fmt.Fprintln(os.Stderr, "EVENT:"+event) })
}
func TestLibOriginalEnumBoxes(t *testing.T) {
	f, err := os.Open("testdata/lib-boxes.tsv.gz")
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
			t.Fatal("box fixture shape")
		}
		events := []string{}
		if p[0] == "identity" {
			seed, err := strconv.ParseUint(p[1], 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			libBoxIdentities(uint32(seed), func(event string) { events = append(events, event) })
		} else if p[3] == "true" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestLibBoxesAbortChild$")
			command.Env = append(os.Environ(), "FERRUGO_LIB_BOX_CHILD="+strings.Join(p[:3], ":"))
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("box abort%v: %v %s", p[:3], err, output)
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
			panicSink, err := strconv.ParseBool(p[2])
			if err != nil {
				t.Fatal(err)
			}
			libBoxAmbient(p[0], fail, panicSink, func(event string) { events = append(events, event) })
		}
		if strings.Join(events, ",") != p[4] {
			t.Fatalf("box%v\nGo: %s\nRust: %s", p[:4], strings.Join(events, ","), p[4])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 40 {
		t.Fatal("box count", count)
	}
	if aborts != 5 {
		t.Fatal("original abort count", aborts)
	}
}
