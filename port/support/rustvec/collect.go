// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped Vec::from_iter compatibility used by translated source wrappers.
// Dispatch follows the validated rustc_index storage adapter; this supporting
// code grants no Rust standard-library source-file completion credit.
package rustvec

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"iter"
	"os"
)

type Context = rustfmt.Context
type Hint struct {
	Lower, Upper uint
	HasUpper     bool
}
type Config struct {
	Context         Context
	DebugAssertions bool
	ElementSize     uint
}

func DefaultConfig(elementSize uint) Config {
	return Config{DebugAssertions: true, ElementSize: elementSize}
}

type ElementOps[T any] struct{ Drop func(*T, Context) }
type IteratorOps[S, T any] struct {
	Next          func(Ref[S]) (T, bool)
	SizeHint      func(Ref[S]) Hint
	Drop          func(*S, Context)
	TrustedLength bool
	ForEach       func(*Owned[S], func(T))
	FoldUnit      func(*Owned[S], func(T))
	// MapToUnit witnesses the selected monomorphized Iterator::map constructor
	// for Map<I, the zero-sized |key| (key, ()) closure>. Nil means the actual
	// trait default. The returned owner is the I field of the returned Map.
	MapToUnit func(*Owned[S], UnitMapping[T]) *Owned[S]
	// VecFromIterator witnesses the concrete iterator type's monomorphized std
	// collection specialization (e.g. in-place collection). It is not a Rust
	// user override. Nil is lawful only for the generic/trusted source strategy.
	VecFromIterator func(*Owned[S], Config, ElementOps[T]) []T
}
type IntoIteratorOps[S, T any] struct {
	IntoIter func(*Owned[S]) Iterator[T]
	Drop     func(*S, Context)
}
type iteratorCell[T any] struct {
	live    bool
	next    func(Context) (T, bool)
	hint    func(Context) Hint
	close   func(Context)
	each    func(Context, func(T))
	fold    func(Context, func(T))
	mapUnit func(Context, ElementOps[T])
	trusted bool
	builtin *VecIntoIter[T]
	collect func(Config, ElementOps[T]) []T
}
type Iterator[T any] struct{ cell *iteratorCell[T] }
type inputCell[T any] struct {
	live  bool
	call  func(Context) Iterator[T]
	close func(Context)
}
type IntoIterator[T any] struct{ cell *inputCell[T] }

