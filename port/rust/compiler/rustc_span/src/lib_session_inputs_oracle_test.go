package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func libSessionInputsObserved(op string, mode int, trace bool) string {
	events := []string{}
	emit := func(text string) {
		events = append(events, text)
		if trace {
			fmt.Fprintln(os.Stderr, "EVENT:"+text)
		}
	}
	ctx := rustfmt.Context{}
	type loader struct{}
	state := NewSpanOwned(loader{}, func(_ *loader, ctx rustfmt.Context) {
		emit(fmt.Sprintf("loader_drop:%t", ctx.Panicking()))
		if mode == 5 || mode == 6 {
			ctx.Panic("loader drop")
		}
	})
	ownedLoader := NewOwnedFileLoaderWithOps(state, FileLoaderOps[loader]{
		FileExists:     func(*loader, RustPath, rustfmt.Context) bool { panic("unexpected exists") },
		ReadFile:       func(*loader, RustPath, rustfmt.Context) (string, error) { panic("unexpected read") },
		ReadBinaryFile: func(*loader, RustPath, rustfmt.Context) (Shared[[]byte], error) { panic("unexpected binary") },
		CurrentDirectory: func(_ *loader, ctx rustfmt.Context) (RustPath, error) {
			emit(fmt.Sprintf("cwd:%t", ctx.Panicking()))
			switch mode {
			case 1:
				ctx.Panic("cwd")
			case 2, 3, 4:
				return RustPath{}, NewSourceIOError(struct{}{}, SourceIOErrorOps[struct{}]{
					Display: func(*struct{}, *rustfmt.Formatter) error { emit("unexpected_display"); panic("unexpected display") },
					Debug: func(_ *struct{}, f *rustfmt.Formatter) error {
						emit(fmt.Sprintf("error_debug:%t", f.Context.Panicking()))
						if mode == 3 {
							f.Context.Panic("debug")
						}
						return f.WriteString("IoError")
					},
					Drop: func(_ *struct{}, ctx rustfmt.Context) {
						emit(fmt.Sprintf("error_drop:%t", ctx.Panicking()))
						if mode == 4 {
							ctx.Panic("error drop")
						}
					},
				})
			}
			return NewRustPath("/cwd"), nil
		},
	})
	inputs := Some(SourceMapInputs{FileLoader: ownedLoader, PathMapping: EmptyFilePathMapping(), HashKind: SourceFileHashMd5})
	type fstate struct{ calls uint32 }
	env := NewSpanOwned(fstate{}, func(v *fstate, ctx rustfmt.Context) { emit(fmt.Sprintf("f_drop:%d:%t", v.calls, ctx.Panicking())) })
	call := func(v *fstate, _ *SessionScope, ctx rustfmt.Context) uint32 {
		emit(fmt.Sprintf("f_call:%d:%t", v.calls, ctx.Panicking()))
		v.calls++
		return 42
	}
	extras := []string{}
	if mode >= 6 {
		extras = []string{"port_duplicate", "port_duplicate"}
	}
	scope := NewSessionScope()
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		switch op {
		case "construct":
			defer spanCloseOwnedCallback(ctx, env.DropWithContext)
			s := NewSessionGlobals(Edition2024, extras, inputs)
			s.Close()
			env.DropWithContext(ctx)
		case "create":
			n := CreateSessionGlobalsThenOwned(scope, Edition2024, extras, inputs, env, call, ctx)
			emit(fmt.Sprintf("result:%d", n))
		case "present":
			CreateDefaultSessionGlobalsThen(scope, func() struct{} {
				CreateSessionGlobalsThenOwned(scope, Edition2018, extras, inputs, env, call, ctx)
				return struct{}{}
			})
		default:
			panic(op)
		}
	}()
	emit(fmt.Sprintf("panic:%t", panicked))
	return strings.Join(events, ",")
}
func TestLibSessionInputsAbortChild(t *testing.T) {
	op := os.Getenv("FERRUGO_LIB_INPUTS_ABORT")
	if op == "" {
		t.Skip("isolated constructor abort")
	}
	mode, err := strconv.Atoi(os.Getenv("FERRUGO_LIB_INPUTS_MODE"))
	if err != nil {
		t.Fatal(err)
	}
	libSessionInputsObserved(op, mode, true)
	t.Fatal("expected abort")
}
func TestLibOriginalSessionInputs(t *testing.T) {
	f, err := os.Open("testdata/lib-session-inputs.tsv.gz")
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
		t.Fatal("session inputs bound")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count, aborts := 0, 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 3 {
			t.Fatal("inputs shape")
		}
		mode, err := strconv.Atoi(p[1])
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		if strings.HasPrefix(p[0], "abort_") {
			aborts++
			op := strings.TrimPrefix(p[0], "abort_")
			cmd := exec.Command(os.Args[0], "-test.run=^TestLibSessionInputsAbortChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_LIB_INPUTS_ABORT="+op, "FERRUGO_LIB_INPUTS_MODE="+p[1])
			output, err := cmd.CombinedOutput()
			status, ok := err.(*exec.ExitError)
			if !ok || status.ExitCode() != 134 {
				t.Fatalf("%v child %v %s", p[:2], err, output)
			}
			events := []string{}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
			got = strings.Join(events, ",")
		} else {
			got = libSessionInputsObserved(p[0], mode, false)
		}
		if got != p[2] {
			t.Fatalf("row %d %v\nGo: %s\nRust: %s", count, p[:2], got, p[2])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 22 || aborts != 8 {
		t.Fatalf("counts %d %d", count, aborts)
	}
}
