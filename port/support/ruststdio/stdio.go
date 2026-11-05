// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped stdio contracts from the selected SDKs; no std source-file credit.
package ruststdio

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Statement is an ephemeral borrowed fmt::Arguments formatter. It receives the
// current operation's context and must obey io.Writer's no-retention/no-mutation
// input borrow. It has no implicit owning closure destructor. Returning an error
// without a failed writer call represents fmt::Error, not an IO failure.
// Formatter chunks have Rust str's valid UTF-8 domain; arbitrary raw bytes use
// StderrLease.WriteAll instead. This precondition adds no validation scan.
type Statement func(io.Writer, rustfmt.Context) error

// Binding borrows a complete ambient _eprint write_fmt operation, including
// source-thread capture and reentrant locking. It returns IO errors only after
// releasing the statement's guard. The caller implements print_to's subsequent
// IO-error panic. Every invocation receives its CURRENT context.
type Binding struct {
	PrintStderr func(Statement, rustfmt.Context) error
}

// Write is the selected raw stderr operation. An error denotes Err and its n
// is ignored, as for Rust Result<usize>. Backing state is borrowed, not owned.
type Write func([]byte, rustfmt.Context) (n int, err error)

// Runtime is one process's reentrant stderr lock and raw backend. All translated
// users of that process's stderr must share this runtime. Its zero value is not
// initialized; construct it with NewRuntime or use DefaultRuntime.
type Runtime struct {
	mu    sync.Mutex
	wake  *sync.Cond
	owner *ThreadScope
	depth uint32
	write Write
}

func NewRuntime(write Write) *Runtime {
	if write == nil {
		panic("ruststdio: missing raw stderr operation")
	}
	r := &Runtime{write: write}
	r.wake = sync.NewCond(&r.mu)
	return r
}

var defaultRuntime = NewRuntime(nativeWrite)

// DefaultRuntime is the common host backend, not a per-component lock.
func DefaultRuntime() *Runtime { return defaultRuntime }

// ThreadScope is an explicit source-thread identity and its output-capture
// slot. Exclusively borrow it on one logical source thread at a time. Separate
// concurrent source threads use distinct scopes (possibly sharing Capture).
// Identity is never inferred from Go goroutines, OS threads, or Context.Runtime.
type ThreadScope struct{ capture *CaptureOwner }

func NewThreadScope() *ThreadScope { return &ThreadScope{} }

// SetOutputCapture moves the slot's capture identity and returns its previous
// identity. Capture objects are shared borrowed handles; this scoped adapter
// does not expose Arc allocation counts or the surrounding TLS destructor API.
// This convenience must not be mixed with a nontrivial CaptureOwner slot; use
// SetOwnedOutputCapture when the source Arc destructor/counts are observable.
func (s *ThreadScope) SetOutputCapture(capture *Capture) *Capture {
	previous := s.capture
	if previous != nil && previous.resource != nil {
		panic("ruststdio: owned capture requires its owning setter")
	}
	s.capture = borrowedCapture(capture)
	if previous != nil {
		previous.live = false
		return previous.capture
	}
	return nil
}

func (s *ThreadScope) OutputCapture() *Capture {
	if s.capture != nil {
		return s.capture.capture
	}
	return nil
}

// Capture is the real mutex-protected byte buffer used for captured statements.
// Its poisoned state follows MutexGuard: an escaping panic poisons it only if
// the guard was acquired while the thread was not already panicking.
type Capture struct {
	mu       sync.Mutex
	data     []byte
	poisoned atomic.Bool
}

func NewCapture() *Capture { return &Capture{data: []byte{}} }

func NewCaptureWithCapacity(capacity uint) *Capture {
	return NewCaptureWithCapacityWithContext(capacity, rustfmt.Context{})
}
func NewCaptureWithCapacityWithContext(capacity uint, context rustfmt.Context) *Capture {
	if capacity > uint(^uint(0)>>1) {
		captureCapacityOverflow(context)
	}
	return &Capture{data: make([]byte, 0, int(capacity))}
}

const CaptureCapacityPanicSite = "rust.alloc.vec.capture.capacity_overflow"