func NewIterator[S, T any](state S, ops IteratorOps[S, T]) Iterator[T] {
	return NewIteratorFromOwned(NewOwned(state, ValueOps[S]{Drop: ops.Drop}), ops)
}
func NewIteratorFromOwned[S, T any](state *Owned[S], ops IteratorOps[S, T]) Iterator[T] {
	if ops.Next == nil {
		panic("rustvec: missing Iterator::next witness")
	}
	state = state.Move()
	cell := &iteratorCell[T]{live: true, trusted: ops.TrustedLength}
	cell.next = func(context Context) (value T, ok bool) {
		runContext(context, func() { value, ok = ops.Next(Ref[S]{state.Borrow(), context}) })
		return
	}
	cell.hint = func(context Context) (hint Hint) {
		if ops.SizeHint != nil {
			runContext(context, func() { hint = ops.SizeHint(Ref[S]{state.Borrow(), context}) })
		}
		return
	}
	cell.close = func(context Context) { state.DropWithContext(context) }
	cell.each = func(context Context, consume func(T)) {
		arg := state.MoveWithContext(context)
		defer cleanupOwned(arg, context)
		if ops.ForEach != nil {
			ops.ForEach(arg, consume)
			return
		}
		if ops.FoldUnit != nil {
			ops.FoldUnit(arg, consume)
			return
		}
		for {
			value, ok := ops.Next(Ref[S]{arg.Borrow(), context})
			if !ok {
				return
			}
			consume(value)
		}
	}
	// Keep fold independent: an adapter's for_each may delegate to the input's
	// fold while deliberately bypassing its separately overridden for_each.
	cell.fold = func(context Context, consume func(T)) {
		arg := state.MoveWithContext(context)
		defer cleanupOwned(arg, context)
		if ops.FoldUnit != nil {
			ops.FoldUnit(arg, consume)
			return
		}
		for {
			value, ok := ops.Next(Ref[S]{arg.Borrow(), context})
			if !ok {
				return
			}
			consume(value)
		}
	}
	cell.mapUnit = func(context Context, elementOps ElementOps[T]) {
		arg := state.MoveWithContext(context)
		defer cleanupOwned(arg, context)
		result := arg
		if ops.MapToUnit != nil {
			result = ops.MapToUnit(arg, UnitMapping[T]{context: context, ops: elementOps})
		}
		if result == nil || !result.IsLive() {
			context.Panic("rustvec: map constructor returned consumed receiver state")
		}
		state = result.MoveWithContext(context)
	}
	if ops.VecFromIterator != nil {
		cell.collect = func(config Config, elementOps ElementOps[T]) []T {
			arg := state.MoveWithContext(config.Context)
			defer cleanupOwned(arg, config.Context)
			return ops.VecFromIterator(arg, config, elementOps)
		}
	}
	return Iterator[T]{cell}
}
func NewIntoIterator[S, T any](state S, ops IntoIteratorOps[S, T]) IntoIterator[T] {
	return NewIntoIteratorFromOwned(NewOwned(state, ValueOps[S]{Drop: ops.Drop}), ops)
}
func NewIntoIteratorFromOwned[S, T any](state *Owned[S], ops IntoIteratorOps[S, T]) IntoIterator[T] {
	if ops.IntoIter == nil {
		panic("rustvec: missing IntoIterator::into_iter witness")
	}
	state = state.Move()
	cell := &inputCell[T]{live: true}
	cell.close = func(context Context) { state.DropWithContext(context) }
	cell.call = func(context Context) Iterator[T] {
		arg := state.MoveWithContext(context)
		defer cleanupOwned(arg, context)
		result := ops.IntoIter(arg)
		if !result.IsLive() {
			context.Panic("rustvec: into_iter returned a consumed iterator")
		}
		return result
	}
	return IntoIterator[T]{cell}
}
func FromIterator[T any](iterator Iterator[T]) IntoIterator[T] {
	return NewIntoIterator(iterator, IntoIteratorOps[Iterator[T], T]{IntoIter: func(owned *Owned[Iterator[T]]) Iterator[T] { return owned.Take() }, Drop: func(value *Iterator[T], context Context) { value.CloseWithContext(context) }})
}
func (it IntoIterator[T]) IntoIter(context Context) Iterator[T] {
	if it.cell == nil || !it.cell.live {
		context.Panic("rustvec: consumed IntoIterator")
	}
	it.cell.live = false
	return it.cell.call(context)
}
func (it IntoIterator[T]) CloseWithContext(context Context) {
	if it.cell == nil || !it.cell.live {
		return
	}
	it.cell.live = false
	it.cell.close(context)
}
func (it Iterator[T]) IsLive() bool { return it.cell != nil && it.cell.live }
func (it Iterator[T]) Next(context Context) (T, bool) {
	if !it.IsLive() {
		context.Panic("rustvec: consumed iterator")
	}
	return it.cell.next(context)
}
func (it Iterator[T]) SizeHint(context Context) Hint {
	if !it.IsLive() {
		context.Panic("rustvec: consumed iterator")
	}
	return it.cell.hint(context)
}
func (it Iterator[T]) CloseWithContext(context Context) {
	if !it.IsLive() {
		return
	}
	it.cell.live = false
	it.cell.close(context)
}
func (it Iterator[T]) Close() { it.CloseWithContext(Context{}) }
func (it Iterator[T]) forEach(context Context, consume func(T)) {
	if !it.IsLive() {
		context.Panic("rustvec: consumed iterator")
	}
	it.cell.live = false
	runContext(context, func() { it.cell.each(context, consume) })
}
func FromSeq[T any](sequence iter.Seq[T]) IntoIterator[T] {
	if sequence == nil {
		panic("rustvec: nil Go sequence")
	}
	next, stop := iter.Pull(sequence)
	it := NewIterator(struct{}{}, IteratorOps[struct{}, T]{Next: func(Ref[struct{}]) (T, bool) { return next() }, Drop: func(*struct{}, Context) { stop() }})
	return FromIterator(it)
}
func dropElement[T any](value *T, ops ElementOps[T], context Context) {
	if ops.Drop != nil {
		runContext(context, func() { ops.Drop(value, context) })
	}
}
func dropValues[T any](values []T, ops ElementOps[T], context Context) {
	for i := range values {
		func() {
			defer func() {
				if p := recover(); p != nil {
					unwind := context
					unwind.Unwinding = true
					cleanupDuringUnwind(func() { dropValues(values[i+1:], ops, unwind) })
					panic(p)
				}
			}()
			dropElement(&values[i], ops, context)
		}()
	}
}
func closeIterator[T any](it Iterator[T], context Context) {
	p := recover()
	if p != nil && context.ProcessingPanic {
		os.Exit(134)
	}
	if p != nil {
		unwind := context
		unwind.Unwinding = true
		cleanupDuringUnwind(func() { it.CloseWithContext(unwind) })
	} else {
		it.CloseWithContext(context)
	}
	if p != nil {
		panic(p)
	}
}
func capacityOverflow(context Context) { context.Panic("capacity overflow") }
func allocate[T any](capacity uint, config Config) []T {
	if config.ElementSize == 0 {
		config.Context.Panic("rustvec: nonzero source element layout witness is required")
	}
	if capacity > uint(^uint(0)>>1)/config.ElementSize {
		capacityOverflow(config.Context)
	}
	var result []T
	func() {
		defer func() {
			if recover() != nil {
				os.Exit(134)
			}
		}()
		result = make([]T, 0, int(capacity))
	}()
	return result
}
func minCapacity(size uint) uint {
	if size == 1 {
		return 8
	}
	if size <= 1024 {
		return 4
	}
	return 1
}
func reserve[T any](values []T, additional uint, config Config) []T {
	length := uint(len(values))
	if additional > ^uint(0)-length {
		capacityOverflow(config.Context)
	}
	required := length + additional
	if required <= uint(cap(values)) {
		return values
	}
	newCapacity := uint(cap(values)) * 2
	if newCapacity < required {
		newCapacity = required
	}
	if minimum := minCapacity(config.ElementSize); newCapacity < minimum {
		newCapacity = minimum
	}
	result := allocate[T](newCapacity, config)
	result = append(result, values...)
	clear(values)
	return result
}
func Collect[T any](source IntoIterator[T], config Config, ops ElementOps[T]) (values []T) {
	context := config.Context
	defer func() {
		if p := recover(); p != nil {
			if context.ProcessingPanic {
				os.Exit(134)
			}
			unwind := context
			unwind.Unwinding = true
			cleanupDuringUnwind(func() { dropValues(values, ops, unwind) })
			panic(p)
		}
	}()
	it := source.IntoIter(context)
	defer closeIterator(it, context)
	if it.cell.builtin != nil {
		return it.cell.builtin.take(config)
	}
	if it.cell.collect != nil {
		it.cell.live = false
		return it.cell.collect(config, ops)
	}
	if it.cell.trusted {
		hint := it.SizeHint(context)
		if !hint.HasUpper {
			capacityOverflow(context)
		}
		values = allocate[T](hint.Upper, config)
		hint = it.SizeHint(context)
		if !hint.HasUpper {
			capacityOverflow(context)
		}
		if config.DebugAssertions && hint.Lower != hint.Upper {
			context.Panic("assertion failed: trusted iterator exact size hint")
		}
		values = reserve(values, hint.Upper, config)
		it.forEach(context, func(value T) {
			if len(values) == cap(values) {
				context.Panic("rustvec: iterator exceeded the trusted allocation domain")
			}
			values = append(values, value)
		})
		return values
	}
	first, ok := it.Next(context)
	if !ok {
		return nil
	}
	func() {
		owned := true
		defer func() {
			if p := recover(); p != nil {
				if owned {
					unwind := context
					unwind.Unwinding = true
					cleanupDuringUnwind(func() { dropElement(&first, ops, unwind) })
				}
				panic(p)
			}
		}()
		hint := it.SizeHint(context)
		capacity := hint.Lower
		if capacity != ^uint(0) {
			capacity++
		}
		if minimum := minCapacity(config.ElementSize); capacity < minimum {
			capacity = minimum
		}
		values = allocate[T](capacity, config)
		values = append(values, first)
		owned = false
	}()
	for {
		value, ok := it.Next(context)
		if !ok {
			break
		}
		if len(values) == cap(values) {
			func() {
				owned := true
				defer func() {
					if p := recover(); p != nil {
						if owned {
							unwind := context
							unwind.Unwinding = true
							cleanupDuringUnwind(func() { dropElement(&value, ops, unwind) })
						}
						panic(p)
					}
				}()
				hint := it.SizeHint(context)
				if hint.Lower == ^uint(0) {
					capacityOverflow(context)
				}
				values = reserve(values, hint.Lower+1, config)
				owned = false
			}()
		}
		values = append(values, value)
	}
	return values
}

