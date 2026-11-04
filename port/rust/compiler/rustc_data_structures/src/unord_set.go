// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL foundation from rustc_data_structures/src/unord.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The actual Fx/SwissTable backing is
// used here. General UnordItems algebra, sorting and codecs remain pending.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// Layout is the Rust value layout, including its actual alignment and ZST size.
// Values supplies the fixed monomorphized Hash/Eq/Clone/Drop/Debug operations.
type UnordSetConfig[T any] struct {
	Layout  rusthashmap.Layout
	Profile rusthashmap.Profile
	Values  rusthashmap.KeyOps[T]
}

type UnordSet[T any] struct {
	inner  *rusthashmap.Set[T]
	config UnordSetConfig[T]
	live   bool
}

func NewUnordSet[T any](config UnordSetConfig[T], contexts ...rusthashmap.Context) *UnordSet[T] {
	return NewUnordSetWithCapacity(0, config, contexts...)
}
func NewUnordSetWithCapacity[T any](capacity uint, config UnordSetConfig[T], contexts ...rusthashmap.Context) *UnordSet[T] {
	c := unordContext(contexts)
	if config.Values.HashOne != nil {
		c.Panic("datastructures: UnordSet's fixed FxBuildHasher uses the default hash_one")
	}
	return &UnordSet[T]{inner: rusthashmap.NewSetWithCapacity(capacity, config.Layout, config.Profile, config.Values, rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}, c), config: config, live: true}
}
func (s *UnordSet[T]) check(c rusthashmap.Context) {
	if s == nil || !s.live {
		c.Panic("datastructures: use of consumed UnordSet")
	}
}
func (s *UnordSet[T]) Len() uint     { s.check(rusthashmap.Context{}); return s.inner.Len() }
func (s *UnordSet[T]) IsEmpty() bool { return s.Len() == 0 }

// GetOnly returns a shared source borrow, invalidated by incompatible mutation,
// move or Drop. The source checks length before constructing/advancing its Iter.
func (s *UnordSet[T]) GetOnly(contexts ...rusthashmap.Context) (*T, bool) {
	c := unordContext(contexts)
	s.check(c)
	if s.inner.Len() != 1 {
		return nil, false
	}
	return s.inner.Iter(c).Next(c)
}
func (s *UnordSet[T]) Insert(value T, contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return s.inner.Insert(value, c)
}
func (s *UnordSet[T]) InsertOwned(value *rustvec.Owned[T], contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return s.inner.InsertOwned(value, c)
}
func (s *UnordSet[T]) Contains(value *T, contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return s.inner.Contains(value, c)
}
func (s *UnordSet[T]) Remove(value *T, contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return s.inner.Remove(value, c)
}
func UnordSetContainsBorrowed[T, Q any](s *UnordSet[T], query *Q, ops rusthashmap.BorrowedLookup[T, Q], contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return rusthashmap.SetContainsBorrowed(s.inner, query, ops, c)
}
func UnordSetRemoveBorrowed[T, Q any](s *UnordSet[T], query *Q, ops rusthashmap.BorrowedLookup[T, Q], contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	return rusthashmap.SetRemoveBorrowed(s.inner, query, ops, c)
}
func (s *UnordSet[T]) Clear(contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	s.check(c)
	s.inner.Clear(c)
}
func (s *UnordSet[T]) Close(contexts ...rusthashmap.Context) {
	if s != nil && s.live {
		s.live = false
		s.inner.Close(unordContext(contexts))
	}
}
func (s *UnordSet[T]) Clone(contexts ...rusthashmap.Context) *UnordSet[T] {
	c := unordContext(contexts)
	s.check(c)
	return &UnordSet[T]{inner: s.inner.Clone(c), config: s.config, live: true}
}
func (s *UnordSet[T]) CloneFrom(source *UnordSet[T], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	s.check(c)
	source.check(c)
	if s == source {
		c.Panic("datastructures: CloneFrom requires disjoint source/destination borrows")
	}
	replacement := source.Clone(c)
	// Derived Clone uses whole-value assignment. Its replacement is installed
	// even when destroying the previous destination starts unwinding.
	defer func() { *s = *replacement; replacement.live = false }()
	s.inner.Close(c)
}
func (s *UnordSet[T]) Equal(other *UnordSet[T], contexts ...rusthashmap.Context) bool {
	c := unordContext(contexts)
	s.check(c)
	other.check(c)
	return s.inner.Equal(other.inner, c)
}
func (s *UnordSet[T]) NotEqual(other *UnordSet[T], contexts ...rusthashmap.Context) bool {
	return !s.Equal(other, contexts...)
}
func (s *UnordSet[T]) Extend(input rustvec.IntoIterator[T], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	s.check(c)
	s.inner.Extend(input, c)
}
func UnordSetFromIterator[T any](input rustvec.IntoIterator[T], config UnordSetConfig[T], contexts ...rusthashmap.Context) *UnordSet[T] {
	c := unordContext(contexts)
	if config.Values.HashOne != nil {
		c.Panic("datastructures: UnordSet's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rusthashmap.SetFromIterator(input, config.Layout, config.Profile, config.Values, func(rusthashmap.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}, c)
	return &UnordSet[T]{inner: inner, config: config, live: true}
}
func (s *UnordSet[T]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	s.check(f.Context)
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "UnordSet", pretty, []rustfmt.Field{{Name: "inner", Value: f.Value(pretty, s.inner.WriteDebugFormatter)}})
}
func (s *UnordSet[T]) RustDebug() string { return rustfmt.String(s.WriteDebugFormatter) }

// This borrowed view intentionally has no public Next/IntoIterator operation.
// It retains the source's negative IntoIterator contract. Source mutation and
// Drop are forbidden while this view or any of its shared borrows is live.
type UnordSetItems[T any] struct {
	iterator *rusthashmap.SetIter[T]
	live     bool
}

func (s *UnordSet[T]) Items(contexts ...rusthashmap.Context) *UnordSetItems[T] {
	c := unordContext(contexts)
	s.check(c)
	return &UnordSetItems[T]{iterator: s.inner.Iter(c), live: true}
}
func (i *UnordSetItems[T]) Clone(contexts ...rusthashmap.Context) *UnordSetItems[T] {
	c := unordContext(contexts)
	if i == nil || !i.live {
		c.Panic("datastructures: use of consumed UnordSetItems")
	}
	return &UnordSetItems[T]{iterator: i.iterator.Clone(), live: true}
}
func (i *UnordSetItems[T]) Close(contexts ...rusthashmap.Context) {
	if i != nil {
		i.live = false
	}
}