func captureCapacityOverflow(context rustfmt.Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(CaptureCapacityPanicSite), CanUnwind: true})
}

type CaptureSnapshot struct {
	Bytes            []byte
	Length, Capacity uint
	Poisoned         bool
}

func (c *Capture) Capacity() uint { c.mu.Lock(); defer c.mu.Unlock(); return uint(cap(c.data)) }
func (c *Capture) TrySnapshot() (CaptureSnapshot, bool) {
	if !c.mu.TryLock() {
		return CaptureSnapshot{}, false
	}
	defer c.mu.Unlock()
	return CaptureSnapshot{Bytes: append([]byte(nil), c.data...), Length: uint(len(c.data)), Capacity: uint(cap(c.data)), Poisoned: c.poisoned.Load()}, true
}

func (c *Capture) reserve(additional int, context rustfmt.Context) {
	maximum := int(^uint(0) >> 1)
	if additional > maximum-len(c.data) {
		captureCapacityOverflow(context)
	}
	required := len(c.data) + additional
	if required <= cap(c.data) {
		return
	}
	capacity := uint(cap(c.data)) * 2
	if capacity < uint(required) {
		capacity = uint(required)
	}
	if capacity < 8 {
		capacity = 8
	}
	if capacity > uint(maximum) {
		captureCapacityOverflow(context)
	}
	next := make([]byte, len(c.data), int(capacity))
	copy(next, c.data)
	c.data = next
}

func (c *Capture) Bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.data...)
}
func (c *Capture) Poisoned() bool {
	return c.poisoned.Load()
}

// StderrLease is one source ReentrantLockGuard. Close it exactly once; moving
// the Go pointer preserves the guard, copying the pointed-to value is invalid.
// A lease can outlive other leases for the same scope, as Rust guards can.
type StderrLease struct {
	runtime *Runtime
	scope   *ThreadScope
	live    bool
}

func (r *Runtime) LockStderr(scope *ThreadScope, context rustfmt.Context) *StderrLease {
	if scope == nil {
		context.Panic("ruststdio: a reentrant stderr lock requires its source-thread scope")
	}
	r.mu.Lock()
	for r.owner != nil && r.owner != scope {
		r.wake.Wait()
	}
	if r.owner == scope && r.depth == ^uint32(0) {
		r.mu.Unlock()
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: "lock count overflow in reentrant mutex", Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(ReentrantCountPanicSite), CanUnwind: true})
	}
	r.owner = scope
	r.depth++
	r.mu.Unlock()
	return &StderrLease{runtime: r, scope: scope, live: true}
}

func (l *StderrLease) Close() {
	if l == nil || !l.live {
		panic("ruststdio: consumed stderr guard")
	}
	l.live = false
	r := l.runtime
	r.mu.Lock()
	r.depth--
	if r.depth == 0 {
		r.owner = nil
		r.wake.Broadcast()
	}
	r.mu.Unlock()
}

func (r *Runtime) Binding(scope *ThreadScope) *Binding {
	return &Binding{PrintStderr: func(statement Statement, context rustfmt.Context) error {
		return r.PrintStderr(scope, statement, context)
	}}
}

// PrintStderr is the common uncaptured, not-already-held host convenience.
// Retain an explicit ThreadScope/Binding for capture or a pre-held stderr lease.
func PrintStderr(statement Statement, context rustfmt.Context) error {
	return defaultRuntime.PrintStderr(NewThreadScope(), statement, context)
}

func (r *Runtime) PrintStderr(scope *ThreadScope, statement Statement, context rustfmt.Context) error {
	if scope == nil {
		scope = NewThreadScope()
	}
	if owner := scope.SetOwnedOutputCapture(nil, context); owner != nil {
		defer closeCaptureDuringReturn(owner, context)
		// This restore is intentionally not deferred. A formatter panic leaves
		// the slot removed, while normal return restores the original identity.
		owner.capture.format(statement, context)
		if previous := scope.SetOwnedOutputCapture(owner, context); previous != nil {
			previous.CloseWithContext(context)
		}
		return nil
	}
	lease := r.LockStderr(scope, context)
	defer func() {
		if payload := recover(); payload != nil {
			// A raw Go callback is only a convenience. In processing mode its
			// escaping panic must abort before source guard cleanup; callback-
			// local Go defers have already followed Go's rules.
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			lease.Close()
			context.ResumeUnwind(payload)
		}
		lease.Close()
	}()
	return lease.WriteFmt(statement, context)
}

