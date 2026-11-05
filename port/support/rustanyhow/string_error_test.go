// SPDX-License-Identifier: MIT OR Apache-2.0
package rustanyhow

import (
	"errors"
	"strings"
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestStringErrorOwnedCellAndPayloadType(t *testing.T) {
	e := NewStringError(strings.Repeat("owned", 20))
	if !e.IsString() || e.IsStaticString() || e.Source() != nil || errors.Unwrap(e) != nil {
		t.Fatal("String payload/source differs")
	}
	cell, trace := e.DowncastOwnedStringRef(), e.Backtrace()
	copyDescriptor := *e
	moved := e.Move()
	if moved.DowncastOwnedStringRef() != cell || moved.Backtrace() != trace {
		t.Fatal("moving error replaced actual cells")
	}
	cell.PushString(" appended")
	if moved.Error() != cell.AsStr() {
		t.Fatal("downcast_mut did not reach real error")
	}
	buffer := unsafe.StringData(cell.AsStr())
	owned := moved.DowncastOwnedString()
	if text := owned.Borrow().AsStr(); text != strings.Repeat("owned", 20)+" appended" || unsafe.StringData(text) != buffer {
		t.Fatal("String downcast failed to move buffer")
	}
	owned.Close()
	for _, old := range []*StringError{e, &copyDescriptor, moved} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("consumed owner remained usable")
				}
			}()
			_ = old.Error()
		}()
	}
}
func TestStringErrorCurrentCaptureAndBacktraceFormatting(t *testing.T) {
	for _, value := range []any{2, nil} {
		context := rustfmt.Context{}
		if value != nil {
			context.Runtime = &rustfmt.RuntimeMetadata{Value: value}
		}
		calls := 0
		ops := BacktraceOperations{Capture: func(current rustfmt.Context) Backtrace {
			calls++
			var observed any
			if current.Runtime != nil {
				observed = current.Runtime.Value
			}
			if observed != value {
				t.Fatal("capture saw stale caller", observed, value)
			}
			return NewBacktraceRecord(BacktraceCaptured, "stack backtrace:\n    actual selected runtime record\n\t\u3000")
		}}
		e := NewStringErrorWithContext("message", ops, context)
		if calls != 1 || e.RustDebug() != "message\n\nStack backtrace:\n    actual selected runtime record" {
			t.Fatal(calls, e.RustDebug())
		}
		var out strings.Builder
		f := rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions()}
		f.Options.Alternate = true
		if err := e.WriteDebugFormatter(&f); err != nil || out.String() != `"message"` {
			t.Fatal(out.String(), err)
		}
		e.CloseWithContext(context)
	}
	if e := NewStringError("message"); e.Backtrace().Status() != BacktraceDisabled || e.Backtrace().DisplayText() != "disabled backtrace" || e.RustDebug() != "message" {
		t.Fatal("disabled profile differs")
	}
}
