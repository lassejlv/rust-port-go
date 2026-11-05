// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped token.rs CommentKind dependency only (original blob 4ec5fe2a8bd84a9578851f6da04e8a8338f48713,
// declaration/derives at lines40–44). This does not complete token.rs.
package rustcast

import (
	"fmt"
	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustcspan "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type CommentKind struct{ tag uint8 }

func CommentKindLine() CommentKind                        { return CommentKind{} }
func CommentKindBlock() CommentKind                       { return CommentKind{tag: 1} }
func (value CommentKind) Clone() CommentKind              { return value }
func (value *CommentKind) CloneFrom(source CommentKind)   { *value = source }
func (value CommentKind) Equal(other CommentKind) bool    { return value == other }
func (value CommentKind) NotEqual(other CommentKind) bool { return !value.Equal(other) }
func (value CommentKind) WriteDebugFormatter(formatter *rustfmt.Formatter) error {
	if value.tag == 0 {
		return formatter.WriteString("Line")
	}
	if value.tag == 1 {
		return formatter.WriteString("Block")
	}
	formatter.Context.Panic("rustcast: invalid Go CommentKind variant")
	return nil
}
func (value CommentKind) RustDebug() string             { return rustfmt.String(value.WriteDebugFormatter) }
func (value CommentKind) Hash(hasher stablehash.Hasher) { stablehash.HashIsize(hasher, int(value.tag)) }
func HashCommentKindSlice(values []CommentKind, hasher stablehash.Hasher) {
	for _, value := range values {
		value.Hash(hasher)
	}
}
func (value CommentKind) StableHash(_ datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	hasher.WriteU8(value.tag)
}
func (value CommentKind) Encode(encoder rustcspan.SpanEncoder) { encoder.EmitU8(value.tag) }
func (value CommentKind) EncodeWithContext(encoder rustcspan.SpanEncoder, context rustfmt.Context) {
	rustcspan.WithSpanEncoderContext(encoder, context, func(encoder rustcspan.SpanEncoder) struct{} { value.Encode(encoder); return struct{}{} })
}

const CommentKindDecodeInvalidTagSite = "rustc_ast.token.CommentKind.decode.invalid_tag"

func decodeCommentKind(decoder rustcspan.SpanDecoder, context rustfmt.Context) CommentKind {
	tag := decoder.ReadU8()
	if tag > 1 {
		context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(CommentKindDecodeInvalidTagSite), CanUnwind: true}, func(rustfmt.Context) any {
			return fmt.Sprintf("invalid enum variant tag while decoding `CommentKind`, expected 0..2, actual %d", tag)
		})
	}
	return CommentKind{tag: tag}
}
func DecodeCommentKind(decoder rustcspan.SpanDecoder) CommentKind {
	context := rustfmt.Context{}
	if current, ok := decoder.(interface{ RuntimeContext() rustfmt.Context }); ok {
		context = current.RuntimeContext()
	}
	return decodeCommentKind(decoder, context)
}
func DecodeCommentKindWithContext(decoder rustcspan.SpanDecoder, context rustfmt.Context) CommentKind {
	return rustcspan.WithSpanDecoderContext(decoder, context, func(decoder rustcspan.SpanDecoder) CommentKind { return decodeCommentKind(decoder, context) })
}
