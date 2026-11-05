// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestIndexMapDrainGuardWorker(t *testing.T) {
	raw := os.Getenv("FERRUGO_INDEXMAP_DRAIN_GUARD")
	if raw == "" {
		return
	}
	var q subject.DrainRequest
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		panic(err)
	}
	subject.RunDrain(q)
	os.Exit(0)
}
func TestOriginalIndexMapDrainGuards(t *testing.T) {
	var cases []struct {
		Request          subject.DrainRequest
		Exit, NativeExit int
		Events           []any
	}
	data, err := os.ReadFile("testdata/drain_guards.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	aborted := 0
	for n, row := range cases {
		if row.Exit == 0 && row.NativeExit != 0 || row.Exit == 134 && row.NativeExit != -6 {
			t.Fatal("native fixture did not retain actual Linux SIGABRT classification")
		}
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			raw, _ := json.Marshal(row.Request)
			cmd := exec.Command(os.Args[0], "-test.run=^TestIndexMapDrainGuardWorker$")
			cmd.Env = append(os.Environ(), "FERRUGO_INDEXMAP_DRAIN_GUARD="+string(raw))
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				e, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				exit = e.ExitCode()
			}
			events := []any{}
			scanner := bufio.NewScanner(bytes.NewReader(output))
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "E:") || strings.HasPrefix(line, "H:") {
					var value any
					if err = json.Unmarshal([]byte(line[2:]), &value); err != nil {
						t.Fatal(err)
					}
					events = append(events, value)
				}
			}
			if err = scanner.Err(); err != nil {
				t.Fatal(err)
			}
			if exit != row.Exit || !reflect.DeepEqual(events, row.Events) {
				t.Fatalf("exit%d want%d\ngot%v\nwant%v\noutput%s", exit, row.Exit, events, row.Events, output)
			}
		})
		if row.Exit != 0 {
			aborted++
		}
	}
	if len(cases) != 164 || aborted != 39 {
		t.Fatalf("native guard/abort evidence: %d/%d want164/39", len(cases), aborted)
	}
}
