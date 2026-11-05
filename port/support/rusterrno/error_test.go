// SPDX-License-Identifier: MIT OR Apache-2.0
package rusterrno

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestBindingBufferContextAndPositiveResult(t *testing.T) {
	marker := &rustfmt.RuntimeMetadata{Value: "current caller"}
	context := rustfmt.Context{Unwinding: true, Runtime: marker}
	var calls []string
	binding := &Binding{Call: func(code int32, buffer *[128]byte, got rustfmt.Context) int32 {
		if code != -42 || got != context || *buffer != [128]byte{} {
			t.Fatal("FFI input/context/zero buffer")
		}
		copy(buffer[:], []byte{'a', 0xe1, 0x80, 'b'})
		return 34 // SDK checks <0, not !=0: this buffer must still be used.
	}}
	sink := writerFunc(func(value string) error { calls = append(calls, value); return nil })
	options := rustfmt.DefaultOptions()
	options.HasWidth = true
	options.Width = 42
	options.HasPrecision = true
	options.Precision = 1
	options.Fill = '*'
	f := &rustfmt.Formatter{Writer: sink, Context: context, Options: options}
	if err := FormatError(-42, f, binding); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"a", "\ufffd", "b"}) || f.Options != options || f.Context != context {
		t.Fatalf("chunks/options: %q", calls)
	}
}

type writerFunc func(string) error

func (fn writerFunc) Write(p []byte) (int, error) { return fn.WriteString(string(p)) }
func (fn writerFunc) WriteString(s string) (int, error) {
	if err := fn(s); err != nil {
		return 0, err
	}
	return len(s), nil
}

func TestBindingFailureUsesCurrentProfileAndOrdinaryHook(t *testing.T) {
	location := rustfmt.SourceLocation{File: "selected/sdk/unix.rs", Line: 238, Column: 13}
	seen := 0
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{PanicStrerrorRFailure: location}, Hook: func(info rustfmt.SourcePanicInfo, got rustfmt.Context) {
		seen++
		if info.Payload != "strerror_r failure" || info.Kind != rustfmt.PanicStaticStr || info.Location == nil || *info.Location != location || !got.ProcessingPanic || !got.Unwinding {
			t.Fatal("wrong ordinary panic contract")
		}
	}})
	defer func() {
		if recover() != "strerror_r failure" || seen != 1 {
			t.Fatal("failure must dispatch one hook and unwind exact static payload")
		}
	}()
	FormatError(1, &rustfmt.Formatter{Writer: &strings.Builder{}, Context: context}, &Binding{Call: func(int32, *[128]byte, rustfmt.Context) int32 { return -1 }})
}

func TestFormatterFailureStopsChunks(t *testing.T) {
	calls := 0
	failure := errors.New("sink failure")
	f := &rustfmt.Formatter{Writer: writerFunc(func(string) error { calls++; return failure })}
	if err := FormatCStrBytes([]byte{0xff, 0xff}, f); err != failure || calls != 1 {
		t.Fatal("writer result/short circuit")
	}
}

func TestNoNativeBindingIsExplicit(t *testing.T) {
	if NativeAvailable() {
		t.Skip("this profile has a real host FFI")
	}
	defer func() {
		if _, ok := recover().(MissingBinding); !ok {
			t.Fatal("must not infer errno text without an FFI binding")
		}
	}()
	FormatError(1, &rustfmt.Formatter{Writer: &strings.Builder{}}, nil)
}
