package stablehash_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/tools/port-rust-stable-hash/contract"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func rows(t *testing.T, name string, visit func([]byte)) int {
	t.Helper()
	file, e := os.Open("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	reader, e := gzip.NewReader(file)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 4096), 4<<20)
	n := 0
	for scan.Scan() {
		n++
		visit(scan.Bytes())
	}
	if e := scan.Err(); e != nil {
		t.Fatal(e)
	}
	return n
}
func equalJSON(t *testing.T, name string, actual any, wanted json.RawMessage) {
	t.Helper()
	b, e := json.Marshal(actual)
	if e != nil {
		t.Fatal(e)
	}
	var a, c any
	_ = json.Unmarshal(b, &a)
	_ = json.Unmarshal(wanted, &c)
	if !reflect.DeepEqual(a, c) {
		t.Fatalf("%s\nGo%s\nRust%s", name, b, wanted)
	}
}
func TestOriginalStreamingOracle(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained native-word corpus is x86_64; original cross-platform tests remain enabled")
	}
	for _, debug := range []bool{false, true} {
		name := fmt.Sprintf("stream_debug%t.jsonl.gz", debug)
		count := rows(t, name, func(raw []byte) {
			var r struct {
				Request contract.Request
				Result  json.RawMessage
			}
			if e := json.Unmarshal(raw, &r); e != nil {
				t.Fatal(e)
			}
			for _, overflow := range []bool{false, true} {
				equalJSON(t, name, contract.Observe(r.Request, hash.ArithmeticProfile{DebugAssertions: debug, OverflowChecks: overflow}), r.Result)
			}
		})
		if count != 3742 {
			t.Fatal(count)
		}
	}
}
func TestOriginalGenericTraceOracle(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native-word trace fixture is x86_64")
	}
	name := fmt.Sprintf("trace_nightly%t.jsonl.gz", hash.FeatureNightly)
	count := rows(t, name, func(raw []byte) {
		var r struct {
			Request contract.TraceRequest
			Result  []string
		}
		if e := json.Unmarshal(raw, &r); e != nil {
			t.Fatal(e)
		}
		actual := contract.ObserveTrace(r.Request)
		if !reflect.DeepEqual(actual, r.Result) {
			t.Fatalf("%+v\nGo%v\nRust%v", r.Request, actual, r.Result)
		}
	})
	if count != 8 {
		t.Fatal(count)
	}
}
func TestOriginalDebugWriterOracle(t *testing.T) {
	count := rows(t, "format.jsonl.gz", func(raw []byte) {
		var r contract.FormatRow
		if e := json.Unmarshal(raw, &r); e != nil {
			t.Fatal(e)
		}
		actual := contract.ObserveFormat(r)
		if !reflect.DeepEqual(actual, r) {
			t.Fatalf("%s mode%d fail%d dispatch%t\nGo%+v\nRust%+v", r.Name, r.Mode, r.Fail, r.Dispatch, actual, r)
		}
	})
	if count != 10594 {
		t.Fatal(count)
	}
}
func TestOwnershipChild(t *testing.T) {
	raw := os.Getenv("FERRUGO_STABLE_HASH_OWNED_CHILD")
	if raw == "" {
		return
	}
	var q contract.OwnershipCase
	if e := json.Unmarshal([]byte(raw), &q); e != nil {
		panic(e)
	}
	contract.ObserveOwnership(q, func(s string) { fmt.Println("EVENT\t" + s) })
}
func TestOriginalOwnedStateOracle(t *testing.T) {
	data, e := os.ReadFile("../../../../../tools/port-rust-stable-hash/ownership.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Request contract.OwnershipCase
		Code    int
		Events  []string
	}
	if e := json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	for i, row := range cases {
		if row.Code == 0 {
			actual := contract.ObserveOwnership(row.Request, nil)
			if !reflect.DeepEqual(actual, row.Events) {
				t.Fatalf("case%d %+v\nGo%v\nRust%v", i, row.Request, actual, row.Events)
			}
			continue
		}
		q, _ := json.Marshal(row.Request)
		cmd := exec.Command(os.Args[0], "-test.run=^TestOwnershipChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_STABLE_HASH_OWNED_CHILD="+string(q))
		out, e := cmd.Output()
		exit, ok := e.(*exec.ExitError)
		if !ok || exit.ExitCode() != 134 {
			t.Fatalf("case%d exit%v/%s", i, e, out)
		}
		events := []string{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(line, "EVENT\t") {
				t.Fatal(line)
			}
			events = append(events, strings.TrimPrefix(line, "EVENT\t"))
		}
		if !reflect.DeepEqual(events, row.Events) {
			t.Fatalf("case%d Go%v Rust%v", i, events, row.Events)
		}
	}
	if len(cases) != 37 {
		t.Fatal(len(cases))
	}
}
