// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/slice.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.
package rustcindex

import (
	"fmt"
	"math/bits"
	"reflect"
	"strconv"
	"strings"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// IndexStorageConfig selects source arithmetic checks and debug assertions.
// Constructors without a config enable both, matching checked Rust builds.
type IndexStorageConfig struct{ OverflowChecks, DebugAssertions bool }

func defaultIndexStorageConfig() IndexStorageConfig { return IndexStorageConfig{true, true} }

// ValueOps supplies the actual Rust element trait implementations. A nil Clone
// means a side-effect-free value copy; use a callback for any other Clone.
// Copy and TrivialClone are distinct: lawful Copy types can override Clone.
// RustZeroSized reports Rust zero-sized representations containing Go metadata.
// Such values must have no distinct per-element payload. Borrow/drop ownership
// and the supplied trait laws remain caller contracts rather than Go lifetimes.
type ValueOps[T any] struct {
	Clone     func(T) T
	CloneFrom func(*T, T)
	Equal     func(T, T) bool
	NotEqual  func(T, T) bool
	Debug     func(T) string
	Compare   func(T, T) int
	CloneRef  func(*T) T
	// Contextual operations borrow the current call's runtime metadata. The
	// dictionary itself must not capture a construction-time runtime scope.
	CloneRefWithContext               func(*T, rustfmt.Context) T
	CloneFromRef                      func(*T, *T)
	EqualRef                          func(*T, *T) bool
	NotEqualRef                       func(*T, *T) bool
	DebugRef                          func(*T) string
	DebugRefWithContext               func(*T, IndexFormatContext) string
	DebugWithContext                  func(T, IndexFormatContext) string
	FormatDebug                       func(*T, *IndexDebugFormatter) error
	CompareRef                        func(*T, *T) int
	Drop                              func(*T)
	DropWithContext                   func(*T, IndexDropContext)
	DropWithRuntimeContext            func(*T, rustfmt.Context)
	Copy, TrivialClone, RustZeroSized bool
	// RustSize optionally supplies the source element size for capacity-growth
	// buckets when its Go representation carries different metadata.
	RustSize uint
}

func (ops ValueOps[T]) clone(v T) T { return ops.cloneAt(&v) }
func (ops ValueOps[T]) cloneAt(v *T) T {
	if ops.CloneRefWithContext != nil {
		return ops.CloneRefWithContext(v, rustfmt.Context{})
	}
	if ops.CloneRef != nil {
		return ops.CloneRef(v)
	}
	if ops.Clone != nil {
		return ops.Clone(*v)
	}
	return *v
}
func (ops ValueOps[T]) cloneFrom(dst *T, src T) { ops.cloneFromAt(dst, &src) }
func (ops ValueOps[T]) cloneFromAt(dst, src *T) {
	if ops.CloneFromRef != nil {
		ops.CloneFromRef(dst, src)
		return
	}
	if ops.CloneFrom != nil {
		ops.CloneFrom(dst, *src)
		return
	}
	value := ops.cloneAt(src)
	// Rust DropAndReplace drops the actual place and installs its replacement
	// both on success and during unwinding from that Drop.
	defer func() { *dst = value }()
	ops.dropAt(dst, IndexDropContext{})
}
func (ops ValueOps[T]) equal(a, b T) bool { return ops.equalAt(&a, &b) }
func (ops ValueOps[T]) equalAt(a, b *T) bool {
	if ops.EqualRef != nil {
		return ops.EqualRef(a, b)
	}
	if ops.Equal != nil {
		return ops.Equal(*a, *b)
	}
	return any(*a) == any(*b)
}
func (ops ValueOps[T]) notEqualAt(a, b *T) bool {
	if ops.NotEqualRef != nil {
		return ops.NotEqualRef(a, b)
	}
	if ops.NotEqual != nil {
		return ops.NotEqual(*a, *b)
	}
	return !ops.equalAt(a, b)
}
func (ops ValueOps[T]) debug(v T) string    { return ops.debugAt(&v) }
func (ops ValueOps[T]) debugAt(v *T) string { return ops.debugAtContext(v, IndexFormatContext{}) }
func (ops ValueOps[T]) debugLegacyAt(v *T) string {
	if ops.DebugRef != nil {
		return ops.DebugRef(v)
	}
	if ops.Debug != nil {
		return ops.Debug(*v)
	}
	if text, ok := any(*v).(string); ok {
		return strconv.Quote(text)
	}
	return fmt.Sprint(*v)
}
func (ops ValueOps[T]) compare(a, b T) int { return ops.compareAt(&a, &b) }
func (ops ValueOps[T]) compareAt(a, b *T) int {
	if ops.CompareRef != nil {
		return ops.CompareRef(a, b)
	}
	if ops.Compare != nil {
		return ops.Compare(*a, *b)
	}
	if ops.RustZeroSized || reflect.TypeFor[T]().Size() == 0 {
		return 0
	}
	x, y := reflect.ValueOf(*a), reflect.ValueOf(*b)
	if !x.IsValid() || !y.IsValid() {
		panic("rustcindex: Ord callback required")
	}
	switch x.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if x.Int() < y.Int() {
			return -1
		}
		if x.Int() > y.Int() {
			return 1
		}
		return 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if x.Uint() < y.Uint() {
			return -1
		}
		if x.Uint() > y.Uint() {
			return 1
		}
		return 0
	case reflect.String:
		if x.String() < y.String() {
			return -1
		}
		if x.String() > y.String() {
			return 1
		}
		return 0
	case reflect.Bool:
		if x.Bool() == y.Bool() {
			return 0
		}
		if !x.Bool() {
			return -1
		}
		return 1
	}
	panic("rustcindex: Ord callback required for this element type")
}

