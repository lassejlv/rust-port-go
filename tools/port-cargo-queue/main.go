// The development oracle compiles the unchanged pinned Rust source and compares
// its results with the Go port. Production queue code never invokes Rust.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lassejlv/rust-port-go/port/cargo/src/util"
)

const sourceHash = "f1ca01db50fd9b0534af5b6b1160b6e47de86b4770ef59191a54346ede72eb2a"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func option(value int, ok bool) string {
	if !ok {
		return "None"
	}
	return fmt.Sprintf("Some(%d)", value)
}

func items(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = strconv.Itoa(value)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func operations() []byte {
	var output bytes.Buffer
	q := util.NewQueue[int](2)
	fmt.Fprintf(&output, "empty=%s\n", option(q.Pop(util.Duration{})))
	q.Push(10)
	q.PushBounded(20)
	q.Push(30)
	fmt.Fprintf(&output, "fifo=%s\ndrain=%s\nempty-drain=%s\n", option(q.Pop(util.Duration{})), items(q.TryPopAll()), items(q.TryPopAll()))
	q = util.NewQueue[int](1)
	huge := make(chan string, 1)
	go func(q *util.Queue[int]) { huge <- option(q.Pop(util.NewDuration(^uint64(0), 999_999_999))) }(q)
	time.Sleep(20 * time.Millisecond)
	q.Push(99)
	fmt.Fprintf(&output, "huge=%s\n", <-huge)
	q = util.NewQueue[int](1)
	q.Push(1)
	done := make(chan struct{}, 1)
	go func(q *util.Queue[int]) { q.PushBounded(2); done <- struct{}{} }(q)
	blocked := false
	select {
	case <-done:
	case <-time.After(20 * time.Millisecond):
		blocked = true
	}
	fmt.Fprintf(&output, "bounded-blocked=%v\nbounded-first=%s\n", blocked, option(q.Pop(util.Duration{})))
	<-done
	fmt.Fprintf(&output, "bounded-second=%s\n", option(q.Pop(util.Duration{})))
	q = util.NewQueue[int](0)
	go func(q *util.Queue[int]) { q.PushBounded(101); done <- struct{}{} }(q)
	q.Push(7)
	fmt.Fprintf(&output, "zero-pop=%s\nzero-drain=%s\n", option(q.Pop(util.Duration{})), items(q.TryPopAll()))
	blocked = false
	select {
	case <-done:
	case <-time.After(20 * time.Millisecond):
		blocked = true
	}
	fmt.Fprintf(&output, "zero-blocked=%v\n", blocked)
	d := util.NewDuration(1, ^uint32(0))
	fmt.Fprintf(&output, "duration-normalized=%d,%d\n", d.Seconds(), d.Nanoseconds())
	d = util.NewDuration(^uint64(0), 999_999_999)
	fmt.Fprintf(&output, "duration-max=%d,%d\n", d.Seconds(), d.Nanoseconds())
	for _, bound := range []uint{0, 1, 2, 17, ^uint(0)} {
		label := strconv.FormatUint(uint64(bound), 10)
		if bound == ^uint(0) {
			label = "usize::MAX"
		}
		fmt.Fprintf(&output, "trace-bound=%s\n", label)
		q := util.NewQueue[int](bound)
		count := uint(0)
		seed := uint64(0x7310acbf)
		for step := 0; step < 20_000; step++ {
			seed = seed*6364136223846793005 + 1442695040888963407
			switch seed >> 61 {
			case 0, 1, 2, 3, 4:
				if seed&1 == 0 && count < bound {
					q.PushBounded(step)
				} else {
					q.Push(step)
				}
				count++
			case 5, 6:
				fmt.Fprintf(&output, "P%d:%s\n", step, option(q.Pop(util.Duration{})))
				if count != 0 {
					count--
				}
			case 7:
				fmt.Fprintf(&output, "D%d:%s\n", step, items(q.TryPopAll()))
				count = 0
			}
		}
		fmt.Fprintf(&output, "trace-final=%s\n", items(q.TryPopAll()))
	}
	return output.Bytes()
}

func command(timeout time.Duration, executable string, args ...string) []byte {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("%s %v: %v\n%s", executable, args, err, output))
	}
	return output
}

func executable(variable, local, fallback string) string {
	name, explicit := os.LookupEnv(variable)
	if !explicit {
		if info, err := os.Stat(local); err == nil && !info.IsDir() {
			name = local
		} else {
			name = fallback
		}
	}
	if name == "" {
		panic(variable + " is explicitly empty")
	}
	resolved, err := exec.LookPath(name)
	if err != nil {
		panic(fmt.Sprintf("%s executable %q: %v", variable, name, err))
	}
	resolved, err = filepath.Abs(resolved)
	must(err)
	return resolved
}

