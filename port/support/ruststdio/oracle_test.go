// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststdio

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type oracleRequest struct {
	SDK, Mode, Ambient, Script string
	Capacity                   uint
}
type oracleRecord struct {
	Request        oracleRequest
	Abort          bool
	Stdout, Stderr string
	Concurrency    *struct {
		Records, ChunksPerRecord int
		Whole                    bool
	}
}
type oracleProfile struct {
	Locations map[string]map[string]rustfmt.SourceLocation
}

func TestRetainedStdioProvenance(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	data, err := os.ReadFile(filepath.Join(root, "tools", "port-rust-stdio", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		FixtureSHA256 string            `json:"fixture_sha256"`
		Inputs        map[string]string `json:"inputs_sha256"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for name, want := range manifest.Inputs {
		if filepath.IsAbs(name) || strings.HasPrefix(filepath.Clean(name), "..") {
			t.Fatal("untrusted provenance input path")
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != want {
			t.Fatalf("stdio retained input differs: %s", name)
		}
	}
	fixture, err := os.ReadFile("testdata/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(fixture)) != manifest.FixtureSHA256 {
		t.Fatal("stdio native fixture digest differs")
	}
}

func readOracleProfile(t *testing.T) oracleProfile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "tools", "port-rust-stdio", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var profile oracleProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatal(err)
	}
	return profile
}

func oracleCatch(call func()) (caught bool) {
	defer func() {
		if recover() != nil {
			caught = true
		}
	}()
	call()
	return false
}

func oracleRun(request oracleRequest, locations map[string]rustfmt.SourceLocation, output io.Writer) (stderr string) {
	log := func(text string) { fmt.Fprintln(output, text) }
	var errorsOut strings.Builder
	actions := strings.Split(request.Script, ",")
	position := 0
	runtime := NewRuntime(func(bytes []byte, context rustfmt.Context) (int, error) {
		log("write:" + hex.EncodeToString(bytes))
		action := ""
		if position < len(actions) {
			action = actions[position]
		}
		position++
		if strings.HasPrefix(action, "e") {
			number, _ := strconv.Atoi(action[1:])
			return 0, syscall.Errno(number)
		}
		if action == "z" {
			return 0, nil
		}
		n := len(bytes)
		if strings.HasPrefix(action, "n") {
			selected, _ := strconv.Atoi(action[1:])
			if selected < n {
				n = selected
			}
		}
		errorsOut.Write(bytes[:n])
		return n, nil
	})
	scope := NewThreadScope()
	capture, alternate := NewCaptureWithCapacity(request.Capacity), NewCapture()
	captureCount, alternateCount := 1, 1
	cloneCapture := func(buffer *Capture, count *int, current rustfmt.Context) *CaptureOwner {
		*count++
		value := rustvec.NewOwned(count, rustvec.ValueOps[*int]{Drop: func(value **int, _ rustfmt.Context) { **value-- }})
		return NewCaptureOwnerWithContext(value, buffer, current)
	}
	counts := func(stage string) { log(fmt.Sprintf("counts:%s:%d:%d", stage, captureCount, alternateCount)) }
	snapshots := func(stage string) {
		for _, item := range []struct {
			name    string
			capture *Capture
		}{{"capture", capture}, {"alternate", alternate}} {
			state := "locked"
			if snapshot, ok := item.capture.TrySnapshot(); ok {
				state = fmt.Sprintf("%d:%d", snapshot.Length, snapshot.Capacity)
			}
			log(fmt.Sprintf("snapshot:%s:%s:%s", stage, item.name, state))
		}
	}
	var run func(rustfmt.Context)
	operations := &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{FormatterErrorPanicSite: locations["format"]}}
	operations.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		location := info.Location
		if location == nil {
			panic("missing declared oracle location")
		}
		log(fmt.Sprintf("hook:%s:%s:%s:%d:%d:%t:%t", info.Kind, hex.EncodeToString([]byte(info.Payload.(string))), hex.EncodeToString([]byte(location.File)), location.Line, location.Column, info.CanUnwind, info.ForceNoBacktrace))
		log(fmt.Sprintf("hook_capture_poison:%t", capture.Poisoned()))
		counts("hook")
		snapshots("hook")
		if request.Mode == "release" && strings.HasPrefix(info.Payload.(string), "failed printing") {
			done := make(chan struct{})
			go func() {
				lease := runtime.LockStderr(NewThreadScope(), current)
				defer lease.Close()
				if err := lease.WriteAll([]byte("THREAD\n"), current); err != nil {
					panic(err)
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				panic("IO failure retained stderr guard through panic hook")
			}
			log("release_thread_complete")
		}
		if info.Payload == "outer" && request.Ambient == "processing" {
			run(current)
		}
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, operations)
	sourceRaise := func(payload string, kind rustfmt.PanicPayloadKind, site string, current rustfmt.Context) {
		location := locations[site]
		current.PanicAt(rustfmt.SourcePanicInfo{Payload: payload, Kind: kind, Location: &location, CanUnwind: true})
	}
	print := func(statement Statement, current rustfmt.Context) {
		if failure := runtime.PrintStderr(scope, statement, current); failure != nil {
			message := "failed to write whole buffer"
			if errors.Is(failure, syscall.EIO) {
				message = "Input/output error (os error 5)"
			} else if failure != ErrWriteZero {
				panic(failure)
			}
			sourceRaise("failed printing to stderr: "+message, rustfmt.PanicOwnedString, "io", current)
		}
	}
	body := func(writer io.Writer, current rustfmt.Context) error {
		log(fmt.Sprintf("body:%t", current.Panicking()))
		counts("body")
		snapshots("body")
		if err := rustfmt.WriteString(writer, "left"); err != nil {
			return err
		}
		if strings.Contains(request.Mode, "nested") {
			print(func(w io.Writer, _ rustfmt.Context) error { return rustfmt.WriteString(w, "[N]") }, current)
		}
		if strings.Contains(request.Mode, "replace") {
			if old := scope.SetOwnedOutputCapture(cloneCapture(alternate, &alternateCount, current), current); old != nil {
				old.CloseWithContext(current)
			}
		}
		if strings.Contains(request.Mode, "panic") || request.Mode == "capture_poison_reuse" {
			sourceRaise("body-panic", rustfmt.PanicStaticStr, "body", current)
		}
		if strings.Contains(request.Mode, "fmt_error") {
			return errors.New("fmt::Error")
		}
		if err := rustfmt.WriteString(writer, ""); err != nil {
			return err
		}
		if err := rustfmt.WriteChar(writer, 'β'); err != nil {
			return err
		}
		if request.Mode == "ignore_error" {
			_ = rustfmt.WriteString(writer, "ignored")
			return nil
		}
		return rustfmt.WriteString(writer, "right")
	}
	statement := func(writer io.Writer, current rustfmt.Context) error {
		if request.Mode == "capture_fatal" {
			for _, part := range []string{"fatal error: rustc does not support text files larger than ", "4294967294", " bytes\n"} {
				if err := rustfmt.WriteString(writer, part); err != nil {
					return err
				}
			}
			return nil
		}
		if err := rustfmt.WriteString(writer, "A"); err != nil {
			return err
		}
		if err := body(writer, current); err != nil {
			return err
		}
		return rustfmt.WriteString(writer, "Z")
	}
	run = func(current rustfmt.Context) {
		counts("before")
		snapshots("before")
		if strings.Contains(request.Mode, "capture") {
			if old := scope.SetOwnedOutputCapture(cloneCapture(capture, &captureCount, current), current); old != nil {
				old.CloseWithContext(current)
			}
		}
		counts("set")
		caught := oracleCatch(func() {
			defer func() {
				if payload := recover(); payload != nil {
					if current.ProcessingPanic {
						current.ResumeUnwind(payload)
					}
					log("scope_drop:true")
					counts("scope_drop")
					current.ResumeUnwind(payload)
				}
				log(fmt.Sprintf("scope_drop:%t", current.Panicking()))
				counts("scope_drop")
			}()
			if strings.Contains(request.Mode, "preheld") {
				lease := runtime.LockStderr(scope, current)
				defer lease.Close()
				print(statement, current)
			} else {
				print(statement, current)
			}
		})
		log(fmt.Sprintf("caught:%t", caught))
		counts("caught")
		snapshots("caught")
		previousOwner := scope.SetOwnedOutputCapture(nil, current)
		var previous *Capture
		if previousOwner != nil {
			previous = previousOwner.BorrowCaptureWithContext(current)
		}
		name := "none"
		if previous == capture {
			name = "original"
		} else if previous == alternate {
			name = "alternate"
		}
		log("slot:" + name)
		log(fmt.Sprintf("capture_poison:%t", capture.Poisoned()))
		log("capture:" + hex.EncodeToString(capture.Bytes()))
		log(fmt.Sprintf("alternate_poison:%t", alternate.Poisoned()))
		log("alternate:" + hex.EncodeToString(alternate.Bytes()))
		if request.Mode == "capture_poison_reuse" {
			if old := scope.SetOwnedOutputCapture(cloneCapture(capture, &captureCount, current), current); old != nil {
				old.CloseWithContext(current)
			}
			print(func(w io.Writer, _ rustfmt.Context) error { return rustfmt.WriteString(w, "reuse") }, current)
			if old := scope.SetOwnedOutputCapture(nil, current); old != nil {
				old.CloseWithContext(current)
			}
			log("reuse:" + hex.EncodeToString(capture.Bytes()))
		}
		if previousOwner != nil {
			previousOwner.CloseWithContext(current)
		}
		counts("after_slot_drop")
		snapshots("after_slot_drop")
	}
	switch request.Ambient {
	case "normal":
		run(context)
	case "unwinding":
		caught := oracleCatch(func() {
			defer func() {
				if payload := recover(); payload != nil {
					unwind := context
					unwind.Unwinding = true
					run(unwind)
					context.ResumeUnwind(payload)
				}
			}()
			sourceRaise("outer", rustfmt.PanicStaticStr, "outer", context)
		})
		log(fmt.Sprintf("outer_caught:%t", caught))
	case "processing":
		log(fmt.Sprintf("outer_caught:%t", oracleCatch(func() { sourceRaise("outer", rustfmt.PanicStaticStr, "outer", context) })))
	default:
		panic("invalid oracle ambient")
	}
	return hex.EncodeToString([]byte(errorsOut.String()))
}

func TestOriginalStdioChild(t *testing.T) {
	if value := os.Getenv("FERRUGO_STDIO_CHILD"); value != "" {
		var request oracleRequest
		if err := json.Unmarshal([]byte(value), &request); err != nil {
			t.Fatal(err)
		}
		oracleRun(request, readOracleProfile(t).Locations[request.SDK], os.Stdout)
		os.Exit(0)
	}
}

func TestOriginalStdio(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("retained original stdio syscall/error-injection profile is Linux-only")
	}
	profile := readOracleProfile(t)
	file, err := os.Open("testdata/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 16*1024*1024+1))
	if err != nil || len(data) > 16*1024*1024 {
		t.Fatal("invalid bounded stdio fixture", err)
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(nil, 2*1024*1024)
	count, aborts := 0, 0
	for scan.Scan() {
		var record oracleRecord
		if err := json.Unmarshal(scan.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		if record.Concurrency != nil {
			verifyConcurrent(t, record.Request.Mode == "capture_concurrent")
			count++
			continue
		}
		if record.Abort {
			request, _ := json.Marshal(record.Request)
			command := exec.Command(os.Args[0], "-test.run=^TestOriginalStdioChild$")
			command.Env = append(os.Environ(), "FERRUGO_STDIO_CHILD="+string(request))
			var stdout, stderr strings.Builder
			command.Stdout = &stdout
			command.Stderr = &stderr
			if err := command.Run(); err == nil || command.ProcessState.ExitCode() != 134 {
				t.Fatalf("%+v: expected exit134, got %v: %s", record.Request, err, stderr.String())
			}
			if stdout.String() != record.Stdout {
				t.Fatalf("%+v abort events differ\ngot %s\nwant%s", record.Request, stdout.String(), record.Stdout)
			}
			aborts++
		} else {
			var output strings.Builder
			stderr := oracleRun(record.Request, profile.Locations[record.Request.SDK], &output)
			if output.String() != record.Stdout || stderr != record.Stderr {
				t.Fatalf("%+v differs\ngot %s\nwant%s\nstderr%s /%s", record.Request, output.String(), record.Stdout, stderr, record.Stderr)
			}
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 178 || aborts != 20 {
		t.Fatalf("stdio corpus count %d/%d", count, aborts)
	}
}

func verifyConcurrent(t *testing.T, captured bool) {
	t.Helper()
	var output strings.Builder
	runtime := NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) { return output.Write(bytes) })
	capture := NewCapture()
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			scope := NewThreadScope()
			if captured {
				scope.SetOutputCapture(capture)
			}
			for j := 0; j < 8; j++ {
				if err := runtime.PrintStderr(scope, func(w io.Writer, _ rustfmt.Context) error {
					if err := rustfmt.WriteString(w, fmt.Sprintf("BEGIN%d:", i)); err != nil {
						return err
					}
					for n := 0; n < 512; n++ {
						if err := rustfmt.WriteString(w, strconv.Itoa(i)); err != nil {
							return err
						}
					}
					return rustfmt.WriteString(w, fmt.Sprintf("END%d\n", i))
				}, rustfmt.Context{}); err != nil {
					panic(err)
				}
			}
		}(i)
	}
	group.Wait()
	if captured {
		if output.Len() != 0 {
			t.Fatal("captured concurrent output escaped to stderr")
		}
		output.Write(capture.Bytes())
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 64 {
		t.Fatal("lost stderr statement")
	}
	for _, line := range lines {
		if len(line) != 523 {
			t.Fatalf("stderr statement length %d", len(line))
		}
		digit := string(line[5])
		if line != "BEGIN"+digit+":"+strings.Repeat(digit, 512)+"END"+digit {
			t.Fatal("stderr statements interleaved")
		}
	}
}
