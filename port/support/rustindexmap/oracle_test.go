// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
)

func TestOriginalIndexMapCore(t *testing.T) {
	file, err := os.Open("testdata/core.jsonl.gz")
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
	scanner.Buffer(make([]byte, 4096), 4<<20)
	rows, ops := 0, 0
	for scanner.Scan() {
		var row struct {
			Request subject.Request
			Result  json.RawMessage
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(subject.Run(row.Request))
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		decode := func(data []byte, target *any) {
			d := json.NewDecoder(strings.NewReader(string(data)))
			d.UseNumber()
			if err := d.Decode(target); err != nil {
				t.Fatal(err)
			}
		}
		decode(got, &a)
		decode(row.Result, &b)
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("case%d request%+v\ngot%s\nwant%s", rows, row.Request, got, row.Result)
		}
		rows++
		ops += len(row.Request.Ops)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows < 60 || ops < 18000 {
		t.Fatalf("incomplete core corpus:%d cases/%d operations", rows, ops)
	}
}
