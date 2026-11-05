// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordCollection is the source module's private marker, exposed only as a Go
// constraint. The exact-owner guard also rejects distinct Go embedding wrappers
// which would otherwise inherit the private method by promotion.
type UnordCollection interface{ unordCollection() }

func (*UnordMap[K, V]) unordCollection() {}
func (*UnordSet[T]) unordCollection()    {}
func (*UnordBag[T]) unordCollection()    {}

// ExtendUnordItems invokes the actual selected C::extend<I> implementation on
// a borrowed collection and the owned concrete iterator. The static operation
// includes its complete source body/local cleanup and is not an owned closure.
func ExtendUnordItems[S, T any, C UnordCollection](collection C, items *UnordItems[T], selected func(C, *rustvec.Owned[S], rustfmt.Context), contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	unordRequireExactCollection(collection, c)
	if selected == nil {
		c.Panic("datastructures: missing selected Extend implementation")
	}
	state := unordTakeConcrete[S](items, c)
	rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) struct{} {
		selected(collection, state, c)
		return struct{}{}
	})
}

func unordOpaqueItems[S, T any](state *rustvec.Owned[S], ops UnordIteratorOps[S, T]) *UnordItems[T] {
	items := NewUnordItems(state, ops)
	items.clonePublic = false
	return items
}

// General transfers a legacy concrete read-items handle into the general
// unordered API. References remain borrowed from their original collection and
// are invalidated by incompatible mutation; no ordered iterator is exposed.
func (items *UnordMapItems[K, V]) General(contexts ...rustfmt.Context) *UnordItems[rusthashmap.BorrowedPair[K, V]] {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordMap items")
	}
	items.live = false
	state := rustvec.Own(items.iterator)
	return unordOpaqueItems(state, unordMapReadOps[K, V]())
}

func unordMapReadOps[K, V any]() UnordIteratorOps[*rusthashmap.Iter[K, V], rusthashmap.BorrowedPair[K, V]] {
	return UnordIteratorOps[*rusthashmap.Iter[K, V], rusthashmap.BorrowedPair[K, V]]{
		Next: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) *rustvec.Owned[rusthashmap.BorrowedPair[K, V]] {
			value, ok := (*ref.Value).Next(ref.Context)
			if !ok {
				return nil
			}
			return rustvec.Own(value)
		},
		SizeHint: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rusthashmap.Iter[K, V]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Clone: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) *rustvec.Owned[*rusthashmap.Iter[K, V]] {
			return rustvec.Own((*ref.Value).Clone())
		},
		CloneItem: true,
		// std HashMap::Iter uses Iterator's default try_fold (its private
		// hashbrown/raw iterator is not the receiver of this trait call).
		Boolean: &UnordBooleanOps[*rusthashmap.Iter[K, V], rusthashmap.BorrowedPair[K, V]]{DefaultTryFold: true},
	}
}

func (collection *UnordMap[K, V]) Keys(contexts ...rustfmt.Context) *UnordItems[*K] {
	c := unordContext(contexts)
	collection.check(c)
	return unordOpaqueItems(rustvec.Own(collection.inner.Iter(c)), UnordIteratorOps[*rusthashmap.Iter[K, V], *K]{
		Next: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) *rustvec.Owned[*K] {
			pair, ok := (*ref.Value).Next(ref.Context)
			if !ok {
				return nil
			}
			return rustvec.Own(pair.Key)
		},
		SizeHint: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rusthashmap.Iter[K, V]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Clone: func(ref rustvec.Ref[*rusthashmap.Iter[K, V]]) *rustvec.Owned[*rusthashmap.Iter[K, V]] {
			return rustvec.Own((*ref.Value).Clone())
		},
		CloneItem: true,
		Boolean:   &UnordBooleanOps[*rusthashmap.Iter[K, V], *K]{DefaultTryFold: true},
	})
}

