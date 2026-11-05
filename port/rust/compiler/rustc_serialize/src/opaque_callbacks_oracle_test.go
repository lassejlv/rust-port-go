package rustcserialize

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func opaqueCallbacksObserved(kind string, mode int, pos uint, trace bool) string {
	events := []string{}
	ctx := rustfmt.Context{}
	emit := func(text string) {
		events = append(events, text)
		if trace {
			fmt.Fprintln(os.Stderr, "EVENT:"+text)
		}
	}
	type state struct{ calls uint32 }
	owned := rustvec.NewOwned(state{}, rustvec.ValueOps[state]{Drop: func(value *state, ctx rustfmt.Context) {
		defer func() {
			p := recover()
			if p != nil && ctx.ProcessingPanic {
				ctx.Panic(p)
			}
			dropctx := ctx
			if p != nil {
				dropctx.Unwinding = true
			}
			emit(fmt.Sprintf("token:%t", dropctx.Panicking()))
			if p != nil {
				panic(p)
			}
		}()
		emit(fmt.Sprintf("drop:%d:%t", value.calls, ctx.Panicking()))
		if mode == 2 || mode == 3 {
			ctx.Panic("drop")
		}
	}})
	invoke := func(ref rustvec.Ref[state], name string) {
		n := ref.Value.calls
		ref.Value.calls++
		emit(fmt.Sprintf("%s:%d:%t", name, n, ref.Context.Panicking()))
	}
	if kind == "mem" {
		e := NewMemEncoder()
		e.EmitU8(17)
		panicked := fileOraclePanic(func() {
			MemEncoderWriteWithOwned(e, 4, owned, func(ref rustvec.Ref[state], window []byte) uint {
				invoke(ref, "once")
				emit("window:" + hex.EncodeToString(window))
				for i := range window {
					window[i] = 0x7b
				}
				if mode == 1 || mode == 3 {
					ref.Context.Panic("call")
				}
				if mode == 4 {
					return 5
				}
				return 3
			}, ctx)
		})
		emit(fmt.Sprintf("panic:%t", panicked))
		emit(fmt.Sprintf("position:%d", e.Position()))
		emit("bytes:" + hex.EncodeToString(e.Finish()))
	} else if kind == "file" {
		directory := os.Getenv("FERRUGO_OPAQUE_DIRECTORY")
		if directory == "" {
			var err error
			directory, err = os.MkdirTemp("", "ferrugo-opaque-callbacks-")
			if err != nil {
				panic(err)
			}
			defer os.RemoveAll(directory)
		}
		path := filepath.Join(directory, "file")
		e, err := NewFileEncoder(path)
		if err != nil {
			panic(err)
		}
		e.EmitU8(17)
		panicked := fileOraclePanic(func() {
			FileEncoderWriteWithOwned(e, 4, owned, func(ref rustvec.Ref[state], window []byte) uint {
				invoke(ref, "once")
				emit("window:" + hex.EncodeToString(window))
				for i := range window {
					window[i] = 0x7b
				}
				if mode == 1 || mode == 3 {
					ref.Context.Panic("call")
				}
				if mode == 4 {
					return 5
				}
				return 3
			}, ctx)
		})
		emit(fmt.Sprintf("panic:%t", panicked))
		emit(fmt.Sprintf("position:%d", e.Position()))
		if _, err := e.Finish(); err != nil {
			panic(err)
		}
		e.Close()
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		emit("bytes:" + hex.EncodeToString(data))
	} else {
		data := append([]byte{17, 34, 51, 68}, []byte(MagicEndBytes)...)
		d, err := NewMemDecoder(data, 1)
		if err != nil {
			panic(err)
		}
		var out byte
		panicked := fileOraclePanic(func() {
			out = MemDecoderWithPositionOwned(d, pos, owned, func(ref rustvec.Ref[state], d *MemDecoder) byte {
				invoke(ref, "fn")
				emit(fmt.Sprintf("position:%d", d.Position()))
				value := d.ReadU8()
				if mode == 1 || mode == 3 {
					ref.Context.Panic("call")
				}
				return value
			}, ctx)
		})
		if panicked {
			emit("panic")
		} else {
			emit(fmt.Sprintf("result:%d", out))
		}
		emit(fmt.Sprintf("position:%d", d.Position()))
	}
	return strings.Join(events, ",")
}
func TestOpaqueCallbacksAbortChild(t *testing.T) {
	kind := os.Getenv("FERRUGO_OPAQUE_ABORT")
	if kind == "" {
		t.Skip("isolated source callback abort")
	}
	mode, err := strconv.Atoi(os.Getenv("FERRUGO_OPAQUE_MODE"))
	if err != nil {
		t.Fatal(err)
	}
	pos, err := strconv.ParseUint(os.Getenv("FERRUGO_OPAQUE_POS"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	opaqueCallbacksObserved(kind, mode, uint(pos), true)
	t.Fatal("expected abort")
}
func TestOriginalOpaqueOwnedCallbacks(t *testing.T) {
	directory := t.TempDir()
	f, err := os.Open("testdata/opaque-callbacks.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 1<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1<<20 {
		t.Fatal("opaque callbacks bound")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count := 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 {
			t.Fatal("opaque callback shape")
		}
		mode, err := strconv.Atoi(p[1])
		if err != nil {
			t.Fatal(err)
		}
		pos, err := strconv.ParseUint(p[2], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if strings.HasPrefix(p[0], "abort_") {
			kind := strings.TrimPrefix(p[0], "abort_")
			cmd := exec.Command(os.Args[0], "-test.run=^TestOpaqueCallbacksAbortChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_OPAQUE_ABORT="+kind, "FERRUGO_OPAQUE_MODE="+p[1], "FERRUGO_OPAQUE_POS="+p[2], "FERRUGO_OPAQUE_DIRECTORY="+directory)
			output, err := cmd.CombinedOutput()
			status, ok := err.(*exec.ExitError)
			if !ok || status.ExitCode() != 134 {
				t.Fatalf("child %v: %v %s", p[:3], err, output)
			}
			events := []string{}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
			got = strings.Join(events, ",")
		} else {
			got = opaqueCallbacksObserved(p[0], mode, uint(pos), false)
		}
		if got != p[3] {
			t.Fatalf("row %d %v\nGo: %s\nRust: %s", count, p[:3], got, p[3])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 62 {
		t.Fatal(count)
	}
}
