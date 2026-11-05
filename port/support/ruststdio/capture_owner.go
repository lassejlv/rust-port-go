// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststdio

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// CaptureOwner owns the actual source Arc value supplied by an embedding.
// Capture is its borrowed Arc dereference. The resource must describe exactly
// that Arc's complete Drop (including reference counts), not an extra closure
// environment. Moves preserve its real rustvec.Owned state cell. The borrowed
// *Capture convenience creates a token with a trivial destructor instead.
type CaptureOwner struct {
	capture  *Capture
	resource captureResource
	live     bool
}

type captureResource interface {
	move(rustfmt.Context) captureResource
	close(rustfmt.Context)
}

type captureResourceCell[S any] struct{ value *rustvec.Owned[S] }

func (c captureResourceCell[S]) move(context rustfmt.Context) captureResource {
	return captureResourceCell[S]{c.value.MoveWithContext(context)}
}
func (c captureResourceCell[S]) close(context rustfmt.Context) { c.value.DropWithContext(context) }

func NewCaptureOwnerWithContext[S any](value *rustvec.Owned[S], capture *Capture, context rustfmt.Context) *CaptureOwner {
	if capture == nil {
		context.Panic("ruststdio: owned capture has no buffer")
	}
	return &CaptureOwner{capture: capture, resource: captureResourceCell[S]{value.MoveWithContext(context)}, live: true}
}

func borrowedCapture(capture *Capture) *CaptureOwner {
	if capture == nil {
		return nil
	}
	return &CaptureOwner{capture: capture, live: true}
}

func (o *CaptureOwner) IsLive() bool { return o != nil && o.live }
func (o *CaptureOwner) BorrowCaptureWithContext(context rustfmt.Context) *Capture {
	if !o.IsLive() {
		context.Panic("ruststdio: consumed capture owner")
	}
	return o.capture
}
func (o *CaptureOwner) MoveWithContext(context rustfmt.Context) *CaptureOwner {
	if !o.IsLive() {
		context.Panic("ruststdio: consumed capture owner")
	}
	resource := o.resource
	if resource != nil {
		resource = resource.move(context)
	}
	o.live = false
	o.resource = nil
	return &CaptureOwner{capture: o.capture, resource: resource, live: true}
}
func (o *CaptureOwner) CloseWithContext(context rustfmt.Context) {
	if !o.IsLive() {
		return
	}
	o.live = false
	resource := o.resource
	o.resource = nil
	if resource != nil {
		resource.close(context)
	}
}

// SetOwnedOutputCapture moves a source Arc into the slot and returns the
// previous owned Arc. Neither its removal nor restoration clones that Arc.
// The returned owner must be moved or closed explicitly by the caller.
func (s *ThreadScope) SetOwnedOutputCapture(owner *CaptureOwner, context rustfmt.Context) *CaptureOwner {
	if owner != nil {
		owner = owner.MoveWithContext(context)
	}
	previous := s.capture
	s.capture = owner
	if previous != nil {
		return previous.MoveWithContext(context)
	}
	return nil
}

// CloseWithContext destroys the scope's remaining owned capture slot. It is
// the explicit lifetime boundary for this scoped token, not a whole TLS port.
func (s *ThreadScope) CloseWithContext(context rustfmt.Context) {
	if owner := s.SetOwnedOutputCapture(nil, context); owner != nil {
		owner.CloseWithContext(context)
	}
}

func closeCaptureDuringReturn(owner *CaptureOwner, context rustfmt.Context) {
	payload := recover()
	if payload != nil && context.ProcessingPanic {
		context.ResumeUnwind(payload)
	}
	if owner.IsLive() {
		if payload != nil {
			unwind := context
			unwind.Unwinding = true
			func() {
				defer func() {
					if recover() != nil {
						unwind.AbortCleanup()
					}
				}()
				owner.CloseWithContext(unwind)
			}()
		} else {
			owner.CloseWithContext(context)
		}
	}
	if payload != nil {
		context.ResumeUnwind(payload)
	}
}
