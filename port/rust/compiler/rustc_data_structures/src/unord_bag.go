// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine UnordBag storage/trait foundation from pinned unord.rs. General
// UnordItems algebra and sorting still keep the whole source file pending.
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordBagValueOps supplies the actual element traits. TrivialClone is Rust's
// unsafe TrivialClone marker, not merely Copy or a side-effect-free example.
// Nil Clone is lawful only for an actual side-effect-free source value copy.
// NotEqual is independent: the source slice equality loop calls T::ne.
// Nil NotEqual selects only its actual !eq default. Trait dictionaries borrow
// cells for each call and may not retain a runtime scope or hidden owned state.
// Drop includes the source destructor's complete automatic field cleanup.
type UnordBagValueOps[T any] struct {
	Clone        func(*T, rustfmt.Context) T
	Drop         func(*T, rustfmt.Context)
	Equal        func(*T, *T, rustfmt.Context) bool
	NotEqual     func(*T, *T, rustfmt.Context) bool
	Debug        func(*T, *rustfmt.Formatter) error
	TrivialClone bool
}
type UnordBagConfig[T any] struct {
	Layout  rusthashmap.Layout
	Profile rustcindex.IndexStorageConfig
	Values  UnordBagValueOps[T]
}

func (config UnordBagConfig[T]) indexOps() rustcindex.ValueOps[T] {
	return rustcindex.ValueOps[T]{RustSize: config.Layout.Size, RustZeroSized: config.Layout.Size == 0, TrivialClone: config.Values.TrivialClone, CloneRefWithContext: config.Values.Clone, DropWithRuntimeContext: config.Values.Drop}
}
func (config UnordBagConfig[T]) check(context rustfmt.Context) {
	if config.Layout.Align == 0 || config.Layout.Align&(config.Layout.Align-1) != 0 || config.Layout.Size%config.Layout.Align != 0 {
		context.Panic("datastructures: invalid source UnordBag element layout")
	}
}

// UnordBag owns existing IndexRaw/IndexVec storage with identity usize indices.
// It has no public ordered iterator or mutable ordered slice, matching the
// source determinism restriction. Current runtime metadata is never stored.
type UnordBag[T any] struct {
	inner  *rustcindex.IndexVec[uint, T]
	config UnordBagConfig[T]
	live   bool
}

func NewUnordBag[T any](config UnordBagConfig[T], contexts ...rustfmt.Context) *UnordBag[T] {
	c := unordContext(contexts)
	config.check(c)
	return &UnordBag[T]{inner: rustcindex.NewIndexVecWithConfig(rustcindex.UsizeIdx(), config.indexOps(), config.Profile), config: config, live: true}
}

