// SPDX-License-Identifier: MIT OR Apache-2.0
// Iterator trait adapters for the pinned slice.rs/vec.rs return types. Source
// shortcuts and Map/Enumerate fold dispatch follow core iter and alloc Vec at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package rustcindex

// Nth/NthBack borrow the iterator; Count/Last consume it, as in Rust. Callers
// must not reuse consumed iterators, including after catching a consuming panic.
// The raw slice shortcuts remain constant-time across the full uint ZST domain.
func (it *RefIterator[T]) Nth(n uint) (*T, bool) {
	if n >= it.Len() {
		it.front = it.back
		return nil, false
	}
	it.front += n
	return it.Next()
}
func (it *RefIterator[T]) NthBack(n uint) (*T, bool) {
	if n >= it.Len() {
		it.back = it.front
		return nil, false
	}
	it.back -= n
	return it.NextBack()
}
func (it *RefIterator[T]) Count() uint { n := it.Len(); it.front = it.back; return n }
func (it *RefIterator[T]) Last() (*T, bool) {
	value, ok := it.NextBack()
	it.front = it.back
	return value, ok
}
func (it *MutIterator[T]) Nth(n uint) (*T, bool)     { return it.inner.Nth(n) }
func (it *MutIterator[T]) NthBack(n uint) (*T, bool) { return it.inner.NthBack(n) }
func (it *MutIterator[T]) Count() uint               { return it.inner.Count() }
func (it *MutIterator[T]) Last() (*T, bool)          { return it.inner.Last() }

// Map does not override nth/count/last: skipping and consuming invoke every
// mapper, including I::new. They cannot use raw iterator length shortcuts.
func (it *IndexIterator[I]) Nth(n uint) (I, bool) {
	for ; n != 0; n-- {
		if _, ok := it.Next(); !ok {
			var zero I
			return zero, false
		}
	}
	return it.Next()
}
func (it *IndexIterator[I]) NthBack(n uint) (I, bool) {
	for ; n != 0; n-- {
		if _, ok := it.NextBack(); !ok {
			var zero I
			return zero, false
		}
	}
	return it.NextBack()
}
func (it *IndexIterator[I]) Count() uint {
	var count uint
	for {
		if _, ok := it.Next(); !ok {
			return count
		}
		count++
	}
}
func (it *IndexIterator[I]) Last() (value I, ok bool) {
	for {
		next, more := it.Next()
		if !more {
			return
		}
		value, ok = next, true
	}
}

func (it *EnumeratedRefIterator[I, T]) Nth(n uint) (I, *T, bool) {
	for ; n != 0; n-- {
		if it.front == it.back {
			var zero I
			return zero, nil, false
		}
		it.front++ // underlying slice advances before the mapper
		_ = it.idx.New(it.count)
		// Enumerate::try_fold increments only after its consumer returns. In
		// particular, an I::new panic leaves count behind the underlying slice.
		it.count++
	}
	return it.Next()
}
func (it *EnumeratedRefIterator[I, T]) NthBack(n uint) (I, *T, bool) {
	for ; n != 0; n-- {
		if _, _, ok := it.NextBack(); !ok {
			var zero I
			return zero, nil, false
		}
	}
	return it.NextBack()
}
func (it *EnumeratedRefIterator[I, T]) Count() uint {
	var count uint
	for {
		if _, _, ok := it.Next(); !ok {
			return count
		}
		count++
	}
}
func (it *EnumeratedRefIterator[I, T]) Last() (key I, value *T, ok bool) {
	for {
		nextKey, next, more := it.Next()
		if !more {
			return
		}
		key, value, ok = nextKey, next, true
	}
}
func (it *EnumeratedMutIterator[I, T]) Nth(n uint) (I, *T, bool)     { return it.inner.Nth(n) }
func (it *EnumeratedMutIterator[I, T]) NthBack(n uint) (I, *T, bool) { return it.inner.NthBack(n) }
func (it *EnumeratedMutIterator[I, T]) Count() uint                  { return it.inner.Count() }
func (it *EnumeratedMutIterator[I, T]) Last() (I, *T, bool)          { return it.inner.Last() }

// Vec::IntoIter::advance_by advances past the whole skipped range before its
// slice drop glue runs. Backward skipping also drops that range in forward order.
func (it *OwnedIterator[T]) advance(n uint, back bool) uint {
	step := n
	if step > it.Len() {
		step = it.Len()
	}
	var skipped IndexRaw[T]
	if back {
		old := it.back
		it.back -= step
		skipped = it.raw.Slice(it.back, old)
	} else {
		old := it.front
		it.front += step
		skipped = it.raw.Slice(old, it.front)
	}
	defer func() {
		if !skipped.zeroSized {
			clear(skipped.data)
		}
	}()
	dropRaw(skipped, it.ops)
	return n - step
}
func (it *OwnedIterator[T]) Nth(n uint) (T, bool) {
	if it.advance(n, false) != 0 {
		var zero T
		return zero, false
	}
	return it.Next()
}
func (it *OwnedIterator[T]) NthBack(n uint) (T, bool) {
	if it.advance(n, true) != 0 {
		var zero T
		return zero, false
	}
	return it.NextBack()
}
func (it *OwnedIterator[T]) Count() uint     { defer it.Close(); return it.Len() }
func (it *OwnedIterator[T]) Last() (T, bool) { defer it.Close(); return it.NextBack() }

