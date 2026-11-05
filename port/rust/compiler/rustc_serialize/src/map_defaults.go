// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual Encoder encode_hash_map/encode_hash_set default algorithms from pinned
// serialize.rs. The scoped genuine std sorting helper preserves comparator
// calls and panic permutations; no Go sorting algorithm is substituted.
package rustcserialize

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"strconv"
)

type encodeComparatorOwner[K any] interface {
	move(rustfmt.Context) encodeComparatorOwner[K]
	call(*K, *K, rustfmt.Context) int
	close(rustfmt.Context)
}
type encodeComparatorCell[S, K any] struct {
	state   *rustvec.Owned[S]
	compare func(rustvec.Ref[S], *K, *K) int
}

func (c encodeComparatorCell[S, K]) move(ctx rustfmt.Context) encodeComparatorOwner[K] {
	return encodeComparatorCell[S, K]{state: c.state.MoveWithContext(ctx), compare: c.compare}
}
func (c encodeComparatorCell[S, K]) call(a, b *K, ctx rustfmt.Context) int {
	return c.compare(rustvec.Ref[S]{Value: c.state.Borrow(), Context: ctx}, a, b)
}
func (c encodeComparatorCell[S, K]) close(ctx rustfmt.Context) { c.state.DropWithContext(ctx) }

// EncodeKeyComparator owns the ContextOrd-returned FnMut environment. Captured
// fields, call-local cleanup and complete Drop belong to its source dictionary.
type EncodeKeyComparator[K any] struct{ owner encodeComparatorOwner[K] }

