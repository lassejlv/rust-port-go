// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustvec"

// Once is an owned FnOnce witness, including the entire capture environment and
// its field destruction. Call consumes it; Close destroys an uncalled closure.
// A result prepared before a throwing capture destructor remains leaked, just
// as an initialized Rust return place does when parameter cleanup throws.
type Once[A, R any] interface {
	Call(A, Context) R
	Close(Context)
}
type onceCell[S, A, R any] struct {
	state *rustvec.Owned[S]
	call  func(*S, A, Context) R
	live  bool
}

func NewOnce[S, A, R any](state *rustvec.Owned[S], call func(*S, A, Context) R) Once[A, R] {
	if state == nil || !state.IsLive() || call == nil {
		panic("rusthashmap: invalid FnOnce witness")
	}
	return &onceCell[S, A, R]{state: state.Move(), call: call, live: true}
}
func (f *onceCell[S, A, R]) Call(arg A, c Context) R {
	if !f.live {
		c.Panic("rusthashmap: call of consumed FnOnce")
	}
	f.live = false
	defer cleanupState(f.state, c)
	return f.call(f.state.Borrow(), arg, c)
}
func (f *onceCell[S, A, R]) Close(c Context) {
	if !f.live {
		return
	}
	f.live = false
	f.state.DropWithContext(c)
}
func cleanupState[S any](state *rustvec.Owned[S], c Context) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		cleanupOnPanic(unwind, func() { state.DropWithContext(unwind) })
		c.ResumeUnwind(p)
	}
	state.DropWithContext(c)
}

// PlainOnce is only for a capture whose source Drop is trivial. Destructible
// Rust captures belong in NewOnce's actual owned state cell instead.
func PlainOnce[A, R any](call func(A, Context) R) Once[A, R] {
	return NewOnce(rustvec.Own(struct{}{}), func(_ *struct{}, arg A, c Context) R { return call(arg, c) })
}
