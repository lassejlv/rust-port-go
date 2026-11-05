// SPDX-License-Identifier: MIT OR Apache-2.0
// Owned ContextOrd closures from compiler/rustc_span/src/def_id.rs.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// DefPathHasherOps witnesses the concrete Fn implementation returned by a
// SpanEncoder. Call is a temporary shared borrow of the actual S cell; only
// source interior mutation is permitted. Drop is complete environment/field
// destruction, including local panic guards. Destructible captures belong in S,
// not hidden Go callback captures. Source panic sites use Ref.Context.Panic.
type DefPathHasherOps[S any] struct {
	Call     func(rustvec.Ref[S], DefId) DefPathHash
	CallMut  func(rustvec.Ref[S], DefId) DefPathHash
	CallOnce func(*rustvec.Owned[S], DefId) DefPathHash
	Drop     func(*S, rustfmt.Context)
}
type defPathHasherState interface {
	live() bool
	move(rustfmt.Context) defPathHasherState
	call(DefId, rustfmt.Context) DefPathHash
	callMut(DefId, rustfmt.Context) DefPathHash
	callOnce(DefId, rustfmt.Context) DefPathHash
	close(rustfmt.Context)
}
type defPathHasherOwned[S any] struct {
	value *rustvec.Owned[S]
	ops   DefPathHasherOps[S]
}

func (state defPathHasherOwned[S]) live() bool { return state.value.IsLive() }
func (state defPathHasherOwned[S]) move(context rustfmt.Context) defPathHasherState {
	return defPathHasherOwned[S]{state.value.MoveWithContext(context), state.ops}
}
func (state defPathHasherOwned[S]) call(value DefId, context rustfmt.Context) DefPathHash {
	return state.ops.Call(rustvec.Ref[S]{Value: state.value.Borrow(), Context: context}, value)
}
func (state defPathHasherOwned[S]) callMut(value DefId, context rustfmt.Context) DefPathHash {
	if state.ops.CallMut != nil {
		return state.ops.CallMut(rustvec.Ref[S]{Value: state.value.Borrow(), Context: context}, value)
	}
	return state.call(value, context)
}
func (state defPathHasherOwned[S]) callOnce(value DefId, context rustfmt.Context) DefPathHash {
	return rustvec.ConsumeValue(state.value, context, func(arg *rustvec.Owned[S]) DefPathHash {
		if state.ops.CallOnce != nil {
			return state.ops.CallOnce(arg, value)
		}
		return (defPathHasherOwned[S]{arg, state.ops}).callMut(value, context)
	})
}
func (state defPathHasherOwned[S]) close(context rustfmt.Context) {
	state.value.DropWithContext(context)
}

// DefPathHasher owns one arbitrary non-Copy Fn environment. Move invalidates
// earlier descriptor copies; borrowed calls retain the source state cell.
// Close is explicit source destruction. A borrowed state pointer cannot remain
// live across Move/Close or an incompatible source borrow.
type DefPathHasher struct {
	state   defPathHasherState
	context rustfmt.Context
}

func NewDefPathHasher[S any](value S, ops DefPathHasherOps[S]) DefPathHasher {
	return NewOwnedDefPathHasherWithOps(rustvec.NewOwned(value, rustvec.ValueOps[S]{Drop: ops.Drop}), ops)
}
func NewDefPathHasherWithContext[S any](value S, ops DefPathHasherOps[S], context rustfmt.Context) DefPathHasher {
	return NewOwnedDefPathHasherWithOpsWithContext(rustvec.NewOwned(value, rustvec.ValueOps[S]{Drop: ops.Drop}), ops, context)
}
func NewOwnedDefPathHasher[S any](value *rustvec.Owned[S], call func(rustvec.Ref[S], DefId) DefPathHash) DefPathHasher {
	return NewOwnedDefPathHasherWithOps(value, DefPathHasherOps[S]{Call: call})
}
func NewOwnedDefPathHasherWithContext[S any](value *rustvec.Owned[S], call func(rustvec.Ref[S], DefId) DefPathHash, context rustfmt.Context) DefPathHasher {
	return NewOwnedDefPathHasherWithOpsWithContext(value, DefPathHasherOps[S]{Call: call}, context)
}

// Nil CallMut/CallOnce slots witness the concrete compiler-generated forwarding
// strategy (CallMut -> Call, CallOnce -> CallMut then self destruction), not Rust
// trait defaults. Manual Fn implementations supply independent overrides.
// For an existing Owned cell its own complete Drop witness remains authoritative.
func NewOwnedDefPathHasherWithOps[S any](value *rustvec.Owned[S], ops DefPathHasherOps[S]) DefPathHasher {
	return NewOwnedDefPathHasherWithOpsWithContext(value, ops, defIdFnConvenienceContext(value.Context()))
}

