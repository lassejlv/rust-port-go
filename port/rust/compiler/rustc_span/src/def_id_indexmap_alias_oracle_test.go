// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-def-id/indexmap-alias/subject"
	"os"
	"reflect"
	"testing"
)

func TestDefIdOriginalIndexMapAlias(t *testing.T) {
	f, err := os.Open("testdata/def_id_indexmap_alias.jsonl.gz")
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
	scan.Buffer(make([]byte, 65536), 8<<20)
	n := 0
	for scan.Scan() {
		var raw struct {
			Request subject.Request
			Result  any
		}
		decoder := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		decoder.UseNumber()
		if err = decoder.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(subject.Run(raw.Request))
		if err != nil {
			t.Fatal(err)
		}
		var actual any
		decoder = json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err = decoder.Decode(&actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, raw.Result) {
			t.Fatalf("case%d alias mismatch\ngot%s\nwant%v", n, data, raw.Result)
		}
		n++
	}
	if scan.Err() != nil {
		t.Fatal(scan.Err())
	}
	if n != 24 {
		t.Fatalf("cases%d want24", n)
	}
}
