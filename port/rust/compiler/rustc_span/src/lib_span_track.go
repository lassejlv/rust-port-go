// SPDX-License-Identifier: MIT OR Apache-2.0
// SPAN_TRACK's static fn-pointer contract from pinned lib.rs. Explicit current
// contexts adapt access to Rust's ambient panic/tracing state; no owned closure
// capture or Go-local defer timing is claimed by this descriptor.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"sync/atomic"
)

type SpanTrackHook struct {
	call   func(LocalDefId, rustfmt.Context)
	legacy func(LocalDefId)
}

func NewSpanTrackHook(call func(LocalDefId, rustfmt.Context)) *SpanTrackHook {
	if call == nil {
		panic("SPAN_TRACK: nil function is not a Rust fn pointer")
	}
	return &SpanTrackHook{call: call}
}

var defaultSpanTrackHook = NewSpanTrackHook(func(LocalDefId, rustfmt.Context) {})
var spanTrack atomic.Pointer[SpanTrackHook]

func LoadSpanTrackHook() *SpanTrackHook {
	if hook := spanTrack.Load(); hook != nil {
		return hook
	}
	return defaultSpanTrackHook
}
func SwapSpanTrackHook(hook *SpanTrackHook) *SpanTrackHook {
	if hook == nil || hook.call == nil {
		panic("SPAN_TRACK: nil function is not a Rust fn pointer")
	}
	if old := spanTrack.Swap(hook); old != nil {
		return old
	}
	return defaultSpanTrackHook
}
func (hook *SpanTrackHook) Call(parent LocalDefId, context rustfmt.Context) {
	defer spanDebugContextGuard(context)
	hook.call(parent, context)
}

// SwapSpanTrack is the original trivial-context convenience. Save/restore a
// primary contextual registration with SwapSpanTrackHook, preserving descriptor
// identity and the next operation's current runtime context.
func SwapSpanTrack(callback func(LocalDefId)) func(LocalDefId) {
	if callback == nil {
		panic("SPAN_TRACK: nil function is not a Rust fn pointer")
	}
	hook := NewSpanTrackHook(func(parent LocalDefId, _ rustfmt.Context) { callback(parent) })
	hook.legacy = callback
	previous := SwapSpanTrackHook(hook)
	if previous.legacy != nil {
		return previous.legacy
	}
	return func(parent LocalDefId) { previous.Call(parent, rustfmt.Context{}) }
}
func spanFullRuntimeContext(context spanPanicContext) rustfmt.Context {
	switch c := context.(type) {
	case rustfmt.Context:
		return c
	case *rustfmt.Context:
		return *c
	case interface{ RuntimeContext() rustfmt.Context }:
		return c.RuntimeContext()
	default:
		return rustfmt.Context{}
	}
}
func trackSpanParentWithPanicContext(parent LocalDefId, context spanPanicContext) {
	LoadSpanTrackHook().Call(parent, spanFullRuntimeContext(context))
}