// DefaultUnordBag models the derived Default implementation. Its source bound
// requires T: Default, but constructing its empty Vec never calls T::default.
func DefaultUnordBag[T any](config UnordBagConfig[T], contexts ...rustfmt.Context) *UnordBag[T] {
	return NewUnordBag(config, contexts...)
}
func (bag *UnordBag[T]) check(context rustfmt.Context) {
	if bag == nil || !bag.live {
		context.Panic("datastructures: use of consumed UnordBag")
	}
}
func (bag *UnordBag[T]) Len() uint {
	bag.check(rustfmt.Context{})
	return bag.inner.Len()
}
func (bag *UnordBag[T]) Push(value T, contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	bag.check(c)
	bag.inner.PushWithContext(value, c)
}
func (bag *UnordBag[T]) PushOwned(value *rustvec.Owned[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	bag.check(c)
	rustvec.ConsumeValue(value, c, func(value *rustvec.Owned[T]) struct{} {
		bag.inner.PushWithContext(value.Take(), c)
		return struct{}{}
	})
}
func (bag *UnordBag[T]) Extend(input rustcindex.IndexIntoIterator[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	bag.check(c)
	bag.inner.ExtendWithContext(input, c)
}
func UnordBagFromIterator[T any](input rustcindex.IndexIntoIterator[T], config UnordBagConfig[T], contexts ...rustfmt.Context) *UnordBag[T] {
	c := unordContext(contexts)
	config.check(c)
	inner := rustcindex.IndexVecFromIteratorWithConfigAndContext(input, rustcindex.UsizeIdx(), config.indexOps(), config.Profile, c)
	return &UnordBag[T]{inner: inner, config: config, live: true}
}
func (bag *UnordBag[T]) Close(contexts ...rustfmt.Context) {
	if bag != nil && bag.live {
		bag.live = false
		bag.inner.CloseWithRuntimeContext(unordContext(contexts))
	}
}
func (bag *UnordBag[T]) Clone(contexts ...rustfmt.Context) *UnordBag[T] {
	c := unordContext(contexts)
	bag.check(c)
	return &UnordBag[T]{inner: bag.inner.CloneWithContext(c), config: bag.config, live: true}
}
func (bag *UnordBag[T]) CloneFrom(source *UnordBag[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	bag.check(c)
	source.check(c)
	if bag == source {
		c.Panic("datastructures: CloneFrom requires disjoint source/destination borrows")
	}
	replacement := source.Clone(c)
	defer func() { *bag = *replacement; replacement.live = false }()
	bag.inner.CloseWithRuntimeContext(c)
}
func (bag *UnordBag[T]) Equal(other *UnordBag[T], contexts ...rustfmt.Context) bool {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	bag.check(c)
	other.check(c)
	if bag.inner.Len() != other.inner.Len() {
		return false
	}
	ops := bag.config.Values
	if ops.Equal == nil && ops.NotEqual == nil {
		c.Panic("datastructures: missing UnordBag PartialEq implementation")
	}
	for i := uint(0); i < bag.inner.Len(); i++ {
		a, b := bag.inner.Raw.At(i), other.inner.Raw.At(i)
		unequal := false
		if ops.NotEqual != nil {
			unequal = ops.NotEqual(a, b, c)
		} else {
			unequal = !ops.Equal(a, b, c)
		}
		if unequal {
			return false
		}
	}
	return true
}
func (bag *UnordBag[T]) NotEqual(other *UnordBag[T], contexts ...rustfmt.Context) bool {
	return !bag.Equal(other, contexts...)
}
func (bag *UnordBag[T]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	defer unordHashBoundary(f.Context)
	bag.check(f.Context)
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "UnordBag", pretty, []rustfmt.Field{{Name: "inner", Value: f.Value(pretty, func(inner *rustfmt.Formatter) error {
		list := rustfmt.BeginList(inner.Writer, inner.Options.Alternate)
		for i := uint(0); i < bag.inner.Len(); i++ {
			value := bag.inner.Raw.At(i)
			list.Entry(inner.Value(inner.Options.Alternate, func(field *rustfmt.Formatter) error {
				if bag.config.Values.Debug == nil {
					field.Context.Panic("datastructures: missing UnordBag Debug implementation")
				}
				return bag.config.Values.Debug(value, field)
			}))
		}
		return list.Finish()
	})}})
}
func (bag *UnordBag[T]) RustDebug() string { return rustfmt.String(bag.WriteDebugFormatter) }
func (bag *UnordBag[T]) StableHash(hcx StableHashCtxt, hasher *StableHasher, valueHash UnordStableHash[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	defer unordHashBoundary(c)
	bag.check(c)
	if valueHash == nil {
		c.Panic("datastructures: missing UnordBag StableHash implementation")
	}
	var current *T
	next := uint(0)
	hashUnordBorrowed(bag.inner.Len(), func() bool {
		if next == bag.inner.Len() {
			return false
		}
		current = bag.inner.Raw.At(next)
		next++
		return true
	}, func(target *StableHasher) { valueHash(current, hcx, target, c) }, hasher)
}

// The opaque concrete item handles do not expose IntoIterator/Next. Their
// general UnordItems operations will share the remaining iterator-algebra port.
type UnordBagItems[T any] struct {
	inner *rustcindex.RefIterator[T]
	live  bool
}

func (bag *UnordBag[T]) Items(contexts ...rustfmt.Context) *UnordBagItems[T] {
	bag.check(unordContext(contexts))
	return &UnordBagItems[T]{inner: bag.inner.Iter(), live: true}
}

func (items *UnordBagItems[T]) Close(contexts ...rustfmt.Context) {
	_ = unordContext(contexts)
	if items != nil {
		items.live = false
	}
}

type UnordBagOwnedItems[T any] struct {
	inner *rustcindex.OwnedIterator[T]
	drop  func(*T, rustfmt.Context)
	live  bool
}

func (bag *UnordBag[T]) IntoItems(contexts ...rustfmt.Context) *UnordBagOwnedItems[T] {
	bag.check(unordContext(contexts))
	bag.live = false
	return &UnordBagOwnedItems[T]{inner: bag.inner.IntoIter(), drop: bag.config.Values.Drop, live: true}
}

func (items *UnordBagOwnedItems[T]) Close(contexts ...rustfmt.Context) {
	if items != nil && items.live {
		items.live = false
		items.inner.CloseWithRuntimeContext(unordContext(contexts))
	}
}

func (items *UnordBagItems[T]) Count(contexts ...rustfmt.Context) uint {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordBag items")
	}
	items.live = false
	return items.inner.Len()
}
func (items *UnordBagItems[T]) GetOnly(contexts ...rustfmt.Context) (*T, bool) {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed UnordBag items")
	}
	items.live = false
	first, ok := items.inner.Next()
	_, second := items.inner.Next()
	if !ok || second {
		return nil, false
	}
	return first, true
}
func (items *UnordBagOwnedItems[T]) Count(contexts ...rustfmt.Context) uint {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed owned UnordBag items")
	}
	length := items.inner.Len()
	items.Close(c)
	return length
}
func bagItemsUnwind(context rustfmt.Context, cleanup func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	cleanup()
}
func bagItemsIteratorCleanup[T any](iterator *rustcindex.OwnedIterator[T], context rustfmt.Context) {
	if p := recover(); p != nil {
		if context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		unwind := context
		unwind.Unwinding = true
		bagItemsUnwind(unwind, func() { iterator.CloseWithRuntimeContext(unwind) })
		context.ResumeUnwind(p)
	}
	iterator.CloseWithRuntimeContext(context)
}
func bagItemsFirstCleanup[T any](value *rustvec.Owned[T], keep *bool, context rustfmt.Context) {
	p := recover()
	if p != nil && context.ProcessingPanic {
		context.ResumeUnwind(p)
	}
	if value != nil && !*keep {
		if p != nil {
			unwind := context
			unwind.Unwinding = true
			bagItemsUnwind(unwind, func() { value.DropWithContext(unwind) })
		} else {
			value.DropWithContext(context)
		}
	}
	if p != nil {
		context.ResumeUnwind(p)
	}
}

// GetOnly preserves next/next/is_some/drop order. The second item is destroyed
// before the first local item and the remaining iterator. A prepared Some
// return is moved before the source iterator parameter Drop.
func (items *UnordBagOwnedItems[T]) GetOnly(contexts ...rustfmt.Context) (*rustvec.Owned[T], bool) {
	c := unordContext(contexts)
	if items == nil || !items.live {
		c.Panic("datastructures: use of consumed owned UnordBag items")
	}
	items.live = false
	defer bagItemsIteratorCleanup(items.inner, c)
	first, ok := items.inner.Next()
	var owner *rustvec.Owned[T]
	if ok {
		owner = rustvec.NewOwned(first, rustvec.ValueOps[T]{Drop: items.drop})
	}
	keep := false
	defer bagItemsFirstCleanup(owner, &keep, c)
	second, some := items.inner.Next()
	if some {
		rustvec.NewOwned(second, rustvec.ValueOps[T]{Drop: items.drop}).DropWithContext(c)
		return nil, false
	}
	keep = true
	return owner, ok
}
