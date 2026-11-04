// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

// UnitMapping is the actual fixed, zero-sized |key| (key, ()) operation used by
// HashSet::extend. The unit field has no storage or destructor. A custom map
// constructor can invoke it on owned keys before returning or panicking, with
// the same move/drop effects as the source mapping FnMut.
type UnitMapping[T any] struct {
	context Context
	ops     ElementOps[T]
}

func (m UnitMapping[T]) Context() Context                { return m.context }
func (m UnitMapping[T]) Own(value T) *Owned[T]           { return NewOwned(value, ValueOps[T]{Drop: m.ops.Drop}) }
func (m UnitMapping[T]) Apply(value *Owned[T]) *Owned[T] { return value.MoveWithContext(m.context) }

// UnitMap is the scoped operation view of Map<I, the fixed key/unit mapping>.
// It retains only operations used by HashSet::extend: hint, for_each and Drop.
// It makes no general Iterator/Vec-specialization or std source-file claim.
type UnitMap[T any] struct{ iterator Iterator[T] }

// MapToUnitWithContext consumes the input iterator into its fixed Map adapter.
// Source captures are represented by the original state cell and optional
// selected constructor witness; the internal mapping has trivial capture/drop.
func (it Iterator[T]) MapToUnitWithContext(context Context, ops ElementOps[T]) UnitMap[T] {
	if !it.IsLive() {
		context.Panic("rustvec: consumed iterator")
	}
	it.cell.live = false
	result := *it.cell
	result.live = true
	if it.cell.mapUnit != nil {
		runContext(context, func() { it.cell.mapUnit(context, ops) })
	}
	result.each = result.fold
	result.builtin = nil
	result.collect = nil
	return UnitMap[T]{Iterator[T]{&result}}
}
func (it UnitMap[T]) SizeHint(context Context) Hint { return it.iterator.SizeHint(context) }
func (it UnitMap[T]) ForEachWithContext(context Context, consume func(T)) {
	it.iterator.ForEachWithContext(context, consume)
}
func (it UnitMap[T]) CloseWithContext(context Context) { it.iterator.CloseWithContext(context) }