func (it *EnumeratedOwnedIterator[I, T]) Nth(n uint) (I, T, bool) {
	for ; n != 0; n-- {
		value, ok := it.inner.Next()
		if !ok {
			var zero I
			var empty T
			return zero, empty, false
		}
		owned := true
		func() {
			defer indexDeferValueOnPanic(&value, it.inner.ops, &owned)
			_ = it.idx.New(it.front)
			owned = false
			it.inner.ops.dropAt(&value, IndexDropContext{})
		}()
		it.front++
	}
	return it.Next()
}
func (it *EnumeratedOwnedIterator[I, T]) NthBack(n uint) (I, T, bool) {
	for ; n != 0; n-- {
		_, value, ok := it.NextBack()
		if !ok {
			var zero I
			var empty T
			return zero, empty, false
		}
		it.inner.ops.dropAt(&value, IndexDropContext{})
	}
	return it.NextBack()
}
func (it *EnumeratedOwnedIterator[I, T]) Count() uint {
	defer it.Close()
	var count uint
	for {
		_, value, ok := it.Next()
		if !ok {
			return count
		}
		it.inner.ops.dropAt(&value, IndexDropContext{})
		count++
	}
}
func (it *EnumeratedOwnedIterator[I, T]) Last() (key I, value T, ok bool) {
	defer it.Close()
	owned := false
	defer indexDeferValueOnPanic(&value, it.inner.ops, &owned)
	for {
		nextKey, next, more := it.Next()
		if !more {
			owned = false
			return
		}
		if owned {
			owned = false
			it.inner.ops.dropAt(&value, IndexDropContext{})
		}
		key, value, ok, owned = nextKey, next, true, true
	}
}

// Drain has no source shortcuts. Its Iterator defaults discard each value
// separately, so a destructor panic leaves the later skipped values in drain.
func (it *DrainIterator[I, T]) Nth(n uint) (T, bool) {
	if !it.owner.Ops.hasDrop() {
		step := n
		if step > it.Len() {
			step = it.Len()
		}
		it.front += step
		if step < n {
			var zero T
			return zero, false
		}
		return it.Next()
	}
	for ; n != 0; n-- {
		value, ok := it.Next()
		if !ok {
			var zero T
			return zero, false
		}
		it.owner.Ops.dropAt(&value, IndexDropContext{})
	}
	return it.Next()
}
func (it *DrainIterator[I, T]) Count() uint {
	defer it.Close()
	if !it.owner.Ops.hasDrop() {
		n := it.Len()
		it.front = it.back
		return n
	}
	var count uint
	for {
		value, ok := it.Next()
		if !ok {
			return count
		}
		it.owner.Ops.dropAt(&value, IndexDropContext{})
		count++
	}
}
func (it *DrainIterator[I, T]) Last() (value T, ok bool) {
	defer it.Close()
	owned := false
	defer indexDeferValueOnPanic(&value, it.owner.Ops, &owned)
	if !it.owner.Ops.hasDrop() {
		value, ok = it.NextBack()
		it.front = it.back
		return
	}
	for {
		next, more := it.Next()
		if !more {
			owned = false
			return
		}
		if owned {
			owned = false
			it.owner.Ops.dropAt(&value, IndexDropContext{})
		}
		value, ok, owned = next, true, true
	}
}
func (it *EnumeratedDrainIterator[I, T]) Nth(n uint) (I, T, bool) {
	for ; n != 0; n-- {
		value, ok := it.inner.Next()
		if !ok {
			var zero I
			var empty T
			return zero, empty, false
		}
		owned := true
		func() {
			defer indexDeferValueOnPanic(&value, it.inner.owner.Ops, &owned)
			_ = it.idx.New(storageAdd(it.begin, it.count, it.config.OverflowChecks))
			owned = false
			it.inner.owner.Ops.dropAt(&value, IndexDropContext{})
		}()
		it.count++
	}
	return it.Next()
}
func (it *EnumeratedDrainIterator[I, T]) Count() uint {
	defer it.Close()
	var count uint
	for {
		_, value, ok := it.Next()
		if !ok {
			return count
		}
		it.inner.owner.Ops.dropAt(&value, IndexDropContext{})
		count++
	}
}
func (it *EnumeratedDrainIterator[I, T]) Last() (key I, value T, ok bool) {
	defer it.Close()
	owned := false
	defer indexDeferValueOnPanic(&value, it.inner.owner.Ops, &owned)
	for {
		nextKey, next, more := it.Next()
		if !more {
			owned = false
			return
		}
		if owned {
			owned = false
			it.inner.owner.Ops.dropAt(&value, IndexDropContext{})
		}
		key, value, ok, owned = nextKey, next, true, true
	}
}