// IndexRaw is the contiguous raw storage/view underlying a typed container.
// Zero-sized Rust elements retain a full uint logical length and usize::MAX
// vector capacity without allocating a Go slice. AsGoSlice is an explicitly
// limited projection; all main operations use this full-domain representation.
type IndexRaw[T any] struct {
	data            []T
	length          uint
	zero            *T
	zeroSized, view bool
}

// IndexRawZeroSized constructs a full-domain raw ZST representation. Prototype
// is borrowed representation metadata, not an additional owned Rust element.
// The caller must supply a valid inhabited Rust zero-sized type: its logical
// elements have no distinct payload, even if Go carries associated metadata.
func IndexRawZeroSized[T any](prototype *T, length uint) IndexRaw[T] {
	cell := new(T)
	if prototype != nil {
		*cell = *prototype
	}
	return IndexRaw[T]{length: length, zero: cell, zeroSized: true}
}

func rawFromSlice[T any](data []T, ops ValueOps[T]) IndexRaw[T] {
	zst := ops.RustZeroSized || reflect.TypeFor[T]().Size() == 0
	r := IndexRaw[T]{data: data, length: uint(len(data)), zeroSized: zst}
	if zst {
		r.zero = new(T)
	}
	if zst && len(data) > 0 {
		r.zero = &data[0]
	}
	if zst {
		r.data = nil
	}
	return r
}
func (r IndexRaw[T]) Len() uint {
	if r.zeroSized {
		return r.length
	}
	return uint(len(r.data))
}
func (r IndexRaw[T]) Cap() uint {
	if r.view {
		return r.Len()
	}
	if r.zeroSized {
		return ^uint(0)
	}
	return uint(cap(r.data))
}
func (r IndexRaw[T]) IsEmpty() bool     { return r.Len() == 0 }
func (r IndexRaw[T]) IsZeroSized() bool { return r.zeroSized }
func (r *IndexRaw[T]) At(i uint) *T {
	if i >= r.Len() {
		panic("rustcindex: raw index out of bounds")
	}
	if r.zeroSized {
		return r.zero
	}
	return &r.data[i]
}
func (r *IndexRaw[T]) Get(i uint) (*T, bool) {
	if i >= r.Len() {
		return nil, false
	}
	return r.At(i), true
}
func (r IndexRaw[T]) Slice(start, end uint) IndexRaw[T] {
	if start > end || end > r.Len() {
		panic("rustcindex: raw slice out of bounds")
	}
	if r.zeroSized {
		return IndexRaw[T]{length: end - start, zero: r.zero, zeroSized: true, view: true}
	}
	return IndexRaw[T]{data: r.data[start:end:end], length: end - start, view: true}
}
func (r IndexRaw[T]) AsGoSlice() []T {
	if !r.zeroSized {
		return r.data
	}
	if r.length > uint(^uint(0)>>1) {
		panic("rustcindex: logical ZST length cannot be projected to a Go slice")
	}
	data := make([]T, int(r.length))
	for i := range data {
		data[i] = *r.zero
	}
	return data
}
func (r IndexRaw[T]) Debug(format func(T) string) string {
	return r.DebugRef(func(v *T) string { return format(*v) })
}
func (r IndexRaw[T]) DebugRef(format func(*T) string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := uint(0); i < r.Len(); i++ {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(format(r.At(i)))
	}
	b.WriteByte(']')
	return b.String()
}

