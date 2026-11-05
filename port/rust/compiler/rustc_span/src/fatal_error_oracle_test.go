// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete original fatal_error observations, including isolated aborts. The
// callback runtime witness below uses scoped ordinary panic operations. The
// component owns local cleanup-abort guards; FatalError Raise/Catch has no hook.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fatalOracleRuntime struct{ localUnwind bool }

func (r *fatalOracleRuntime) raise(context rustfmt.Context, cleanup bool) {
	r.localUnwind = true
	FatalError{}.RaiseWithContext(context)
}
func (r *fatalOracleRuntime) ordinary(tag string, context rustfmt.Context, cleanup bool) {
	r.localUnwind = true
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: tag, Kind: rustfmt.PanicStaticStr, CanUnwind: true})
}
func (r *fatalOracleRuntime) drop(name, action string, context rustfmt.Context) {
	fmt.Printf("drop:%s:%t\n", name, context.Panicking())
	switch action {
	case "ordinary":
		r.ordinary("drop", context, true)
	case "fatal":
		r.raise(context, true)
	}
}
func (r *fatalOracleRuntime) invoke(testCase string, context rustfmt.Context) {
	action, envAction, bodyAction := "ok", "none", "none"
	switch testCase {
	case "ok":
	case "fatal":
		action = "fatal"
	case "ordinary":
		action = "ordinary"
	case "env_ordinary":
		envAction = "ordinary"
	case "env_fatal":
		envAction = "fatal"
	case "body_ordinary":
		bodyAction = "ordinary"
	case "fatal_env_ordinary":
		action, envAction = "fatal", "ordinary"
	case "ordinary_env_ordinary":
		action, envAction = "ordinary", "ordinary"
	case "fatal_body_ordinary":
		action, bodyAction = "fatal", "ordinary"
	default:
		panic("invalid fatal test case")
	}
	value, failure := CatchFatalErrorsOwnedWithContext(context, FatalFnOnce[struct{}]{
		Call: func(ctx rustfmt.Context) *SpanOwned[struct{}] {
			defer spanCloseOwnedCallback(ctx, func(dropContext rustfmt.Context) { r.drop("body", bodyAction, dropContext) })
			fmt.Printf("call:%t\n", ctx.Panicking())
			switch action {
			case "fatal":
				r.raise(ctx, false)
			case "ordinary":
				r.ordinary("ordinary", ctx, false)
			}
			return NewSpanOwned(struct{}{}, func(_ *struct{}, ctx rustfmt.Context) { r.drop("return", "none", ctx) })
		},
		Close: func(ctx rustfmt.Context) { r.drop("env", envAction, ctx) },
	})
	r.localUnwind = false // The source catch has finished before this caller resumes.
	if failure.IsSome() {
		fmt.Printf("result:fatal:%t\n", context.Panicking())
	} else {
		fmt.Printf("result:ok:%t\n", context.Panicking())
		value.DropWithContext(context)
	}
}
func fatalOracleTag(payload any) string {
	if _, ok := payload.(FatalErrorMarker); ok {
		return "fatal"
	}
	if v, ok := payload.(string); ok {
		return v
	}
	return "other"
}
func (r *fatalOracleRuntime) report(context rustfmt.Context, body func()) {
	defer func() {
		if payload := recover(); payload != nil {
			r.localUnwind = false
			fmt.Printf("caught:%s:%t\n", fatalOracleTag(payload), context.Panicking())
		}
	}()
	body()
	r.localUnwind = false
	fmt.Printf("caught:none:%t\n", context.Panicking())
}
func fatalOracleChild(mode, testCase string) {
	r := &fatalOracleRuntime{}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{
		CleanupAbort: &cleanup,
		Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
			tag := fatalOracleTag(info.Payload)
			if info.Payload == cleanup.Payload {
				tag = "cleanup"
			}
			fmt.Printf("hook:%s:%t\n", tag, context.Panicking())
			if mode == "processing" {
				r.invoke(testCase, context)
				fmt.Printf("hook_return:%t\n", context.Panicking())
			}
		},
	})
	switch mode {
	case "normal":
		r.report(ctx, func() { r.invoke(testCase, ctx) })
	case "processing":
		r.report(ctx, func() { r.ordinary("outer", ctx, false) })
	case "inherited_caught", "inherited_escape":
		r.report(ctx, func() {
			defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
				fmt.Println("outer_drop:true")
				if mode == "inherited_caught" {
					r.report(unwind, func() { r.invoke(testCase, unwind) })
				} else {
					r.invoke(testCase, unwind)
				}
			})
			r.ordinary("outer", ctx, false)
		})
	default:
		panic("bad fatal mode")
	}
}
func TestFatalErrorNativeChild(t *testing.T) {
	if os.Getenv("FERRUGO_FATAL_NATIVE_CHILD") != "1" {
		return
	}
	fatalOracleChild(os.Getenv("FERRUGO_FATAL_MODE"), os.Getenv("FERRUGO_FATAL_CASE"))
	os.Exit(0)
}
func TestFatalErrorNativeUnwindOracle(t *testing.T) {
	f, err := os.Open("testdata/fatal_error_unwind.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	aborts := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 {
			t.Fatal("invalid fatal fixture")
		}
		childContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(childContext, binary, "-test.run=^TestFatalErrorNativeChild$")
		command.Env = append(os.Environ(), "FERRUGO_FATAL_NATIVE_CHILD=1", "FERRUGO_FATAL_MODE="+p[0], "FERRUGO_FATAL_CASE="+p[1])
		output, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%s/%s unexpected host failure: %v, %s", p[0], p[1], err, output)
			}
			kind = "abort"
			aborts++
		}
		want := sourceMapOracleString(p[3])
		if kind != p[2] || string(output) != want {
			t.Fatalf("%s/%s got %s/%q want %s/%q", p[0], p[1], kind, output, p[2], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d original native cases, %d isolated aborts", count, aborts)
}
