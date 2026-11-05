// SPDX-License-Identifier: MIT OR Apache-2.0
package stablehash_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/lassejlv/rust-port-go/tools/port-rust-stable-hash/panic-guards/subject"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type panicGuardCase struct {
	Request subject.Request
	Exit    int
	Events  []any
}

func TestStableHashPanicGuardWorker(t *testing.T) {
	data := os.Getenv("FERRUGO_STABLE_PANIC_REQUEST")
	if data == "" {
		return
	}
	var q subject.Request
	if err := json.Unmarshal([]byte(data), &q); err != nil {
		panic(err)
	}
	subject.Run(q)
	os.Exit(0)
}
func TestOriginalStableHashPanicGuards(t *testing.T) {
	data, err := os.ReadFile("testdata/panic_guards.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []panicGuardCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Request.Operation+"/"+c.Request.Context+map[bool]string{true: "/drop-fail"}[c.Request.DropFail], func(t *testing.T) {
			q, _ := json.Marshal(c.Request)
			cmd := exec.Command(os.Args[0], "-test.run=^TestStableHashPanicGuardWorker$")
			cmd.Env = append(os.Environ(), "FERRUGO_STABLE_PANIC_REQUEST="+string(q))
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			var events []any
			s := bufio.NewScanner(bytes.NewReader(output))
			for s.Scan() {
				line := s.Text()
				if strings.HasPrefix(line, "E:") {
					events = append(events, line[2:])
				} else if strings.HasPrefix(line, "H:") {
					var info any
					if err = json.Unmarshal([]byte(line[2:]), &info); err != nil {
						t.Fatal(err)
					}
					events = append(events, info)
				}
			}
			if s.Err() != nil {
				t.Fatal(s.Err())
			}
			if exit != c.Exit || !reflect.DeepEqual(events, c.Events) {
				t.Fatalf("exit=%d want=%d\nevents=%v\nwant=%v\noutput=%s", exit, c.Exit, events, c.Events, output)
			}
		})
	}
}
