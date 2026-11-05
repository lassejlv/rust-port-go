// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete owned callback boundaries consumed by the scoped opaque adapters.
// Dictionaries borrow the real environment cell and represent the specifically
// selected FnOnce (write_with) or Fn (with_position) method, independently of
// any other Fn/FnMut/FnOnce implementations on the source type.
package rustcserialize

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func opaqueCloseOwned[S any](value *rustvec.Owned[S], ctx rustfmt.Context) {
	p := recover()
	if p != nil && ctx.ProcessingPanic {
		ctx.ResumeUnwind(p)
	}
	if p != nil {
		unwind := ctx
		unwind.Unwinding = true
		func() {
			defer func() {
				if recover() != nil {
					unwind.AbortCleanup()
				}
			}()
			value.DropWithContext(unwind)
		}()
		ctx.ResumeUnwind(p)
	}
	value.DropWithContext(ctx)
}
func MemEncoderWriteWithOwned[S any](encoder *MemEncoder, n uint, environment *rustvec.Owned[S], visitor func(rustvec.Ref[S], []byte) uint, ctx rustfmt.Context) {
	owned := environment.MoveWithContext(ctx)
	WithRuntimeContext(encoder, ctx, func(e *MemEncoder) struct{} {
		defer opaqueCloseOwned(owned, ctx)
		e.WriteWith(n, func(window []byte) uint {
			defer opaqueCloseOwned(owned, ctx)
			return visitor(rustvec.Ref[S]{Value: owned.Borrow(), Context: ctx}, window)
		})
		return struct{}{}
	})
}
func FileEncoderWriteWithOwned[S any](encoder *FileEncoder, n uint, environment *rustvec.Owned[S], visitor func(rustvec.Ref[S], []byte) uint, ctx rustfmt.Context) {
	owned := environment.MoveWithContext(ctx)
	WithRuntimeContext(encoder, ctx, func(e *FileEncoder) struct{} {
		defer opaqueCloseOwned(owned, ctx)
		e.WriteWith(n, func(window []byte) uint {
			defer opaqueCloseOwned(owned, ctx)
			return visitor(rustvec.Ref[S]{Value: owned.Borrow(), Context: ctx}, window)
		})
		return struct{}{}
	})
}
func MemDecoderWithPositionOwned[R, S any](decoder *MemDecoder, position uint, environment *rustvec.Owned[S], call func(rustvec.Ref[S], *MemDecoder) R, ctx rustfmt.Context) R {
	owned := environment.MoveWithContext(ctx)
	return WithRuntimeContext(decoder, ctx, func(d *MemDecoder) R {
		defer opaqueCloseOwned(owned, ctx)
		var out R
		d.WithPosition(position, func(d *MemDecoder) { out = call(rustvec.Ref[S]{Value: owned.Borrow(), Context: ctx}, d) })
		return out
	})
}
