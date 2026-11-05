// SPDX-License-Identifier: MIT OR Apache-2.0
// Owned generic argument/default-override adaptation for SpanEncoder's
// encode_def_key. The complete value lives in one real movable state cell.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func SpanDefPathHasher(encoder SpanEncoder) DefPathHasher {
	return SpanDefPathHasherWithContext(encoder, spanCodecContext(encoder))
}
func SpanDefPathHasherWithContext(encoder SpanEncoder, ctx rustfmt.Context) DefPathHasher {
	return captureDefPathHasher(encoder, ctx)
}
func DefaultSpanDefPathHasher(encoder SpanEncoder) DefPathHasher {
	return DefaultSpanDefPathHasherWithContext(encoder, spanCodecContext(encoder))
}
func DefaultSpanDefPathHasherWithContext(_ SpanEncoder, context rustfmt.Context) DefPathHasher {
	return spanDefPathHasherDefault(context)
}
func spanDefPathHasherDefault(context rustfmt.Context) DefPathHasher {
	spanStaticPanicAt(context, spanLibLocation(1382, 9), "Deterministic encoding of HashMap<DefId, V> requires overriding `SpanEncoder::def_path_hasher`")
	return DefPathHasher{}
}

type spanEncodableOwner interface {
	move(rustfmt.Context) spanEncodableOwner
	close(rustfmt.Context)
	encode(SpanEncoder, rustfmt.Context)
	borrow() any
}
type spanEncodableCell[T any] struct {
	owned       *SpanOwned[T]
	encodeValue func(*T, SpanEncoder, rustfmt.Context)
}

func (value spanEncodableCell[T]) move(ctx rustfmt.Context) spanEncodableOwner {
	return spanEncodableCell[T]{owned: value.owned.MoveWithContext(ctx), encodeValue: value.encodeValue}
}
func (value spanEncodableCell[T]) close(ctx rustfmt.Context) { value.owned.DropWithContext(ctx) }
func (value spanEncodableCell[T]) encode(e SpanEncoder, ctx rustfmt.Context) {
	value.encodeValue(value.owned.Borrow(), e, ctx)
}
func (value spanEncodableCell[T]) borrow() any { return value.owned.Borrow() }

type SpanEncodableValue struct {
	owner        spanEncodableOwner
	RustTypeName string
}

func NewOwnedSpanEncodableValue[T any](owned *SpanOwned[T], typeName string, encode func(*T, SpanEncoder, rustfmt.Context), ctx rustfmt.Context) SpanEncodableValue {
	if encode == nil {
		ctx.Panic("rustcspan: missing Encodable dictionary")
	}
	return SpanEncodableValue{owner: spanEncodableCell[T]{owned: owned.MoveWithContext(ctx), encodeValue: encode}, RustTypeName: typeName}
}
func NewSpanEncodableValue[T any](value T, typeName string, encode func(*T, SpanEncoder, rustfmt.Context), drop func(*T, rustfmt.Context)) SpanEncodableValue {
	return NewOwnedSpanEncodableValue(NewSpanOwned(value, drop), typeName, encode, rustfmt.Context{})
}
func (value SpanEncodableValue) MoveWithContext(ctx rustfmt.Context) SpanEncodableValue {
	if value.owner == nil {
		ctx.Panic("rustcspan: missing owned Encodable argument")
	}
	return SpanEncodableValue{owner: value.owner.move(ctx), RustTypeName: value.RustTypeName}
}
func (value SpanEncodableValue) CloseWithContext(ctx rustfmt.Context) {
	if value.owner != nil {
		value.owner.close(ctx)
	}
}

// Borrow returns the actual *T cell for a correctly monomorphized dictionary.
// A callback must respect shared Rust borrowing and may not retain the pointer.
func (value SpanEncodableValue) Borrow() any { return value.owner.borrow() }
func (value SpanEncodableValue) Encode(e SpanEncoder, ctx rustfmt.Context) {
	value.owner.encode(e, ctx)
}
func SpanEncodeDefKey(e SpanEncoder, parent Option[DefIndex], data SpanEncodableValue) {
	ctx := spanCodecContext(e)
	owned := data.MoveWithContext(ctx)
	defer spanCloseOwnedCallback(ctx, owned.CloseWithContext)
	if override, ok := e.(interface {
		EncodeDefKey(Option[DefIndex], SpanEncodableValue)
	}); ok {
		override.EncodeDefKey(parent, owned)
		return
	}
	defaultSpanEncodeDefKeyBorrowed(e, parent, owned, ctx)
}
func DefaultSpanEncodeDefKey(e SpanEncoder, parent Option[DefIndex], data SpanEncodableValue) {
	ctx := spanCodecContext(e)
	owned := data.MoveWithContext(ctx)
	defer spanCloseOwnedCallback(ctx, owned.CloseWithContext)
	defaultSpanEncodeDefKeyBorrowed(e, parent, owned, ctx)
}
func defaultSpanEncodeDefKeyBorrowed(e SpanEncoder, parent Option[DefIndex], data SpanEncodableValue, ctx rustfmt.Context) {
	if p, ok := parent.Get(); ok {
		e.EmitU8(1)
		p.Encode(e)
	} else {
		e.EmitU8(0)
	}
	data.Encode(e, ctx)
}
