// SPDX-License-Identifier: MIT OR Apache-2.0
// Generic Spanned and marker blocks from pinned rustc_span/lib.rs. Dictionaries
// borrow actual node cells and represent the selected source trait impls.
package rustcspan

import (
	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Respan/DummySpanned move node into the returned aggregate. Go descriptor
// assignment is a lawful source Copy only when the selected T is itself Copy.
type Spanned[T any] struct {
	Node T
	Span Span
}

func Respan[T any](span Span, node T) Spanned[T] { return Spanned[T]{Node: node, Span: span} }
func DummySpanned[T any](node T) Spanned[T]      { return Respan(DummySpan(), node) }

type SpannedOps[T any] struct {
	Clone      func(*T, rustfmt.Context) T
	Drop       func(*T, rustfmt.Context)
	Equal      func(*T, *T, rustfmt.Context) bool
	Debug      func(*T, *rustfmt.Formatter) error
	Hash       func(*T, stablehash.Hasher, rustfmt.Context)
	StableHash func(*T, datastructures.StableHashCtxt, *datastructures.StableHasher, rustfmt.Context)
	Encode     func(*T, SpanEncoder, rustfmt.Context)
}

func (value *Spanned[T]) CloneWith(ops SpannedOps[T], ctx rustfmt.Context) Spanned[T] {
	if ops.Clone == nil {
		ctx.Panic("rustcspan: missing node Clone implementation")
	}
	return Spanned[T]{Node: ops.Clone(&value.Node, ctx), Span: value.Span.Clone()}
}
func (value *Spanned[T]) CloseWith(ops SpannedOps[T], ctx rustfmt.Context) {
	if ops.Drop != nil {
		ops.Drop(&value.Node, ctx)
	}
}
func (value *Spanned[T]) CloneFromWith(source *Spanned[T], ops SpannedOps[T], ctx rustfmt.Context) {
	replacement := source.CloneWith(ops, ctx)
	defer func() { *value = replacement }()
	value.CloseWith(ops, ctx)
}
func (value *Spanned[T]) EqualWith(other *Spanned[T], ops SpannedOps[T], ctx rustfmt.Context) bool {
	if ops.Equal == nil {
		ctx.Panic("rustcspan: missing node PartialEq implementation")
	}
	return ops.Equal(&value.Node, &other.Node, ctx) && value.Span == other.Span
}
func (value *Spanned[T]) NotEqualWith(other *Spanned[T], ops SpannedOps[T], ctx rustfmt.Context) bool {
	return !value.EqualWith(other, ops, ctx)
}
func (value *Spanned[T]) HashWith(h stablehash.Hasher, ops SpannedOps[T], ctx rustfmt.Context) {
	if ops.Hash == nil {
		ctx.Panic("rustcspan: missing node Hash implementation")
	}
	ops.Hash(&value.Node, h, ctx)
	stablehash.HashU64(h, value.Span.ToRawSpan().Value)
}
func (value *Spanned[T]) StableHashWith(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ops SpannedOps[T], ctx rustfmt.Context) {
	if ops.StableHash == nil {
		ctx.Panic("rustcspan: missing node StableHash implementation")
	}
	ops.StableHash(&value.Node, hcx, h, ctx)
	value.Span.StableHashWithContext(hcx, h, ctx)
}
func (value *Spanned[T]) WriteDebugFormatterWith(session *SessionGlobals, f *rustfmt.Formatter, ops SpannedOps[T]) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "Spanned", p)
	b.Field("node", f.Value(p, func(child *rustfmt.Formatter) error {
		if ops.Debug == nil {
			child.Context.Panic("rustcspan: missing node Debug implementation")
		}
		return ops.Debug(&value.Node, child)
	}))
	b.Field("span", f.Value(p, func(child *rustfmt.Formatter) error { return value.Span.WriteDebugFormatter(session, child) }))
	return b.Finish()
}
func (value *Spanned[T]) EncodeWith(e SpanEncoder, ops SpannedOps[T]) {
	ctx := spanCodecContext(e)
	if ops.Encode == nil {
		ctx.Panic("rustcspan: missing node Encodable implementation")
	}
	ops.Encode(&value.Node, e, ctx)
	value.Span.Encode(e)
}
func DecodeSpanned[T any](d SpanDecoder, decodeNode func(SpanDecoder, rustfmt.Context) T, dropNode func(*T, rustfmt.Context)) (out Spanned[T]) {
	ctx := spanCodecContext(d)
	out.Node = decodeNode(d, ctx)
	complete := false
	defer func() {
		if payload := recover(); payload != nil {
			if ctx.ProcessingPanic {
				ctx.ResumeUnwind(payload)
			}
			if !complete && dropNode != nil {
				unwind := ctx
				unwind.Unwinding = true
				spanCleanupDuringUnwindWithContext(unwind, func() { dropNode(&out.Node, unwind) })
			}
			ctx.ResumeUnwind(payload)
		}
	}()
	out.Span = DecodeSpan(d)
	complete = true
	return out
}