func main() {
	checkOnly := flag.Bool("check", false, "verify the retained evidence without rewriting it or repeating Go race/vet checks")
	flag.Parse()
	if flag.NArg() != 0 {
		panic("unexpected positional arguments")
	}
	cargoSource, explicit := os.LookupEnv("FERRUGO_CARGO_SOURCE")
	if !explicit {
		cargoSource = ".toolchain/upstream-cargo"
	} else if cargoSource == "" {
		panic("FERRUGO_CARGO_SOURCE is explicitly empty")
	}
	source, err := filepath.Abs(filepath.Join(cargoSource, "src/util/queue.rs"))
	must(err)
	data, err := os.ReadFile(source)
	must(err)
	if hash := fmt.Sprintf("%x", sha256.Sum256(data)); hash != sourceHash {
		panic("upstream queue source hash differs: " + hash)
	}
	temporary, err := os.MkdirTemp("", "ferrugo-queue-oracle-")
	must(err)
	defer os.RemoveAll(temporary)
	rustc := executable("FERRUGO_RUSTC", ".toolchain/rust/bin/rustc", "rustc")
	goTool := executable("FERRUGO_GO", ".toolchain/go/bin/go", "go")
	must(os.Setenv("CARGO_QUEUE_SOURCE", source))
	binary := filepath.Join(temporary, "queue-oracle")
	command(30*time.Second, rustc, "--edition=2021", "tools/port-cargo-queue/oracle.rs", "-o", binary)
	want := command(15*time.Second, binary)
	got := operations()
	if !bytes.Equal(got, want) {
		must(os.WriteFile(filepath.Join(temporary, "go-output.txt"), got, 0600))
		must(os.WriteFile(filepath.Join(temporary, "rust-output.txt"), want, 0600))
		for index := 0; index < min(len(got), len(want)); index++ {
			if got[index] != want[index] {
				panic(fmt.Sprintf("oracle mismatch at byte %d: Go %q; Rust %q", index, got[index:min(index+100, len(got))], want[index:min(index+100, len(want))]))
			}
		}
		panic("oracle output lengths differ")
	}
	if *checkOnly {
		data, err := os.ReadFile("tools/port-cargo-queue/validation.json")
		must(err)
		var retained struct {
			Revision string `json:"upstream_revision"`
			Source   string `json:"source_sha256"`
			Bytes    int    `json:"oracle_output_bytes"`
			Hash     string `json:"oracle_output_sha256"`
		}
		must(json.Unmarshal(data, &retained))
		hash := fmt.Sprintf("%x", sha256.Sum256(want))
		if retained.Revision != "24de4721844c7a4d839072bab6fb66d41c9bc1f4" || retained.Source != sourceHash || retained.Bytes != len(want) || retained.Hash != hash {
			panic("live oracle output does not match retained evidence")
		}
		fmt.Printf("Queue oracle check: 100000 operations and edge scenarios match Rust and retained SHA-256 %s; evidence unchanged.\n", hash)
		return
	}
	checks := []struct {
		Args   []string `json:"args"`
		Output string   `json:"output"`
	}{
		{Args: []string{"test", "-race", "./port/cargo/src/util", "-run", "TestQueue", "-count=10", "-timeout=90s"}},
		{Args: []string{"vet", "./port/cargo/src/util"}},
	}
	for index := range checks {
		checks[index].Output = string(command(120*time.Second, goTool, checks[index].Args...))
	}
	evidence := map[string]any{
		"upstream_revision": "24de4721844c7a4d839072bab6fb66d41c9bc1f4",
		"source_path":       "src/util/queue.rs", "source_sha256": sourceHash,
		"host":              runtime.GOOS + "/" + runtime.GOARCH,
		"rustc":             strings.TrimSpace(string(command(time.Second, rustc, "--version"))),
		"go":                strings.TrimSpace(string(command(time.Second, goTool, "version"))),
		"oracle_operations": 100000, "trace_bounds": []string{"0", "1", "2", "17", "usize::MAX"},
		"oracle_output_bytes": len(want), "oracle_output_sha256": fmt.Sprintf("%x", sha256.Sum256(want)),
		"oracle_status": "exact match", "checks": checks,
		"timeout_reacquisition": map[string]string{
			"rust_reference": "Direct unchanged Queue::pop operations cover queued items with a zero timeout; the original method uses std Condvar::wait_timeout_while predicate rechecking. No forced Rust mutex-reacquisition interleaving is claimed.",
			"go_test":        "TestQueueTimeoutReacquisitionChecksPredicateFirst directly exercises Go Queue.Pop with a registered waiter, timer expiry while holding its mutex, and item insertion before releasing the mutex; requires 37. This controlled race is Go-only.",
		},
		"skips": []string{},
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	must(err)
	encoded = append(encoded, '\n')
	must(os.WriteFile("tools/port-cargo-queue/validation.json", encoded, 0644))
	fmt.Printf("Queue oracle: 100000 operations and edge scenarios match (%d output bytes, SHA-256 %x); race tests x10 and vet passed.\n", len(want), sha256.Sum256(want))
}
