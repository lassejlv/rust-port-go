// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"testing"

	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-index/validation"
)

func TestOriginalSourceOracle(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("committed original Rust oracle uses 64-bit usize; host-independent unit tests still run")
	}
	file, err := os.Open("idx_testdata/oracle.jsonl.gz")
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
	count, skipped := 0, 0
	for {
		var record oracle.Record
		if err := decoder.Decode(&record); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if record.Nightly && !oracle.NightlyEnabled {
			skipped++
			continue
		}
		if got := oracle.Execute(record.Request, record.OverflowChecks); got != record.Response {
			t.Fatalf("record %d nightly=%t checked=%t request=%+v\nGo=%s\nRust=%s", count, record.Nightly, record.OverflowChecks, record.Request, got, record.Response)
		}
		count++
	}
	want := len(oracle.Requests(false)) * 2
	if oracle.NightlyEnabled {
		want += len(oracle.Requests(true)) * 2
	}
	if count != want {
		t.Fatalf("matched %d oracle records, want %d", count, want)
	}
	t.Logf("matched %d unchanged original-source records; %d require nightly build tag", count, skipped)
}
