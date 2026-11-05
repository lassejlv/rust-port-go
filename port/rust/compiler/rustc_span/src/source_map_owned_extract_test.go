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
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

const sourceMapExtractText = " aλz"

type sourceMapExtractSetup struct {
	sm             *SourceMap
	session        *SessionGlobals
	local, foreign Shared[SourceFile]
	reads          atomic.Uint64
}

func newSourceMapExtractSetup(config SpanEncodingConfig) *sourceMapExtractSetup {
	setup := &sourceMapExtractSetup{session: NewSpanEncodingSession(config)}
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(struct{}{}, nil), FileLoaderOps[struct{}]{FileExists: func(*struct{}, RustPath, rustfmt.Context) bool { return true }, ReadFile: func(*struct{}, RustPath, rustfmt.Context) (string, error) {
		setup.reads.Add(1)
		return sourceMapExtractText, nil
	}, ReadBinaryFile: func(*struct{}, RustPath, rustfmt.Context) (Shared[[]byte], error) {
		return NewShared([]byte(sourceMapExtractText), nil), nil
	}, CurrentDirectory: func(*struct{}, rustfmt.Context) (RustPath, error) { return NewRustPath("/proof"), nil }})
	setup.sm = NewSourceMapWithInputsAndConfig(SourceMapInputs{FileLoader: loader, PathMapping: EmptyFilePathMapping(), HashKind: SourceFileHashMd5}, config)
	setup.local = setup.sm.NewSourceFile(CustomFileName("local.rs"), sourceMapExtractText)
	template, failure := NewSourceFile(RealFileNameFileName(RealFileNameFromVirtualPath(NewRustPath("foreign.rs"))), sourceMapExtractText, SourceFileHashMd5, None[SourceFileHashAlgorithm]())
	if failure != nil {
		panic(failure)
	}
	if src, ok := template.Src.Get(); ok {
		src.Close()
	}
	setup.foreign = setup.sm.NewImportedSourceFile(template.Name, template.SrcHash, template.ChecksumHash, template.StableId, template.NormalizedSourceLen.ToU32(), template.UnnormalizedSourceLen, CrateNumFromU32(1), template.Lines, template.MultibyteChars, template.NormalizedPos, 0)
	return setup
}
func sourceMapExtractPanic(ctx rustfmt.Context, tag string) {
	line, column := uint32(12), uint32(17)
	switch tag {
	case "env":
		line, column = 13, 16
	case "outer":
		line, column = 14, 18
	}
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: tag, Kind: rustfmt.PanicStaticStr, Location: &rustfmt.SourceLocation{File: "owned_extract.rs", Line: line, Column: column}, CanUnwind: true})
}
func (setup *sourceMapExtractSetup) invoke(input, action string, ctx rustfmt.Context) {
	f := setup.foreign.Borrow().StartPos
	lo, hi, file := BytePos(0), BytePos(2), setup.local
	switch input {
	case "external":
		lo, hi, file = f, f+2, setup.foreign
	case "bad":
		lo, hi, file = f+99, f+100, setup.foreign
	case "distinct":
		lo, hi = 0, f+1
	}
	type env struct {
		file  Shared[SourceFile]
		calls uint
		tag   []byte
	}
	var address uintptr
	state := NewSpanOwned(env{file: file, tag: []byte{1, 2}}, func(value *env, context rustfmt.Context) {
		writable := false
		if guard, ok := value.file.Borrow().ExternalSrc.TryWriteWithContext(context); ok {
			writable = true
			guard.Close()
		}
		at := uintptr(unsafe.Pointer(value))
		fmt.Printf("env_drop:%d:%t:%d:%t:%t\n", value.calls, address == 0 || address == at, value.file.StrongCount(), writable, context.Panicking())
		if action == "env" || action == "both" {
			sourceMapExtractPanic(context, "env")
		}
	})
	result, failure := SpanToSourceOwned(setup.sm, setup.session, WithRootCtxt(setup.session, lo, hi, setup.sm.config), state, func(value *env, src string, start, end uint, context rustfmt.Context) (*SpanOwned[[]byte], *SpanSnippetError) {
		address = uintptr(unsafe.Pointer(value))
		value.calls++
		source := ""
		if text, ok := value.file.Borrow().Src.Get(); ok {
			source = text.String()
		} else {
			guard := value.file.Borrow().ExternalSrc.ReadWithContext(context)
			source, _ = guard.Get().GetSource().Get()
			guard.Close()
		}
		borrowed := unsafe.StringData(src) == unsafe.StringData(source)
		fmt.Printf("call:%d:%d:%t:%d:%d:%t\n", start, end, borrowed, value.file.StrongCount(), setup.reads.Load(), context.Panicking())
		if action == "body" || action == "both" {
			sourceMapExtractPanic(context, "body")
		}
		return NewSpanOwned([]byte{7, 8}, func(_ *[]byte, context rustfmt.Context) { fmt.Printf("value_drop:%t\n", context.Panicking()) }), nil
	}, ctx)
	if failure == nil {
		fmt.Printf("result:ok:%t\n", ctx.Panicking())
		result.DropWithContext(ctx)
	} else {
		tag := "other"
		switch failure.kind {
		case snippetDistinct:
			tag = "distinct"
		case snippetMalformed:
			tag = "malformed"
		}
		fmt.Printf("result:%s:%t\n", tag, ctx.Panicking())
	}
}
func (setup *sourceMapExtractSetup) invokePredicate(input, action, operation string, ctx rustfmt.Context) {
	f := setup.foreign.Borrow().StartPos
	lo, hi, file := BytePos(0), BytePos(2), setup.local
	switch input {
	case "external":
		lo, hi, file = f, f+2, setup.foreign
	case "bad":
		lo, hi, file = f+99, f+100, setup.foreign
	case "distinct":
		lo, hi = 0, f+1
	}
	if operation == "prev" && (input == "local" || input == "external") {
		lo += 2
		hi += 2
	}
	type env struct {
		file  Shared[SourceFile]
		calls uint
		tag   []byte
	}
	var address uintptr
	owned := NewSpanOwned(env{file: file, tag: []byte{1, 2}}, func(value *env, context rustfmt.Context) {
		writable := false
		if guard, ok := value.file.Borrow().ExternalSrc.TryWriteWithContext(context); ok {
			writable = true
			guard.Close()
		}
		at := uintptr(unsafe.Pointer(value))
		fmt.Printf("env_drop:%d:%t:%d:%t:%t\n", value.calls, address == 0 || address == at, value.file.StrongCount(), writable, context.Panicking())
		if action == "env" || action == "both" {
			sourceMapExtractPanic(context, "env")
		}
	})
	predicate := func(value *env, char rune, context rustfmt.Context) bool {
		address = uintptr(unsafe.Pointer(value))
		value.calls++
		fmt.Printf("pred:%x:%d:%d:%t\n", char, value.file.StrongCount(), setup.reads.Load(), context.Panicking())
		if action == "body" || action == "both" {
			sourceMapExtractPanic(context, "body")
		}
		return true
	}
	span := WithRootCtxt(setup.session, lo, hi, setup.sm.config)
	var out Span
	var failure *SpanSnippetError
	switch operation {
	case "while":
		out, failure = SpanExtendWhileOwned(setup.sm, setup.session, span, owned, predicate, ctx)
	case "prev":
		out, failure = SpanExtendPrevWhileOwned(setup.sm, setup.session, span, owned, predicate, ctx)
	case "take":
		out = SpanTakeWhileOwned(setup.sm, setup.session, span, owned, func(value *env, char *rune, ctx rustfmt.Context) bool { return predicate(value, *char, ctx) }, ctx)
	default:
		panic("invalid predicate operation")
	}
	if failure == nil {
		fmt.Printf("result:span:%d:%d:%t\n", out.Lo(setup.session), out.Hi(setup.session), ctx.Panicking())
	} else {
		tag := "other"
		switch failure.kind {
		case snippetDistinct:
			tag = "distinct"
		case snippetMalformed:
			tag = "malformed"
		}
		fmt.Printf("result:%s:%t\n", tag, ctx.Panicking())
	}
}
func (setup *sourceMapExtractSetup) invokeSelected(input, action string, ctx rustfmt.Context) {
	op := os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_OPERATION")
	if op == "" || op == "extract" {
		setup.invoke(input, action, ctx)
	} else {
		setup.invokePredicate(input, action, op, ctx)
	}
}
func sourceMapExtractReport(ctx rustfmt.Context, body func()) {
	tag := "none"
	func() {
		defer func() {
			if p := recover(); p != nil {
				if value, ok := p.(string); ok {
					tag = value
				} else {
					tag = "other"
				}
			}
		}()
		body()
	}()
	fmt.Printf("caught:%s:%t\n", tag, ctx.Panicking())
}
func sourceMapExtractChild(profile, mode, input, action string) {
	config := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	setup := newSourceMapExtractSetup(config)
	defer func() { setup.local.Close(); setup.foreign.Close(); setup.sm.Close(); setup.session.Close() }()
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
		fmt.Printf("hook:%s:%s:%d:%d:%v\n", info.Kind, info.Location.File, info.Location.Line, info.Location.Column, info.Payload)
		if mode == "processing" && info.Payload == "outer" {
			setup.invokeSelected(input, action, context)
			fmt.Printf("hook_return:%t\n", context.Panicking())
		}
	}})
	switch mode {
	case "normal":
		sourceMapExtractReport(ctx, func() { setup.invokeSelected(input, action, ctx) })
	case "processing":
		sourceMapExtractReport(ctx, func() { sourceMapExtractPanic(ctx, "outer") })
	case "inherited_caught", "inherited_escape":
		sourceMapExtractReport(ctx, func() {
			defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
				fmt.Printf("outer_drop:%t\n", unwind.Panicking())
				if mode == "inherited_caught" {
					sourceMapExtractReport(unwind, func() { setup.invokeSelected(input, action, unwind) })
				} else {
					setup.invokeSelected(input, action, unwind)
				}
			})
			sourceMapExtractPanic(ctx, "outer")
		})
	default:
		panic("invalid owned extraction mode")
	}
}
func TestSourceMapOwnedExtractionChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_CHILD") != "1" {
		return
	}
	sourceMapExtractChild(os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_PROFILE"), os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_MODE"), os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_INPUT"), os.Getenv("FERRUGO_SOURCE_MAP_EXTRACT_ACTION"))
	os.Exit(0)
}
func TestSourceMapOwnedExtractionNativeOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_owned_extract.tsv.gz")
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
		if len(p) != 6 {
			t.Fatal("invalid extraction fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapOwnedExtractionChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_EXTRACT_CHILD=1", "FERRUGO_SOURCE_MAP_EXTRACT_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_EXTRACT_OPERATION=extract", "FERRUGO_SOURCE_MAP_EXTRACT_MODE="+p[1], "FERRUGO_SOURCE_MAP_EXTRACT_INPUT="+p[2], "FERRUGO_SOURCE_MAP_EXTRACT_ACTION="+p[3])
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected host failure%s %v %s", strings.Join(p[:4], "/"), err, out)
			}
			kind = "abort"
			aborts++
		}
		want := sourceMapOracleString(p[5])
		if kind != p[4] || string(out) != want {
			t.Fatalf("%s got%s/%q want%s/%q", strings.Join(p[:4], "/"), kind, out, p[4], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s whole-source generic extraction scripts, %d isolated aborts", strconv.Itoa(count), aborts)
}

func TestSourceMapOwnedPredicatesNativeOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_owned_predicates.tsv.gz")
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
		if len(p) != 7 {
			t.Fatal("invalid owned predicate fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapOwnedExtractionChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_EXTRACT_CHILD=1", "FERRUGO_SOURCE_MAP_EXTRACT_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_EXTRACT_OPERATION="+p[1], "FERRUGO_SOURCE_MAP_EXTRACT_MODE="+p[2], "FERRUGO_SOURCE_MAP_EXTRACT_INPUT="+p[3], "FERRUGO_SOURCE_MAP_EXTRACT_ACTION="+p[4])
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected predicate host failure %s: %v %s", strings.Join(p[:5], "/"), err, out)
			}
			kind = "abort"
			aborts++
		}
		want := sourceMapOracleString(p[6])
		if kind != p[5] || string(out) != want {
			t.Fatalf("%s got%s/%q want%s/%q", strings.Join(p[:5], "/"), kind, out, p[5], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d whole-source owned predicate scripts, %d isolated aborts", count, aborts)
}