func unordNextBoolean[T any](next func() *rustvec.Owned[T], f UnordFunction[T, bool], any bool, c rustfmt.Context) bool {
	for {
		value := next()
		if value == nil {
			return !any
		}
		result := f.CallMut(value, c).Take()
		if result == any {
			return any
		}
	}
}

func (items *UnordBagItems[T]) General(contexts ...rustfmt.Context) *UnordItems[*T] {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordBag items")
	}
	items.live = false
	next := func(ref rustvec.Ref[*rustcindex.RefIterator[T]]) *rustvec.Owned[*T] {
		value, ok := (*ref.Value).Next()
		if !ok {
			return nil
		}
		return rustvec.Own(value)
	}
	return unordOpaqueItems(rustvec.Own(items.inner), UnordIteratorOps[*rustcindex.RefIterator[T], *T]{
		Next: next,
		SizeHint: func(ref rustvec.Ref[*rustcindex.RefIterator[T]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rustcindex.RefIterator[T]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Clone: func(ref rustvec.Ref[*rustcindex.RefIterator[T]]) *rustvec.Owned[*rustcindex.RefIterator[T]] {
			return rustvec.Own((*ref.Value).Clone())
		},
		CloneItem: true,
		Boolean: &UnordBooleanOps[*rustcindex.RefIterator[T], *T]{
			All: func(ref rustvec.Ref[*rustcindex.RefIterator[T]], f UnordFunction[*T, bool]) bool {
				return unordNextBoolean(func() *rustvec.Owned[*T] { return next(ref) }, f, false, ref.Context)
			},
			Any: func(ref rustvec.Ref[*rustcindex.RefIterator[T]], f UnordFunction[*T, bool]) bool {
				return unordNextBoolean(func() *rustvec.Owned[*T] { return next(ref) }, f, true, ref.Context)
			},
		},
	})
}

func (items *UnordBagOwnedItems[T]) General(contexts ...rustfmt.Context) *UnordItems[T] {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordBag owned items")
	}
	items.live = false
	drop := items.drop
	next := func(ref rustvec.Ref[*rustcindex.OwnedIterator[T]]) *rustvec.Owned[T] {
		value, ok := (*ref.Value).Next()
		if !ok {
			return nil
		}
		return rustvec.NewOwned(value, rustvec.ValueOps[T]{Drop: drop})
	}
	stateOps := rustvec.ValueOps[*rustcindex.OwnedIterator[T]]{Drop: func(iterator **rustcindex.OwnedIterator[T], c rustfmt.Context) {
		(*iterator).CloseWithRuntimeContext(c)
	}}
	return unordOpaqueItems(rustvec.NewOwned(items.inner, stateOps), UnordIteratorOps[*rustcindex.OwnedIterator[T], T]{
		Next: next,
		SizeHint: func(ref rustvec.Ref[*rustcindex.OwnedIterator[T]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rustcindex.OwnedIterator[T]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Boolean: &UnordBooleanOps[*rustcindex.OwnedIterator[T], T]{TryFold: func(ref rustvec.Ref[*rustcindex.OwnedIterator[T]], consume UnordTryFoldFunction[T]) bool {
			for {
				value := next(ref)
				if value == nil {
					return true
				}
				if !consume.CallMut(value, ref.Context) {
					return false
				}
			}
		}},
	})
}

func (items *UnordSetItems[T]) General(contexts ...rustfmt.Context) *UnordItems[*T] {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordSet items")
	}
	items.live = false
	return unordOpaqueItems(rustvec.Own(items.iterator), UnordIteratorOps[*rusthashmap.SetIter[T], *T]{
		Next: func(ref rustvec.Ref[*rusthashmap.SetIter[T]]) *rustvec.Owned[*T] {
			value, ok := (*ref.Value).Next(ref.Context)
			if !ok {
				return nil
			}
			return rustvec.Own(value)
		},
		SizeHint: func(ref rustvec.Ref[*rusthashmap.SetIter[T]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rusthashmap.SetIter[T]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Clone: func(ref rustvec.Ref[*rusthashmap.SetIter[T]]) *rustvec.Owned[*rusthashmap.SetIter[T]] {
			return rustvec.Own((*ref.Value).Clone())
		},
		CloneItem: true,
		Boolean:   &UnordBooleanOps[*rusthashmap.SetIter[T], *T]{DefaultTryFold: true},
	})
}

func dropUnordPair[K, V any](pair *rusthashmap.Pair[K, V], config UnordMapConfig[K, V], c rustfmt.Context) {
	dropValue := func(c rustfmt.Context) {
		if config.Values.Drop != nil {
			config.Values.Drop(&pair.Value, c)
		}
	}
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			bagItemsUnwind(unwind, func() { dropValue(unwind) })
			c.ResumeUnwind(p)
		}
		dropValue(c)
	}()
	if config.Keys.Drop != nil {
		config.Keys.Drop(&pair.Key, c)
	}
}

// IntoItems transfers the actual owning hash-table iterator, preserving its
// remaining element destruction and the fixed FxBuildHasher drop boundary.
func (collection *UnordMap[K, V]) IntoItems(contexts ...rustfmt.Context) *UnordItems[rusthashmap.Pair[K, V]] {
	c := unordContext(contexts)
	collection.check(c)
	collection.live = false
	iterator := collection.inner.IntoIter(c)
	config := collection.config
	stateOps := rustvec.ValueOps[*rusthashmap.IntoIter[K, V]]{Drop: func(iterator **rusthashmap.IntoIter[K, V], c rustfmt.Context) { (*iterator).Close(c) }}
	return unordOpaqueItems(rustvec.NewOwned(iterator, stateOps), UnordIteratorOps[*rusthashmap.IntoIter[K, V], rusthashmap.Pair[K, V]]{
		Next: func(ref rustvec.Ref[*rusthashmap.IntoIter[K, V]]) *rustvec.Owned[rusthashmap.Pair[K, V]] {
			pair, ok := (*ref.Value).Next(ref.Context)
			if !ok {
				return nil
			}
			return rustvec.NewOwned(pair, rustvec.ValueOps[rusthashmap.Pair[K, V]]{Drop: func(pair *rusthashmap.Pair[K, V], c rustfmt.Context) { dropUnordPair(pair, config, c) }})
		},
		SizeHint: func(ref rustvec.Ref[*rusthashmap.IntoIter[K, V]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rusthashmap.IntoIter[K, V]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Boolean: &UnordBooleanOps[*rusthashmap.IntoIter[K, V], rusthashmap.Pair[K, V]]{DefaultTryFold: true},
	})
}

func (collection *UnordSet[T]) IntoItems(contexts ...rustfmt.Context) *UnordItems[T] {
	c := unordContext(contexts)
	collection.check(c)
	collection.live = false
	iterator := collection.inner.IntoIter(c)
	config := collection.config
	stateOps := rustvec.ValueOps[*rusthashmap.SetIntoIter[T]]{Drop: func(iterator **rusthashmap.SetIntoIter[T], c rustfmt.Context) { (*iterator).Close(c) }}
	return unordOpaqueItems(rustvec.NewOwned(iterator, stateOps), UnordIteratorOps[*rusthashmap.SetIntoIter[T], T]{
		Next: func(ref rustvec.Ref[*rusthashmap.SetIntoIter[T]]) *rustvec.Owned[T] {
			value, ok := (*ref.Value).Next(ref.Context)
			if !ok {
				return nil
			}
			return rustvec.NewOwned(value, rustvec.ValueOps[T]{Drop: config.Values.Drop})
		},
		SizeHint: func(ref rustvec.Ref[*rusthashmap.SetIntoIter[T]]) rustvec.Hint {
			n := (*ref.Value).Len()
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(state *rustvec.Owned[*rusthashmap.SetIntoIter[T]], _ rustfmt.Context) uint {
			return (*state.Borrow()).Len()
		},
		Boolean: &UnordBooleanOps[*rusthashmap.SetIntoIter[T], T]{DefaultTryFold: true},
	})
}
