// SPDX-License-Identifier: MIT OR Apache-2.0
// Per-operation reborrows retain the original codec's complete dictionary and
// current ambient metadata. No serializer owns a construction-time Scope.
package rustcspan

import (
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (e *SpanEncoderRef[S]) BorrowSpanEncoderWithContext(ctx rustfmt.Context) SpanEncoder {
	return e.WithContext(ctx)
}
func (d *SpanDecoderRef[S]) BorrowSpanDecoderWithContext(ctx rustfmt.Context) SpanDecoder {
	return d.WithContext(ctx)
}
func WithSpanEncoderContext[R any](e SpanEncoder, ctx rustfmt.Context, operation func(SpanEncoder) R) R {
	if view, ok := e.(interface {
		BorrowSpanEncoderWithContext(rustfmt.Context) SpanEncoder
	}); ok {
		return operation(view.BorrowSpanEncoderWithContext(ctx))
	}
	if cell, ok := e.(rustcserialize.RuntimeContextCell); ok {
		return rustcserialize.WithRuntimeContext(cell, ctx, func(rustcserialize.RuntimeContextCell) R { return operation(e) })
	}
	ctx.Panic("rustcspan: contextual Encoder requires a complete borrowed dictionary or opaque context cell")
	var zero R
	return zero
}
func WithSpanDecoderContext[R any](d SpanDecoder, ctx rustfmt.Context, operation func(SpanDecoder) R) R {
	if view, ok := d.(interface {
		BorrowSpanDecoderWithContext(rustfmt.Context) SpanDecoder
	}); ok {
		return operation(view.BorrowSpanDecoderWithContext(ctx))
	}
	if cell, ok := d.(rustcserialize.RuntimeContextCell); ok {
		return rustcserialize.WithRuntimeContext(cell, ctx, func(rustcserialize.RuntimeContextCell) R { return operation(d) })
	}
	ctx.Panic("rustcspan: contextual Decoder requires a complete borrowed dictionary or opaque context cell")
	var zero R
	return zero
}
func (value *SourceFile) EncodeWithContext(e SpanEncoder, ctx rustfmt.Context) {
	WithSpanEncoderContext(e, ctx, func(e SpanEncoder) struct{} { value.Encode(e); return struct{}{} })
}
func DecodeSourceFileWithContext(d SpanDecoder, ctx rustfmt.Context) SourceFile {
	return WithSpanDecoderContext(d, ctx, DecodeSourceFile)
}
