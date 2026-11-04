// SPDX-License-Identifier: MIT OR Apache-2.0
package semver_test

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"github.com/lassejlv/rust-port-go/tools/port-semver/contract"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func replay(t *testing.T, name string, wanted int) {
	t.Helper()
	f, e := os.Open("testdata/" + name + ".jsonl.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	r, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), 1<<20)
	n := 0
	for s.Scan() {
		n++
		var row struct {
			Request json.RawMessage
			Result  json.RawMessage
		}
		if e := json.Unmarshal(s.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		q, e := contract.DecodeRequest(row.Request)
		if e != nil {
			t.Fatal(e)
		}
		gotRaw, e := json.Marshal(contract.Observe(q))
		if e != nil {
			t.Fatal(e)
		}
		var got, want any
		_ = json.Unmarshal(gotRaw, &got)
		_ = json.Unmarshal(row.Result, &want)
		// Rust oracle runs on64-bit. Concrete primitive callbacks are portable; raw
		// native-word bytes are tested separately on the executing Go architecture.
		if strconv.IntSize != 64 {
			for _, v := range []any{got, want} {
				if m, ok := v.(map[string]any); ok {
					if h, ok := m["hash"].(map[string]any); ok {
						delete(h, "raw")
					}
				}
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("record%d %s\ngot %s\nwant%s", n, row.Request, gotRaw, row.Result)
		}
	}
	if e := s.Err(); e != nil {
		t.Fatal(e)
	}
	if n != wanted {
		t.Fatalf("fixture count%d wanted%d", n, wanted)
	}
}
func TestOriginalCoreOracle(t *testing.T) { replay(t, "core", 33982) }
