// SPDX-License-Identifier: MIT OR Apache-2.0
// Additive operation-time runtime bridge for the existing IndexVec storage.
// This adds no standard-library source-file completion credit.
package rustcindex

import (
	"reflect"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// PanicVectorCapacitySite selects the caller's immutable source-location
// profile for the actual Vec capacity-overflow raise. No Go location is used.
const PanicVectorCapacitySite = "rustcindex.vec.capacity_overflow"
const PanicExtendTrustedOverflowSite = "rustcindex.vec.extend_trusted_overflow"
const PanicCollectTrustedOverflowSite = "rustcindex.vec.collect_trusted_overflow"

func indexVectorCapacity(context rustfmt.Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(PanicVectorCapacitySite), CanUnwind: true})
}
func indexTrustedOverflow(context rustfmt.Context, site string) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(site), CanUnwind: true})
}
func indexRuntimeUnwind(context rustfmt.Context) rustfmt.Context {
	context.Unwinding = true
	return context
}
func indexRuntimeCleanup(context rustfmt.Context, cleanup func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	cleanup()
}
func indexRuntimeForward(context rustfmt.Context, body func()) {
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	body()
}
func (ops ValueOps[T]) cloneAtRuntime(value *T, context rustfmt.Context) (result T) {
	indexRuntimeForward(context, func() {
		if ops.CloneRefWithContext != nil {
			result = ops.CloneRefWithContext(value, context)
		} else {
			result = ops.cloneAt(value)
		}
	})
	return
}
func (ops ValueOps[T]) hasRuntimeDrop() bool {
	return ops.DropWithRuntimeContext != nil || ops.hasDrop()
}
func (ops ValueOps[T]) dropAtRuntime(value *T, context rustfmt.Context) {
	indexRuntimeForward(context, func() {
		if ops.DropWithRuntimeContext != nil {
			ops.DropWithRuntimeContext(value, context)
		} else {
			ops.dropAt(value, IndexDropContext{Unwinding: context.Unwinding})
		}
	})
}

// A current context is borrowed only for this operation. In particular neither
// IndexVec, its Ops, nor an escaping iterator stores the runtime pointer.
func (v *IndexVec[I, T]) reserveExactRuntime(capacity uint, context rustfmt.Context) {
	if v.Raw.zeroSized || capacity <= v.Raw.Cap() {
		return
	}
	size := v.Ops.RustSize
	if size == 0 {
		size = uint(reflect.TypeFor[T]().Size())
	}
	maximum := uint(^uint(0) >> 1)
	if capacity > maximum || size != 0 && capacity > maximum/size {
		indexVectorCapacity(context)
	}
	// All source capacity failures were checked at their actual allocation
	// phase, before invoking the established storage implementation.
	v.reserveExactRaw(capacity)
}
func (v *IndexVec[I, T]) reserveRuntime(additional uint, context rustfmt.Context) {
	if additional > ^uint(0)-v.Len() {
		indexVectorCapacity(context)
	}
	needed := v.Len() + additional
	if v.Raw.zeroSized || needed <= v.Raw.Cap() {
		return
	}
	capacity := v.Raw.Cap()
	if capacity <= ^uint(0)/2 {
		capacity *= 2
	} else {
		capacity = ^uint(0)
	}
	capacity = max(capacity, needed)
	size := v.Ops.RustSize
	if size == 0 {
		size = uint(reflect.TypeFor[T]().Size())
	}
	minimum := uint(1)
	if size == 1 {
		minimum = 8
	} else if size <= 1024 {
		minimum = 4
	}
	v.reserveExactRuntime(max(capacity, minimum), context)
}
func (v *IndexVec[I, T]) pushRuntime(value T, context rustfmt.Context) {
	v.reserveRuntime(1, context)
	v.pushRaw(value)
}
func indexRuntimeValueOnPanic[T any](value *T, ops ValueOps[T], owned *bool, context rustfmt.Context) {
	if p := recover(); p != nil {
		if context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		if *owned {
			unwind := indexRuntimeUnwind(context)
			indexRuntimeCleanup(unwind, func() { ops.dropAtRuntime(value, unwind) })
		}
		context.ResumeUnwind(p)
	}
}

func (v *IndexVec[I, T]) PushWithContext(value T, context rustfmt.Context) (key I) {
	owned := true
	defer indexRuntimeValueOnPanic(&value, v.Ops, &owned, context)
	key = v.NextIndex()
	v.pushRuntime(value, context)
	owned = false
	return
}

func dropRawRuntime[T any](raw IndexRaw[T], ops ValueOps[T], context rustfmt.Context) {
	if !ops.hasRuntimeDrop() {
		return
	}
	var first any
	for i := uint(0); i < raw.Len(); i++ {
		if first == nil {
			first = indexCapturePanic(func() { ops.dropAtRuntime(raw.At(i), context) })
			if first != nil && context.ProcessingPanic {
				context.ResumeUnwind(first)
			}
		} else {
			unwind := indexRuntimeUnwind(context)
			indexRuntimeCleanup(unwind, func() { ops.dropAtRuntime(raw.At(i), unwind) })
		}
	}
	if first != nil {
		context.ResumeUnwind(first)
	}
}

