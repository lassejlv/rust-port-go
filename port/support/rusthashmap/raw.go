// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped backing operations from hashbrown 0.17.1 src/raw.rs, as shipped with
// Rust 1.99.1. This package does not claim a port of the complete hashbrown crate.
package rusthashmap

import (
	"math/bits"
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const (
	emptyTag   byte = 0xff
	deletedTag byte = 0x80
)

// Context is explicit caller unwind state. A callback that initiates a source
// panic must use Context.Panic, including when ProcessingPanic is true.
type Context = rustfmt.Context

// Layout is the size and alignment of the Rust element, rather than the Go
// representation. Hashbrown's small-table bucket count depends on this size.
// Source type dictionaries must supply it even for zero-sized Rust values.
type Layout struct{ Size, Align uint }

// Profile records the source target's pointer and SIMD control-group widths.
// DefaultProfile is the recorded x86-64 Rust reference configuration.
// The scalar eight-byte profile is little-endian; a big-endian target requires
// a distinct source profile rather than using the Go host's byte order.
type Profile struct {
	PointerBits uint
	GroupWidth  uint
}

func DefaultProfile() Profile { return Profile{64, 16} }

func (p Profile) validate(c Context) {
	if (p.PointerBits != 32 && p.PointerBits != 64) || (p.GroupWidth != 8 && p.GroupWidth != 16) {
		c.Panic("rusthashmap: invalid source target profile")
	}
}

func (l Layout) validate(c Context) {
	if l.Align == 0 || l.Align&(l.Align-1) != 0 || l.Size%l.Align != 0 {
		c.Panic("rusthashmap: invalid Rust element layout")
	}
}

// ElementOps are the complete source operations used by this backing table.
// All pointers are temporary borrows into live cells. A callback must not retain
// one across relocation, removal, Drop, or another exclusive table operation.
// TrivialClone is an unsafe witness of Rust's TrivialClone specialization; it
// must not be inferred merely from a Go type being comparable or pointer-free.
type ElementOps[T any] struct {
	Hash         func(*T, Context) uint64
	Clone        func(*T, Context) T
	Drop         func(*T, Context)
	TrivialClone bool
}

// RawTable is the scoped SwissTable substrate used by Map and Set. Its explicit
// Close, Clear, Clone and ownership adapters preserve Rust destruction; Go GC is
// never used as an element destructor. The zero Go value is not initialized.
type RawTable[T any] struct {
	ctrl       []byte
	values     []T
	items      uint
	growthLeft uint
	layout     Layout
	profile    Profile
	ops        ElementOps[T]
	live       bool
}

func NewRawTable[T any](layout Layout, profile Profile, ops ElementOps[T]) *RawTable[T] {
	return NewRawTableWithCapacity(0, layout, profile, ops, Context{})
}

func NewRawTableWithCapacity[T any](capacity uint, layout Layout, profile Profile, ops ElementOps[T], c Context) *RawTable[T] {
	layout.validate(c)
	profile.validate(c)
	t := &RawTable[T]{layout: layout, profile: profile, ops: ops, live: true}
	if capacity != 0 {
		t.allocate(t.capacityToBuckets(capacity, c), c)
	}
	return t
}

func (t *RawTable[T]) check(c Context) {
	if t == nil || !t.live {
		c.Panic("rusthashmap: use of an uninitialized or consumed table")
	}
}

func (t *RawTable[T]) wordMax() uint64 {
	if t.profile.PointerBits == 32 {
		return 1<<32 - 1
	}
	return ^uint64(0)
}

func (t *RawTable[T]) capacityToBuckets(capacity uint, c Context) uint {
	if capacity < 15 {
		minimum := uint(3)
		if t.profile.GroupWidth == 16 && t.layout.Size <= 1 {
			minimum = 14
		} else if (t.profile.GroupWidth == 16 && t.layout.Size <= 3) || (t.profile.GroupWidth == 8 && t.layout.Size <= 1) {
			minimum = 7
		}
		if capacity < minimum {
			capacity = minimum
		}
		if capacity < 4 {
			return 4
		}
		if capacity < 8 {
			return 8
		}
		return 16
	}
	if uint64(capacity) > t.wordMax()/8 {
		c.Panic("Hash table capacity overflow")
	}
	adjusted := capacity * 8 / 7
	return uint(1) << uint(bits.Len64(uint64(adjusted-1)))
}

func capacityForBuckets(n uint) uint {
	if n < 8 {
		return n - 1
	}
	return n / 8 * 7
}

func (t *RawTable[T]) allocate(n uint, c Context) {
	align := t.layout.Align
	if align < t.profile.GroupWidth {
		align = t.profile.GroupWidth
	}
	max := t.wordMax() >> 1
	// This is RawTable's checked allocation layout, including the control tail.
	if uint64(t.layout.Size) > max/uint64(n) {
		c.Panic("Hash table capacity overflow")
	}
	dataSize := uint64(t.layout.Size) * uint64(n)
	ctrlOffset := (dataSize + uint64(align) - 1) &^ (uint64(align) - 1)
	if ctrlOffset > max || uint64(n)+uint64(t.profile.GroupWidth) > max-ctrlOffset || ctrlOffset+uint64(n)+uint64(t.profile.GroupWidth) > max-(uint64(align)-1) {
		c.Panic("Hash table capacity overflow")
	}
	if uint64(n)+uint64(t.profile.GroupWidth) > uint64(^uint(0)>>1) || (strconv.IntSize == 32 && uint64(n) > 1<<31-1) {
		c.Panic("rusthashmap: source allocation exceeds Go host address space")
	}
	t.ctrl = make([]byte, n+t.profile.GroupWidth)
	t.values = make([]T, n)
	t.clearMetadata()
}

func (t *RawTable[T]) clearMetadata() {
	for i := range t.ctrl {
		t.ctrl[i] = emptyTag
	}
	t.items = 0
	if len(t.values) == 0 {
		t.growthLeft = 0
	} else {
		t.growthLeft = capacityForBuckets(uint(len(t.values)))
	}
}

func (t *RawTable[T]) Len() uint     { t.check(Context{}); return t.items }
func (t *RawTable[T]) IsEmpty() bool { return t.Len() == 0 }
func (t *RawTable[T]) Capacity() uint {
	t.check(Context{})
	return t.items + t.growthLeft
}

func (t *RawTable[T]) tag(hash uint64) byte {
	return byte((hash >> (t.profile.PointerBits - 7)) & 0x7f)
}

func (t *RawTable[T]) ctrlAt(index uint) byte {
	if len(t.ctrl) == 0 {
		return emptyTag
	}
	return t.ctrl[index]
}

func (t *RawTable[T]) matchTag(pos uint, tag byte) uint16 {
	if t.profile.GroupWidth == 8 {
		var word uint64
		for i := uint(0); i < 8; i++ {
			word |= uint64(t.ctrlAt(pos+i)) << (i * 8)
		}
		cmp := word ^ (uint64(tag) * 0x0101010101010101)
		mask := (cmp - 0x0101010101010101) &^ cmp & 0x8080808080808080
		var result uint16
		for i := uint(0); i < 8; i++ {
			if mask&(uint64(0x80)<<(i*8)) != 0 {
				result |= 1 << i
			}
		}
		return result
	}
	var result uint16
	for i := uint(0); i < t.profile.GroupWidth; i++ {
		if t.ctrlAt(pos+i) == tag {
			result |= 1 << i
		}
	}
	return result
}

func (t *RawTable[T]) setCtrl(index uint, tag byte) {
	mask := uint(len(t.values) - 1)
	t.ctrl[index] = tag
	// Small tables have an EMPTY gap before their mirrored control bytes.
	mirror := ((index - t.profile.GroupWidth) & mask) + t.profile.GroupWidth
	t.ctrl[mirror] = tag
}

func (t *RawTable[T]) fixInsert(index uint) uint {
	if t.ctrlAt(index)&0x80 == 0 {
		for i := uint(0); i < t.profile.GroupWidth; i++ {
			if t.ctrlAt(i)&0x80 != 0 {
				return i
			}
		}
	}
	return index
}

func (t *RawTable[T]) findInsert(hash uint64) uint {
	if len(t.values) == 0 {
		return 0
	}
	mask := uint(len(t.values) - 1)
	pos, stride := uint(hash)&mask, uint(0)
	for {
		for bit := uint(0); bit < t.profile.GroupWidth; bit++ {
			if t.ctrlAt(pos+bit)&0x80 != 0 {
				return t.fixInsert((pos + bit) & mask)
			}
		}
		stride += t.profile.GroupWidth
		pos = (pos + stride) & mask
	}
}

// Find preserves group scanning and equality-callback order, including matches
// that appear after an EMPTY byte in the same SIMD group.
func (t *RawTable[T]) Find(hash uint64, equal func(*T) bool, c Context) (uint, bool) {
	t.check(c)
	if len(t.values) == 0 {
		return 0, false
	}
	mask := uint(len(t.values) - 1)
	pos, stride, tag := uint(hash)&mask, uint(0), t.tag(hash)
	for {
		hasEmpty := false
		matches := t.matchTag(pos, tag)
		for bit := uint(0); bit < t.profile.GroupWidth; bit++ {
			control := t.ctrlAt(pos + bit)
			if matches&(1<<bit) != 0 && equal(&t.values[(pos+bit)&mask]) {
				return (pos + bit) & mask, true
			}
			hasEmpty = hasEmpty || control == emptyTag
		}
		if hasEmpty {
			return 0, false
		}
		stride += t.profile.GroupWidth
		pos = (pos + stride) & mask
	}
}

// FindOrInsert reserves BEFORE checking equality, as std HashMap::insert does.
// Even replacement of an existing key can therefore rehash the entire table.
func (t *RawTable[T]) FindOrInsert(hash uint64, equal func(*T) bool, c Context) (index uint, occupied bool) {
	t.Reserve(1, c)
	mask := uint(len(t.values) - 1)
	pos, stride, tag := uint(hash)&mask, uint(0), t.tag(hash)
	var insert uint
	hasInsert := false
	for {
		hasEmpty := false
		matches := t.matchTag(pos, tag)
		for bit := uint(0); bit < t.profile.GroupWidth; bit++ {
			control := t.ctrlAt(pos + bit)
			if matches&(1<<bit) != 0 && equal(&t.values[(pos+bit)&mask]) {
				return (pos + bit) & mask, true
			}
			hasEmpty = hasEmpty || control == emptyTag
		}
		if !hasInsert {
			for bit := uint(0); bit < t.profile.GroupWidth; bit++ {
				if t.ctrlAt(pos+bit)&0x80 != 0 {
					insert, hasInsert = (pos+bit)&mask, true
					break
				}
			}
		}
		if hasEmpty {
			return t.fixInsert(insert), false
		}
		stride += t.profile.GroupWidth
		pos = (pos + stride) & mask
	}
}

func (t *RawTable[T]) At(index uint, c Context) *T {
	t.check(c)
	if index >= uint(len(t.values)) || t.ctrl[index]&0x80 != 0 {
		c.Panic("rusthashmap: invalid or vacant bucket")
	}
	return &t.values[index]
}

// InsertAt requires the still-valid vacant position from FindOrInsert.
func (t *RawTable[T]) InsertAt(hash uint64, index uint, value T, c Context) *T {
	t.check(c)
	if index >= uint(len(t.values)) || t.ctrl[index]&0x80 == 0 {
		c.Panic("rusthashmap: invalid occupied insertion bucket")
	}
	if t.ctrl[index] == emptyTag {
		t.growthLeft--
	}
	t.setCtrl(index, t.tag(hash))
	t.items++
	t.values[index] = value
	return &t.values[index]
}

func (t *RawTable[T]) insertUnique(hash uint64, value T, c Context) (uint, *T) {
	t.check(c)
	inserted := false
	defer func() {
		if p := recover(); p != nil {
			if !inserted && t.ops.Drop != nil {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(func() { t.ops.Drop(&value, unwind) })
			}
			panic(p)
		}
	}()
	index := t.findInsert(hash)
	if t.growthLeft == 0 && t.ctrlAt(index) == emptyTag {
		t.Reserve(1, c)
		index = t.findInsert(hash)
	}
	result := t.InsertAt(hash, index, value, c)
	inserted = true
	return index, result
}

func (t *RawTable[T]) erase(index uint) {
	mask := uint(len(t.values) - 1)
	before := (index - t.profile.GroupWidth) & mask
	leading, trailing := uint(0), uint(0)
	for i := t.profile.GroupWidth; i > 0 && t.ctrlAt(before+i-1) != emptyTag; i-- {
		leading++
	}
	for trailing < t.profile.GroupWidth && t.ctrlAt(index+trailing) != emptyTag {
		trailing++
	}
	if leading+trailing >= t.profile.GroupWidth {
		t.setCtrl(index, deletedTag)
	} else {
		t.growthLeft++
		t.setCtrl(index, emptyTag)
	}
	t.items--
}

func (t *RawTable[T]) RemoveAt(index uint, c Context) T {
	t.At(index, c)
	t.erase(index)
	value := t.values[index]
	var zero T
	t.values[index] = zero
	return value
}

func (t *RawTable[T]) Reserve(additional uint, c Context) {
	t.check(c)
	if additional <= t.growthLeft {
		return
	}
	if uint64(additional) > t.wordMax()-uint64(t.items) {
		c.Panic("Hash table capacity overflow")
	}
	newItems := t.items + additional
	fullCapacity := uint(0)
	if len(t.values) != 0 {
		fullCapacity = capacityForBuckets(uint(len(t.values)))
	}
	if newItems <= fullCapacity/2 {
		t.rehashInPlace(c)
		return
	}
	capacity := fullCapacity + 1
	if newItems > capacity {
		capacity = newItems
	}
	t.resize(capacity, c)
}

func (t *RawTable[T]) resize(capacity uint, c Context) {
	next := NewRawTableWithCapacity(capacity, t.layout, t.profile, t.ops, c)
	// Values are only borrowed/copied until every hash succeeds. On panic the
	// original table is untouched and temporary copies have no ownership.
	for i := range t.values {
		if t.ctrl[i]&0x80 == 0 {
			hash := t.ops.Hash(&t.values[i], c)
			index := next.findInsert(hash)
			next.setCtrl(index, next.tag(hash))
			next.values[index] = t.values[i]
		}
	}
	next.items = t.items
	next.growthLeft -= t.items
	t.ctrl, t.values, t.items, t.growthLeft = next.ctrl, next.values, next.items, next.growthLeft
}

func (t *RawTable[T]) rehashInPlace(c Context) {
	for i := range t.values {
		if t.ctrl[i]&0x80 == 0 {
			t.setCtrl(uint(i), deletedTag)
		} else {
			t.setCtrl(uint(i), emptyTag)
		}
	}
	success := false
	defer func() {
		if success {
			return
		}
		p := recover()
		unwind := c
		unwind.Unwinding = true
		for i := range t.values {
			if t.ctrl[i] == deletedTag {
				t.setCtrl(uint(i), emptyTag)
				if t.ops.Drop != nil {
					cleanupOnPanic(func() { t.ops.Drop(&t.values[i], unwind) })
				}
				var zero T
				t.values[i] = zero
				t.items--
			}
		}
		t.growthLeft = capacityForBuckets(uint(len(t.values))) - t.items
		if p != nil {
			panic(p)
		}
	}()
	mask := uint(len(t.values) - 1)
	for i := range t.values {
		if t.ctrl[i] != deletedTag {
			continue
		}
		for {
			hash := t.ops.Hash(&t.values[i], c)
			index := t.findInsert(hash)
			pos := uint(hash) & mask
			oldGroup := (uint(i) - pos) & mask / t.profile.GroupWidth
			newGroup := (index - pos) & mask / t.profile.GroupWidth
			if oldGroup == newGroup {
				t.setCtrl(uint(i), t.tag(hash))
				break
			}
			previous := t.ctrl[index]
			t.setCtrl(index, t.tag(hash))
			if previous == emptyTag {
				t.setCtrl(uint(i), emptyTag)
				t.values[index] = t.values[i]
				var zero T
				t.values[i] = zero
				break
			}
			t.values[i], t.values[index] = t.values[index], t.values[i]
		}
	}
	t.growthLeft = capacityForBuckets(uint(len(t.values))) - t.items
	success = true
}

// Clear resets metadata even if one destructor panics; remaining values are
// leaked in that case, matching RawTable::clear's scope guard.
func (t *RawTable[T]) Clear(c Context) {
	t.check(c)
	if t.items == 0 {
		return
	}
	defer t.clearMetadata()
	if t.ops.Drop != nil {
		for i := range t.values {
			if t.ctrl[i]&0x80 == 0 {
				t.ops.Drop(&t.values[i], c)
			}
		}
	}
	clear(t.values)
}

// Close destroys the owned table once. A throwing element destructor leaks the
// not-yet-dropped elements, as hashbrown's Drop implementation does.
func (t *RawTable[T]) Close(c Context) {
	if t == nil || !t.live {
		return
	}
	t.live = false
	if t.ops.Drop != nil {
		for i := range t.values {
			if t.ctrl[i]&0x80 == 0 {
				t.ops.Drop(&t.values[i], c)
			}
		}
	}
	t.values, t.ctrl = nil, nil
}
