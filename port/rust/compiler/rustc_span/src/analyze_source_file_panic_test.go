// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func analysisPanicOuter(ctx rustfmt.Context) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &rustfmt.SourceLocation{File: "analyzer-panic.rs", Line: 9, Column: 18}, CanUnwind: true})
}
func analysisPanicRun(caseName string, config SourceFileAnalysisConfig, ctx rustfmt.Context, catch bool) {
	lines := []RelativeBytePos{0}
	multi := []MultiByteChar{}
	src, scan, offset := "α\n", uint(3), RelativeBytePos(0)
	switch caseName {
	case "assert_scan":
		src = "λ"
	case "overflow_line":
		src, scan, offset = "\n", 1, RelativeBytePos(^uint32(0))
	case "overflow_pos":
		src, offset = "aα", RelativeBytePos(^uint32(0))
	}
	run := func() uint { return analyzeSourceFileGeneric(src, scan, offset, &lines, &multi, config, ctx) }
	if !catch {
		run()
		fmt.Println("escaped_ok")
		return
	}
	failed := false
	overflow := uint(0)
	func() {
		defer func() {
			if recover() != nil {
				failed = true
			}
		}()
		overflow = run()
	}()
	a, b := []string{}, []string{}
	for _, v := range lines {
		a = append(a, fmt.Sprint(v))
	}
	for _, v := range multi {
		b = append(b, fmt.Sprintf("%d/%d", v.Pos, v.Bytes))
	}
	fmt.Printf("state:%t:%d:%s:%s\n", failed, overflow, strings.Join(a, ","), strings.Join(b, ","))
}
func analysisPanicChild(profile, mode, caseName string) {
	config := SourceFileAnalysisConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	locations := LibPanicLocationsAtPinnedRevision("original/compiler/rustc_span/src/lib.rs", "unused-scoped-tls")
	locations["rustc_span::analyze_source_file::generic.assert_scan_length"] = rustfmt.SourceLocation{File: "analyzer-original/compiler/rustc_span/src/analyze_source_file.rs", Line: 266, Column: 5}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: locations, CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
		fmt.Printf("hook:%s:%s:%d:%d:%v:%t\n", info.Kind, info.Location.File, info.Location.Line, info.Location.Column, info.Payload, processing.Panicking())
		if mode == "processing" && info.Payload == "outer" {
			analysisPanicRun(caseName, config, processing, true)
			fmt.Println("hook_return")
		}
	}})
	caught := false
	func() {
		defer func() {
			if recover() != nil {
				caught = true
			}
		}()
		switch mode {
		case "normal":
			analysisPanicRun(caseName, config, ctx, true)
		case "processing":
			analysisPanicOuter(ctx)
		case "inherited_caught", "inherited_escape":
			func() {
				defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
					fmt.Printf("outer_drop:%t\n", unwind.Panicking())
					analysisPanicRun(caseName, config, unwind, mode == "inherited_caught")
				})
				analysisPanicOuter(ctx)
			}()
		default:
			panic("invalid analyzer mode")
		}
	}()
	fmt.Printf("done:%t:false\n", caught)
}
func TestAnalyzeSourceFilePanicChild(t *testing.T) {
	if os.Getenv("FERRUGO_ANALYZE_PANIC_CHILD") != "1" {
		return
	}
	analysisPanicChild(os.Getenv("FERRUGO_ANALYZE_PANIC_PROFILE"), os.Getenv("FERRUGO_ANALYZE_PANIC_MODE"), os.Getenv("FERRUGO_ANALYZE_PANIC_CASE"))
	os.Exit(0)
}
func TestAnalyzeSourceFileNativePanicOracle(t *testing.T) {
	f, err := os.Open("testdata/analyze_source_file_panic.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(z)
	count, aborts := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 {
			t.Fatal("invalid analyzer panic fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestAnalyzeSourceFilePanicChild$")
		command.Env = append(os.Environ(), "FERRUGO_ANALYZE_PANIC_CHILD=1", "FERRUGO_ANALYZE_PANIC_PROFILE="+p[0], "FERRUGO_ANALYZE_PANIC_MODE="+p[1], "FERRUGO_ANALYZE_PANIC_CASE="+p[2])
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected host failure %v %s", err, out)
			}
			kind = "abort"
			aborts++
		}
		want := sourceMapOracleString(p[4])
		if kind != p[3] || string(out) != want {
			t.Fatalf("%s got%s/%q want%s/%q", strings.Join(p[:3], "/"), kind, out, p[3], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s native analyzer runtime scripts, %d isolated aborts", strconv.Itoa(count), aborts)
}
