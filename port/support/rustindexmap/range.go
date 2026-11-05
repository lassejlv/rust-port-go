// SPDX-License-Identifier: MIT OR Apache-2.0
// Used RangeBounds operations and exact simplify_range/try_simplify_range from
// indexmap2.14.0 src/util.rs. No whole standard RangeBounds port is claimed.
package rustindexmap

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type BoundKind uint8

const (
	Unbounded BoundKind = iota
	Included
	Excluded
)

// Bound borrows the actual source usize cell for Included/Excluded. The pointer
// remains a shared source borrow, valid only during its RangeBounds lifetime.
type Bound struct {
	Kind  BoundKind
	Value *uint
}
type BoundValue struct {
	Kind  BoundKind
	Value uint
}
type RangeLimits struct{ Start, End uint }
type RangeOps[S any] struct{ StartBound, EndBound func(rustvec.Ref[S]) Bound }
type rangeState interface {
	live() bool
	move(Context) rangeState
	start(Context) Bound
	end(Context) Bound
	close(Context)
}
type rangeOwned[S any] struct {
	value *rustvec.Owned[S]
	ops   RangeOps[S]
}

func (r rangeOwned[S]) live() bool { return r.value.IsLive() }
func (r rangeOwned[S]) move(c Context) rangeState {
	return rangeOwned[S]{r.value.MoveWithContext(c), r.ops}
}
func (r rangeOwned[S]) start(c Context) Bound {
	return builderInvoke(c, func() Bound { return r.ops.StartBound(rustvec.Ref[S]{Value: r.value.Borrow(), Context: c}) })
}
func (r rangeOwned[S]) end(c Context) Bound {
	return builderInvoke(c, func() Bound { return r.ops.EndBound(rustvec.Ref[S]{Value: r.value.Borrow(), Context: c}) })
}
func (r rangeOwned[S]) close(c Context) { r.value.DropWithContext(c) }

// Range owns the actual arbitrary non-Copy range state. Its static operations
// own no hidden destructible captures; complete fields/cleanup belong in S's
// Owned destructor. Bounds borrow shared state, permitting only actual source
// interior mutation. Descriptor copies are not implicit Clone or owning copies.
type Range struct{ state rangeState }

func NewOwnedRange[S any](value *rustvec.Owned[S], ops RangeOps[S], contexts ...Context) Range {
	c := current(contexts)
	if !value.IsLive() || ops.StartBound == nil || ops.EndBound == nil {
		c.Panic("rustindexmap: incomplete owned RangeBounds witness")
	}
	return Range{rangeOwned[S]{value.MoveWithContext(c), ops}}
}
func (r Range) check(c Context) {
	if r.state == nil || !r.state.live() {
		c.Panic("rustindexmap: consumed range state")
	}
}
func (r Range) MoveWithContext(c Context) Range { r.check(c); return Range{r.state.move(c)} }
func (r Range) Close(c Context) {
	if r.state != nil {
		r.state.close(c)
	}
}
func NumericRange(start, end BoundValue) Range {
	type bounds struct{ start, end BoundValue }
	return NewOwnedRange(rustvec.Own(bounds{start, end}), RangeOps[bounds]{StartBound: func(r rustvec.Ref[bounds]) Bound { return Bound{r.Value.start.Kind, &r.Value.start.Value} }, EndBound: func(r rustvec.Ref[bounds]) Bound { return Bound{r.Value.end.Kind, &r.Value.end.Value} }})
}

const (
	IndexMapRangeStartPanicSite = "indexmap.simplify_range.start.caller"
	IndexMapRangeEndPanicSite   = "indexmap.simplify_range.end.caller"
	IndexMapRangeOrderPanicSite = "indexmap.simplify_range.order.caller"
)

