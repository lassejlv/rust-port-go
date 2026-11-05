// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
	"os"
	"reflect"
	"testing"
)

func TestOriginalIndexMapBorrowedIterators(t *testing.T) {
	f, err := os.Open("testdata/borrowed_iterator.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 65536), 4<<20)
	n := 0
	for scan.Scan() {
		var row struct {
			Request subject.IteratorRequest
			Result  any
		}
		decoder := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		decoder.UseNumber()
		if err = decoder.Decode(&row); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(subject.RunIterator(row.Request))
		if err != nil {
			t.Fatal(err)
		}
		var actual any
		decoder = json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err = decoder.Decode(&actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, row.Result) {
			t.Fatalf("case%d %v\ngot%s\nwant%v", n, row.Request, data, row.Result)
		}
		n++
	}
	if scan.Err() != nil {
		t.Fatal(scan.Err())
	}
	if n != 506 {
		t.Fatalf("cases%d want506", n)
	}
}
