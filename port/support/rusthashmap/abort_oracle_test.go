// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

type abortRequest struct {
	Scenario int    `json:"scenario"`
	Ambient  string `json:"ambient"`
	Fail     int    `json:"fail"`
	Second   int    `json:"second"`
}
type abortResponse struct {
	Abort         bool     `json:"abort"`
	Events        []string `json:"events"`
	Action        string   `json:"action,omitempty"`
	Final         string   `json:"final,omitempty"`
	Counter       int      `json:"counter,omitempty"`
	State         string   `json:"state,omitempty"`
	ReferenceExit string   `json:"reference_exit,omitempty"`
}
type abortRecord struct {
	Request  abortRequest  `json:"request"`
	Response abortResponse `json:"response"`
}

func TestHashMapAbortHelper(t *testing.T) {
	text := os.Getenv("FERRUGO_HASHMAP_ABORT_REQUEST")
	if text == "" {
		return
	}
	var request abortRequest
	if err := json.Unmarshal([]byte(text), &request); err != nil {
		t.Fatal(err)
	}
	control := &auditControl{fail: request.Fail, second: request.Second, live: true}
	c := Context{Unwinding: request.Ambient == "unwinding", ProcessingPanic: request.Ambient == "processing_panic"}
	action, final, state := auditWork(request.Scenario, control, c)
	response := abortResponse{Events: control.events, Action: action, Final: final, Counter: control.counter, State: state}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("HASHMAP_RESULT\t%s\n", body)
}

func TestOriginalHashMapAbortPrefixes(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/abort_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Owned     string `json:"owned_generator_sha256"`
		Support   string `json:"support_generator_sha256"`
		Profiles  []struct {
			Aborts int `json:"aborts"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/ownership_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/abort_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/ownership_generate.py": manifest.Owned, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("abort reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/aborts.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 864 || len(manifest.Profiles) != 2 || manifest.Profiles[0].Aborts != 837 || manifest.Profiles[1].Aborts != 837 {
		t.Fatal("abort reference fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 32<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("abort reference body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases, aborts := 0, 0
	for scanner.Scan() {
		cases++
		var record abortRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		request, err := json.Marshal(record.Request)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHashMapAbortHelper$", "-test.count=1")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "FERRUGO_HASHMAP_ABORT_REQUEST=") && !strings.HasPrefix(entry, "GORACE=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		// These children intentionally terminate before ordinary teardown. They
		// are single-threaded witnesses; preserve other requested race options
		// while omitting the race runtime's one-second exit grace period.
		cmd.Env = append(cmd.Env, "FERRUGO_HASHMAP_ABORT_REQUEST="+string(request), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
		output, runErr := cmd.CombinedOutput()
		cancel()
		got := abortResponse{Events: []string{}}
		completed := false
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, "HASHMAP_EVENT\t") {
				b, err := hex.DecodeString(strings.TrimPrefix(line, "HASHMAP_EVENT\t"))
				if err != nil {
					t.Fatal(err)
				}
				got.Events = append(got.Events, string(b))
			}
			if strings.HasPrefix(line, "HASHMAP_RESULT\t") {
				if completed {
					t.Fatal("duplicate completed child response")
				}
				completed = true
				var result abortResponse
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "HASHMAP_RESULT\t")), &result); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result.Events, got.Events) {
					t.Fatalf("child telemetry differs:%s", output)
				}
				got = result
			}
		}
		if runErr != nil {
			exit, ok := runErr.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 || completed {
				t.Fatalf("case%d unexpected child failure:%v:%s", cases, runErr, output)
			}
			got.Abort = true
			aborts++
		} else if !completed {
			t.Fatalf("case%d missing completed response:%s", cases, output)
		}
		want := record.Response
		want.ReferenceExit = "" // native SIGABRT metadata; Go's source adapter exits134.
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("abort case%d (%s):\ngot%+v\nwant%+v\n%s", cases, request, got, want, output)
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases || aborts != 837 {
		t.Fatalf("abort proof incomplete:%d/%d:%v", cases, aborts, err)
	}
}
