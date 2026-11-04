// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func storageAbortRequests() []storageRequest {
	var out []storageRequest
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, op := range []string{"outer_panic", "drain_owned", "drain_enum_owned"} {
				failure := "bounds_close"
				if op == "outer_panic" {
					failure = "drop"
				}
				out = append(out, storageRequest{Kind: "abort", Name: "abort-" + op, Checked: checked, Debug: debug, Values: []int64{1, 3, 5}, Other: []int64{}, Capacity: 3, Limit: 64, Actions: []storageAction{{Op: op, A: 4, B: 3, StartKind: 1, EndKind: 2, Panic: failure, PanicAt: 1, Commands: []string{"close"}}}})
			}
		}
	}
	return out
}

func TestStorageAbortHelper(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_STORAGE_ABORT_HELPER")
	if !ok {
		t.Skip("isolated original-source abort parity helper")
	}
	var r storageRequest
	if err := json.Unmarshal([]byte(encoded), &r); err != nil {
		t.Fatal(err)
	}
	storageReplay(r)
	t.Fatal("double panic unexpectedly returned to the caller")
}

func storageReplayAbort(t *testing.T, r storageRequest) map[string]any {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestStorageAbortHelper$")
	cmd.Env = append(os.Environ(), "FERRUGO_STORAGE_ABORT_HELPER="+mustStorageJSON(r), "FERRUGO_STORAGE_STREAM_EVENTS=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	exited, ok := err.(*exec.ExitError)
	if ok {
		exit = exited
	}
	if exit == nil || exit.ExitCode() != 134 {
		t.Fatalf("Go double panic must abort with exit134, got %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	events := []string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "ORACLE_EVENT:") {
			var event string
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "ORACLE_EVENT:")), &event); err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
	}
	return map[string]any{"abort": true, "events": events}
}

func storageStreamEvent(event string) {
	if os.Getenv("FERRUGO_STORAGE_STREAM_EVENTS") == "1" {
		data, _ := json.Marshal(event)
		fmt.Fprintln(os.Stdout, "ORACLE_EVENT:"+string(data))
	}
}