// CloseWithRuntimeContext destroys the value in the current call. Ambient
// unwinding alone is not a compiler cleanup edge: a caller may catch a new
// panic inside its outer destructor. Actual cleanup guards below own aborts.
func (v *IndexVec[I, T]) CloseWithRuntimeContext(context rustfmt.Context) {
	tail := v.Raw
	v.setLen(0)
	defer func() {
		if !tail.zeroSized {
			clear(tail.data)
			v.Raw.data = nil
		}
	}()
	dropRawRuntime(tail, v.Ops, context)
}
func indexRuntimeVectorOnPanic[I, T any](v *IndexVec[I, T], context rustfmt.Context) {
	if p := recover(); p != nil {
		if context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		unwind := indexRuntimeUnwind(context)
		indexRuntimeCleanup(unwind, func() { v.CloseWithRuntimeContext(unwind) })
		context.ResumeUnwind(p)
	}
}
func cloneRawRuntime[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig, context rustfmt.Context) *IndexVec[I, T] {
	v := NewIndexVecWithConfig(idx, ops, config)
	defer indexRuntimeVectorOnPanic(v, context)
	v.reserveExactRuntime(raw.Len(), context)
	if ops.TrivialClone || raw.zeroSized && ops.Clone == nil && ops.CloneRef == nil && ops.CloneRefWithContext == nil {
		v.extendRawCopy(raw)
		return v
	}
	for i := uint(0); i < raw.Len(); i++ {
		v.pushRuntime(ops.cloneAtRuntime(raw.At(i), context), context)
	}
	return v
}
func (v *IndexVec[I, T]) CloneWithContext(context rustfmt.Context) *IndexVec[I, T] {
	return cloneRawRuntime(v.Raw, v.Idx, v.Ops, v.Config, context)
}
func (v *IndexVec[I, T]) CloneFromWithContext(source *IndexVec[I, T], context rustfmt.Context) {
	cloned := source.CloneWithContext(context)
	old := *v
	*v = *cloned
	cloned.Raw = rawFromSlice([]T{}, cloned.Ops)
	dropRawRuntime(old.Raw, old.Ops, context)
}
func (it *OwnedIterator[T]) CloneWithContext(context rustfmt.Context) *OwnedIterator[T] {
	v := cloneRawRuntime(it.AsSlice(), UsizeIdx(), it.ops, defaultIndexStorageConfig(), context)
	return v.IntoIter()
}
func (it *OwnedIterator[T]) CloseWithRuntimeContext(context rustfmt.Context) {
	if it.closed {
		return
	}
	remaining := it.AsSlice()
	it.front = it.back
	it.closed = true
	defer func() {
		if !it.raw.zeroSized {
			clear(it.raw.data)
			it.raw.data = nil
		}
	}()
	dropRawRuntime(remaining, it.ops, context)
}

