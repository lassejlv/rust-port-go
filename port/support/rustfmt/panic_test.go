// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type nativeStaticPayload string
type nativeOwnedPayload string
type nativeOpaquePayload struct{ bytes []byte }

type nativePanicRuntime struct{ expected *nativeOpaquePayload }

func (r *nativePanicRuntime) tag(value any) string {
	switch value := value.(type) {
	case nativeStaticPayload:
		return fmt.Sprintf("static_str:%x", string(value))
	case nativeOwnedPayload:
		return fmt.Sprintf("owned_string:%x", string(value))
	case *nativeOpaquePayload:
		return fmt.Sprintf("opaque:%d:%t", len(value.bytes), value == r.expected)
	case string: // The selected compiler-generated cleanup payload is a static str.
		return fmt.Sprintf("static_str:%x", value)
	default:
		return "other"
	}
}

func nativePanicInfo(payload any, kind rf.PanicPayloadKind, line, column uint32) rf.SourcePanicInfo {
	return rf.SourcePanicInfo{Payload: payload, Kind: kind, CanUnwind: true, Location: &rf.SourceLocation{File: "tools/port-rust-panic-runtime/prototype.rs", Line: line, Column: column}}
}

func nativePanicDrop(ctx rf.Context, name string, fails bool) {
	fmt.Printf("drop:%s:%t\n", name, ctx.Panicking())
	if fails {
		ctx.PanicAt(nativePanicInfo(nativeStaticPayload("drop panic"), rf.PanicStaticStr, 16, 105))
	}
}

// This is the authored source client's actual local cleanup witness. It emits
// ordinary Drop panics itself; the escaping cleanup boundary owns the runtime
// diagnostic. No expected trace/request mask supplies those hook events.
func nativePanicClose(ctx rf.Context, close func(rf.Context)) {
	payload := recover()
	if payload == nil {
		close(ctx)
		return
	}
	unwind := ctx
	unwind.Unwinding = true
	func() {
		defer func() {
			if recover() != nil {
				ctx.AbortCleanup()
			}
		}()
		close(unwind)
	}()
	ctx.ResumeUnwind(payload)
}

func (r *nativePanicRuntime) action(ctx rf.Context, action string) {
	defer nativePanicClose(ctx, func(c rf.Context) { nativePanicDrop(c, "body", action == "double") })
	fmt.Printf("call:%s:%t\n", action, ctx.Panicking())
	switch action {
	case "return":
	case "static":
		ctx.PanicAt(nativePanicInfo(nativeStaticPayload("literal panic"), rf.PanicStaticStr, 28, 12))
	case "owned":
		ctx.PanicAt(nativePanicInfo(nativeOwnedPayload("owned value"), rf.PanicOwnedString, 29, 11))
	case "opaque", "resume_opaque":
		value := &nativeOpaquePayload{[]byte{1, 2, 3}}
		r.expected = value
		if action == "opaque" {
			ctx.PanicAt(nativePanicInfo(value, rf.PanicOpaque, 30, 148))
		} else {
			ctx.ResumeUnwind(value)
		}
	case "resume_static":
		ctx.ResumeUnwind(nativeStaticPayload("resume payload"))
	case "expect", "debug_panic":
		defer nativePanicClose(ctx, func(c rf.Context) { nativePanicDrop(c, "error", false) })
		ctx.PanicFormattedAt(nativePanicInfo(nil, rf.PanicOwnedString, 31, 101), func(processing rf.Context) any {
			fmt.Printf("error_debug:%t\n", processing.Panicking())
			defer nativePanicClose(processing, func(c rf.Context) { nativePanicDrop(c, "debug_local", false) })
			if action == "debug_panic" {
				processing.PanicAt(nativePanicInfo(nativeStaticPayload("debug panic"), rf.PanicStaticStr, 20, 16))
			}
			return nativeOwnedPayload("expect failed: Error")
		})
	case "double":
		ctx.PanicAt(nativePanicInfo(nativeStaticPayload("body panic"), rf.PanicStaticStr, 33, 12))
	default:
		panic("invalid prototype action")
	}
}

func (r *nativePanicRuntime) report(ctx rf.Context, call func()) {
	defer func() {
		if payload := recover(); payload != nil {
			fmt.Printf("caught:%s:%t\n", r.tag(payload), ctx.Panicking())
			if value, ok := payload.(*nativeOpaquePayload); ok {
				fmt.Printf("payload_drop:%d:%t\n", len(value.bytes), ctx.Panicking())
			}
		}
	}()
	call()
	fmt.Printf("caught:none:%t\n", ctx.Panicking())
}

func TestNativePanicOperationChild(t *testing.T) {
	if os.Getenv("FERRUGO_PANIC_OPERATION_CHILD") != "1" {
		return
	}
	mode, action, hook := os.Getenv("FERRUGO_PANIC_MODE"), os.Getenv("FERRUGO_PANIC_ACTION"), os.Getenv("FERRUGO_PANIC_HOOK")
	r := nativePanicRuntime{}
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup}
	ops.Hook = func(info rf.SourcePanicInfo, ctx rf.Context) {
		location := ""
		if info.Location != nil {
			location = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		if strings.SplitN(r.tag(info.Payload), ":", 2)[0] != string(info.Kind) {
			panic("wrong source payload kind")
		}
		fmt.Printf("hook:%s:%t:%t:%t:%s\n", r.tag(info.Payload), ctx.Panicking(), info.CanUnwind, info.ForceNoBacktrace, location)
		defer nativePanicClose(ctx, func(c rf.Context) { nativePanicDrop(c, "hook_local", hook == "drop_panic") })
		switch hook {
		case "panic":
			ctx.PanicAt(nativePanicInfo(nativeStaticPayload("hook panic"), rf.PanicStaticStr, 51, 36))
		case "resume":
			ctx.ResumeUnwind(nativeStaticPayload("hook resume"))
		}
	}
	ctx := rf.WithPanicOperations(rf.Context{}, ops)
	if mode == "normal" {
		r.report(ctx, func() { r.action(ctx, action) })
	} else {
		r.report(ctx, func() {
			defer nativePanicClose(ctx, func(unwind rf.Context) {
				fmt.Printf("drop:outer:%t\n", unwind.Panicking())
				if mode == "inherited_caught" {
					r.report(unwind, func() { r.action(unwind, action) })
				} else {
					r.action(unwind, action)
				}
			})
			ctx.PanicAt(nativePanicInfo(nativeStaticPayload("outer panic"), rf.PanicStaticStr, 55, 114))
		})
	}
	os.Exit(0)
}