func NewOwnedEncodeKeyComparator[S, K any](state *rustvec.Owned[S], compare func(rustvec.Ref[S], *K, *K) int, ctx rustfmt.Context) EncodeKeyComparator[K] {
	if compare == nil {
		ctx.Panic("rustcserialize: missing ContextOrd FnMut implementation")
	}
	return EncodeKeyComparator[K]{owner: encodeComparatorCell[S, K]{state: state.MoveWithContext(ctx), compare: compare}}
}
func (c EncodeKeyComparator[K]) MoveWithContext(ctx rustfmt.Context) EncodeKeyComparator[K] {
	if c.owner == nil {
		ctx.Panic("rustcserialize: missing ContextOrd result")
	}
	return EncodeKeyComparator[K]{owner: c.owner.move(ctx)}
}
func (c EncodeKeyComparator[K]) CompareWithContext(a, b *K, ctx rustfmt.Context) int {
	defer func() {
		if p := recover(); p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	return c.owner.call(a, b, ctx)
}
func (c EncodeKeyComparator[K]) CloseWithContext(ctx rustfmt.Context) {
	if c.owner != nil {
		c.owner.close(ctx)
	}
}
func encodeComparatorCleanup[K any](c EncodeKeyComparator[K], ctx rustfmt.Context) {
	p := recover()
	if p != nil && ctx.ProcessingPanic {
		ctx.ResumeUnwind(p)
	}
	if p != nil {
		unwind := ctx
		unwind.Unwinding = true
		func() {
			defer func() {
				if recover() != nil {
					unwind.AbortCleanup()
				}
			}()
			c.CloseWithContext(unwind)
		}()
		ctx.ResumeUnwind(p)
	}
	c.CloseWithContext(ctx)
}

// HashMap/Set ops are the statically selected source Encodable and ContextOrd
// dictionaries. Factory borrows encoder as &Self; mutation must be through
// source interior mutability. Override represents an explicit method override.
type HashMapEncodeOps[K, V any] struct {
	Key        func(*K, Encoder, rustfmt.Context)
	Value      func(*V, Encoder, rustfmt.Context)
	Comparator func(Encoder, rustfmt.Context) EncodeKeyComparator[K]
	Override   func(Encoder, *rusthashmap.Map[K, V], rustfmt.Context)
}
type HashSetEncodeOps[K any] struct {
	Key        func(*K, Encoder, rustfmt.Context)
	Comparator func(Encoder, rustfmt.Context) EncodeKeyComparator[K]
	Override   func(Encoder, *rusthashmap.Set[K], rustfmt.Context)
}

func DeterministicHashMapEncode(e Encoder) bool {
	if p, ok := e.(interface{ DeterministicHashMapEncode() bool }); ok {
		return p.DeterministicHashMapEncode()
	}
	return false
}
func EncodeHashMap[K, V any](e Encoder, m *rusthashmap.Map[K, V], ops HashMapEncodeOps[K, V]) {
	ctx := codecContext(e)
	if ops.Override != nil {
		ops.Override(e, m, ctx)
		return
	}
	DefaultEncodeHashMap(e, m, ops)
}
func DefaultEncodeHashMap[K, V any](e Encoder, m *rusthashmap.Map[K, V], ops HashMapEncodeOps[K, V]) {
	ctx := codecContext(e)
	e.EmitUsize(m.Len())
	if DeterministicHashMapEncode(e) {
		values := make([]rusthashmap.BorrowedPair[K, V], 0, m.Len())
		iter := m.Iter(ctx)
		for {
			pair, ok := iter.Next(ctx)
			if !ok {
				break
			}
			values = append(values, pair)
		}
		if ops.Comparator == nil {
			ctx.Panic("rustcserialize: missing ContextOrd dictionary")
		}
		cmp := ops.Comparator(e, ctx).MoveWithContext(ctx)
		func() {
			defer encodeComparatorCleanup(cmp, ctx)
			pointerBytes := uint(strconv.IntSize / 8)
			rustsort.Stable(values, rustsort.Type{Size: 2 * pointerBytes, Align: pointerBytes, Freeze: true, Copy: true}, func(a, b *rusthashmap.BorrowedPair[K, V], context rustfmt.Context) bool {
				return cmp.CompareWithContext(a.Key, b.Key, context) < 0
			}, ctx)
		}()
		for _, pair := range values {
			ops.Key(pair.Key, e, ctx)
			ops.Value(pair.Value, e, ctx)
		}
	} else {
		iter := m.Iter(ctx)
		for {
			pair, ok := iter.Next(ctx)
			if !ok {
				break
			}
			ops.Key(pair.Key, e, ctx)
			ops.Value(pair.Value, e, ctx)
		}
	}
}
func EncodeHashSet[K any](e Encoder, m *rusthashmap.Set[K], ops HashSetEncodeOps[K]) {
	ctx := codecContext(e)
	if ops.Override != nil {
		ops.Override(e, m, ctx)
		return
	}
	DefaultEncodeHashSet(e, m, ops)
}
func DefaultEncodeHashSet[K any](e Encoder, m *rusthashmap.Set[K], ops HashSetEncodeOps[K]) {
	ctx := codecContext(e)
	e.EmitUsize(m.Len())
	if DeterministicHashMapEncode(e) {
		values := make([]*K, 0, m.Len())
		iter := m.Iter(ctx)
		for {
			key, ok := iter.Next(ctx)
			if !ok {
				break
			}
			values = append(values, key)
		}
		if ops.Comparator == nil {
			ctx.Panic("rustcserialize: missing ContextOrd dictionary")
		}
		cmp := ops.Comparator(e, ctx).MoveWithContext(ctx)
		func() {
			defer encodeComparatorCleanup(cmp, ctx)
			pointerBytes := uint(strconv.IntSize / 8)
			rustsort.Stable(values, rustsort.Type{Size: pointerBytes, Align: pointerBytes, Freeze: true, Copy: true}, func(a, b **K, context rustfmt.Context) bool { return cmp.CompareWithContext(*a, *b, context) < 0 }, ctx)
		}()
		for _, key := range values {
			ops.Key(key, e, ctx)
		}
	} else {
		iter := m.Iter(ctx)
		for {
			key, ok := iter.Next(ctx)
			if !ok {
				break
			}
			ops.Key(key, e, ctx)
		}
	}
}