// VecIntoIter is the concrete standard Vec specialization, not a marker that
// custom iterators may use to suppress their callbacks. Its owned slice is
// transferred/compacted according to Vec::IntoIter's allocation reuse rules.
type VecIntoIter[T any] struct {
	values      []T
	front, back int
	ops         ElementOps[T]
	live        bool
}

func NewVecIntoIter[T any](values []T, ops ElementOps[T]) *VecIntoIter[T] {
	return &VecIntoIter[T]{values: values, back: len(values), ops: ops, live: true}
}
func (it *VecIntoIter[T]) Next() (value T, ok bool) {
	if !it.live {
		panic("rustvec: consumed Vec::IntoIter")
	}
	if it.front == it.back {
		return value, false
	}
	value = it.values[it.front]
	var zero T
	it.values[it.front] = zero
	it.front++
	return value, true
}
func (it *VecIntoIter[T]) NextBack() (value T, ok bool) {
	if !it.live {
		panic("rustvec: consumed Vec::IntoIter")
	}
	if it.front == it.back {
		return value, false
	}
	it.back--
	value = it.values[it.back]
	var zero T
	it.values[it.back] = zero
	return value, true
}
func (it *VecIntoIter[T]) CloseWithContext(context Context) {
	if !it.live {
		return
	}
	it.live = false
	dropValues(it.values[it.front:it.back], it.ops, context)
	clear(it.values)
	it.values = nil
}
func (it *VecIntoIter[T]) IntoIterator() IntoIterator[T] {
	cell := &iteratorCell[T]{live: true, builtin: it, trusted: true}
	cell.next = func(Context) (T, bool) { return it.Next() }
	cell.hint = func(Context) Hint { n := uint(it.back - it.front); return Hint{n, n, true} }
	cell.close = it.CloseWithContext
	cell.fold = func(context Context, consume func(T)) {
		defer closeBuiltinFold(it, context)
		for {
			value, ok := it.Next()
			if !ok {
				return
			}
			consume(value)
		}
	}
	cell.each = cell.fold
	return FromIterator(Iterator[T]{cell})
}
func (it *VecIntoIter[T]) take(config Config) []T {
	if !it.live {
		config.Context.Panic("rustvec: consumed Vec::IntoIter")
	}
	remaining := it.back - it.front
	var result []T
	if it.front == 0 || remaining >= cap(it.values)/2 {
		copy(it.values[:remaining], it.values[it.front:it.back])
		clear(it.values[remaining:])
		result = it.values[:remaining]
	} else {
		if remaining > 0 {
			result = allocate[T](max(uint(remaining), minCapacity(config.ElementSize)), config)
			result = append(result, it.values[it.front:it.back]...)
		}
		clear(it.values)
	}
	it.front = it.back
	it.values = nil
	it.live = false
	return result
}
