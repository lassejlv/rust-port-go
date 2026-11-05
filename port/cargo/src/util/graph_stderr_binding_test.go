// SPDX-License-Identifier: MIT OR Apache-2.0
package util

import (
	"fmt"
	"syscall"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestGraphStderrUsesCurrentLibcBinding(t *testing.T) {
	for _, inherited := range []bool{false, true} {
		for _, located := range []bool{false, true} {
			t.Run(fmt.Sprintf("unwind=%t/location=%t", inherited, located), func(t *testing.T) {
				location := rustfmt.SourceLocation{File: "selected/stdio.rs", Line: 42, Column: 9}
				marker := &rustfmt.RuntimeMetadata{Value: "current caller"}
				ctx := rustfmt.Context{Runtime: marker, Unwinding: inherited}
				seen, calls := 0, 0
				operations := &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, hook rustfmt.Context) {
					seen++
					if info.Payload != "failed printing to stderr: selected libc message (os error 999)" || info.Kind != rustfmt.PanicOwnedString || !info.CanUnwind || info.ForceNoBacktrace || !hook.ProcessingPanic || !hook.Unwinding || hook.Runtime.Value != "current caller" {
						t.Fatalf("wrong printing panic: %+v", info)
					}
					if located {
						if info.Location == nil || *info.Location != location {
							t.Fatalf("lost declared source location: %+v", info.Location)
						}
					} else if info.Location != nil {
						t.Fatalf("invented source location: %+v", info.Location)
					}
				}}
				if located {
					operations.Locations = map[string]rustfmt.SourceLocation{"rust.std.stdio.stderr.print_error": location}
				}
				ctx = rustfmt.WithPanicOperations(ctx, operations)
				g := &OwnedGraph[int, int]{config: OwnedGraphConfig{UnixErrorBinding: &rusterrno.Binding{Call: func(code int32, buffer *[128]byte, current rustfmt.Context) int32 {
					calls++
					if code != 999 || *buffer != [128]byte{} || !current.ProcessingPanic || !current.Unwinding || current.Runtime != ctx.Runtime {
						t.Fatal("errno formatting did not receive current panic-processing context")
					}
					copy(buffer[:], "selected libc message")
					return 0
				}}}}
				defer func() {
					if recover() != "failed printing to stderr: selected libc message (os error 999)" || seen != 1 || calls != 1 {
						t.Fatal("printing must dispatch the exact panic once after libc formatting")
					}
				}()
				g.stderrPanic(syscall.Errno(999), ctx)
			})
		}
	}
}
