package semver_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"github.com/lassejlv/rust-port-go/tools/port-semver/contract"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestOriginalOwnedCollectorOracle(t *testing.T) {
	file, e := os.Open("testdata/collector.jsonl.gz")
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
	scan.Buffer(make([]byte, 4096), 1<<20)
	n := 0
	for scan.Scan() {
		n++
		var row struct {
			Request contract.CollectorRequest
			Result  json.RawMessage
		}
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		actual, e := json.Marshal(contract.ObserveCollector(row.Request, nil))
		if e != nil {
			t.Fatal(e)
		}
		var a, b any
		_ = json.Unmarshal(actual, &a)
		_ = json.Unmarshal(row.Result, &b)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("row%d %+v\nGo%s\nRust%s", n, row.Request, actual, row.Result)
		}
	}
	if e := scan.Err(); e != nil {
		t.Fatal(e)
	}
	if n != 4142 {
		t.Fatalf("incomplete collector fixture:%d", n)
	}
}
func TestCollectorAbortChild(t *testing.T) {
	raw := os.Getenv("FERRUGO_COLLECTOR_CHILD")
	if raw == "" {
		return
	}
	var q contract.CollectorRequest
	if e := json.Unmarshal([]byte(raw), &q); e != nil {
		panic(e)
	}
	contract.ObserveCollector(q, func(s string) { fmt.Println("EVENT\t" + s) })
	os.Exit(0)
}
func TestOriginalCollectorAbortPrefixes(t *testing.T) {
	data, e := os.ReadFile("../../../../../tools/port-semver/collector-aborts.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Request contract.CollectorRequest
		Class   string
		Events  []string
	}
	if e := json.Unmarshal(data, &rows); e != nil {
		t.Fatal(e)
	}
	for n, row := range rows {
		request, _ := json.Marshal(row.Request)
		cmd := exec.Command(os.Args[0], "-test.run=^TestCollectorAbortChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_COLLECTOR_CHILD="+string(request))
		output, e := cmd.Output()
		exit, ok := e.(*exec.ExitError)
		if !ok || exit.ExitCode() != 134 || row.Class != "abort" {
			t.Fatalf("row%d expected abort, got%v/%s", n, e, output)
		}
		events := []string{}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if !strings.HasPrefix(line, "EVENT\t") {
				t.Fatalf("unexpected child output%q", line)
			}
			events = append(events, strings.TrimPrefix(line, "EVENT\t"))
		}
		if !reflect.DeepEqual(events, row.Events) {
			t.Fatalf("row%d %v want%v", n, events, row.Events)
		}
	}
	if len(rows) != 30 {
		t.Fatalf("incomplete abort fixture:%d", len(rows))
	}
}
func TestOwnedCollectorRetainsStateCellAndMovesInput(t *testing.T) {
	type observed struct{ next, hints, drops int }
	trace := &observed{}
	type state struct {
		position, hints, next int
		nonCopy               []int
		observed              *observed
	}
	original := rustvec.NewOwned(state{nonCopy: []int{7}, observed: trace}, rustvec.ValueOps[state]{Drop: func(value *state, ctx rustvec.Context) {
		value.observed.next = value.next
		value.observed.hints = value.hints
		value.observed.drops++
		if ctx.Panicking() {
			t.Fatal("unexpected unwind")
		}
	}})
	identity := original.Borrow()
	ops := rustvec.IteratorOps[state, semver.Comparator]{Next: func(ref rustvec.Ref[state]) (semver.Comparator, bool) {
		if ref.Value != identity {
			t.Fatal("copied iterator state")
		}
		ref.Value.next++
		if ref.Value.position == 2 {
			return semver.Comparator{}, false
		}
		ref.Value.position++
		return semver.Comparator{Major: uint64(ref.Value.position)}, true
	}, SizeHint: func(ref rustvec.Ref[state]) rustvec.Hint {
		if ref.Value != identity {
			t.Fatal("copied hint state")
		}
		ref.Value.hints++
		return rustvec.Hint{Lower: uint(2 - ref.Value.position)}
	}}
	input := rustvec.NewIntoIteratorFromOwned(original, rustvec.IntoIteratorOps[state, semver.Comparator]{IntoIter: func(arg *rustvec.Owned[state]) rustvec.Iterator[semver.Comparator] {
		if arg.Borrow() != identity {
			t.Fatal("copied IntoIterator state")
		}
		return rustvec.NewIteratorFromOwned(arg.Move(), ops)
	}})
	req := semver.FromOwnedIterator(input)
	if len(req.Comparators) != 2 || cap(req.Comparators) != 4 || trace.next != 3 || trace.hints != 1 || trace.drops != 1 {
		t.Fatal(req, cap(req.Comparators), trace)
	}
	original.Drop()
	if trace.drops != 1 {
		t.Fatal("double drop")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("consumed input remained borrowable")
		}
	}()
	original.Borrow()
}
