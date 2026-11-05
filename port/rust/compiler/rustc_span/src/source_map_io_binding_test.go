//go:build !windows

// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSourceMapRealLoaderExplicitUnixBindingIsLazyAndCurrent(t *testing.T) {
	prepared := &rustfmt.RuntimeMetadata{Value: "constructor"}
	current := &rustfmt.RuntimeMetadata{Value: "format caller"}
	calls := 0
	binding := &rusterrno.Binding{Call: func(_ int32, buffer *[128]byte, ctx rustfmt.Context) int32 {
		calls++
		if ctx.Runtime != current || !ctx.Unwinding {
			t.Fatal("error formatting captured construction-time runtime")
		}
		copy(buffer[:], "actual supplied FFI result")
		return 0
	}}
	loader := RealFileLoader{UnixErrorBinding: binding}
	_, err := loader.ReadFileWithContext(rustfmt.Context{Runtime: prepared}, NewRustPath(t.TempDir()+"/missing"))
	if err == nil || calls != 0 {
		t.Fatal("native error must defer strerror_r until formatting")
	}
	var out strings.Builder
	f := &rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions(), Context: rustfmt.Context{Runtime: current, Unwinding: true}}
	if failure := FormatSourceIOErrorDebug(err, f); failure != nil {
		t.Fatal(failure)
	}
	if calls != 1 || !strings.Contains(out.String(), "actual supplied FFI result") {
		t.Fatalf("explicit binding: %d/%q", calls, out.String())
	}
	CloseSourceIOError(err, f.Context)
}
