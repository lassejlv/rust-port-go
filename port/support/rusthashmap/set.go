// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// Set uses the source HashMap<T,()> representation. The explicit Rust layout is
// that of T (tuple-with-unit has the same size/alignment), including source ZSTs.
type Set[T any] struct{ inner *Map[T, struct{}] }

func NewSet[T any](layout Layout, profile Profile, ops KeyOps[T], builder Builder) *Set[T] {
	return NewSetWithCapacity(0, layout, profile, ops, builder, Context{})
}
func NewSetWithCapacity[T any](capacity uint, layout Layout, profile Profile, ops KeyOps[T], builder Builder, c Context) *Set[T] {
	unit := ValueOps[struct{}]{TrivialClone: true, Equal: func(*struct{}, *struct{}, Context) bool { return true }}
	return &Set[T]{NewMapWithCapacity(capacity, layout, profile, ops, unit, builder, c)}
}
func (s *Set[T]) Len() uint      { return s.inner.Len() }
func (s *Set[T]) IsEmpty() bool  { return s.inner.IsEmpty() }
func (s *Set[T]) Capacity() uint { return s.inner.Capacity() }
func (s *Set[T]) Insert(value T, c Context) bool {
	_, existed := s.inner.Insert(value, struct{}{}, c)
	return !existed
}
func (s *Set[T]) InsertOwned(value *rustvec.Owned[T], c Context) bool {
	old, existed := s.inner.InsertOwned(value, rustvec.Own(struct{}{}), c)
	if existed {
		old.DropWithContext(c)
	}
	return !existed
}
func (s *Set[T]) Contains(value *T, c Context) bool { return s.inner.ContainsKey(value, c) }
func (s *Set[T]) Get(value *T, c Context) (*T, bool) {
	m := s.inner
	m.check(c)
	hash := m.hash(value, c)
	index, ok := m.table.Find(hash, func(pair *Pair[T, struct{}]) bool { return m.keys.Equal(value, &pair.Key, c) }, c)
	if !ok {
		return nil, false
	}
	return &m.table.At(index, c).Key, true
}
func (s *Set[T]) Remove(value *T, c Context) bool {
	_, removed := s.inner.Remove(value, c)
	return removed
}

func SetGetBorrowed[T, Q any](set *Set[T], query *Q, ops BorrowedLookup[T, Q], c Context) (*T, bool) {
	index, ok := findBorrowed(set.inner, query, ops, c)
	if !ok {
		return nil, false
	}
	return &set.inner.table.At(index, c).Key, true
}
func SetContainsBorrowed[T, Q any](set *Set[T], query *Q, ops BorrowedLookup[T, Q], c Context) bool {
	_, ok := SetGetBorrowed(set, query, ops, c)
	return ok
}
func SetRemoveBorrowed[T, Q any](set *Set[T], query *Q, ops BorrowedLookup[T, Q], c Context) bool {
	_, ok := RemoveBorrowed(set.inner, query, ops, c)
	return ok
}
func (s *Set[T]) Clear(c Context)                        { s.inner.Clear(c) }
func (s *Set[T]) Close(c Context)                        { s.inner.Close(c) }
func (s *Set[T]) Clone(c Context) *Set[T]                { return &Set[T]{s.inner.Clone(c)} }
func (s *Set[T]) CloneFrom(source *Set[T], c Context)    { s.inner.CloneFrom(source.inner, c) }
func (s *Set[T]) Equal(other *Set[T], c Context) bool    { return s.inner.Equal(other.inner, c) }
func (s *Set[T]) NotEqual(other *Set[T], c Context) bool { return !s.Equal(other, c) }

type SetIter[T any] struct{ inner *Iter[T, struct{}] }

func (s *Set[T]) Iter(c Context) *SetIter[T]    { return &SetIter[T]{s.inner.Iter(c)} }
func (i *SetIter[T]) Next(c Context) (*T, bool) { pair, ok := i.inner.Next(c); return pair.Key, ok }
func (i *SetIter[T]) Len() uint                 { return i.inner.Len() }
func (i *SetIter[T]) Clone() *SetIter[T]        { return &SetIter[T]{i.inner.Clone()} }

type SetIntoIter[T any] struct{ inner *IntoIter[T, struct{}] }

func (s *Set[T]) IntoIter(c Context) *SetIntoIter[T] { return &SetIntoIter[T]{s.inner.IntoIter(c)} }
func (i *SetIntoIter[T]) Next(c Context) (T, bool)   { pair, ok := i.inner.Next(c); return pair.Key, ok }
func (i *SetIntoIter[T]) Len() uint                  { return i.inner.Len() }
func (i *SetIntoIter[T]) Close(c Context)            { i.inner.Close(c) }

// WriteDebugFormatter is the scoped DebugSet builder protocol. Keeping this
// small used helper here avoids broadening the frozen neutral formatter API.
func (s *Set[T]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	s.inner.check(f.Context)
	if s.inner.keys.Debug == nil {
		f.Context.Panic("rusthashmap: missing set Debug implementation")
	}
	pretty := f.Options.Alternate
	result := f.WriteString("{")
	count := 0
	iterator := s.Iter(f.Context)
	for {
		value, ok := iterator.Next(f.Context)
		if !ok {
			break
		}
		if result != nil {
			continue
		}
		if pretty {
			if count == 0 {
				result = f.WriteString("\n")
			}
			padded := rustfmt.NewIndentWriter(f.Writer)
			if result == nil {
				result = s.inner.keys.Debug(value, f.CloneWithWriter(padded))
			}
			if result == nil {
				result = rustfmt.WriteString(padded, ",\n")
			}
		} else {
			if count > 0 {
				result = f.WriteString(", ")
			}
			if result == nil {
				result = s.inner.keys.Debug(value, f)
			}
		}
		count++
	}
	if result == nil {
		result = f.WriteString("}")
	}
	return result
}