func boundValue(bound Bound, c Context) uint {
	if bound.Kind != Included && bound.Kind != Excluded || bound.Value == nil {
		c.Panic("rustindexmap: invalid source Bound witness")
	}
	return *bound.Value
}
func boundDebug(bound Bound, c Context) string {
	switch bound.Kind {
	case Unbounded:
		return "Unbounded"
	case Included:
		return fmt.Sprintf("Included(%d)", boundValue(bound, c))
	case Excluded:
		return fmt.Sprintf("Excluded(%d)", boundValue(bound, c))
	default:
		c.Panic("rustindexmap: invalid source Bound witness")
		return ""
	}
}
func rangePanic(c Context, site string, render func(Context) any) {
	c.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: c.SourcePanicLocation(site), CanUnwind: true}, render)
}
func simplifyRangeOwned(input Range, length uint, panics bool, c Context) (limits RangeLimits, ok bool) {
	rangeValue := input.MoveWithContext(c)
	defer cleanupCall(c, rangeValue.Close)
	start := rangeValue.state.start(c)
	switch start.Kind {
	case Unbounded:
		limits.Start = 0
	case Included:
		limits.Start = boundValue(start, c)
		if limits.Start > length {
			if panics {
				rangePanic(c, IndexMapRangeStartPanicSite, func(Context) any {
					return fmt.Sprintf("range start index %d out of range for slice of length %d", *start.Value, length)
				})
			}
			return RangeLimits{}, false
		}
	case Excluded:
		value := boundValue(start, c)
		if value >= length {
			if panics {
				rangePanic(c, IndexMapRangeStartPanicSite, func(Context) any {
					return fmt.Sprintf("range start index %d out of range for slice of length %d", *start.Value, length)
				})
			}
			return RangeLimits{}, false
		}
		limits.Start = value + 1
	default:
		c.Panic("rustindexmap: invalid source Bound witness")
	}
	end := rangeValue.state.end(c)
	switch end.Kind {
	case Unbounded:
		limits.End = length
	case Excluded:
		limits.End = boundValue(end, c)
		if limits.End > length {
			if panics {
				rangePanic(c, IndexMapRangeEndPanicSite, func(Context) any {
					return fmt.Sprintf("range end index %d out of range for slice of length %d", *end.Value, length)
				})
			}
			return RangeLimits{}, false
		}
	case Included:
		value := boundValue(end, c)
		if value >= length {
			if panics {
				rangePanic(c, IndexMapRangeEndPanicSite, func(Context) any {
					return fmt.Sprintf("range end index %d out of range for slice of length %d", *end.Value, length)
				})
			}
			return RangeLimits{}, false
		}
		limits.End = value + 1
	default:
		c.Panic("rustindexmap: invalid source Bound witness")
	}
	if limits.Start > limits.End {
		if panics {
			againStart := rangeValue.state.start(c)
			againEnd := rangeValue.state.end(c)
			rangePanic(c, IndexMapRangeOrderPanicSite, func(processing Context) any {
				return fmt.Sprintf("range start index %s should be <= range end index %s", boundDebug(againStart, processing), boundDebug(againEnd, processing))
			})
		}
		return RangeLimits{}, false
	}
	return limits, true
}
func (s Slice[K, V]) GetRangeOwned(rangeValue Range, contexts ...Context) (Slice[K, V], bool) {
	c := current(contexts)
	limits, ok := simplifyRangeOwned(rangeValue, s.Len(), false, c)
	if !ok {
		return Slice[K, V]{}, false
	}
	return Slice[K, V]{entries: s.entries[limits.Start:limits.End], config: s.config}, true
}
func (s Slice[K, V]) GetRangeMutOwned(rangeValue Range, contexts ...Context) (Slice[K, V], bool) {
	c := current(contexts)
	if !s.mutable {
		c.Panic("rustindexmap: mutable range through shared Slice")
	}
	value, ok := s.GetRangeOwned(rangeValue, c)
	value.mutable = ok
	return value, ok
}
func (m *Map[K, V]) GetRangeOwned(rangeValue Range, contexts ...Context) (Slice[K, V], bool) {
	return m.AsSlice(contexts...).GetRangeOwned(rangeValue, contexts...)
}
func (m *Map[K, V]) GetRangeMutOwned(rangeValue Range, contexts ...Context) (Slice[K, V], bool) {
	return m.AsMutSlice(contexts...).GetRangeMutOwned(rangeValue, contexts...)
}
