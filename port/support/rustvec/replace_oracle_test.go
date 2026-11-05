// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/replace-owned/subject"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type replaceCase struct {
	Request subject.Request
	Exit    int
	Events  []any
}

func TestReplaceOwnedWorker(t *testing.T) {
	raw := os.Getenv("FERRUGO_REPLACE_OWNED_REQUEST")
	if raw == "" {
		return
	}
	var q subject.Request
	if err := json.Unmarshal([]byte(raw), &q); err != nil {
		panic(err)
	}
	subject.Run(q)
	os.Exit(0)
}
func TestOriginalReplaceOwned(t *testing.T) {
	data, err := os.ReadFile("testdata/replace_owned.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []replaceCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, row := range cases {
		t.Run(row.Request.Operation+"/"+row.Request.Context+"/"+row.Request.Observer+"/"+row.Request.Failure, func(t *testing.T) {
			raw, _ := json.Marshal(row.Request)
			cmd := exec.Command(os.Args[0], "-test.run=^TestReplaceOwnedWorker$")
			cmd.Env = append(os.Environ(), "FERRUGO_REPLACE_OWNED_REQUEST="+string(raw))
			output, err := cmd.CombinedOutput()
			exit := 0
			if err != nil {
				if e, ok := err.(*exec.ExitError); ok {
					exit = e.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			events := []any{}
			scanner := bufio.NewScanner(bytes.NewReader(output))
			for scanner.Scan() {
				line := scanner.Text()
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
			if scanner.Err() != nil {
				t.Fatal(scanner.Err())
			}
			if exit != row.Exit || !reflect.DeepEqual(events, row.Events) {
				t.Fatalf("exit%d want%d\nevents%v\nwant%v\noutput%s", exit, row.Exit, events, row.Events, output)
			}
		})
	}
	if len(cases) != 72 {
		t.Fatalf("cases%d want72", len(cases))
	}
}
