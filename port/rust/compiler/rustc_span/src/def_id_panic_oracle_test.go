package rustcspan

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type defIDPanicCase struct {
	Op, Debug, Context string
	Local              bool
}

func defIDObservePanic(q defIDPanicCase, live bool) []string {
	events := []string{}
	event := func(s string) {
		events = append(events, s)
		if live {
			fmt.Println("EVENT\t" + s)
		}
	}
	original := LoadDefIdDebug()
	var first, second *DefIdDebugCallback
	second = NewDefIdDebugCallback(func(_ DefId, f *rustfmt.Formatter) error { event("second"); return f.WriteString("second") })
	first = NewDefIdDebugCallback(func(_ DefId, f *rustfmt.Formatter) error {
		event("first")
		SwapDefIdDebug(second)
		return f.WriteString("first")
	})
	custom := NewDefIdDebugCallback(func(_ DefId, f *rustfmt.Formatter) error {
		event(fmt.Sprintf("debug:panic=%t", f.Context.Panicking()))
		switch q.Debug {
		case "error":
			return errors.New("fmt::Error")
		case "partial":
			if e := f.WriteString("piece"); e != nil {
				return e
			}
			return errors.New("fmt::Error")
		case "panic":
			f.Context.Panic("debug panic")
		}
		return f.WriteString("custom")
	})
	old := original
	if q.Debug == "swap" {
		old = SwapDefIdDebug(first)
	} else if q.Debug != "default" {
		old = SwapDefIdDebug(custom)
	}
	event(fmt.Sprintf("original:%t", old == original))
	ctx := rustfmt.Context{Unwinding: q.Context == "inherited", ProcessingPanic: q.Context == "processing"}
	if q.Context == "inherited" {
		event("inherited:panic=true")
	}
	if q.Context == "processing" {
		event("processing:panic=true")
	}
	crate := uint32(1)
	if q.Local {
		crate = 0
	}
	id := DefId{Krate: CrateNumFromU32(crate), Index: DefIndexFromU32(2)}
	p, failed := defIDCatch(func() any {
		if q.Op == "format" {
			value := defIdDebugString(ctx, id.WriteDebugFormatter)
			if q.Debug == "swap" {
				value += "|" + defIdDebugString(ctx, id.WriteDebugFormatter)
			}
			event("value:" + value)
		} else {
			event(fmt.Sprintf("value:%d", id.ExpectLocalWithContext(ctx).LocalDefIndex.AsU32()))
		}
		return nil
	})
	event(fmt.Sprintf("caught:%t", failed))
	if failed {
		event("payload:" + fmt.Sprint(p))
	}
	if q.Context == "inherited" || q.Context == "processing" {
		event("outer-caught:true")
	}
	SwapDefIdDebug(old)
	event(fmt.Sprintf("restored:%t", LoadDefIdDebug() == original))
	return events
}
func TestDefIdPanicChild(t *testing.T) {
	raw := os.Getenv("FERRUGO_DEF_ID_PANIC_CHILD")
	if raw == "" {
		return
	}
	var q defIDPanicCase
	if e := json.Unmarshal([]byte(raw), &q); e != nil {
		panic(e)
	}
	defIDObservePanic(q, true)
	os.Exit(0)
}
func TestDefIdOriginalLazyPanic(t *testing.T) {
	path := os.Getenv("FERRUGO_DEF_ID_PANIC_FIXTURE")
	if path == "" {
		path = "testdata/def_id_panic.jsonl"
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	n := 0
	for scan.Scan() {
		n++
		var row struct {
			Request defIDPanicCase
			Code    int
			Events  []string
		}
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		if row.Code == 0 {
			actual := defIDObservePanic(row.Request, false)
			if !reflect.DeepEqual(actual, row.Events) {
				t.Fatalf("row%d %+v\nGo%v\nRust%v", n, row.Request, actual, row.Events)
			}
			continue
		}
		q, _ := json.Marshal(row.Request)
		cmd := exec.Command(os.Args[0], "-test.run=^TestDefIdPanicChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_DEF_ID_PANIC_CHILD="+string(q))
		out, e := cmd.Output()
		exit, ok := e.(*exec.ExitError)
		if !ok || exit.ExitCode() != 134 {
			t.Fatalf("row%d exit%v/%s", n, e, out)
		}
		events := []string{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(line, "EVENT\t") {
				t.Fatal(line)
			}
			events = append(events, strings.TrimPrefix(line, "EVENT\t"))
		}
		if !reflect.DeepEqual(events, row.Events) {
			t.Fatalf("row%d %+v\nGo%v\nRust%v", n, row.Request, events, row.Events)
		}
	}
	if e := scan.Err(); e != nil {
		t.Fatal(e)
	}
	if n != 72 {
		t.Fatal("incomplete lazy panic corpus", n)
	}
}
