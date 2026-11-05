// SPDX-License-Identifier: MIT OR Apache-2.0
// Full current-context dictionary for the source's four required Pos methods.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// PosMethods is static method code plus borrowed implementation metadata. It
// is not an owning Fn environment. From methods construct an owned P; To
// methods borrow the actual P cell, including arbitrary non-Copy providers.
type PosMethods[P any] struct {
	FromUsize func(uint, rustfmt.Context) P
	ToUsize   func(*P, rustfmt.Context) uint
	FromU32   func(uint32, rustfmt.Context) P
	ToU32     func(*P, rustfmt.Context) uint32
}

type PosOperations[P any] struct{ methods PosMethods[P] }

func NewPosOperations[P any](methods PosMethods[P]) PosOperations[P] {
	if methods.FromUsize == nil || methods.ToUsize == nil || methods.FromU32 == nil || methods.ToU32 == nil {
		panic("rustcspan: incomplete Pos dictionary")
	}
	return PosOperations[P]{methods}
}
func (ops PosOperations[P]) FromUsize(value uint, ctx rustfmt.Context) (out P) {
	spanRunContext(ctx, func() { out = ops.methods.FromUsize(value, ctx) })
	return
}
func (ops PosOperations[P]) ToUsize(value *P, ctx rustfmt.Context) (out uint) {
	spanRunContext(ctx, func() { out = ops.methods.ToUsize(value, ctx) })
	return
}
func (ops PosOperations[P]) FromU32(value uint32, ctx rustfmt.Context) (out P) {
	spanRunContext(ctx, func() { out = ops.methods.FromU32(value, ctx) })
	return
}
func (ops PosOperations[P]) ToU32(value *P, ctx rustfmt.Context) (out uint32) {
	spanRunContext(ctx, func() { out = ops.methods.ToU32(value, ctx) })
	return
}

// Bind is one ephemeral current-operation view for the legacy Pos surface.
// Discard it after that operation; reusable dictionaries retain no Runtime.
func (ops PosOperations[P]) Bind(ctx rustfmt.Context) Pos[P] {
	return Pos[P]{FromUsize: func(v uint) P { return ops.FromUsize(v, ctx) }, ToUsize: func(v *P) uint { return ops.ToUsize(v, ctx) }, FromU32: func(v uint32) P { return ops.FromU32(v, ctx) }, ToU32: func(v *P) uint32 { return ops.ToU32(v, ctx) }}
}
func BytePosOperations() PosOperations[BytePos] {
	return NewPosOperations(PosMethods[BytePos]{FromUsize: func(v uint, _ rustfmt.Context) BytePos { return BytePosFromUsize(v) }, ToUsize: func(v *BytePos, _ rustfmt.Context) uint { return v.ToUsize() }, FromU32: func(v uint32, _ rustfmt.Context) BytePos { return BytePosFromU32(v) }, ToU32: func(v *BytePos, _ rustfmt.Context) uint32 { return v.ToU32() }})
}
func RelativeBytePosOperations() PosOperations[RelativeBytePos] {
	return NewPosOperations(PosMethods[RelativeBytePos]{FromUsize: func(v uint, _ rustfmt.Context) RelativeBytePos { return RelativeBytePosFromUsize(v) }, ToUsize: func(v *RelativeBytePos, _ rustfmt.Context) uint { return v.ToUsize() }, FromU32: func(v uint32, _ rustfmt.Context) RelativeBytePos { return RelativeBytePosFromU32(v) }, ToU32: func(v *RelativeBytePos, _ rustfmt.Context) uint32 { return v.ToU32() }})
}
func CharPosOperations() PosOperations[CharPos] {
	return NewPosOperations(PosMethods[CharPos]{FromUsize: func(v uint, _ rustfmt.Context) CharPos { return CharPosFromUsize(v) }, ToUsize: func(v *CharPos, _ rustfmt.Context) uint { return v.ToUsize() }, FromU32: func(v uint32, _ rustfmt.Context) CharPos { return CharPosFromU32(v) }, ToU32: func(v *CharPos, _ rustfmt.Context) uint32 { return v.ToU32() }})
}
