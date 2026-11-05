// SPDX-License-Identifier: MIT OR Apache-2.0
// Whole unchanged SourceMap constructor/expect ownership and runtime traces.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func sourceMapOwnedPanic(context rustfmt.Context, payload string, line, column uint32) {
	location := sourceMapOwnedCallbackLocation(context, payload, line, column)
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: payload, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func sourceMapOwnedReport(context rustfmt.Context, body func()) {
	caught := false
	func() {
		defer func() {
			if recover() != nil {
				caught = true
			}
		}()
		body()
	}()
	fmt.Printf("caught:%t:%t\n", caught, context.Panicking())
}
func sourceMapOwnedConstruct(context rustfmt.Context, testCase string, config SpanEncodingConfig) {
	type state struct{ tag []byte }
	var loaderCell, errorCell *state
	errorDrop := func(value *state, ctx rustfmt.Context) {
		fmt.Printf("error_drop:%t:%t\n", ctx.Panicking(), value == errorCell)
		sourceMapOwnedTrace(ctx, "error_drop")
		if strings.Contains(testCase, "error") {
			sourceMapOwnedPanic(ctx, "error Drop panic", 17, 166)
		}
	}
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(state{[]byte{1, 2}}, func(value *state, ctx rustfmt.Context) {
		fmt.Printf("loader_drop:%t:%t\n", ctx.Panicking(), value == loaderCell)
		sourceMapOwnedTrace(ctx, "loader_drop")
		if strings.Contains(testCase, "loader") {
			sourceMapOwnedPanic(ctx, "loader Drop panic", 20, 174)
		}
	}), FileLoaderOps[state]{
		FileExists:     func(*state, RustPath, rustfmt.Context) bool { return true },
		ReadFile:       func(*state, RustPath, rustfmt.Context) (string, error) { return "", nil },
		ReadBinaryFile: func(*state, RustPath, rustfmt.Context) (Shared[[]byte], error) { return NewShared([]byte{}, nil), nil },
		CurrentDirectory: func(value *state, ctx rustfmt.Context) (RustPath, error) {
			loaderCell = value
			fmt.Printf("cwd:%t\n", ctx.Panicking())
			sourceMapOwnedTrace(ctx, "cwd")
			failure := NewSourceIOError(state{[]byte{7, 8}}, SourceIOErrorOps[state]{
				Display: func(_ *state, f *rustfmt.Formatter) error { return f.WriteString("E") },
				Debug: func(value *state, f *rustfmt.Formatter) error {
					b := rustfmt.BeginStruct(f.Writer, "Custom", f.Options.Alternate)
					b.Field("kind", f.Value(f.Options.Alternate, func(f *rustfmt.Formatter) error { return f.WriteString("Other") }))
					b.Field("error", f.Value(f.Options.Alternate, func(f *rustfmt.Formatter) error {
						errorCell = value
						fmt.Printf("error_debug:%t\n", f.Context.Panicking())
						sourceMapOwnedTrace(f.Context, "error_debug")
						defer spanCloseOwnedCallback(f.Context, func(ctx rustfmt.Context) {
							fmt.Printf("debug_local_drop:%t\n", ctx.Panicking())
							sourceMapOwnedTrace(ctx, "debug_local_drop")
						})
						if strings.Contains(testCase, "debug") {
							sourceMapOwnedPanic(f.Context, "error Debug panic", 14, 248)
						}
						if testCase == "format" {
							_ = f.WriteString("partial")
							return fmt.Errorf("authored fmt::Error")
						}
						return f.WriteString("E")
					}))
					return b.Finish()
				}, Drop: errorDrop,
			})
			return RustPath{}, failure
		},
	})
	sm := NewSourceMapWithContext(context, SourceMapInputs{FileLoader: loader, PathMapping: NewFilePathMapping(nil, EmptyRemapPathScopeComponents()), HashKind: SourceFileHashMd5}, config)
	sm.CloseWithContext(context)
}
func sourceMapOwnedChild(mode, testCase, profile string, ambient ...rustfmt.Context) {
	config := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	var hook func(rustfmt.SourcePanicInfo, rustfmt.Context)
	hook = func(info rustfmt.SourcePanicInfo, ctx rustfmt.Context) {
		fmt.Printf("hook:%t:%s:%s:%d:%d\n", ctx.Panicking(), info.Kind, info.Location.File, info.Location.Line, info.Location.Column)
		fmt.Printf("payload:%v\n", info.Payload)
		sourceMapOwnedTrace(ctx, "hook")
		if mode == "processing" && info.Payload == "outer" {
			sourceMapOwnedConstruct(ctx, testCase, config)
		}
	}
	base := rustfmt.Context{}
	if len(ambient) != 0 {
		base = ambient[0]
	}
	ctx := rustfmt.WithPanicOperations(base, &rustfmt.PanicOperations{Hook: hook, CleanupAbort: &cleanup, Locations: map[string]rustfmt.SourceLocation{"rustc_span::source_map::with_inputs.expect_cwd": {File: "original/compiler/rustc_span/src/source_map.rs", Line: 255, Column: 14}}})
	switch mode {
	case "normal":
		sourceMapOwnedReport(ctx, func() { sourceMapOwnedConstruct(ctx, testCase, config) })
	case "processing":
		sourceMapOwnedReport(ctx, func() { sourceMapOwnedPanic(ctx, "outer", 25, 860) })
	case "inherited_caught", "inherited_escape":
		sourceMapOwnedReport(ctx, func() {
			defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
				fmt.Printf("outer_drop:%t\n", unwind.Panicking())
				sourceMapOwnedTrace(unwind, "outer_drop")
				if mode == "inherited_caught" {
					sourceMapOwnedReport(unwind, func() { sourceMapOwnedConstruct(unwind, testCase, config) })
				} else {
					sourceMapOwnedConstruct(unwind, testCase, config)
				}
			})
			sourceMapOwnedPanic(ctx, "outer", 25, 819)
		})
	default:
		panic("invalid owned mode")
	}
}
func TestSourceMapOwnedNativeChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_OWNED_CHILD") != "1" {
		return
	}
	sourceMapOwnedChild(os.Getenv("FERRUGO_SOURCE_MAP_OWNED_MODE"), os.Getenv("FERRUGO_SOURCE_MAP_OWNED_CASE"), os.Getenv("FERRUGO_SOURCE_MAP_OWNED_PROFILE"))
	os.Exit(0)
}
func TestSourceMapOwnedNativeOracle(t *testing.T) {
	sourceMapRunOwnedNativeOracle(t, "testdata/source_map_owned.tsv.gz")
}
func TestSourceMapOwnedFormatErrorNativeOracle(t *testing.T) {
	sourceMapRunOwnedNativeOracle(t, "testdata/source_map_owned_format_error.tsv.gz")
}
func sourceMapRunOwnedNativeOracle(t *testing.T, path string) {
	f, err := os.Open(path)
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
			t.Fatal("bad owned fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapOwnedNativeChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_OWNED_CHILD=1", "FERRUGO_SOURCE_MAP_OWNED_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_OWNED_MODE="+p[1], "FERRUGO_SOURCE_MAP_OWNED_CASE="+p[2])
		output, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected host failure %s/%s/%s: %v %s", p[0], p[1], p[2], err, output)
			}
			kind = "abort"
			aborts++
		}
		if want := sourceMapOracleString(p[4]); kind != p[3] || string(output) != want {
			t.Fatalf("%s/%s/%s got %s/%q want %s/%q", p[0], p[1], p[2], kind, output, p[3], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d whole-source owned cases, %d isolated aborts", count, aborts)
}
