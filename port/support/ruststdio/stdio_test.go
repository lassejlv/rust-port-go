// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststdio

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func TestCurrentContextAndBorrowedBinding(t *testing.T) {
	var seen []*rustfmt.RuntimeMetadata
	runtime := NewRuntime(func(bytes []byte, current rustfmt.Context) (int, error) {
		seen = append(seen, current.Runtime)
		return len(bytes), nil
	})
	scope := NewThreadScope()
	binding := runtime.Binding(scope)
	for _, name := range []string{"first source scope", "later source scope"} {
		current := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: name}}
		if err := binding.PrintStderr(func(writer io.Writer, received rustfmt.Context) error {
			if received != current {
				t.Fatal("captured an earlier callback context")
			}
			return rustfmt.WriteString(writer, "statement")
		}, current); err != nil {
			t.Fatal(err)
		}
		if seen[len(seen)-1] != current.Runtime {
			t.Fatal("raw IO lost current context identity")
		}
	}
}

func TestConcurrentStatements(t *testing.T) {
	verifyConcurrent(t, false)
	verifyConcurrent(t, true)
}

func TestOwnedCaptureRealCellAndCurrentDropContext(t *testing.T) {
	buffer := NewCapture()
	scope := NewThreadScope()
	drops := 0
	type arc struct {
		Buffer     *Capture
		References int
	}
	var cell *arc
	current := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "current drop scope"}}
	value := rustvec.NewOwned(arc{buffer, 1}, rustvec.ValueOps[arc]{Drop: func(actual *arc, received rustfmt.Context) {
		if actual != cell || actual.Buffer != buffer || actual.References != 1 || received.Runtime != current.Runtime || !received.Unwinding || received.ProcessingPanic {
			t.Fatal("capture owner copied cell or lost current unwind context")
		}
		actual.References--
		drops++
	}})
	cell = value.Borrow()
	owner := NewCaptureOwnerWithContext(value, buffer, rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "old constructor scope"}})
	if previous := scope.SetOwnedOutputCapture(owner, rustfmt.Context{}); previous != nil {
		t.Fatal("unexpected capture")
	}
	if owner.IsLive() || value.IsLive() {
		t.Fatal("source owner not consumed")
	}
	runtime := NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) { return len(bytes), nil })
	if !oracleCatch(func() {
		_ = runtime.PrintStderr(scope, func(w io.Writer, c rustfmt.Context) error {
			if err := rustfmt.WriteString(w, "partial"); err != nil {
				return err
			}
			c.Panic("source failure")
			return nil
		}, current)
	}) {
		t.Fatal("missing source panic")
	}
	if drops != 1 || scope.OutputCapture() != nil || !buffer.Poisoned() {
		t.Fatal("capture Arc cleanup/poison boundary differs")
	}
}

func TestPrivateReentrantLimitSourceBranch(t *testing.T) {
	// The private state is intentionally synthetic. This is not a claim to
	// construct 2^32 native public guards; source width/checked_add/expect are
	// verified from the two exact archived SDK source files.
	runtime := NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) { return len(bytes), nil })
	scope := NewThreadScope()
	runtime.owner = scope
	runtime.depth = ^uint32(0)
	hooks := 0
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) {
		hooks++
		if info.Payload != "lock count overflow in reentrant mutex" || info.Kind != rustfmt.PanicOwnedString || info.Location != nil || !info.CanUnwind || info.ForceNoBacktrace {
			t.Fatal("reentrant limit source panic differs")
		}
		if runtime.owner != scope || runtime.depth != ^uint32(0) {
			t.Fatal("overflow changed owned lock count")
		}
	}})
	if !oracleCatch(func() { runtime.LockStderr(scope, context) }) || hooks != 1 {
		t.Fatal("missing source checked overflow")
	}
}