// IndexSlice is a borrowed, fixed-length typed view. As in Rust, mutation and
// vector growth must not overlap outstanding borrowed references/iterators.
type IndexSlice[I, T any] struct {
	Raw    IndexRaw[T]
	Idx    Idx[I]
	Ops    ValueOps[T]
	Config IndexStorageConfig
}

func EmptyIndexSlice[I, T any](idx Idx[I], ops ValueOps[T]) *IndexSlice[I, T] {
	return IndexSliceFromRaw([]T{}, idx, ops)
}
func IndexSliceFromRaw[I, T any](raw []T, idx Idx[I], ops ValueOps[T]) *IndexSlice[I, T] {
	return IndexSliceFromRawWithConfig(raw, idx, ops, defaultIndexStorageConfig())
}
func IndexSliceFromRawWithConfig[I, T any](raw []T, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexSlice[I, T] {
	return IndexSliceFromStorageWithConfig(rawFromSlice(raw, ops), idx, ops, config)
}
func IndexSliceFromStorage[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T]) *IndexSlice[I, T] {
	return IndexSliceFromStorageWithConfig(raw, idx, ops, defaultIndexStorageConfig())
}
func IndexSliceFromStorageWithConfig[I, T any](raw IndexRaw[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexSlice[I, T] {
	raw.view = true
	return &IndexSlice[I, T]{raw, idx, ops, config}
}
func IndexSliceFromRawMut[I, T any](raw []T, idx Idx[I], ops ValueOps[T]) *IndexSlice[I, T] {
	return IndexSliceFromRaw(raw, idx, ops)
}
func IndexSliceFromRawMutWithConfig[I, T any](raw []T, idx Idx[I], ops ValueOps[T], config IndexStorageConfig) *IndexSlice[I, T] {
	return IndexSliceFromRawWithConfig(raw, idx, ops, config)
}
func (s *IndexSlice[I, T]) Len() uint     { return s.Raw.Len() }
func (s *IndexSlice[I, T]) IsEmpty() bool { return s.Raw.IsEmpty() }
func (s *IndexSlice[I, T]) NextIndex() I  { return s.Idx.New(s.Len()) }
func (s *IndexSlice[I, T]) LastIndex() (I, bool) {
	if s.Len() == 0 {
		var z I
		return z, false
	}
	return s.Idx.New(s.Len() - 1), true
}
func (s *IndexSlice[I, T]) At(key I) *T             { return s.Raw.At(s.Idx.Index(key)) }
func (s *IndexSlice[I, T]) AtMut(key I) *T          { return s.At(key) }
func (s *IndexSlice[I, T]) Get(key I) (*T, bool)    { return s.Raw.Get(s.Idx.Index(key)) }
func (s *IndexSlice[I, T]) GetMut(key I) (*T, bool) { return s.Get(key) }

// SliceRef retains whether a SliceIndex output is one element or a raw slice.
// Raw views support logical ZST lengths larger than Go's signed int domain.
type SliceRef[T any] struct {
	Single  bool
	Element *T
	Raw     IndexRaw[T]
}

func (s *IndexSlice[I, T]) GetIndex(result TypedSliceIndexResult) (SliceRef[T], bool) {
	if result.Kind == SliceIndexSingle {
		p, ok := s.Raw.Get(result.Index)
		return SliceRef[T]{Single: true, Element: p}, ok
	}
	start, end, ok := sliceIndexRange(result, s.Len())
	if !ok {
		return SliceRef[T]{}, false
	}
	return SliceRef[T]{Raw: s.Raw.Slice(start, end)}, true
}
func (s *IndexSlice[I, T]) GetMutIndex(result TypedSliceIndexResult) (SliceRef[T], bool) {
	return s.GetIndex(result)
}
func (s *IndexSlice[I, T]) Index(result TypedSliceIndexResult) SliceRef[T] {
	r, ok := s.GetIndex(result)
	if !ok {
		panic("rustcindex: slice index out of bounds")
	}
	return r
}
func (s *IndexSlice[I, T]) IndexMut(result TypedSliceIndexResult) SliceRef[T] { return s.Index(result) }

// SliceIndexer supplies the associated Output: SliceIndex<[T]> operations for
// custom IntoSliceIdx implementations. The tagged built-ins use GetIndex;
// this generic boundary does not narrow arbitrary associated output types.
type SliceIndexer[T, Output any] struct {
	Get      func(IndexRaw[T], Output) (SliceRef[T], bool)
	Index    func(IndexRaw[T], Output) SliceRef[T]
	GetMut   func(IndexRaw[T], Output) (SliceRef[T], bool)
	IndexMut func(IndexRaw[T], Output) SliceRef[T]
}

func IndexSliceGetWith[I, T, Input, Output any](s *IndexSlice[I, T], input Input, convert IntoSliceIdx[Input, Output], output SliceIndexer[T, Output]) (SliceRef[T], bool) {
	return output.Get(s.Raw, convert.IntoSliceIdx(input))
}
func IndexSliceIndexWith[I, T, Input, Output any](s *IndexSlice[I, T], input Input, convert IntoSliceIdx[Input, Output], output SliceIndexer[T, Output]) SliceRef[T] {
	return output.Index(s.Raw, convert.IntoSliceIdx(input))
}
func IndexSliceGetMutWith[I, T, Input, Output any](s *IndexSlice[I, T], input Input, convert IntoSliceIdx[Input, Output], output SliceIndexer[T, Output]) (SliceRef[T], bool) {
	r := convert.IntoSliceIdx(input)
	if output.GetMut != nil {
		return output.GetMut(s.Raw, r)
	}
	return output.Get(s.Raw, r)
}
func IndexSliceIndexMutWith[I, T, Input, Output any](s *IndexSlice[I, T], input Input, convert IntoSliceIdx[Input, Output], output SliceIndexer[T, Output]) SliceRef[T] {
	r := convert.IntoSliceIdx(input)
	if output.IndexMut != nil {
		return output.IndexMut(s.Raw, r)
	}
	return output.Index(s.Raw, r)
}
func IndexSliceCopyWithinWith[I, T, Input, Output any](s *IndexSlice[I, T], input Input, convert IntoSliceIdx[Input, Output], bounds func(Output) IndexRangeBounds, dest I) {
	r := convert.IntoSliceIdx(input)
	s.CopyWithinBounds(bounds(r), dest)
}

func sliceIndexRange(r TypedSliceIndexResult, length uint) (start, end uint, ok bool) {
	switch r.Kind {
	case SliceIndexFull:
		return 0, length, true
	case SliceIndexRange, SliceIndexCoreRange:
		start, end = r.Start, r.End
	case SliceIndexFrom, SliceIndexCoreFrom:
		start, end = r.Start, length
	case SliceIndexTo:
		start, end = 0, r.End
	case SliceIndexInclusive, SliceIndexToInclusive, SliceIndexCoreInclusive, SliceIndexCoreToInclusive:
		start = r.Start
		last := r.End
		if r.Kind == SliceIndexToInclusive || r.Kind == SliceIndexCoreToInclusive {
			start = 0
		}
		if r.Kind == SliceIndexCoreInclusive || r.Kind == SliceIndexCoreToInclusive {
			last = r.Last
		}
		if last >= length {
			return 0, 0, false
		}
		end = last + 1
		if r.Exhausted && r.Kind == SliceIndexInclusive {
			start = end
		}
	default:
		return 0, 0, false
	}
	return start, end, start <= end && end <= length
}

type IndexBoundKind uint8

const (
	IndexUnbounded IndexBoundKind = iota
	IndexIncluded
	IndexExcluded
)

type IndexBound struct {
	Kind  IndexBoundKind
	Value uint
}

// IndexRangeBounds independently calls start_bound/end_bound, including custom
// implementations with interior state. DrainEnumerated reads start twice.
type IndexRangeBounds struct {
	StartBound, EndBound func() IndexBound
	Close                func()
	CloseWithContext     func(IndexDropContext)
}

func BoundsFromSliceIndex(r TypedSliceIndexResult) IndexRangeBounds {
	start, end := IndexBound{}, IndexBound{}
	switch r.Kind {
	case SliceIndexFull:
	case SliceIndexRange, SliceIndexCoreRange:
		start = IndexBound{IndexIncluded, r.Start}
		end = IndexBound{IndexExcluded, r.End}
	case SliceIndexFrom, SliceIndexCoreFrom:
		start = IndexBound{IndexIncluded, r.Start}
	case SliceIndexTo:
		end = IndexBound{IndexExcluded, r.End}
	case SliceIndexInclusive:
		start = IndexBound{IndexIncluded, r.Start}
		end = IndexBound{IndexIncluded, r.End}
	case SliceIndexToInclusive:
		end = IndexBound{IndexIncluded, r.End}
	case SliceIndexCoreInclusive:
		start = IndexBound{IndexIncluded, r.Start}
		end = IndexBound{IndexIncluded, r.Last}
	case SliceIndexCoreToInclusive:
		end = IndexBound{IndexIncluded, r.Last}
	default:
		panic("rustcindex: range bounds required")
	}
	return IndexRangeBounds{StartBound: func() IndexBound { return start }, EndBound: func() IndexBound { return end }}
}
func normalizeRange(bounds IndexRangeBounds, length uint) (start, end uint) {
	owned := true
	defer indexDeferCallback(bounds.Close, bounds.CloseWithContext, &owned)
	// Rust evaluates both callbacks left-to-right, then validates end first.
	a, b := bounds.StartBound(), bounds.EndBound()
	switch b.Kind {
	case IndexIncluded:
		if b.Value >= length {
			panic("rustcindex: range end out of bounds")
		}
		end = b.Value + 1
	case IndexExcluded:
		if b.Value > length {
			panic("rustcindex: range end out of bounds")
		}
		end = b.Value
	case IndexUnbounded:
		end = length
	default:
		panic("rustcindex: invalid bound")
	}
	switch a.Kind {
	case IndexIncluded:
		if a.Value > end {
			panic("rustcindex: range start exceeds end")
		}
		start = a.Value
	case IndexExcluded:
		if a.Value >= end {
			panic("rustcindex: range start exceeds end")
		}
		start = a.Value + 1
	case IndexUnbounded:
		start = 0
	default:
		panic("rustcindex: invalid bound")
	}
	return
}
func (s *IndexSlice[I, T]) Swap(a, b I) {
	ai, bi := s.Idx.Index(a), s.Idx.Index(b)
	x, y := s.Raw.At(ai), s.Raw.At(bi)
	*x, *y = *y, *x
}
func (s *IndexSlice[I, T]) CopyWithin(src TypedSliceIndexResult, dest I) {
	s.CopyWithinBounds(BoundsFromSliceIndex(src), dest)
}
func (s *IndexSlice[I, T]) CopyWithinBounds(src IndexRangeBounds, dest I) {
	owned := true
	defer indexDeferCallback(src.Close, src.CloseWithContext, &owned)
	d := s.Idx.Index(dest)
	owned = false
	start, end := normalizeRange(src, s.Len())
	count := end - start
	if d > s.Len()-count {
		panic("rustcindex: copy destination out of bounds")
	}
	if !s.Raw.zeroSized {
		copy(s.Raw.data[d:d+count], s.Raw.data[start:end])
	}
}
func (s *IndexSlice[I, T]) Pick2Mut(a, b I) (*T, *T) {
	ai, bi := s.Idx.Index(a), s.Idx.Index(b)
	if ai >= s.Len() || bi >= s.Len() {
		panic(fmt.Sprintf("Some indices among (%d, %d) are out of bounds", ai, bi))
	}
	if ai == bi {
		panic(fmt.Sprintf("Indices %d and %d are not disjoint!", ai, bi))
	}
	return s.Raw.At(ai), s.Raw.At(bi)
}
func (s *IndexSlice[I, T]) Pick3Mut(a, b, c I) (*T, *T, *T) {
	ai, bi, ci := s.Idx.Index(a), s.Idx.Index(b), s.Idx.Index(c)
	values := [3]uint{ai, bi, ci}
	for i, v := range values {
		if v >= s.Len() {
			panic(fmt.Sprintf("Some indices among (%d, %d, %d) are out of bounds", ai, bi, ci))
		}
		for _, previous := range values[:i] {
			if v == previous {
				panic(fmt.Sprintf("Indices %d, %d and %d are not disjoint!", ai, bi, ci))
			}
		}
	}
	return s.Raw.At(ai), s.Raw.At(bi), s.Raw.At(ci)
}

// BinarySearch uses the pinned standard slice's comparison sequence, then
// constructs the typed Ok/Err index exactly once, including for an empty slice.
func (s *IndexSlice[I, T]) BinarySearch(value T) (I, bool) { return s.BinarySearchRef(&value) }
func (s *IndexSlice[I, T]) BinarySearchRef(value *T) (I, bool) {
	size := s.Len()
	base := uint(0)
	if size == 0 {
		return s.Idx.New(0), false
	}
	for size > 1 {
		half := size / 2
		mid := base + half
		cmp := s.Ops.compareAt(s.Raw.At(mid), value)
		if cmp <= 0 {
			base = mid
		}
		size -= half
	}
	cmp := s.Ops.compareAt(s.Raw.At(base), value)
	if cmp == 0 {
		return s.Idx.New(base), true
	}
	if cmp < 0 {
		base++
	}
	return s.Idx.New(base), false
}

type RefIterator[T any] struct {
	raw         IndexRaw[T]
	front, back uint
}

func (s *IndexSlice[I, T]) Iter() *RefIterator[T] { return &RefIterator[T]{raw: s.Raw, back: s.Len()} }
func (it *RefIterator[T]) Next() (*T, bool) {
	if it.front == it.back {
		return nil, false
	}
	i := it.front
	it.front++
	return it.raw.At(i), true
}
func (it *RefIterator[T]) NextBack() (*T, bool) {
	if it.front == it.back {
		return nil, false
	}
	it.back--
	return it.raw.At(it.back), true
}
func (it *RefIterator[T]) Len() uint              { return it.back - it.front }
func (it *RefIterator[T]) SizeHint() (uint, uint) { n := it.Len(); return n, n }
func (it *RefIterator[T]) Clone() *RefIterator[T] { copy := *it; return &copy }

type MutIterator[T any] struct{ inner RefIterator[T] }

func (s *IndexSlice[I, T]) IterMut() *MutIterator[T] {
	return &MutIterator[T]{RefIterator[T]{raw: s.Raw, back: s.Len()}}
}
func (it *MutIterator[T]) Next() (*T, bool)       { return it.inner.Next() }
func (it *MutIterator[T]) NextBack() (*T, bool)   { return it.inner.NextBack() }
func (it *MutIterator[T]) Len() uint              { return it.inner.Len() }
func (it *MutIterator[T]) SizeHint() (uint, uint) { return it.inner.SizeHint() }

type IndexIterator[I any] struct {
	idx         Idx[I]
	front, back uint
}

func (s *IndexSlice[I, T]) Indices() *IndexIterator[I] {
	_ = s.Idx.New(s.Len())
	return &IndexIterator[I]{idx: s.Idx, back: s.Len()}
}
func (it *IndexIterator[I]) Next() (I, bool) {
	if it.front == it.back {
		var z I
		return z, false
	}
	i := it.front
	it.front++
	return it.idx.New(i), true
}
func (it *IndexIterator[I]) NextBack() (I, bool) {
	if it.front == it.back {
		var z I
		return z, false
	}
	it.back--
	return it.idx.New(it.back), true
}
func (it *IndexIterator[I]) Len() uint                { return it.back - it.front }
func (it *IndexIterator[I]) SizeHint() (uint, uint)   { n := it.Len(); return n, n }
func (it *IndexIterator[I]) Clone() *IndexIterator[I] { copy := *it; return &copy }

type EnumeratedRefIterator[I, T any] struct {
	idx         Idx[I]
	raw         IndexRaw[T]
	front, back uint
	count       uint
}

func (s *IndexSlice[I, T]) IterEnumerated() *EnumeratedRefIterator[I, T] {
	_ = s.Idx.New(s.Len())
	return &EnumeratedRefIterator[I, T]{idx: s.Idx, raw: s.Raw, back: s.Len()}
}
func (it *EnumeratedRefIterator[I, T]) Next() (I, *T, bool) {
	if it.front == it.back {
		var z I
		return z, nil, false
	}
	i := it.front
	it.front++
	n := it.count
	it.count++
	return it.idx.New(n), it.raw.At(i), true
}
func (it *EnumeratedRefIterator[I, T]) NextBack() (I, *T, bool) {
	if it.front == it.back {
		var z I
		return z, nil, false
	}
	it.back--
	return it.idx.New(it.count + it.Len()), it.raw.At(it.back), true
}
func (it *EnumeratedRefIterator[I, T]) Len() uint              { return it.back - it.front }
func (it *EnumeratedRefIterator[I, T]) SizeHint() (uint, uint) { n := it.Len(); return n, n }

type EnumeratedMutIterator[I, T any] struct{ inner EnumeratedRefIterator[I, T] }

func (s *IndexSlice[I, T]) IterEnumeratedMut() *EnumeratedMutIterator[I, T] {
	it := s.IterEnumerated()
	return &EnumeratedMutIterator[I, T]{*it}
}
func (it *EnumeratedMutIterator[I, T]) Next() (I, *T, bool)     { return it.inner.Next() }
func (it *EnumeratedMutIterator[I, T]) NextBack() (I, *T, bool) { return it.inner.NextBack() }
func (it *EnumeratedMutIterator[I, T]) Len() uint               { return it.inner.Len() }
func (it *EnumeratedMutIterator[I, T]) SizeHint() (uint, uint)  { return it.inner.SizeHint() }
func (s *IndexSlice[I, T]) IntoIter() *RefIterator[T]           { return s.Iter() }
func (s *IndexSlice[I, T]) IntoIterMut() *MutIterator[T]        { return s.IterMut() }

func (s *IndexSlice[I, T]) ToOwned() *IndexVec[I, T] {
	return cloneRawIntoVec(s.Raw, s.Idx, s.Ops, s.Config)
}
func (s *IndexSlice[I, T]) CloneInto(target *IndexVec[I, T]) {
	if s.Ops.TrivialClone {
		target.Truncate(0)
		target.extendRawCopy(s.Raw)
		return
	}
	target.Truncate(s.Len())
	initial := target.Len()
	for i := uint(0); i < initial; i++ {
		s.Ops.cloneFromAt(target.Raw.At(i), s.Raw.At(i))
	}
	target.reserveRaw(s.Len() - initial)
	for i := initial; i < s.Len(); i++ {
		target.pushRaw(s.Ops.cloneAt(s.Raw.At(i)))
	}
}
func (s *IndexSlice[I, T]) Equal(other *IndexSlice[I, T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	if s.Raw.zeroSized && s.Ops.Equal == nil && s.Ops.EqualRef == nil && s.Ops.NotEqual == nil && s.Ops.NotEqualRef == nil {
		return true
	}
	for i := uint(0); i < s.Len(); i++ {
		// The standard slice equality loop uses T::ne, not T::eq.
		if s.Ops.notEqualAt(s.Raw.At(i), other.Raw.At(i)) {
			return false
		}
	}
	return true
}
func (s *IndexSlice[I, T]) NotEqual(other *IndexSlice[I, T]) bool { return !s.Equal(other) }
func (s *IndexSlice[I, T]) Debug() string                         { return s.DebugWithContext(IndexFormatContext{}) }
func (s *IndexSlice[I, T]) Hash(hashRaw func(IndexRaw[T]))        { hashRaw(s.Raw) }

// HashIndexRaw provides the standard slice hash's length prefix and element
// sequence while retaining Hash::hash_slice's independently overridable method.
func HashIndexRaw[T, State any](raw IndexRaw[T], state State, length func(State, uint), element func(State, T), hashSlice func(State, IndexRaw[T])) {
	HashIndexRawRef(raw, state, length, func(state State, value *T) { element(state, *value) }, hashSlice)
}
func HashIndexRawRef[T, State any](raw IndexRaw[T], state State, length func(State, uint), element func(State, *T), hashSlice func(State, IndexRaw[T])) {
	length(state, raw.Len())
	if hashSlice != nil {
		hashSlice(state, raw)
		return
	}
	for i := uint(0); i < raw.Len(); i++ {
		element(state, raw.At(i))
	}
}

func (s *IndexSlice[I, T]) InvertBijectiveMapping(valueIdx Idx[T], keyOps ValueOps[I]) *IndexVec[T, I] {
	check := func(raw IndexRaw[T]) bool {
		var high, low uint64
		for i := uint(0); i < raw.Len(); i++ {
			var carry uint64
			low, carry = bits.Add64(low, uint64(valueIdx.Index(*raw.At(i))), 0)
			high += carry
		}
		return sumIndices(raw.Len(), high, low)
	}
	if s.Config.DebugAssertions && !check(s.Raw) {
		// Rust formats this diagnostic while already processing an assertion
		// panic. A further formatting panic is unrecoverable; ordinary Debug
		// calls outside this failed-assert path keep their normal behavior.
		indexStorageUnwindCleanup(func() { _ = s.DebugWithContext(IndexFormatContext{ProcessingPanic: true, Unwinding: true}) })
		panic("rustcindex: values aren't 0..N in input")
	}
	inverse := IndexVecFromElemNWithConfig(s.Idx.New(0), s.Len(), valueIdx, keyOps, s.Config)
	done := false
	defer func() {
		if p := recover(); p != nil {
			if !done {
				inverse.CloseWithContext(IndexDropContext{true})
			}
			panic(p)
		}
	}()
	indices := s.IterEnumerated()
	for {
		key, value, ok := indices.Next()
		if !ok {
			break
		}
		*inverse.At(*value) = key
	}
	if s.Config.DebugAssertions {
		var high, low uint64
		for i := uint(0); i < inverse.Len(); i++ {
			var carry uint64
			low, carry = bits.Add64(low, uint64(s.Idx.Index(*inverse.Raw.At(i))), 0)
			high += carry
		}
		if !sumIndices(inverse.Len(), high, low) {
			indexStorageUnwindCleanup(func() { _ = s.DebugWithContext(IndexFormatContext{ProcessingPanic: true, Unwinding: true}) })
			panic("rustcindex: values aren't 0..N in result")
		}
	}
	done = true
	return inverse
}
func sumIndices(length uint, high, low uint64) bool {
	n := uint64(length)
	a, b := n, uint64(0)
	if n != 0 {
		b = n - 1
	}
	if a%2 == 0 {
		a /= 2
	} else {
		b /= 2
	}
	expectedHigh, expectedLow := bits.Mul64(a, b)
	return high == expectedHigh && low == expectedLow
}
