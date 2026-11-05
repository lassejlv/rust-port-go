// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
	"os"
	"reflect"
	"testing"
)

func TestOriginalIndexMapOwned(t *testing.T) {
	f, err := os.Open("testdata/owned.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 65536), 2<<20)
	n := 0
	for scanner.Scan() {
		var row struct {
			Request subject.OwnedRequest
			Result  any
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		actual, err := json.Marshal(subject.RunOwned(row.Request))
		if err != nil {
			t.Fatal(err)
		}
		var normalized any
		if err = json.Unmarshal(actual, &normalized); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(normalized, row.Result) {
			t.Fatalf("case%d %v\ngot %s\nwant %v", n, row.Request, actual, row.Result)
		}
		n++
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if n != 34056 {
		t.Fatalf("case count%d want34056", n)
	}
}
