// SPDX-License-Identifier: MIT OR Apache-2.0
package unionfind_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"

	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-union-find/validation"
)

func TestOriginalSourceOracle(t *testing.T) {
	file, err := os.Open("testdata/oracle.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	count := 0
	for {
		var record oracle.Record
		if err := decoder.Decode(&record); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if got := oracle.Execute(record.Request, record.DebugAssertions); got != record.Response {
			t.Fatalf("case %d debug=%t request=%+v\nGo=%s\nRust=%s", count, record.DebugAssertions, record.Request, got, record.Response)
		}
		count++
	}
	if want := len(oracle.Requests()) * 2; count != want {
		t.Fatalf("oracle has %d records, want %d", count, want)
	}
	t.Logf("matched %d original-source records, including callback traces, panics and table state", count)
}
