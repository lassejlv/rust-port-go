// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

// BorrowedConsumer is the scoped call view of a source FnMut with only static
// or temporarily borrowed captures. It is not an arbitrary owned Fn object.
// Copies borrow the same source closure environment and grant no concurrent
// mutable-call rights. Calls and all retained metadata must remain within the
// enclosing source operation's borrow lifetime. T moves into Call: the body
// owns its complete parameter cleanup, including on panic.
type BorrowedConsumer[T any] struct{ call func(T, Context) }

func NewBorrowedConsumer[T any](call func(T, Context)) BorrowedConsumer[T] {
	return BorrowedConsumer[T]{call}
}
func (c BorrowedConsumer[T]) Call(value T, current Context) {
	if c.call == nil {
		current.Panic("rustvec: invalid borrowed consumer")
	}
	runContext(current, func() { c.call(value, current) })
}

// ScopedIteratorOps supplies the actual independently selected for_each/fold
// bodies when they can invoke their consumer in a different current context,
// for example from a destructor under a nested caught unwind. The actual S
// owner and complete Drop stay in the existing protected iterator machinery.
// Nil slots select the existing default or same-context legacy witness.
// Dictionaries have no destructible hidden captures. IteratorOps callbacks and
// func(T) consumer conveniences describe same-context calls; the scoped slots
// are the primary witnesses for dynamic current-context implementations.
type ScopedIteratorOps[S, T any] struct {
	ForEachScoped  func(*Owned[S], BorrowedConsumer[T])
	FoldUnitScoped func(*Owned[S], BorrowedConsumer[T])
}
type consumerBridge[T any] struct {
	active   bool
	consumer BorrowedConsumer[T]
}

func (b *consumerBridge[T]) selected(legacy func(T)) BorrowedConsumer[T] {
	if b.active {
		return b.consumer
	}
	return NewBorrowedConsumer(func(value T, _ Context) { legacy(value) })
}
func (b *consumerBridge[T]) invoke(c Context, consumer BorrowedConsumer[T], body func(Context, func(T))) {
	if b.active {
		c.Panic("rustvec: overlapping exclusive borrowed-consumer operation")
	}
	b.active = true
	b.consumer = consumer
	defer func() { b.active = false; b.consumer = BorrowedConsumer[T]{} }()
	body(c, func(value T) { consumer.Call(value, c) })
}

// NewScopedIteratorFromOwned adds only scoped operation metadata to an actual
// iterator. Every construction receives a separate bridge; constructing an
// iterator from an actual cloned S must call this constructor again. Generic
// Iterator has no Clone operation; descriptor copies are owning aliases, not
// source Clone. The returned ordinary Iterator retains these slots through its
// existing IntoIterator transfer without a global side table or state copy.
func NewScopedIteratorFromOwned[S, T any](state *Owned[S], ops IteratorOps[S, T], scoped ScopedIteratorOps[S, T]) Iterator[T] {
	bridge := new(consumerBridge[T])
	if scoped.ForEachScoped != nil {
		ops.ForEach = func(arg *Owned[S], legacy func(T)) { scoped.ForEachScoped(arg, bridge.selected(legacy)) }
	}
	if scoped.FoldUnitScoped != nil {
		ops.FoldUnit = func(arg *Owned[S], legacy func(T)) { scoped.FoldUnitScoped(arg, bridge.selected(legacy)) }
	}
	it := NewIteratorFromOwned(state, ops)
	// Capture the guarded raw operation, not the original iterator cell/live
	// flag: Map<I,F> transfers/copies its descriptor while consuming that cell.
	each, fold := it.cell.each, it.cell.fold
	it.cell.scopedEach = func(c Context, consumer BorrowedConsumer[T]) { bridge.invoke(c, consumer, each) }
	it.cell.scopedFold = func(c Context, consumer BorrowedConsumer[T]) { bridge.invoke(c, consumer, fold) }
	return it
}

func (it Iterator[T]) ForEachScopedWithContext(c Context, consumer BorrowedConsumer[T]) {
	if !it.IsLive() {
		c.Panic("rustvec: consumed iterator")
	}
	it.cell.live = false
	runContext(c, func() {
		if it.cell.scopedEach != nil {
			it.cell.scopedEach(c, consumer)
			return
		}
		it.cell.each(c, func(value T) { consumer.Call(value, c) })
	})
}
func (it Iterator[T]) FoldUnitScopedWithContext(c Context, consumer BorrowedConsumer[T]) {
	if !it.IsLive() {
		c.Panic("rustvec: consumed iterator")
	}
	it.cell.live = false
	runContext(c, func() {
		if it.cell.scopedFold != nil {
			it.cell.scopedFold(c, consumer)
			return
		}
		it.cell.fold(c, func(value T) { consumer.Call(value, c) })
	})
}

// MapToUnitScopedWithContext retains the original fixed Map<I,F> operation,
// including actual I::map dispatch and state transfer. Its consuming for_each
// selects I::fold's scoped slot and bypasses a distinct I::for_each override.
func (it Iterator[T]) MapToUnitScopedWithContext(c Context, ops ElementOps[T]) UnitMap[T] {
	result := it.MapToUnitWithContext(c, ops)
	result.iterator.cell.scopedEach = result.iterator.cell.scopedFold
	return result
}
func (it UnitMap[T]) ForEachScopedWithContext(c Context, consumer BorrowedConsumer[T]) {
	it.iterator.ForEachScopedWithContext(c, consumer)
}

// These forms give the fixed mapper the actual Fn-call scope when a custom
// I::map invokes it under a nested observer or unwind. The old methods remain
// same-context conveniences; the mapping itself still owns no environment.
func (m UnitMapping[T]) OwnWithContext(value T, c Context) *Owned[T] {
	return NewOwned(value, ValueOps[T]{Drop: m.ops.Drop}).MoveWithContext(c)
}
func (m UnitMapping[T]) ApplyWithContext(value *Owned[T], c Context) *Owned[T] {
	return value.MoveWithContext(c)
}