func (l *StderrLease) WriteFmt(statement Statement, context rustfmt.Context) (err error) {
	defer forwardPanic(context)
	if l == nil || !l.live {
		context.Panic("ruststdio: consumed stderr guard")
	}
	return format(statement, &rawWriter{runtime: l.runtime, context: context}, context)
}

// WriteAll exposes the actual held StderrLock's unbuffered write_all operation.
func (l *StderrLease) WriteAll(bytes []byte, context rustfmt.Context) (err error) {
	defer forwardPanic(context)
	if l == nil || !l.live {
		context.Panic("ruststdio: consumed stderr guard")
	}
	return (&rawWriter{runtime: l.runtime, context: context}).all(bytes)
}

func forwardPanic(context rustfmt.Context) {
	if payload := recover(); payload != nil {
		context.ResumeUnwind(payload)
	}
}

const FormatterErrorPanicSite = "rust.std.io.write_fmt.formatter_error"
const ReentrantCountPanicSite = "rust.std.sync.reentrant_lock.lock_count_overflow"

var ErrWriteZero = errors.New("failed to write whole buffer")

type rawWriter struct {
	runtime *Runtime
	context rustfmt.Context
}

func (w *rawWriter) all(bytes []byte) error {
	for len(bytes) != 0 {
		n, err := w.runtime.write(bytes, w.context)
		if err != nil {
			if nativeBadFD(err) {
				return nil
			}
			if nativeInterrupted(err) {
				continue
			}
			return err
		}
		if n < 0 || n > len(bytes) {
			w.context.Panic("ruststdio: raw write returned an invalid length")
		}
		if n == 0 {
			return ErrWriteZero
		}
		bytes = bytes[n:]
	}
	return nil
}
func (w *rawWriter) Write(bytes []byte) (int, error) {
	if err := w.all(bytes); err != nil {
		return 0, err
	}
	return len(bytes), nil
}

type formatWriter struct {
	inner io.Writer
	error error
}

func (w *formatWriter) Write(bytes []byte) (int, error) {
	n, err := w.inner.Write(bytes)
	if err != nil {
		w.error = err
	}
	return n, err
}
func (w *formatWriter) WriteString(text string) (int, error) {
	// str.as_bytes is an immutable borrow, not an owned byte allocation.
	return w.Write(unsafe.Slice(unsafe.StringData(text), len(text)))
}
func (w *formatWriter) WriteChar(char rune) error {
	if !utf8.ValidRune(char) {
		panic("ruststdio: invalid Rust char")
	}
	var bytes [utf8.UTFMax]byte
	n := utf8.EncodeRune(bytes[:], char)
	_, err := w.Write(bytes[:n])
	return err
}

func format(statement Statement, writer io.Writer, context rustfmt.Context) error {
	adapter := &formatWriter{inner: writer}
	if err := statement(adapter, context); err != nil {
		if adapter.error != nil {
			return adapter.error
		}
		context.PanicAt(rustfmt.SourcePanicInfo{
			Payload: "a formatting trait implementation returned an error when the underlying stream did not",
			Kind:    rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(FormatterErrorPanicSite), CanUnwind: true,
		})
	}
	return nil
}

type captureWriter struct {
	capture *Capture
	context rustfmt.Context
}

func (w captureWriter) Write(bytes []byte) (int, error) {
	w.capture.reserve(len(bytes), w.context)
	w.capture.data = append(w.capture.data, bytes...)
	return len(bytes), nil
}

func (c *Capture) format(statement Statement, context rustfmt.Context) {
	c.mu.Lock()
	alreadyPanicking := context.Panicking()
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			if !alreadyPanicking {
				c.poisoned.Store(true)
			}
			c.mu.Unlock()
			context.ResumeUnwind(payload)
		}
		c.mu.Unlock()
	}()
	_ = format(statement, captureWriter{c, context}, context)
}