// WithContext constructors take the current source caller's ambient context.
// An Owned.Context snapshot is an operation guard, not implicit current TLS.
// No-context conveniences preserve panic guards but clear borrowed Runtime.
func NewOwnedDefPathHasherWithOpsWithContext[S any](value *rustvec.Owned[S], ops DefPathHasherOps[S], context rustfmt.Context) DefPathHasher {
	if ops.Call == nil {
		context.Panic("def_id: missing Fn call witness")
	}
	return DefPathHasher{state: defPathHasherOwned[S]{value.MoveWithContext(context), ops}, context: context}
}
func defIdFnConvenienceContext(context rustfmt.Context) rustfmt.Context {
	context.Runtime = nil
	return context
}

// DefPathHasherFunc is the static/trivial-ownership function convenience. Use
// the owned generic API when the source Fn environment has destructor state.
func DefPathHasherFunc(call func(DefId) DefPathHash) DefPathHasher {
	if call == nil {
		panic("def_id: nil Fn function")
	}
	return NewDefPathHasher(call, DefPathHasherOps[func(DefId) DefPathHash]{Call: func(ref rustvec.Ref[func(DefId) DefPathHash], value DefId) DefPathHash { return (*ref.Value)(value) }})
}
func (hasher DefPathHasher) IsLive() bool { return hasher.state != nil && hasher.state.live() }
func (hasher DefPathHasher) check(context rustfmt.Context) {
	if !hasher.IsLive() {
		context.Panic("def_id: use of consumed Fn environment")
	}
}
func (hasher DefPathHasher) Move() DefPathHasher {
	return hasher.MoveWithContext(defIdFnConvenienceContext(hasher.context))
}
func (hasher DefPathHasher) MoveWithContext(context rustfmt.Context) DefPathHasher {
	hasher.check(context)
	return DefPathHasher{state: hasher.state.move(context), context: context}
}
func (hasher DefPathHasher) Call(value DefId) DefPathHash {
	return hasher.CallWithContext(value, defIdFnConvenienceContext(hasher.context))
}
func (hasher DefPathHasher) CallWithContext(value DefId, context rustfmt.Context) DefPathHash {
	hasher.check(context)
	return defIdInvoke(context, func() DefPathHash { return hasher.state.call(value, context) })
}
func (hasher DefPathHasher) CallMut(value DefId) DefPathHash {
	return hasher.CallMutWithContext(value, defIdFnConvenienceContext(hasher.context))
}
func (hasher DefPathHasher) CallMutWithContext(value DefId, context rustfmt.Context) DefPathHash {
	hasher.check(context)
	return defIdInvoke(context, func() DefPathHash { return hasher.state.callMut(value, context) })
}
func (hasher DefPathHasher) CallOnce(value DefId) DefPathHash {
	return hasher.CallOnceWithContext(value, defIdFnConvenienceContext(hasher.context))
}
func (hasher DefPathHasher) CallOnceWithContext(value DefId, context rustfmt.Context) DefPathHash {
	hasher.check(context)
	return defIdInvoke(context, func() DefPathHash { return hasher.state.callOnce(value, context) })
}
func (hasher DefPathHasher) Close() {
	hasher.CloseWithContext(defIdFnConvenienceContext(hasher.context))
}
func (hasher DefPathHasher) CloseWithContext(context rustfmt.Context) {
	if hasher.IsLive() {
		hasher.state.close(context)
	}
}
func defIdInvoke[T any](context rustfmt.Context, call func() T) T {
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	return call()
}

func captureDefPathHasher(provider any, context rustfmt.Context) DefPathHasher {
	var result DefPathHasher
	switch source := provider.(type) {
	case interface {
		DefPathHasherWithContext(rustfmt.Context) DefPathHasher
	}:
		result = defIdInvoke(context, func() DefPathHasher { return source.DefPathHasherWithContext(context) })
	case interface{ DefPathHasher() DefPathHasher }:
		result = defIdInvoke(context, source.DefPathHasher)
	default:
		return spanDefPathHasherDefault(context)
	}
	return result.MoveWithContext(context)
}

// A provider represents a borrowed real SpanEncoder. Go's structural used-
// operation witness permits the optional method's actual default panic when it
// is absent. The returned closure owns the Fn result, not the encoder itself.
type DefIdComparator struct{ hasher DefPathHasher }
type CrateNumComparator struct{ inner DefIdComparator }
type LocalDefIdComparator struct{ inner DefIdComparator }