func TestNativePanicOperationOracle(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	tool := filepath.Join(root, "tools/port-rust-panic-runtime")
	metadata, err := os.ReadFile(filepath.Join(tool, "prototype-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases           int
		RawSHA256       string            `json:"raw_sha256"`
		PrototypeSHA256 string            `json:"prototype_sha256"`
		GeneratorSHA256 string            `json:"generator_sha256"`
		ToolchainSHA256 string            `json:"toolchain_selection_sha256"`
		GoBindings      map[string]string `json:"go_bindings"`
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	for name, expected := range manifest.GoBindings {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("panic primitive binding changed:%s", name)
		}
	}
	for name, expected := range map[string]string{"tools/port-rust-panic-runtime/prototype.rs": manifest.PrototypeSHA256, "tools/port-rust-panic-runtime/generate.py": manifest.GeneratorSHA256, "tools/port-rust-span-reference/toolchain.json": manifest.ToolchainSHA256} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("native panic proof source changed:%s", name)
		}
	}
	file, err := os.Open(filepath.Join(tool, "prototype.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	digest := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(z, digest))
	scan.Buffer(make([]byte, 65536), 1<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for scan.Scan() {
		var row struct {
			Request struct{ Mode, Action, Hook string }
			Want    struct {
				Abort  bool
				Output string
			}
		}
		if err = json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestNativePanicOperationChild$")
		command.Env = append(os.Environ(), "FERRUGO_PANIC_OPERATION_CHILD=1", "FERRUGO_PANIC_MODE="+row.Request.Mode, "FERRUGO_PANIC_ACTION="+row.Request.Action, "FERRUGO_PANIC_HOOK="+row.Request.Hook)
		output, err := command.Output()
		cancel()
		abort := false
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected host failure %v: %v %s", row.Request, err, output)
			}
			abort = true
		}
		if abort != row.Want.Abort || string(output) != row.Want.Output {
			t.Fatalf("%v abort=%t/%t\nactual:\n%s\nexpected:\n%s", row.Request, abort, row.Want.Abort, output, row.Want.Output)
		}
		count++
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Cases || hex.EncodeToString(digest.Sum(nil)) != manifest.RawSHA256 {
		t.Fatal("native panic proof count/checksum differs")
	}
}

func TestPanicMetadataBorrowAndResumeIdentity(t *testing.T) {
	value := []byte{1, 2, 3}
	original := rf.Context{Runtime: &rf.RuntimeMetadata{Value: value}}
	seen := 0
	payload := &struct{ Value []byte }{[]byte{4, 5}}
	ops := &rf.PanicOperations{Hook: func(info rf.SourcePanicInfo, processing rf.Context) {
		seen++
		if info.Payload != payload || info.Kind != rf.PanicOpaque || info.Location != nil || !processing.ProcessingPanic || !processing.Unwinding {
			t.Fatal("wrong ordinary hook metadata")
		}
		borrow := processing.Runtime.Value.([]byte)
		if &borrow[0] != &value[0] {
			t.Fatal("runtime metadata was replaced")
		}
	}}
	ctx := rf.WithPanicOperations(original, ops)
	copy := ctx
	if copy != ctx {
		t.Fatal("context lost comparability")
	}
	func() {
		defer func() {
			caught := recover()
			if caught != payload {
				t.Fatal("panic payload was wrapped")
			}
			func() {
				defer func() {
					if recover() != payload {
						t.Fatal("resume changed payload")
					}
				}()
				ctx.ResumeUnwind(caught)
			}()
		}()
		ctx.Panic(payload)
	}()
	if seen != 1 || original.Runtime.Panic != nil {
		t.Fatal("resume duplicated hook or changed original metadata")
	}
}

func TestPanicSourceLocationsAreExplicitProfileMetadata(t *testing.T) {
	const site = "rustc_span::source_map::with_inputs.expect_cwd"
	location := rf.SourceLocation{File: "actual-selected-build/original/compiler/rustc_span/src/source_map.rs", Line: 255, Column: 14}
	ops := &rf.PanicOperations{Locations: map[string]rf.SourceLocation{site: location}}
	ctx := rf.WithPanicOperations(rf.Context{}, ops)
	if ctx.SourcePanicLocation(site) == nil || *ctx.SourcePanicLocation(site) != location {
		t.Fatal("compiled location was inferred or normalized")
	}
	if ctx.SourcePanicLocation("missing") != nil || (rf.Context{}).SourcePanicLocation(site) != nil {
		t.Fatal("missing profile invented a source location")
	}
	borrow := ctx.SourcePanicLocation(site)
	borrow.File = "Go-only copy mutation"
	if *ctx.SourcePanicLocation(site) != location {
		t.Fatal("caller changed immutable profile metadata through location result")
	}
}
