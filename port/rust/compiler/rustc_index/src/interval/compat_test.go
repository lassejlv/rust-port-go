// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	compat "github.com/lassejlv/rust-port-go/tools/port-rust-interval/compat/validation"
)

func intervalCompatDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repositoryRoot(t), "tools/port-rust-interval/compat")
}

type intervalCompatAbort struct {
	Request compat.Request
	Trace   []string
	Class   string
}

func TestIntervalCompatibilityOriginal(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native reference pins64bit usize")
	}
	dir := intervalCompatDir(t)
	var manifest struct {
		Revision   string            `json:"revision"`
		Records    int               `json:"records"`
		Aborts     int               `json:"aborts"`
		Abort      string            `json:"abort_sha256"`
		Inputs     map[string]string `json:"inputs"`
		Compressed string            `json:"compressed_sha256"`
		Decoded    string            `json:"decoded_sha256"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" {
		t.Fatal("source pin changed")
	}
	for path, want := range manifest.Inputs {
		data, err = os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if digest(data) != want {
			t.Fatalf("compatibility oracle input changed: %s; regenerate original output", path)
		}
	}
	data, err = os.ReadFile(filepath.Join(dir, "cases.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(data) != manifest.Compressed {
		t.Fatal("compressed native checksum changed")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	decoded, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if digest(decoded) != manifest.Decoded {
		t.Fatal("decoded native checksum changed")
	}
	requests := compat.Requests()
	scan := bufio.NewScanner(bytes.NewReader(decoded))
	scan.Buffer(make([]byte, 64*1024), 4*1024*1024)
	n := 0
	for scan.Scan() {
		var record compat.Record
		if err = json.Unmarshal(scan.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if n >= len(requests) || record.Request != requests[n] {
			t.Fatalf("retained request%d changed", n)
		}
		actual := compat.Replay(record.Request, nil)
		if !reflect.DeepEqual(actual, record.Response) {
			t.Fatalf("case%d %+v\nGo:%+v\nRust:%+v", n, record.Request, actual, record.Response)
		}
		n++
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if n != manifest.Records || n != len(requests) {
		t.Fatalf("record count%d want%d/%d", n, manifest.Records, len(requests))
	}
	data, err = os.ReadFile(filepath.Join(dir, "aborts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(data) != manifest.Abort {
		t.Fatal("native abort records checksum changed")
	}
	var aborts []intervalCompatAbort
	if err = json.Unmarshal(data, &aborts); err != nil {
		t.Fatal(err)
	}
	wanted := compat.AbortRequests()
	if len(aborts) != manifest.Aborts || len(aborts) != len(wanted) {
		t.Fatal("native abort count changed")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for n, record := range aborts {
		if record.Request != wanted[n] || record.Class != "abort" {
			t.Fatalf("abort request%d changed", n)
		}
		encoded, _ := json.Marshal(record.Request)
		cmd := exec.Command(executable, "-test.run=^TestIntervalCompatibilityAbortChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_INTERVAL_COMPAT_CHILD="+string(encoded))
		output, err := cmd.Output()
		status, ok := err.(*exec.ExitError)
		if !ok || status.ExitCode() != 134 {
			t.Fatalf("abort%d got %v\n%s", n, err, output)
		}
		events := []string{}
		for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "event:") {
				t.Fatalf("unexpected child output:%s", line)
			}
			event, err := hex.DecodeString(strings.TrimPrefix(line, "event:"))
			if err != nil {
				t.Fatal(err)
			}
			events = append(events, string(event))
		}
		if !reflect.DeepEqual(events, record.Trace) {
			t.Fatalf("abort%d request%+v\nGo:%v\nRust:%v", n, record.Request, events, record.Trace)
		}
	}
}
func TestIntervalCompatibilityAbortChild(t *testing.T) {
	text, ok := os.LookupEnv("FERRUGO_INTERVAL_COMPAT_CHILD")
	if !ok {
		t.Skip("isolated child only")
	}
	var request compat.Request
	if err := json.Unmarshal([]byte(text), &request); err != nil {
		t.Fatal(err)
	}
	result := compat.Replay(request, func(event string) { fmt.Println("event:" + hex.EncodeToString([]byte(event))) })
	t.Fatalf("required source abort returned:%+v", result)
}

func TestIntervalOwnedRangeDestructionAndAmbientCatch(t *testing.T) {
	for _, ambient := range []bool{false, true} {
		var events []string
		set := index.NewIntervalSet(index.UsizeIdx(), 32, index.UsizeIntervalStep())
		failCloneDrop := false
		var makeRange func(uint) index.IntervalRangeBounds[uint]
		makeRange = func(generation uint) index.IntervalRangeBounds[uint] {
			return index.IntervalRangeBounds[uint]{
				CloneWithContextFn: func(context index.IntervalPanicContext) index.IntervalRangeBounds[uint] {
					events = append(events, fmt.Sprint("clone:", generation, ":", context.Panicking()))
					return makeRange(generation + 1)
				},
				StartBoundWithContextFn: func(context index.IntervalPanicContext) index.IntervalBound[uint] {
					events = append(events, fmt.Sprint("start:", generation, ":", context.Panicking()))
					return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 2}
				},
				EndBoundWithContextFn: func(context index.IntervalPanicContext) index.IntervalBound[uint] {
					events = append(events, fmt.Sprint("end:", generation, ":", context.Panicking()))
					return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 8}
				},
				CloseWithContext: func(context index.IntervalPanicContext) {
					events = append(events, fmt.Sprint("drop:", generation, ":", context.Panicking()))
					if generation == 1 && failCloneDrop {
						context.Panic("clone drop")
					}
				},
			}
		}
		context := index.IntervalPanicContext{Unwinding: ambient}
		if !set.InsertRangeWithContext(context, makeRange(0)) {
			t.Fatal("valid insertion did not change set")
		}
		flag := fmt.Sprint(ambient)
		want := []string{"clone:0:" + flag, "start:1:" + flag, "drop:1:" + flag, "end:0:" + flag, "drop:0:" + flag}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("normal range phases=%v", events)
		}
		events = nil
		failCloneDrop = true
		panicked := false
		func() {
			defer func() { panicked = recover() != nil }()
			set.IntersectsRangeWithContext(context, makeRange(0))
		}()
		if !panicked {
			t.Fatal("normal clone destruction panic was not catchable")
		}
		want = []string{"clone:0:" + flag, "start:1:" + flag, "drop:1:" + flag, "drop:0:true"}
		if !reflect.DeepEqual(events, want) {
			t.Fatalf("clone-Drop failure phases=%v", events)
		}
	}
}

func TestIntervalOwnedRangeClosesWhenMatrixEnsureFails(t *testing.T) {
	var events []string
	row := index.NewIdx(func(n uint) uint { return n }, func(n uint) uint { events = append(events, "row"); panic("row index") })
	matrix := index.NewSparseIntervalMatrix(row, func(a, b uint) bool { return a == b }, index.UsizeIdx(), 32, index.UsizeIntervalStep())
	rangeValue := index.IntervalRangeBounds[uint]{CloseWithContext: func(context index.IntervalPanicContext) {
		events = append(events, fmt.Sprint("drop:", context.Panicking()))
	}, CloneFn: func() index.IntervalRangeBounds[uint] {
		t.Fatal("range cloned before row succeeded")
		return index.IntervalRangeBounds[uint]{}
	}}
	panicked := false
	func() { defer func() { panicked = recover() != nil }(); matrix.InsertRange(4, rangeValue) }()
	if !panicked || !reflect.DeepEqual(events, []string{"row", "drop:true"}) {
		t.Fatalf("matrix failure phases=%v", events)
	}
}

func TestIntervalDebugRequiresCanonicalTypeName(t *testing.T) {
	set := index.NewIntervalSet(index.UsizeIdx(), 8, index.IntervalStep[uint]{})
	f := index.IndexDebugFormatter{Options: index.DefaultIndexFormatOptions(), Write: func(string) error { return nil }}
	panicked := false
	func() { defer func() { panicked = recover() != nil }(); _ = set.FormatDebug(&f) }()
	if !panicked {
		t.Fatal("missing Rust type-name witness was silently replaced")
	}
}