func comparatorContext(ambient []rustfmt.Context) rustfmt.Context {
	if len(ambient) > 0 {
		return ambient[0]
	}
	return rustfmt.Context{}
}
func DefIdStableCmpWithContext(provider any, ambient ...rustfmt.Context) DefIdComparator {
	context := comparatorContext(ambient)
	return DefIdComparator{captureDefPathHasher(provider, context)}
}
func CrateNumStableCmpWithContext(provider any, ambient ...rustfmt.Context) CrateNumComparator {
	return CrateNumComparator{DefIdStableCmpWithContext(provider, ambient...)}
}
func LocalDefIdStableCmpWithContext(provider any, ambient ...rustfmt.Context) LocalDefIdComparator {
	return LocalDefIdComparator{DefIdStableCmpWithContext(provider, ambient...)}
}
func (comparator DefIdComparator) Compare(left, right DefId) int {
	return comparator.CompareWithContext(left, right, defIdFnConvenienceContext(comparator.hasher.context))
}
func (comparator DefIdComparator) CompareWithContext(left, right DefId, context rustfmt.Context) int {
	a := comparator.hasher.CallWithContext(left, context)
	b := comparator.hasher.CallWithContext(right, context)
	return a.Compare(b)
}
func (comparator DefIdComparator) Move() DefIdComparator {
	return DefIdComparator{comparator.hasher.Move()}
}
func (comparator DefIdComparator) MoveWithContext(context rustfmt.Context) DefIdComparator {
	return DefIdComparator{comparator.hasher.MoveWithContext(context)}
}
func (comparator DefIdComparator) Close() { comparator.hasher.Close() }
func (comparator DefIdComparator) CloseWithContext(context rustfmt.Context) {
	comparator.hasher.CloseWithContext(context)
}

// The returned ContextOrd closure's FnOnce call consumes/destroys its captured
// H after evaluating both keys. Its body still calls H's Fn, not H's FnOnce.
func (comparator DefIdComparator) CallOnce(a, b DefId) int {
	return comparator.CallOnceWithContext(a, b, defIdFnConvenienceContext(comparator.hasher.context))
}
func (comparator DefIdComparator) CallOnceWithContext(a, b DefId, context rustfmt.Context) int {
	arg := rustvec.NewOwned(comparator.MoveWithContext(context), rustvec.ValueOps[DefIdComparator]{Drop: func(value *DefIdComparator, context rustfmt.Context) { value.CloseWithContext(context) }})
	return rustvec.ConsumeValue(arg, context, func(value *rustvec.Owned[DefIdComparator]) int {
		return value.Borrow().CompareWithContext(a, b, context)
	})
}
func (comparator CrateNumComparator) Compare(a, b CrateNum) int {
	return comparator.inner.Compare(a.AsDefId(), b.AsDefId())
}
func (comparator CrateNumComparator) CompareWithContext(a, b CrateNum, context rustfmt.Context) int {
	return comparator.inner.CompareWithContext(a.AsDefId(), b.AsDefId(), context)
}
func (comparator CrateNumComparator) Move() CrateNumComparator {
	return CrateNumComparator{comparator.inner.Move()}
}
func (comparator CrateNumComparator) MoveWithContext(context rustfmt.Context) CrateNumComparator {
	return CrateNumComparator{comparator.inner.MoveWithContext(context)}
}
func (comparator CrateNumComparator) Close() { comparator.inner.Close() }
func (comparator CrateNumComparator) CloseWithContext(context rustfmt.Context) {
	comparator.inner.CloseWithContext(context)
}
func (comparator CrateNumComparator) CallOnce(a, b CrateNum) int {
	return comparator.inner.CallOnce(a.AsDefId(), b.AsDefId())
}
func (comparator CrateNumComparator) CallOnceWithContext(a, b CrateNum, context rustfmt.Context) int {
	return comparator.inner.CallOnceWithContext(a.AsDefId(), b.AsDefId(), context)
}
func (comparator LocalDefIdComparator) Compare(a, b LocalDefId) int {
	return comparator.inner.Compare(a.ToDefId(), b.ToDefId())
}
func (comparator LocalDefIdComparator) CompareWithContext(a, b LocalDefId, context rustfmt.Context) int {
	return comparator.inner.CompareWithContext(a.ToDefId(), b.ToDefId(), context)
}
func (comparator LocalDefIdComparator) Move() LocalDefIdComparator {
	return LocalDefIdComparator{comparator.inner.Move()}
}
func (comparator LocalDefIdComparator) MoveWithContext(context rustfmt.Context) LocalDefIdComparator {
	return LocalDefIdComparator{comparator.inner.MoveWithContext(context)}
}
func (comparator LocalDefIdComparator) Close() { comparator.inner.Close() }
func (comparator LocalDefIdComparator) CloseWithContext(context rustfmt.Context) {
	comparator.inner.CloseWithContext(context)
}
func (comparator LocalDefIdComparator) CallOnce(a, b LocalDefId) int {
	return comparator.inner.CallOnce(a.ToDefId(), b.ToDefId())
}
func (comparator LocalDefIdComparator) CallOnceWithContext(a, b LocalDefId, context rustfmt.Context) int {
	return comparator.inner.CallOnceWithContext(a.ToDefId(), b.ToDefId(), context)
}
