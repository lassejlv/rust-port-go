//go:build linux

// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststdio"
)

func sourceMapStderrReport(ctx rustfmt.Context, body func()) {
	caught, fatal := false, false
	func() {
		defer func() {
			if value := recover(); value != nil {
				caught = true
				_, fatal = value.(FatalErrorMarker)
			}
		}()
		body()
	}()
	fmt.Printf("caught:%t:%t:%t\n", caught, fatal, ctx.Panicking())
}
func sourceMapStderrConstruct(ctx rustfmt.Context, code string, cfg SpanEncodingConfig) {
	var stderr *SourceMapStderr
	var runtime *ruststdio.Runtime
	var thread *ruststdio.ThreadScope
	if code != "closed" {
		runtime = ruststdio.NewRuntime(func(data []byte, current rustfmt.Context) (int, error) {
			if current != ctx {
				panic("stderr lost current caller runtime")
			}
			fmt.Printf("stderr_write:%d:%x\n", len(data), data)
			if code == "open" || code == "capture" || code == "held" {
				return syscall.Write(2, data)
			}
			n, _ := strconv.ParseInt(code, 10, 32)
			return -1, syscall.Errno(uintptr(uint32(int32(n))))
		})
		thread = ruststdio.NewThreadScope()
		stderr = &SourceMapStderr{Binding: runtime.Binding(thread)}
	}
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(struct{}{}, nil), FileLoaderOps[struct{}]{FileExists: func(*struct{}, RustPath, rustfmt.Context) bool { return true }, ReadFile: func(*struct{}, RustPath, rustfmt.Context) (string, error) { return "", nil }, ReadBinaryFile: func(*struct{}, RustPath, rustfmt.Context) (Shared[[]byte], error) {
		return NewShared([]byte{}, nil), nil
	}, CurrentDirectory: func(*struct{}, rustfmt.Context) (RustPath, error) { return NewRustPath("/foo"), nil }})
	sm := NewSourceMapWithContext(ctx, SourceMapInputs{FileLoader: loader, HashKind: SourceFileHashMd5, PathMapping: EmptyFilePathMapping(), Stderr: stderr}, cfg)
	defer spanCloseOwnedCallback(ctx, sm.CloseWithContext)
	sourceMapLockedImport(sm, "max", ^uint32(0), ctx)
	if code == "capture" {
		capture := ruststdio.NewCapture()
		thread.SetOutputCapture(capture)
		defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
			thread.SetOutputCapture(nil)
			fmt.Printf("capture:%x:%t\n", capture.Bytes(), unwind.Panicking())
		})
	}
	if code == "held" {
		lease := runtime.LockStderr(thread, ctx)
		defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) { fmt.Printf("held_drop:%t\n", unwind.Panicking()); lease.Close() })
	}
	sm.NewSourceFileWithContext(CustomFileName("fatal"), "a", ctx).CloseWithContext(ctx)
}
func TestSourceMapStderrChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_STDERR_CHILD") != "1" {
		return
	}
	mode, code, profile := os.Getenv("FERRUGO_SOURCE_MAP_STDERR_MODE"), os.Getenv("FERRUGO_SOURCE_MAP_STDERR_CODE"), os.Getenv("FERRUGO_SOURCE_MAP_STDERR_PROFILE")
	if code == "closed" {
		syscall.Close(2)
	}
	cfg := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rustfmt.SourceLocation{SourceMapPanicStderr: {File: "/rustc/708c94ef093ecc42fb8851e17d8383d999727abc/library/std/src/io/stdio.rs", Line: 1162, Column: 9}}, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		fmt.Printf("hook:%t:%s:%s:%d:%d\npayload:%v\n", current.Panicking(), info.Kind, info.Location.File, info.Location.Line, info.Location.Column, info.Payload)
		if mode == "processing" && info.Payload == "outer" {
			sourceMapStderrConstruct(current, code, cfg)
		}
	}})
	outer := func() {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: &rustfmt.SourceLocation{File: "stderr.rs", Line: 9, Column: 18}})
	}
	switch mode {
	case "normal":
		sourceMapStderrReport(ctx, func() { sourceMapStderrConstruct(ctx, code, cfg) })
	case "processing":
		sourceMapStderrReport(ctx, outer)
	case "inherited_caught", "inherited_escape":
		sourceMapStderrReport(ctx, func() {
			defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
				fmt.Printf("outer_drop:%t\n", unwind.Panicking())
				if mode == "inherited_caught" {
					sourceMapStderrReport(unwind, func() { sourceMapStderrConstruct(unwind, code, cfg) })
				} else {
					sourceMapStderrConstruct(unwind, code, cfg)
				}
			})
			outer()
		})
	}
	os.Exit(0)
}
func TestSourceMapStderrNativeOracle(t *testing.T) {
	if !rusterrno.NativeAvailable() {
		t.Skip("native host error-display proof requires cgo")
	}
	file, err := os.Open("testdata/source_map_stderr.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count, aborts := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("invalid stderr fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapStderrChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_STDERR_CHILD=1", "FERRUGO_SOURCE_MAP_STDERR_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_STDERR_MODE="+p[1], "FERRUGO_SOURCE_MAP_STDERR_CODE="+p[2])
		var stderr bytes.Buffer
		command.Stderr = &stderr
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected stderr host failure %v %v %s %s", p[:3], err, out, stderr.String())
			}
			kind = "abort"
			aborts++
		}
		if want := sourceMapOracleString(p[4]); kind != p[3] || string(out) != want || kind == "ok" && stderr.String() != sourceMapOracleString(p[5]) {
			t.Fatalf("%v got%s/%q stderr%q want%s/%q stderr%q", p[:3], kind, out, stderr.String(), p[3], want, sourceMapOracleString(p[5]))
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d actual SDK/whole-SourceMap stderr scripts, %d isolated aborts", count, aborts)
}