func indexRuntimeInto[T any](source IndexIntoIterator[T], context rustfmt.Context) (it IndexValueIterator[T]) {
	indexRuntimeForward(context, func() {
		if source.IntoIterWithContext != nil {
			it = source.IntoIterWithContext(context)
		} else {
			it = source.IntoIter()
		}
	})
	return
}
func indexRuntimeNext[T any](it IndexValueIterator[T], context rustfmt.Context) (value T, ok bool) {
	indexRuntimeForward(context, func() {
		if it.NextWithContext != nil {
			value, ok = it.NextWithContext(context)
		} else {
			value, ok = it.Next()
		}
	})
	return
}
func indexRuntimeHint[T any](it IndexValueIterator[T], context rustfmt.Context) (lower, upper uint, some bool) {
	indexRuntimeForward(context, func() {
		if it.SizeHintWithContext != nil {
			lower, upper, some = it.SizeHintWithContext(context)
		} else {
			lower, upper, some = iteratorHint(it)
		}
	})
	return
}
func indexRuntimeEach[T any](it IndexValueIterator[T], context rustfmt.Context, consume func(T)) {
	indexRuntimeForward(context, func() {
		switch {
		case it.ForEachWithContextOverride != nil:
			it.ForEachWithContextOverride(context, consume)
		case it.ForEachOverride != nil:
			it.ForEachOverride(consume)
		case it.FoldUnitWithContextOverride != nil:
			it.FoldUnitWithContextOverride(context, consume)
		case it.FoldUnitOverride != nil:
			it.FoldUnitOverride(consume)
		default:
			for {
				value, ok := indexRuntimeNext(it, context)
				if !ok {
					return
				}
				consume(value)
			}
		}
	})
}
func indexRuntimeIteratorClose[T any](it IndexValueIterator[T], context rustfmt.Context) {
	indexRuntimeForward(context, func() {
		if it.Owned != nil {
			it.Owned.CloseWithRuntimeContext(context)
		} else if it.CloseWithRuntimeContext != nil {
			it.CloseWithRuntimeContext(context)
		} else if it.CloseWithContext != nil {
			it.CloseWithContext(IndexDropContext{Unwinding: context.Unwinding})
		} else if it.Close != nil {
			it.Close()
		}
	})
}
func indexRuntimeIteratorDefer[T any](it IndexValueIterator[T], context rustfmt.Context) {
	p := recover()
	if p != nil {
		if context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		unwind := indexRuntimeUnwind(context)
		indexRuntimeCleanup(unwind, func() { indexRuntimeIteratorClose(it, unwind) })
		context.ResumeUnwind(p)
	}
	indexRuntimeIteratorClose(it, context)
}
func (v *IndexVec[I, T]) extendRuntime(it IndexValueIterator[T], context rustfmt.Context) {
	defer indexRuntimeIteratorDefer(it, context)
	if it.Owned != nil {
		v.reserveRuntime(it.Owned.Len(), context)
		v.extendRawCopy(it.Owned.AsSlice())
		it.Owned.forgetRemaining()
		return
	}
	if it.TrustedLength {
		lower, upper, some := indexRuntimeHint(it, context)
		if !some {
			indexTrustedOverflow(context, PanicExtendTrustedOverflowSite)
		}
		if v.Config.DebugAssertions && lower != upper {
			// This violates the unsafe TrustedLen dictionary contract. Keep
			// the legacy adapter diagnostic, without fabricating a source
			// assert payload/location for an invalid supplied witness.
			context.Panic("rustcindex: trusted iterator hint is not exact")
		}
		v.reserveRuntime(upper, context)
		indexRuntimeEach(it, context, func(value T) { v.pushRuntime(value, context) })
		return
	}
	for {
		value, ok := indexRuntimeNext(it, context)
		if !ok {
			return
		}
		if v.Len() == v.Raw.Cap() {
			owned := true
			func() {
				defer indexRuntimeValueOnPanic(&value, v.Ops, &owned, context)
				lower, _, _ := indexRuntimeHint(it, context)
				if lower == ^uint(0) {
					indexVectorCapacity(context)
				}
				v.reserveRuntime(lower+1, context)
				owned = false
			}()
		}
		v.pushRuntime(value, context)
	}
}
func (v *IndexVec[I, T]) ExtendWithContext(source IndexIntoIterator[T], context rustfmt.Context) {
	v.extendRuntime(indexRuntimeInto(source, context), context)
}

func IndexVecFromIteratorWithContext[I, T any](source IndexIntoIterator[T], idx Idx[I], ops ValueOps[T], context rustfmt.Context) *IndexVec[I, T] {
	return IndexVecFromIteratorWithConfigAndContext(source, idx, ops, defaultIndexStorageConfig(), context)
}
func IndexVecFromIteratorWithConfigAndContext[I, T any](source IndexIntoIterator[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig, context rustfmt.Context) *IndexVec[I, T] {
	v := NewIndexVecWithConfig(idx, ops, config)
	defer indexRuntimeVectorOnPanic(v, context)
	it := indexRuntimeInto(source, context)
	defer indexRuntimeIteratorDefer(it, context)
	if it.Owned != nil {
		owned := it.Owned
		if owned.front == 0 || owned.Len() >= owned.raw.Cap()/2 {
			v.Raw = owned.takeRaw()
			return v
		}
		v.reserveRuntime(owned.Len(), context)
		v.extendRawCopy(owned.AsSlice())
		owned.forgetRemaining()
		return v
	}
	if it.TrustedLength {
		_, upper, some := indexRuntimeHint(it, context)
		if !some {
			indexTrustedOverflow(context, PanicCollectTrustedOverflowSite)
		}
		v.reserveExactRuntime(upper, context)
		it.Close, it.CloseWithContext, it.CloseWithRuntimeContext = nil, nil, nil
		v.extendRuntime(it, context)
		return v
	}
	first, ok := indexRuntimeNext(it, context)
	if !ok {
		return v
	}
	owned := true
	func() {
		defer indexRuntimeValueOnPanic(&first, ops, &owned, context)
		lower, _, _ := indexRuntimeHint(it, context)
		capacity := lower
		if capacity != ^uint(0) {
			capacity++
		}
		size := ops.RustSize
		if size == 0 {
			size = uint(reflect.TypeFor[T]().Size())
		}
		minimum := uint(1)
		if size == 1 {
			minimum = 8
		} else if size <= 1024 {
			minimum = 4
		}
		v.reserveExactRuntime(max(capacity, minimum), context)
		v.pushRuntime(first, context)
		owned = false
	}()
	it.Close, it.CloseWithContext, it.CloseWithRuntimeContext = nil, nil, nil
	v.extendRuntime(it, context)
	return v
}