func TestFormatterPanicKeepsGuardUntilHook(t *testing.T) {
	for _, captured := range []bool{false, true} {
		runtime := NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) { return len(bytes), nil })
		scope := NewThreadScope()
		capture := NewCapture()
		if captured {
			scope.SetOutputCapture(capture)
		}
		hooks := 0
		context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
			hooks++
			if info.Location != nil || info.Kind != rustfmt.PanicStaticStr || !info.CanUnwind || info.ForceNoBacktrace {
				t.Fatal("unexpected default source metadata")
			}
			if captured {
				if capture.Poisoned() {
					t.Fatal("capture was poisoned before the initial hook")
				}
				if capture.mu.TryLock() {
					capture.mu.Unlock()
					t.Fatal("released capture lock before panic hook")
				}
				if scope.OutputCapture() != nil {
					t.Fatal("restored capture before hook")
				}
			} else {
				runtime.mu.Lock()
				held := runtime.owner == scope && runtime.depth == 1
				runtime.mu.Unlock()
				if !held {
					t.Fatal("released stderr before formatter-error hook")
				}
				lease := runtime.LockStderr(scope, current)
				if err := lease.WriteAll([]byte("hook"), current); err != nil {
					t.Fatal(err)
				}
				lease.Close()
			}
		}})
		if !oracleCatch(func() {
			_ = runtime.PrintStderr(scope, func(io.Writer, rustfmt.Context) error { return errors.New("fmt::Error") }, context)
		}) || hooks != 1 {
			t.Fatal("missing formatter source panic")
		}
		runtime.mu.Lock()
		held := runtime.owner != nil
		runtime.mu.Unlock()
		if held {
			t.Fatal("unwind leaked stderr guard")
		}
	}
}

func TestIOFailureReturnsAfterGuardRelease(t *testing.T) {
	selected := errors.New("owned IO failure identity")
	runtime := NewRuntime(func([]byte, rustfmt.Context) (int, error) { return 0, selected })
	got := runtime.PrintStderr(NewThreadScope(), func(w io.Writer, _ rustfmt.Context) error { return rustfmt.WriteString(w, "failed") }, rustfmt.Context{})
	if got != selected {
		t.Fatal("lost actual IO error identity")
	}
	runtime.mu.Lock()
	held := runtime.owner != nil
	runtime.mu.Unlock()
	if held {
		t.Fatal("returned IO failure with held stderr guard")
	}
}

func TestExplicitRawProcessingGuard(t *testing.T) {
	if os.Getenv("FERRUGO_STDIO_RAW_PANIC_CHILD") == "1" {
		runtime := NewRuntime(func([]byte, rustfmt.Context) (int, error) { panic("raw backend convenience") })
		scope := NewThreadScope()
		lease := runtime.LockStderr(scope, rustfmt.Context{})
		defer func() { os.Stdout.WriteString("caller_cleanup\n"); lease.Close() }()
		_ = lease.WriteAll([]byte("x"), rustfmt.Context{ProcessingPanic: true})
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestExplicitRawProcessingGuard$")
	command.Env = append(os.Environ(), "FERRUGO_STDIO_RAW_PANIC_CHILD=1")
	var output strings.Builder
	command.Stdout = &output
	if err := command.Run(); err == nil || command.ProcessState.ExitCode() != 134 || output.Len() != 0 {
		t.Fatalf("processing panic ran caller guard cleanup: %v/%s", err, output.String())
	}
}

func FuzzWriteAll(f *testing.F) {
	f.Add([]byte("binary\x00β"), uint8(1))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("whole statement"), uint8(7))
	f.Fuzz(func(t *testing.T, input []byte, chunk uint8) {
		if len(input) > 4096 {
			t.Skip()
		}
		var output []byte
		size := int(chunk) + 1
		runtime := NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) {
			n := len(bytes)
			if n > size {
				n = size
			}
			output = append(output, bytes[:n]...)
			return n, nil
		})
		lease := runtime.LockStderr(NewThreadScope(), rustfmt.Context{})
		defer lease.Close()
		if err := lease.WriteAll(input, rustfmt.Context{}); err != nil {
			t.Fatal(err)
		}
		if string(output) != string(input) {
			t.Fatal("partial writes changed bytes")
		}
	})
}
