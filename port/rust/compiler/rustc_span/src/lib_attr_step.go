// SPDX-License-Identifier: MIT OR Apache-2.0
// Remaining orderable AttrId newtype_index! expansion, including std Step.
package rustcspan

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (value AttrId) NotEqual(other AttrId) bool              { return !value.Equal(other) }
func (value AttrId) PartialCompare(other AttrId) Option[int] { return Some(value.Compare(other)) }
func (value AttrId) Add(amount uint, config ...SpanEncodingConfig) AttrId {
	return value.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (value AttrId) AddWithContext(amount uint, ctx rustfmt.Context, config ...SpanEncodingConfig) AttrId {
	return AttrIdFromUsizeWithContext(spanCharPosAdd(value.Index(), amount, spanEncodingConfig(nil, config...), ctx, spanLibLocation(1370, 1)), ctx)
}
func (value *AttrId) AddAssign(amount uint, config ...SpanEncodingConfig) {
	*value = value.Add(amount, config...)
}
func (value *AttrId) AddAssignWithContext(amount uint, ctx rustfmt.Context, config ...SpanEncodingConfig) {
	*value = value.AddWithContext(amount, ctx, config...)
}
func AttrIdStepsBetween(start, end AttrId) (uint, Option[uint]) {
	if start.Index() > end.Index() {
		return 0, None[uint]()
	}
	n := end.Index() - start.Index()
	return n, Some(n)
}
func (value AttrId) ForwardChecked(amount uint) Option[AttrId] {
	return value.ForwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value AttrId) ForwardCheckedWithContext(amount uint, ctx rustfmt.Context) Option[AttrId] {
	if amount > ^uint(0)-value.Index() {
		return None[AttrId]()
	}
	return Some(AttrIdFromUsizeWithContext(value.Index()+amount, ctx))
}
func (value AttrId) BackwardChecked(amount uint) Option[AttrId] {
	return value.BackwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value AttrId) BackwardCheckedWithContext(amount uint, ctx rustfmt.Context) Option[AttrId] {
	if amount > value.Index() {
		return None[AttrId]()
	}
	return Some(AttrIdFromUsizeWithContext(value.Index()-amount, ctx))
}
func (value AttrId) ForwardOverflowing(amount uint) (AttrId, bool) {
	return value.ForwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value AttrId) ForwardOverflowingWithContext(amount uint, ctx rustfmt.Context) (AttrId, bool) {
	return AttrIdFromUsizeWithContext(value.Index()+amount, ctx), amount > ^uint(0)-value.Index()
}
func (value AttrId) BackwardOverflowing(amount uint) (AttrId, bool) {
	return value.BackwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value AttrId) BackwardOverflowingWithContext(amount uint, ctx rustfmt.Context) (AttrId, bool) {
	return AttrIdFromUsizeWithContext(value.Index()-amount, ctx), amount > value.Index()
}
func (value AttrId) Forward(amount uint) AttrId {
	return value.ForwardWithContext(amount, rustfmt.Context{})
}
func (value AttrId) ForwardWithContext(amount uint, ctx rustfmt.Context) AttrId {
	if result, ok := value.ForwardCheckedWithContext(amount, ctx).Get(); ok {
		return result
	}
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: ctx.SourcePanicLocation(AttrIdStepForwardPanicSite), CanUnwind: true}, func(rustfmt.Context) any { return "overflow in `Step::forward`" })
	return AttrId{}
}
func (value AttrId) Backward(amount uint) AttrId {
	return value.BackwardWithContext(amount, rustfmt.Context{})
}
func (value AttrId) BackwardWithContext(amount uint, ctx rustfmt.Context) AttrId {
	if result, ok := value.BackwardCheckedWithContext(amount, ctx).Get(); ok {
		return result
	}
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: ctx.SourcePanicLocation(AttrIdStepBackwardPanicSite), CanUnwind: true}, func(rustfmt.Context) any { return "overflow in `Step::backward`" })
	return AttrId{}
}
func (value AttrId) ForwardUnchecked(amount uint) AttrId  { return value.Forward(amount) }
func (value AttrId) BackwardUnchecked(amount uint) AttrId { return value.Backward(amount) }
func AttrIdIdx() index.Idx[AttrId]                        { return AttrIdIdxWithContext(rustfmt.Context{}) }
func AttrIdIdxWithContext(ctx rustfmt.Context, explicit ...SpanEncodingConfig) index.Idx[AttrId] {
	config := spanEncodingConfig(nil, explicit...)
	result := index.NewIdxWithConfig(func(value uint) AttrId { return AttrIdFromUsizeWithContext(value, ctx) }, func(value AttrId) uint { return value.Index() }, index.IdxConfig{OverflowChecks: config.OverflowChecks})
	result.PlusOverride = func(value AttrId, amount uint) AttrId {
		index := value.Index()
		if config.OverflowChecks && amount > ^uint(0)-index {
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "attempt to add with overflow", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("source:rustc_index/src/idx.rs:22:19"), CanUnwind: true})
		}
		return AttrIdFromUsizeWithContext(index+amount, ctx)
	}
	return result
}
func AttrIdStep(ambient ...rustfmt.Context) index.IntervalStep[AttrId] {
	ctx := comparatorContext(ambient)
	return defIdStep("rustc_span::AttrId", func(a, b AttrId) int { return a.Compare(b) }, AttrIdStepsBetween, func(value AttrId, n uint) (AttrId, bool) { return value.ForwardCheckedWithContext(n, ctx).Get() }, func(value AttrId, n uint) (AttrId, bool) { return value.BackwardCheckedWithContext(n, ctx).Get() }, func(value AttrId, n uint) (AttrId, bool) { return value.ForwardOverflowingWithContext(n, ctx) }, func(value AttrId, n uint) (AttrId, bool) { return value.BackwardOverflowingWithContext(n, ctx) }, func(value AttrId, n uint) AttrId { return value.ForwardWithContext(n, ctx) }, func(value AttrId, n uint) AttrId { return value.BackwardWithContext(n, ctx) })
}