type ErrorGuaranteed struct{ valid bool }

// UncheckedErrorGuaranteed mirrors the explicitly unsafe-in-policy constructor
// reserved by the source for diagnostic emission. Go zero values are outside
// its constructible Rust domain and are rejected by the marker operations.
func UncheckedErrorGuaranteed() ErrorGuaranteed { return ErrorGuaranteed{valid: true} }
func (value ErrorGuaranteed) check(ctx rustfmt.Context) {
	if !value.valid {
		ctx.Panic("rustcspan: unconstructed ErrorGuaranteed")
	}
}
func (value ErrorGuaranteed) Clone() ErrorGuaranteed            { value.check(rustfmt.Context{}); return value }
func (value *ErrorGuaranteed) CloneFrom(source ErrorGuaranteed) { *value = source.Clone() }
func (value ErrorGuaranteed) Equal(other ErrorGuaranteed) bool {
	value.check(rustfmt.Context{})
	other.check(rustfmt.Context{})
	return true
}
func (value ErrorGuaranteed) Compare(other ErrorGuaranteed) int {
	value.check(rustfmt.Context{})
	other.check(rustfmt.Context{})
	return 0
}
func (value ErrorGuaranteed) Hash(stablehash.Hasher) { value.check(rustfmt.Context{}) }
func (value ErrorGuaranteed) StableHash(_ datastructures.StableHashCtxt, _ *datastructures.StableHasher) {
	value.check(rustfmt.Context{})
}
func (value ErrorGuaranteed) RaiseFatal() { value.RaiseFatalWithContext(rustfmt.Context{}) }
func (value ErrorGuaranteed) RaiseFatalWithContext(ctx rustfmt.Context) {
	value.check(ctx)
	(FatalError{}).RaiseWithContext(ctx)
}
func (value ErrorGuaranteed) WriteDebugFormatter(f *rustfmt.Formatter) error {
	value.check(f.Context)
	p := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "ErrorGuaranteed", p)
	b.Field(f.Value(p, func(f *rustfmt.Formatter) error { return f.PadString("()") }))
	return b.Finish()
}
func (value ErrorGuaranteed) Encode(e rustcserialize.Encoder) {
	ctx := spanCodecContext(e)
	value.check(ctx)
	spanStaticPanicAt(ctx, spanLibLocation(2917, 9), "should never serialize an `ErrorGuaranteed`, as we do not write metadata or incremental caches in case errors occurred")
}
func DecodeErrorGuaranteed(d rustcserialize.Decoder) ErrorGuaranteed {
	spanStaticPanicAt(spanCodecContext(d), spanLibLocation(2926, 9), "`ErrorGuaranteed` should never have been serialized to metadata or incremental caches")
	return ErrorGuaranteed{}
}
func (value InnerSpan) Clone() InnerSpan            { return value }
func (value *InnerSpan) CloneFrom(source InnerSpan) { *value = source }
func (value InnerSpan) Equal(other InnerSpan) bool  { return value == other }
func (value InnerSpan) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "InnerSpan", p)
	b.Field("start", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.Start), uint(spanUintBits()), f)
	}))
	b.Field("end", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.End), uint(spanUintBits()), f)
	}))
	return b.Finish()
}
