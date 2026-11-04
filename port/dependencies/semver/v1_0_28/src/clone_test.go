package semver_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"github.com/lassejlv/rust-port-go/tools/port-semver/contract"
)

// The unchanged crate is observed with source/destination Vec spare capacity,
// including sizes where Go's append allocation rounds beyond the source len.
func TestOriginalCloneCapacityAndDefaultCloneFrom(t *testing.T) {
	file, err := os.Open("testdata/clone.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	n := 0
	for scanner.Scan() {
		n++
		var row struct {
			Request contract.CloneRequest
			Result  json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		actual, err := json.Marshal(contract.ObserveClone(row.Request))
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		if err := json.Unmarshal(actual, &a); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(row.Result, &b); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("row%d %+v\nGo%s\nRust%s", n, row.Request, actual, row.Result)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 108 || n != len(contract.CloneCorpus()) {
		t.Fatalf("incomplete clone fixture:%d", n)
	}
}

func TestFromComparatorsCopiesExactLength(t *testing.T) {
	source := make([]semver.Comparator, 24, 100)
	source[0].Major = 1
	req := semver.FromComparators(source)
	if len(req.Comparators) != 24 || cap(req.Comparators) != 24 {
		t.Fatalf("len=%d cap=%d", len(req.Comparators), cap(req.Comparators))
	}
	req.Comparators[0].Major = 2
	if source[0].Major != 1 {
		t.Fatal("convenience constructor retained mutable source storage")
	}
}
