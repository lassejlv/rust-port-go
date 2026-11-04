// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/vec.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.
package rustcindex

import (
	"reflect"
)

// IndexVec owns a typed vector. Raw supports the complete uint length domain
// for Rust zero-sized elements; references and iterator ownership follow the
// Rust borrowing contract, with explicit Close for deterministic Drop callbacks.
type IndexVec[I, T any] struct {
	Raw    IndexRaw[T]
	Idx    Idx[I]
	Ops    ValueOps[T]
	Config IndexStorageConfig
}

func NewIndexVec[I, T any](idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return NewIndexVecWithConfig(idx, ops, defaultIndexStorageConfig())
}
func NewIndexVecWithConfig[I, T any](idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	return &IndexVec[I, T]{rawFromSlice([]T{}, ops), idx, ops, config}
}
func IndexVecFromRaw[I, T any](raw []T, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromRawWithConfig(raw, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromRawWithConfig[I, T any](raw []T, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	return IndexVecFromStorageWithConfig(rawFromSlice(raw, ops), idx, ops, config)
}

// IndexVecFromStorage is the complete owned from_raw boundary; the []T helper
// above is convenient for addressable Go allocations. Ownership transfers here.
func IndexVecFromStorage[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromStorageWithConfig(raw, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromStorageWithConfig[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	raw.view = false
	return &IndexVec[I, T]{raw, idx, ops, config}
}
func IndexVecWithCapacity[I, T any](capacity uint, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	v := NewIndexVec(idx, ops)
	v.reserveExactRaw(capacity)
	return v
}
func IndexVecWithCapacityAndConfig[I, T any](capacity uint, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	v := NewIndexVecWithConfig(idx, ops, config)
	v.reserveExactRaw(capacity)
	return v
}
func IndexVecFromElemN[I, T any](elem T, n uint, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromElemNWithConfig(elem, n, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromElemNWithConfig[I, T any](elem T, n uint, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) (v *IndexVec[I, T]) {
	v = NewIndexVecWithConfig(idx, ops, config)
	owned := true
	defer func() {
		if p := recover(); p != nil {
			if owned {
				indexDropDuringUnwind(&elem, ops)
			}
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	v.reserveExactRaw(n)
	owned = false
	v.extendWith(n, elem)
	return
}
func IndexVecFromElem[I, T, U any](elem T, universe *IndexSlice[I, U], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromElemNWithConfig(elem, universe.Len(), universe.Idx, ops, universe.Config)
}
func IndexVecFromFnN[I, T any](fn func(I) T, n uint, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromFnNWithConfig(fn, n, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromFnNWithConfig[I, T any](fn func(I) T, n uint, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	return IndexVecFromFnNOwnedWithConfig(IndexFnMut[I, T]{Call: fn}, n, idx, ops, config)
}
func IndexVecFromFnNOwned[I, T any](factory IndexFnMut[I, T], n uint, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromFnNOwnedWithConfig(factory, n, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromFnNOwnedWithConfig[I, T any](factory IndexFnMut[I, T], n uint, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) (v *IndexVec[I, T]) {
	defer func() {
		if p := recover(); p != nil {
			if v != nil {
				v.CloseWithContext(IndexDropContext{true})
			}
			panic(p)
		}
	}()
	owned := true
	defer indexDeferCallback(factory.Close, factory.CloseWithContext, &owned)
	_ = idx.New(n)
	v = NewIndexVecWithConfig(idx, ops, config)
	v.reserveExactRaw(n)
	for i := uint(0); i < n; i++ {
		v.pushRaw(factory.Call(idx.New(i)))
	}
	return
}
func (v *IndexVec[I, T]) Len() uint     { return v.Raw.Len() }
func (v *IndexVec[I, T]) IsEmpty() bool { return v.Raw.IsEmpty() }
func (v *IndexVec[I, T]) AsSlice() *IndexSlice[I, T] {
	raw := v.Raw
	raw.view = true
	return &IndexSlice[I, T]{raw, v.Idx, v.Ops, v.Config}
}
func (v *IndexVec[I, T]) AsMutSlice() *IndexSlice[I, T] { return v.AsSlice() }
func (v *IndexVec[I, T]) Deref() *IndexSlice[I, T]      { return v.AsSlice() }
func (v *IndexVec[I, T]) DerefMut() *IndexSlice[I, T]   { return v.AsMutSlice() }
func (v *IndexVec[I, T]) Borrow() *IndexSlice[I, T]     { return v.AsSlice() }
func (v *IndexVec[I, T]) BorrowMut() *IndexSlice[I, T]  { return v.AsMutSlice() }
func (v *IndexVec[I, T]) At(key I) *T                   { return v.Raw.At(v.Idx.Index(key)) }
func (v *IndexVec[I, T]) AtMut(key I) *T                { return v.At(key) }
func (v *IndexVec[I, T]) Get(key I) (*T, bool)          { return v.Raw.Get(v.Idx.Index(key)) }
func (v *IndexVec[I, T]) GetMut(key I) (*T, bool)       { return v.Get(key) }
func (v *IndexVec[I, T]) GetIndex(key TypedSliceIndexResult) (SliceRef[T], bool) {
	return v.AsSlice().GetIndex(key)
}
func (v *IndexVec[I, T]) GetMutIndex(key TypedSliceIndexResult) (SliceRef[T], bool) {
	return v.GetIndex(key)
}
func (v *IndexVec[I, T]) Index(key TypedSliceIndexResult) SliceRef[T]    { return v.AsSlice().Index(key) }
func (v *IndexVec[I, T]) IndexMut(key TypedSliceIndexResult) SliceRef[T] { return v.Index(key) }
func (v *IndexVec[I, T]) NextIndex() I                                   { return v.Idx.New(v.Len()) }
func (v *IndexVec[I, T]) LastIndex() (I, bool)                           { return v.AsSlice().LastIndex() }
func (v *IndexVec[I, T]) Iter() *RefIterator[T]                          { return v.AsSlice().Iter() }
func (v *IndexVec[I, T]) IterMut() *MutIterator[T]                       { return v.AsSlice().IterMut() }
func (v *IndexVec[I, T]) IterEnumerated() *EnumeratedRefIterator[I, T] {
	return v.AsSlice().IterEnumerated()
}
func (v *IndexVec[I, T]) IterEnumeratedMut() *EnumeratedMutIterator[I, T] {
	return v.AsSlice().IterEnumeratedMut()
}
func (v *IndexVec[I, T]) Indices() *IndexIterator[I] { return v.AsSlice().Indices() }
func (v *IndexVec[I, T]) Swap(a, b I)                { v.AsMutSlice().Swap(a, b) }
func (v *IndexVec[I, T]) CopyWithin(src TypedSliceIndexResult, dest I) {
	v.AsMutSlice().CopyWithin(src, dest)
}
func (v *IndexVec[I, T]) Pick2Mut(a, b I) (*T, *T)        { return v.AsMutSlice().Pick2Mut(a, b) }
func (v *IndexVec[I, T]) Pick3Mut(a, b, c I) (*T, *T, *T) { return v.AsMutSlice().Pick3Mut(a, b, c) }
func (v *IndexVec[I, T]) BinarySearch(value T) (I, bool)  { return v.AsSlice().BinarySearch(value) }
func (v *IndexVec[I, T]) BinarySearchRef(value *T) (I, bool) {
	return v.AsSlice().BinarySearchRef(value)
}

func storageAdd(a, b uint, checked bool) uint {
	if checked && b > ^uint(0)-a {
		panic("rustcindex: usize addition overflow")
	}
	return a + b
}
func (v *IndexVec[I, T]) reserveRaw(additional uint) {
	if additional > ^uint(0)-v.Len() {
		panic("rustcindex: vector capacity overflow")
	}
	needed := v.Len() + additional
	if v.Raw.zeroSized {
		return
	}
	if needed <= uint(cap(v.Raw.data)) {
		return
	}
	capacity := uint(cap(v.Raw.data))
	if capacity <= ^uint(0)/2 {
		capacity *= 2
	} else {
		capacity = ^uint(0)
	}
	if needed > capacity {
		capacity = needed
	}
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
	if capacity < minimum {
		capacity = minimum
	}
	v.reserveExactRaw(capacity)
}
func (v *IndexVec[I, T]) reserveExactRaw(capacity uint) {
	if v.Raw.zeroSized {
		return
	}
	if capacity <= uint(cap(v.Raw.data)) {
		return
	}
	maxInt := uint(^uint(0) >> 1)
	size := v.Ops.RustSize
	if size == 0 {
		size = uint(reflect.TypeFor[T]().Size())
	}
	if capacity > maxInt || size != 0 && capacity > maxInt/size {
		panic("rustcindex: vector capacity exceeds addressable memory")
	}
	data := make([]T, len(v.Raw.data), int(capacity))
	copy(data, v.Raw.data)
	v.Raw.data = data
}
func (v *IndexVec[I, T]) setLen(n uint) {
	if v.Raw.zeroSized {
		v.Raw.length = n
	} else {
		v.Raw.data = v.Raw.data[:n]
	}
}
func (v *IndexVec[I, T]) pushRaw(value T) {
	v.reserveRaw(1)
	if v.Raw.zeroSized {
		if v.Len() == 0 {
			*v.Raw.zero = value
		}
		v.Raw.length++
		return
	}
	v.Raw.data = append(v.Raw.data, value)
}
func (v *IndexVec[I, T]) Push(value T) (key I) {
	owned := true
	defer indexDeferValueOnPanic(&value, v.Ops, &owned)
	key = v.NextIndex()
	v.pushRaw(value)
	owned = false
	return
}
func (v *IndexVec[I, T]) Pop() (T, bool) {
	if v.Len() == 0 {
		var zero T
		return zero, false
	}
	last := v.Len() - 1
	value := *v.Raw.At(last)
	if !v.Raw.zeroSized {
		var zero T
		v.Raw.data[last] = zero
	}
	v.setLen(last)
	return value, true
}
func dropRaw[T any](raw IndexRaw[T], ops ValueOps[T]) { dropRawContext(raw, ops, IndexDropContext{}) }
func dropRawContext[T any](raw IndexRaw[T], ops ValueOps[T], context IndexDropContext) {
	if !ops.hasDrop() {
		return
	}
	var first any
	for i := uint(0); i < raw.Len(); i++ {
		cell := raw.At(i)
		if first == nil {
			first = indexCapturePanic(func() { ops.dropAt(cell, context) })
		} else {
			indexStorageUnwindCleanup(func() { ops.dropAt(cell, IndexDropContext{true}) })
		}
	}
	if first != nil {
		panic(first)
	}
}
func (v *IndexVec[I, T]) Truncate(n uint) { v.truncateContext(n, IndexDropContext{}) }
func (v *IndexVec[I, T]) TruncateWithContext(n uint, context IndexDropContext) {
	if context.Unwinding {
		indexStorageUnwindCleanup(func() { v.truncateContext(n, context) })
		return
	}
	v.truncateContext(n, context)
}
func (v *IndexVec[I, T]) truncateContext(n uint, context IndexDropContext) {
	old := v.Len()
	if n >= old {
		return
	}
	tail := v.Raw.Slice(n, old)
	v.setLen(n)
	defer func() {
		if !tail.zeroSized {
			clear(tail.data)
		}
	}()
	dropRawContext(tail, v.Ops, context)
}
func (v *IndexVec[I, T]) Close() {
	if p := recover(); p != nil {
		v.CloseWithContext(IndexDropContext{true})
		panic(p)
	}
	v.CloseWithContext(IndexDropContext{})
}
func (v *IndexVec[I, T]) CloseWithContext(context IndexDropContext) {
	close := func() {
		defer func() {
			if !v.Raw.zeroSized {
				v.Raw.data = nil
			}
		}()
		v.truncateContext(0, context)
	}
	if context.Unwinding {
		indexStorageUnwindCleanup(close)
	} else {
		close()
	}
}
func (v *IndexVec[I, T]) ShrinkToFit() {
	if v.Raw.zeroSized {
		return
	}
	data := make([]T, len(v.Raw.data))
	copy(data, v.Raw.data)
	v.Raw.data = data
}
func (v *IndexVec[I, T]) EnsureContainsElem(elem I, fill func() T) *T {
	return v.EnsureContainsElemOwned(elem, IndexFactory[T]{Call: fill})
}
func (v *IndexVec[I, T]) EnsureContainsElemOwned(elem I, fill IndexFactory[T]) *T {
	owned := true
	defer indexDeferCallback(fill.Close, fill.CloseWithContext, &owned)
	minimum := storageAdd(v.Idx.Index(elem), 1, v.Config.OverflowChecks)
	if v.Len() < minimum {
		owned = false
		v.resizeWithOwned(minimum, fill)
	}
	return v.At(elem)
}
func (v *IndexVec[I, T]) ResizeToElem(elem I, fill func() T) {
	v.ResizeToElemOwned(elem, IndexFactory[T]{Call: fill})
}
func (v *IndexVec[I, T]) ResizeToElemOwned(elem I, fill IndexFactory[T]) {
	owned := true
	defer indexDeferCallback(fill.Close, fill.CloseWithContext, &owned)
	minimum := storageAdd(v.Idx.Index(elem), 1, v.Config.OverflowChecks)
	owned = false
	v.resizeWithOwned(minimum, fill)
}
func (v *IndexVec[I, T]) resizeWith(newLen uint, fill func() T) {
	v.resizeWithOwned(newLen, IndexFactory[T]{Call: fill})
}
func (v *IndexVec[I, T]) resizeWithOwned(newLen uint, fill IndexFactory[T]) {
	owned := true
	defer indexDeferCallback(fill.Close, fill.CloseWithContext, &owned)
	if newLen <= v.Len() {
		v.Truncate(newLen)
		return
	}
	v.reserveRaw(newLen - v.Len())
	for v.Len() < newLen {
		v.pushRaw(fill.Call())
	}
}

// extendWith mirrors Vec::extend_with: n-1 clones followed by moving the owned
// original value. Its Drop runs even when a clone or allocation panics.
func (v *IndexVec[I, T]) extendWith(n uint, value T) {
	owned := true
	defer indexDeferOwnedValue(&value, v.Ops, &owned)
	v.reserveRaw(n)
	if n == 0 {
		return
	}
	if v.Raw.zeroSized && (v.Ops.TrivialClone || v.Ops.Clone == nil && v.Ops.CloneRef == nil) {
		if v.Len() == 0 {
			*v.Raw.zero = value
		}
		v.Raw.length += n
		owned = false
		return
	}
	for i := uint(0); i < n-1; i++ {
		v.pushRaw(v.Ops.cloneAt(&value))
	}
	v.pushRaw(value)
	owned = false
}
func (v *IndexVec[I, T]) Resize(newLen uint, value T) {
	if newLen > v.Len() {
		v.extendWith(newLen-v.Len(), value)
		return
	}
	owned := true
	defer indexDeferOwnedValue(&value, v.Ops, &owned)
	v.Truncate(newLen)
}
func (v *IndexVec[I, T]) Append(other *IndexVec[I, T]) {
	if v == other {
		panic("rustcindex: append requires distinct exclusive vectors")
	}
	v.reserveRaw(other.Len())
	if v.Raw.zeroSized {
		if v.Len() == 0 {
			*v.Raw.zero = *other.Raw.zero
		}
		v.Raw.length += other.Len()
		other.Raw.length = 0
		return
	}
	v.Raw.data = append(v.Raw.data, other.Raw.data...)
	clear(other.Raw.data)
	other.Raw.data = other.Raw.data[:0]
}
func (v *IndexVec[I, T]) extendRawCopy(raw IndexRaw[T]) {
	v.reserveRaw(raw.Len())
	if v.Raw.zeroSized {
		if v.Len() == 0 {
			*v.Raw.zero = *raw.zero
		}
		v.Raw.length += raw.Len()
		return
	}
	v.Raw.data = append(v.Raw.data, raw.data...)
}
func cloneRawIntoVec[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig) (v *IndexVec[I, T]) {
	v = NewIndexVecWithConfig(idx, ops, config)
	defer func() {
		if p := recover(); p != nil {
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	v.reserveExactRaw(raw.Len())
	if ops.TrivialClone || (raw.zeroSized && ops.Clone == nil && ops.CloneRef == nil) {
		v.extendRawCopy(raw)
		return
	}
	for i := uint(0); i < raw.Len(); i++ {
		v.pushRaw(ops.cloneAt(raw.At(i)))
	}
	return
}
func (v *IndexVec[I, T]) Clone() *IndexVec[I, T] {
	return cloneRawIntoVec(v.Raw, v.Idx, v.Ops, v.Config)
}

// Derive(Clone) supplies no clone_from override for IndexVec. Its inherited
// default clones the whole source, then replaces/drops the old vector. This is
// distinct from IndexSlice::clone_into, which reuses Vec element clone_from.
func (v *IndexVec[I, T]) CloneFrom(source *IndexVec[I, T]) {
	cloned := source.Clone()
	old := *v
	*v = *cloned
	cloned.Raw = rawFromSlice([]T{}, cloned.Ops)
	dropRaw(old.Raw, old.Ops)
}
func (v *IndexVec[I, T]) Equal(other *IndexVec[I, T]) bool    { return v.AsSlice().Equal(other.AsSlice()) }
func (v *IndexVec[I, T]) NotEqual(other *IndexVec[I, T]) bool { return !v.Equal(other) }
func (v *IndexVec[I, T]) Debug() string                       { return v.AsSlice().Debug() }
func (v *IndexVec[I, T]) Hash(hashRaw func(IndexRaw[T]))      { hashRaw(v.Raw) }
func IndexVecFromArray[I, T any](values []T, idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromArrayWithConfig(values, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromArrayWithConfig[I, T any](values []T, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexVec[I, T] {
	v := NewIndexVecWithConfig(idx, ops, config)
	owned := true
	defer func() {
		if p := recover(); p != nil {
			if owned {
				indexStorageUnwindCleanup(func() { dropRawContext(rawFromSlice(values, ops), ops, IndexDropContext{true}) })
			}
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	v.reserveExactRaw(uint(len(values)))
	if v.Raw.zeroSized {
		if len(values) != 0 {
			*v.Raw.zero = values[0]
		}
		v.Raw.length = uint(len(values))
	} else {
		v.Raw.data = append(v.Raw.data, values...)
	}
	clear(values)
	owned = false
	return v
}

type OwnedIterator[T any] struct {
	raw         IndexRaw[T]
	ops         ValueOps[T]
	front, back uint
	closed      bool
}

func (v *IndexVec[I, T]) IntoIter() *OwnedIterator[T] {
	raw := v.Raw
	v.Raw = rawFromSlice([]T{}, v.Ops)
	return &OwnedIterator[T]{raw: raw, ops: v.Ops, back: raw.Len()}
}
func (it *OwnedIterator[T]) Next() (T, bool) {
	if it.front == it.back {
		var zero T
		return zero, false
	}
	i := it.front
	it.front++
	value := *it.raw.At(i)
	if !it.raw.zeroSized {
		var zero T
		it.raw.data[i] = zero
	}
	return value, true
}
func (it *OwnedIterator[T]) NextBack() (T, bool) {
	if it.front == it.back {
		var zero T
		return zero, false
	}
	it.back--
	value := *it.raw.At(it.back)
	if !it.raw.zeroSized {
		var zero T
		it.raw.data[it.back] = zero
	}
	return value, true
}
func (it *OwnedIterator[T]) Len() uint              { return it.back - it.front }
func (it *OwnedIterator[T]) SizeHint() (uint, uint) { n := it.Len(); return n, n }
func (it *OwnedIterator[T]) AsSlice() IndexRaw[T]   { return it.raw.Slice(it.front, it.back) }
func (it *OwnedIterator[T]) Close() {
	if p := recover(); p != nil {
		it.CloseWithContext(IndexDropContext{true})
		panic(p)
	}
	it.CloseWithContext(IndexDropContext{})
}
func (it *OwnedIterator[T]) CloseWithContext(context IndexDropContext) {
	close := func() {
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
		dropRawContext(remaining, it.ops, context)
	}
	if context.Unwinding {
		indexStorageUnwindCleanup(close)
	} else {
		close()
	}
}
func (it *OwnedIterator[T]) Clone() *OwnedIterator[T] {
	v := cloneRawIntoVec(it.AsSlice(), UsizeIdx(), it.ops, defaultIndexStorageConfig())
	return v.IntoIter()
}

type EnumeratedOwnedIterator[I, T any] struct {
	inner       *OwnedIterator[T]
	idx         Idx[I]
	front, back uint
}

func (v *IndexVec[I, T]) IntoIterEnumerated() (result *EnumeratedOwnedIterator[I, T]) {
	defer func() {
		if p := recover(); p != nil {
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	_ = v.Idx.New(v.Len())
	n := v.Len()
	return &EnumeratedOwnedIterator[I, T]{inner: v.IntoIter(), idx: v.Idx, back: n}
}
func (it *EnumeratedOwnedIterator[I, T]) Next() (key I, value T, ok bool) {
	value, ok = it.inner.Next()
	if !ok {
		return
	}
	n := it.front
	it.front++
	owned := true
	defer indexDeferValueOnPanic(&value, it.inner.ops, &owned)
	key = it.idx.New(n)
	owned = false
	return
}
func (it *EnumeratedOwnedIterator[I, T]) NextBack() (key I, value T, ok bool) {
	value, ok = it.inner.NextBack()
	if !ok {
		return
	}
	it.back--
	owned := true
	defer indexDeferValueOnPanic(&value, it.inner.ops, &owned)
	key = it.idx.New(it.front + it.inner.Len())
	owned = false
	return
}
func (it *EnumeratedOwnedIterator[I, T]) Len() uint              { return it.inner.Len() }
func (it *EnumeratedOwnedIterator[I, T]) SizeHint() (uint, uint) { return it.inner.SizeHint() }
func (it *EnumeratedOwnedIterator[I, T]) Close() {
	if p := recover(); p != nil {
		it.inner.CloseWithContext(IndexDropContext{true})
		panic(p)
	}
	it.inner.Close()
}
func (it *EnumeratedOwnedIterator[I, T]) CloseWithContext(context IndexDropContext) {
	it.inner.CloseWithContext(context)
}

type DrainIterator[I, T any] struct {
	owner                   *IndexVec[I, T]
	raw                     IndexRaw[T]
	head, tail, front, back uint
	closed                  bool
}

func (v *IndexVec[I, T]) Drain(bounds IndexRangeBounds) *DrainIterator[I, T] {
	start, end := normalizeRange(bounds, v.Len())
	raw := v.Raw
	v.setLen(start)
	return &DrainIterator[I, T]{owner: v, raw: raw, head: start, tail: end, front: start, back: end}
}
func (it *DrainIterator[I, T]) Next() (T, bool) {
	if it.front == it.back {
		var zero T
		return zero, false
	}
	i := it.front
	it.front++
	value := *it.raw.At(i)
	if !it.raw.zeroSized {
		var zero T
		it.raw.data[i] = zero
	}
	return value, true
}
func (it *DrainIterator[I, T]) NextBack() (T, bool) {
	if it.front == it.back {
		var zero T
		return zero, false
	}
	it.back--
	value := *it.raw.At(it.back)
	if !it.raw.zeroSized {
		var zero T
		it.raw.data[it.back] = zero
	}
	return value, true
}
func (it *DrainIterator[I, T]) Len() uint              { return it.back - it.front }
func (it *DrainIterator[I, T]) SizeHint() (uint, uint) { n := it.Len(); return n, n }
func (it *DrainIterator[I, T]) Close() {
	if p := recover(); p != nil {
		it.CloseWithContext(IndexDropContext{true})
		panic(p)
	}
	it.CloseWithContext(IndexDropContext{})
}
func (it *DrainIterator[I, T]) CloseWithContext(context IndexDropContext) {
	close := func() {
		if it.closed {
			return
		}
		it.closed = true
		remaining := it.raw.Slice(it.front, it.back)
		it.front = it.back
		tailLen := it.raw.Len() - it.tail
		newLen := it.head + tailLen
		if it.raw.zeroSized {
			it.owner.Raw.length = newLen
			dropRawContext(remaining, it.owner.Ops, context)
			return
		}
		defer func() {
			copy(it.raw.data[it.head:newLen], it.raw.data[it.tail:])
			clear(it.raw.data[newLen:])
			it.owner.Raw.data = it.raw.data[:newLen]
		}()
		dropRawContext(remaining, it.owner.Ops, context)
	}
	if context.Unwinding {
		indexStorageUnwindCleanup(close)
	} else {
		close()
	}
}

type EnumeratedDrainIterator[I, T any] struct {
	inner        *DrainIterator[I, T]
	idx          Idx[I]
	begin, count uint
	config       IndexStorageConfig
}

func (v *IndexVec[I, T]) DrainEnumerated(bounds IndexRangeBounds) *EnumeratedDrainIterator[I, T] {
	owned := true
	defer indexDeferCallback(bounds.Close, bounds.CloseWithContext, &owned)
	b := bounds.StartBound()
	begin := uint(0)
	switch b.Kind {
	case IndexIncluded:
		begin = b.Value
	case IndexExcluded:
		if b.Value == ^uint(0) {
			panic("rustcindex: excluded drain start overflow")
		}
		begin = b.Value + 1
	case IndexUnbounded:
	default:
		panic("rustcindex: invalid bound")
	}
	owned = false
	return &EnumeratedDrainIterator[I, T]{inner: v.Drain(bounds), idx: v.Idx, begin: begin, config: v.Config}
}
func (it *EnumeratedDrainIterator[I, T]) Next() (key I, value T, ok bool) {
	value, ok = it.inner.Next()
	if !ok {
		return
	}
	n := it.count
	it.count++
	owned := true
	defer indexDeferValueOnPanic(&value, it.inner.owner.Ops, &owned)
	key = it.idx.New(storageAdd(it.begin, n, it.config.OverflowChecks))
	owned = false
	return
}
func (it *EnumeratedDrainIterator[I, T]) Len() uint              { return it.inner.Len() }
func (it *EnumeratedDrainIterator[I, T]) SizeHint() (uint, uint) { return it.inner.SizeHint() }
func (it *EnumeratedDrainIterator[I, T]) Close() {
	if p := recover(); p != nil {
		it.inner.CloseWithContext(IndexDropContext{true})
		panic(p)
	}
	it.inner.Close()
}
func (it *EnumeratedDrainIterator[I, T]) CloseWithContext(context IndexDropContext) {
	it.inner.CloseWithContext(context)
}

// IndexValueIterator represents an owned source iterator. Exact/trusted hints
// select the same eager-reserve behavior as Vec's trusted-length specialization.
// Close supplies source iterator Drop; callers constructing wrappers retain the
// Rust ownership contract and must not reuse a moved iterator.
type IndexValueIterator[T any] struct {
	Next             func() (T, bool)
	SizeHint         func() (uint, uint, bool)
	Close            func()
	CloseWithContext func(IndexDropContext)
	TrustedLength    bool
	// Owned identifies the actual Vec::IntoIter specialization, allowing moves
	// and capacity reuse without manufacturing calls to next or size_hint.
	Owned *OwnedIterator[T]
	// Trusted-length extension calls Iterator::for_each. Its default dispatches
	// to fold with a unit accumulator; both independently overridable slots are
	// retained for the unit instantiation used by this storage implementation.
	ForEachOverride  func(func(T))
	FoldUnitOverride func(func(T))
}
type IndexIntoIterator[T any] struct{ IntoIter func() IndexValueIterator[T] }

func iteratorHint[T any](it IndexValueIterator[T]) (uint, uint, bool) {
	if it.SizeHint != nil {
		return it.SizeHint()
	}
	return 0, 0, false
}
func iteratorForEach[T any](it IndexValueIterator[T], consume func(T)) {
	if it.ForEachOverride != nil {
		it.ForEachOverride(consume)
		return
	}
	if it.FoldUnitOverride != nil {
		it.FoldUnitOverride(consume)
		return
	}
	for {
		value, ok := it.Next()
		if !ok {
			return
		}
		consume(value)
	}
}
func (it *OwnedIterator[T]) IntoIterator() IndexIntoIterator[T] {
	return IndexIntoIterator[T]{IntoIter: func() IndexValueIterator[T] {
		return IndexValueIterator[T]{Next: it.Next, SizeHint: func() (uint, uint, bool) { n := it.Len(); return n, n, true }, Close: it.Close, CloseWithContext: it.CloseWithContext, TrustedLength: true, Owned: it}
	}}
}
func (it *OwnedIterator[T]) forgetRemaining() {
	it.front = it.back
	it.closed = true
	if !it.raw.zeroSized {
		clear(it.raw.data)
		it.raw.data = nil
	}
}
func (it *OwnedIterator[T]) takeRaw() IndexRaw[T] {
	raw := it.raw
	n := it.Len()
	if raw.zeroSized {
		raw.length = n
	} else {
		copy(raw.data[:n], raw.data[it.front:it.back])
		clear(raw.data[n:])
		raw.data = raw.data[:n]
	}
	raw.view = false
	it.front = it.back
	it.closed = true
	it.raw.data = nil
	return raw
}

func (v *IndexVec[I, T]) Extend(source IndexIntoIterator[T]) { v.extendIterator(source.IntoIter()) }
func (v *IndexVec[I, T]) extendIterator(it IndexValueIterator[T]) {
	defer indexDeferValueIterator(it)
	if it.Owned != nil {
		v.extendRawCopy(it.Owned.AsSlice())
		it.Owned.forgetRemaining()
		return
	}
	if it.TrustedLength {
		lower, upper, some := iteratorHint(it)
		if !some {
			panic("rustcindex: trusted iterator length overflow")
		}
		if v.Config.DebugAssertions && lower != upper {
			panic("rustcindex: trusted iterator hint is not exact")
		}
		v.reserveRaw(upper)
		iteratorForEach(it, func(value T) { v.pushRaw(value) })
		return
	}
	for {
		value, ok := it.Next()
		if !ok {
			break
		}
		if v.Len() == v.Raw.Cap() {
			owned := true
			func() {
				defer func() {
					if p := recover(); p != nil {
						if owned {
							indexDropDuringUnwind(&value, v.Ops)
						}
						panic(p)
					}
				}()
				lower, _, _ := iteratorHint(it)
				if lower == ^uint(0) {
					panic("rustcindex: vector capacity overflow")
				}
				v.reserveRaw(lower + 1)
				owned = false
			}()
		}
		v.pushRaw(value)
	}
}
func IndexVecFromIterator[I, T any](source IndexIntoIterator[T], idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return IndexVecFromIteratorWithConfig(source, idx, ops, defaultIndexStorageConfig())
}
func IndexVecFromIteratorWithConfig[I, T any](source IndexIntoIterator[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig) (v *IndexVec[I, T]) {
	v = NewIndexVecWithConfig(idx, ops, config)
	defer func() {
		if p := recover(); p != nil {
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	it := source.IntoIter()
	defer indexDeferValueIterator(it)
	if it.Owned != nil {
		owned := it.Owned
		if owned.front == 0 || owned.Len() >= owned.raw.Cap()/2 {
			v.Raw = owned.takeRaw()
			return
		}
		v.extendRawCopy(owned.AsSlice())
		owned.forgetRemaining()
		return
	}
	if it.TrustedLength {
		_, upper, some := iteratorHint(it)
		if !some {
			panic("rustcindex: trusted iterator length overflow")
		}
		v.reserveExactRaw(upper)
		it.Close = nil
		it.CloseWithContext = nil
		v.extendIterator(it)
		return
	}
	first, ok := it.Next()
	if !ok {
		return
	}
	owned := true
	func() {
		defer func() {
			if p := recover(); p != nil {
				if owned {
					indexDropDuringUnwind(&first, ops)
				}
				panic(p)
			}
		}()
		lower, _, _ := iteratorHint(it)
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
		if capacity < minimum {
			capacity = minimum
		}
		v.reserveExactRaw(capacity)
		v.pushRaw(first)
		owned = false
	}()
	it.Close = nil
	it.CloseWithContext = nil
	v.extendIterator(it)
	return
}

type IndexSliceMapView[I, T any] struct {
	slice        *IndexSlice[I, T]
	formatKey    func(I) string
	formatKeyRef func(*I, *IndexDebugFormatter) error
}

func (v *IndexVec[I, T]) DebugMapView(formatKey ...func(I) string) IndexSliceMapView[I, T] {
	var format func(I) string
	if len(formatKey) > 0 {
		format = formatKey[0]
	}
	return IndexSliceMapView[I, T]{slice: v.AsSlice(), formatKey: format}
}
func (view IndexSliceMapView[I, T]) Debug() string {
	return indexFormatString(view.FormatDebug, IndexFormatContext{})
}

type Option[T any] struct {
	Value   T
	Present bool
}

func Some[T any](value T) Option[T] { return Option[T]{Value: value, Present: true} }
func None[T any]() Option[T]        { return Option[T]{} }
func OptionValueOps[T any](inner ValueOps[T]) ValueOps[Option[T]] {
	ops := ValueOps[Option[T]]{Copy: inner.Copy, TrivialClone: inner.TrivialClone}
	if reflect.TypeFor[T]().Kind() == reflect.Bool && (inner.RustSize == 0 || inner.RustSize == 1) {
		ops.RustSize = 1
	}
	ops.CloneRef = func(v *Option[T]) Option[T] {
		if v.Present {
			return Some(inner.cloneAt(&v.Value))
		}
		return None[T]()
	}
	ops.Clone = func(v Option[T]) Option[T] { return ops.CloneRef(&v) }
	ops.CloneFromRef = func(dst, src *Option[T]) {
		if dst.Present && src.Present {
			inner.cloneFromAt(&dst.Value, &src.Value)
			return
		}
		if src.Present {
			value := inner.cloneAt(&src.Value)
			*dst = Some(value)
			return
		}
		if dst.Present {
			defer func() { *dst = None[T]() }()
			inner.dropAt(&dst.Value, IndexDropContext{})
		}
	}
	ops.CloneFrom = func(dst *Option[T], src Option[T]) { ops.CloneFromRef(dst, &src) }
	ops.EqualRef = func(a, b *Option[T]) bool {
		if a.Present != b.Present {
			return false
		}
		return !a.Present || inner.equalAt(&a.Value, &b.Value)
	}
	ops.Equal = func(a, b Option[T]) bool { return ops.EqualRef(&a, &b) }
	ops.NotEqualRef = func(a, b *Option[T]) bool {
		// Option supplies eq but inherits the default ne, so slice equality's
		// Option::ne dispatch reaches the inner T::eq rather than T::ne.
		return !ops.EqualRef(a, b)
	}
	ops.NotEqual = func(a, b Option[T]) bool { return ops.NotEqualRef(&a, &b) }
	ops.FormatDebug = func(v *Option[T], f *IndexDebugFormatter) error { return formatIndexOption(v, inner, f) }
	ops.DebugRef = func(v *Option[T]) string {
		if v.Present {
			return "Some(" + inner.debugAt(&v.Value) + ")"
		}
		return "None"
	}
	ops.Debug = func(v Option[T]) string { return ops.DebugRef(&v) }
	ops.DropWithContext = func(v *Option[T], context IndexDropContext) {
		if v.Present {
			inner.dropAt(&v.Value, context)
		}
		*v = None[T]()
	}
	ops.Drop = func(v *Option[T]) { ops.DropWithContext(v, IndexDropContext{}) }
	return ops
}

// OptionLayout describes the whole source Option<T>, including niches that
// cannot be inferred from a Go representation (for example nonnull pointers or
// macro-generated bounded indices). It does not assert a Rust ABI for Go.
type OptionLayout struct {
	RustSize      uint
	RustZeroSized bool
}

func OptionValueOpsWithLayout[T any](inner ValueOps[T], layout OptionLayout) ValueOps[Option[T]] {
	ops := OptionValueOps(inner)
	ops.RustSize = layout.RustSize
	ops.RustZeroSized = layout.RustZeroSized
	return ops
}

// OptionIndexVec exposes the source specialized IndexVec<I,Option<T>> methods
// through a Go wrapper, retaining the same underlying IndexVec storage.
type OptionIndexVec[I, T any] struct {
	*IndexVec[I, Option[T]]
	ElementOps ValueOps[T]
}

func NewOptionIndexVec[I, T any](idx Idx[I], ops ValueOps[T]) *OptionIndexVec[I, T] {
	return &OptionIndexVec[I, T]{NewIndexVec(idx, OptionValueOps(ops)), ops}
}
func NewOptionIndexVecWithLayout[I, T any](idx Idx[I], ops ValueOps[T], layout OptionLayout) *OptionIndexVec[I, T] {
	return &OptionIndexVec[I, T]{NewIndexVec(idx, OptionValueOpsWithLayout(ops, layout)), ops}
}
func AsOptionIndexVec[I, T any](v *IndexVec[I, Option[T]], ops ValueOps[T]) *OptionIndexVec[I, T] {
	return &OptionIndexVec[I, T]{v, ops}
}
func (v *OptionIndexVec[I, T]) Insert(key I, value T) Option[T] {
	owned := true
	defer indexDeferValueOnPanic(&value, v.ElementOps, &owned)
	place := v.EnsureContainsElem(key, func() Option[T] { return None[T]() })
	old := *place
	*place = Some(value)
	owned = false
	return old
}
func (v *OptionIndexVec[I, T]) GetOrInsertWith(key I, value func() T) *T {
	return v.GetOrInsertWithOwned(key, IndexOnceFactory[T]{Call: value})
}
func (v *OptionIndexVec[I, T]) GetOrInsertWithOwned(key I, value IndexOnceFactory[T]) *T {
	owned := true
	defer indexDeferCallback(value.Close, value.CloseWithContext, &owned)
	place := v.EnsureContainsElem(key, func() Option[T] { return None[T]() })
	if !place.Present {
		owned = false
		*place = Some(indexCallOnce(value))
	}
	return &place.Value
}
func (v *OptionIndexVec[I, T]) Remove(key I) Option[T] {
	place, ok := v.GetMut(key)
	if !ok {
		return None[T]()
	}
	old := *place
	*place = None[T]()
	return old
}
func (v *OptionIndexVec[I, T]) Contains(key I) bool {
	place, ok := v.Get(key)
	return ok && place.Present
}

type IndexSliceMapViewCompact[I, T any] struct {
	slice        *IndexSlice[I, Option[T]]
	formatKey    func(I) string
	formatKeyRef func(*I, *IndexDebugFormatter) error
	ops          ValueOps[T]
}

func (v *OptionIndexVec[I, T]) DebugMapViewCompact(formatKey ...func(I) string) IndexSliceMapViewCompact[I, T] {
	var format func(I) string
	if len(formatKey) > 0 {
		format = formatKey[0]
	}
	return IndexSliceMapViewCompact[I, T]{slice: v.AsSlice(), formatKey: format, ops: v.ElementOps}
}
func (view IndexSliceMapViewCompact[I, T]) Debug() string {
	return indexFormatString(view.FormatDebug, IndexFormatContext{})
}
