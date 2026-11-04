// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/interval.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See licenses in port/rust.
package rustcindex

// IntervalConfig selects the upstream crate's independently configurable
// overflow checks and debug assertions. Ordinary constructors enable both.
type IntervalConfig struct {
	OverflowChecks  bool
	DebugAssertions bool
}

// IntervalOrdering preserves PartialOrd's unordered result as well as the three
// ordered results. The operator callbacks below may be overridden independently.
type IntervalOrdering int8

const (
	IntervalLess      IntervalOrdering = -1
	IntervalEqual     IntervalOrdering = 0
	IntervalGreater   IntervalOrdering = 1
	IntervalUnordered IntervalOrdering = 2
)

// IntervalStep supplies the public Rust Step/Clone/PartialEq/PartialOrd contracts
// used by this file. Keys need not be Go-comparable. A nil CloneValue means a
// Rust Copy-like shallow value copy. Other unused callbacks need not be supplied;
// operations requiring them propagate callback panics without substituting an
// integer-only implementation. Unsafe stepping callbacks retain Rust's caller
// preconditions. This adapter does not claim to port the core Step source file.
type IntervalStep[I any] struct {
	TypeName                                            string
	CloneValue                                          func(I) I
	EqualFn                                             func(I, I) bool
	PartialCompareFn                                    func(I, I) IntervalOrdering
	LessFn, LessEqualFn, GreaterFn, GreaterEqualFn      func(I, I) bool
	StepsBetweenFn                                      func(I, I) (uint, uint, bool)
	ForwardCheckedFn, BackwardCheckedFn                 func(I, uint) (I, bool)
	ForwardOverflowingFn, BackwardOverflowingFn         func(I, uint) (I, bool)
	ForwardOverride, BackwardOverride                   func(I, uint) I
	ForwardUncheckedOverride, BackwardUncheckedOverride func(I, uint) I
}

func (step IntervalStep[I]) Clone(value I) I {
	if step.CloneValue != nil {
		return step.CloneValue(value)
	}
	return value
}
func (step IntervalStep[I]) Equal(a, b I) bool { return step.EqualFn(a, b) }
func (step IntervalStep[I]) Less(a, b I) bool {
	if step.LessFn != nil {
		return step.LessFn(a, b)
	}
	return step.PartialCompareFn(a, b) == IntervalLess
}
func (step IntervalStep[I]) LessEqual(a, b I) bool {
	if step.LessEqualFn != nil {
		return step.LessEqualFn(a, b)
	}
	order := step.PartialCompareFn(a, b)
	return order == IntervalLess || order == IntervalEqual
}
func (step IntervalStep[I]) Greater(a, b I) bool {
	if step.GreaterFn != nil {
		return step.GreaterFn(a, b)
	}
	return step.PartialCompareFn(a, b) == IntervalGreater
}
func (step IntervalStep[I]) GreaterEqual(a, b I) bool {
	if step.GreaterEqualFn != nil {
		return step.GreaterEqualFn(a, b)
	}
	order := step.PartialCompareFn(a, b)
	return order == IntervalGreater || order == IntervalEqual
}
func (step IntervalStep[I]) StepsBetween(a, b I) (uint, uint, bool) { return step.StepsBetweenFn(a, b) }
func (step IntervalStep[I]) ForwardChecked(a I, n uint) (I, bool)   { return step.ForwardCheckedFn(a, n) }
func (step IntervalStep[I]) BackwardChecked(a I, n uint) (I, bool) {
	return step.BackwardCheckedFn(a, n)
}
func (step IntervalStep[I]) ForwardOverflowing(a I, n uint) (I, bool) {
	return step.ForwardOverflowingFn(a, n)
}
func (step IntervalStep[I]) BackwardOverflowing(a I, n uint) (I, bool) {
	return step.BackwardOverflowingFn(a, n)
}
func (step IntervalStep[I]) Forward(a I, n uint) I {
	if step.ForwardOverride != nil {
		return step.ForwardOverride(a, n)
	}
	value, ok := step.ForwardChecked(a, n)
	if !ok {
		panic("overflow in Step::forward")
	}
	return value
}
func (step IntervalStep[I]) Backward(a I, n uint) I {
	if step.BackwardOverride != nil {
		return step.BackwardOverride(a, n)
	}
	value, ok := step.BackwardChecked(a, n)
	if !ok {
		panic("overflow in Step::backward")
	}
	return value
}
func (step IntervalStep[I]) ForwardUnchecked(a I, n uint) I {
	if step.ForwardUncheckedOverride != nil {
		return step.ForwardUncheckedOverride(a, n)
	}
	return step.Forward(a, n)
}
func (step IntervalStep[I]) BackwardUnchecked(a I, n uint) I {
	if step.BackwardUncheckedOverride != nil {
		return step.BackwardUncheckedOverride(a, n)
	}
	return step.Backward(a, n)
}

func U32IntervalStep() IntervalStep[uint32] {
	return U32IntervalStepWithConfig(IntervalConfig{OverflowChecks: true, DebugAssertions: true})
}
func U32IntervalStepWithConfig(config IntervalConfig) IntervalStep[uint32] {
	step := IntervalStep[uint32]{TypeName: "u32", EqualFn: func(a, b uint32) bool { return a == b },
		PartialCompareFn: func(a, b uint32) IntervalOrdering {
			if a < b {
				return IntervalLess
			}
			if a > b {
				return IntervalGreater
			}
			return IntervalEqual
		},
		StepsBetweenFn: func(a, b uint32) (uint, uint, bool) {
			if a > b {
				return 0, 0, false
			}
			n := uint(b - a)
			return n, n, true
		},
		ForwardCheckedFn: func(a uint32, n uint) (uint32, bool) {
			if n > uint(^uint32(0))-uint(a) {
				return 0, false
			}
			return a + uint32(n), true
		},
		BackwardCheckedFn: func(a uint32, n uint) (uint32, bool) {
			if n > uint(a) {
				return 0, false
			}
			return a - uint32(n), true
		},
		ForwardOverflowingFn:  func(a uint32, n uint) (uint32, bool) { return a + uint32(n), n > uint(^uint32(0))-uint(a) },
		BackwardOverflowingFn: func(a uint32, n uint) (uint32, bool) { return a - uint32(n), n > uint(a) },
	}
	step.ForwardOverride = func(a uint32, n uint) uint32 {
		if config.OverflowChecks && n > uint(^uint32(0))-uint(a) {
			panic("u32 step overflow")
		}
		return a + uint32(n)
	}
	step.BackwardOverride = func(a uint32, n uint) uint32 {
		if config.OverflowChecks && n > uint(a) {
			panic("u32 step overflow")
		}
		return a - uint32(n)
	}
	step.ForwardUncheckedOverride = func(a uint32, n uint) uint32 { return a + uint32(n) }
	step.BackwardUncheckedOverride = func(a uint32, n uint) uint32 { return a - uint32(n) }
	return step
}
func UsizeIntervalStep() IntervalStep[uint] {
	return UsizeIntervalStepWithConfig(IntervalConfig{OverflowChecks: true, DebugAssertions: true})
}
func UsizeIntervalStepWithConfig(config IntervalConfig) IntervalStep[uint] {
	step := IntervalStep[uint]{TypeName: "usize", EqualFn: func(a, b uint) bool { return a == b },
		PartialCompareFn: func(a, b uint) IntervalOrdering {
			if a < b {
				return IntervalLess
			}
			if a > b {
				return IntervalGreater
			}
			return IntervalEqual
		},
		StepsBetweenFn: func(a, b uint) (uint, uint, bool) {
			if a > b {
				return 0, 0, false
			}
			return b - a, b - a, true
		},
		ForwardCheckedFn: func(a, n uint) (uint, bool) {
			if n > ^uint(0)-a {
				return 0, false
			}
			return a + n, true
		},
		BackwardCheckedFn: func(a, n uint) (uint, bool) {
			if n > a {
				return 0, false
			}
			return a - n, true
		},
		ForwardOverflowingFn:  func(a, n uint) (uint, bool) { return a + n, n > ^uint(0)-a },
		BackwardOverflowingFn: func(a, n uint) (uint, bool) { return a - n, n > a },
	}
	step.ForwardOverride = func(a, n uint) uint {
		if config.OverflowChecks && n > ^uint(0)-a {
			panic("usize step overflow")
		}
		return a + n
	}
	step.BackwardOverride = func(a, n uint) uint {
		if config.OverflowChecks && n > a {
			panic("usize step overflow")
		}
		return a - n
	}
	step.ForwardUncheckedOverride = func(a, n uint) uint { return a + n }
	step.BackwardUncheckedOverride = func(a, n uint) uint { return a - n }
	return step
}

type IntervalBoundKind uint8

const (
	IntervalUnbounded IntervalBoundKind = iota
	IntervalIncluded
	IntervalExcluded
)

type IntervalBound[I any] struct {
	Kind  IntervalBoundKind
	Value I
}

// IntervalRangeBounds is an extensible RangeBounds + Clone adapter. Clone,
// start_bound and end_bound are separate calls, in the source's original order.
// Getter results model borrowed bound values; callbacks retain ownership of
// any metadata referenced by those values.
type IntervalRangeBounds[I any] struct {
	CloneFn                                        func() IntervalRangeBounds[I]
	StartBoundFn, EndBoundFn                       func() IntervalBound[I]
	CloneWithContextFn                             func(IntervalPanicContext) IntervalRangeBounds[I]
	StartBoundWithContextFn, EndBoundWithContextFn func(IntervalPanicContext) IntervalBound[I]
	Close                                          func()
	CloseWithContext                               func(IntervalPanicContext)
}

func (bounds IntervalRangeBounds[I]) Clone() IntervalRangeBounds[I] {
	return bounds.CloneWithContext(IntervalPanicContext{})
}
func (bounds IntervalRangeBounds[I]) CloneWithContext(context IntervalPanicContext) IntervalRangeBounds[I] {
	if bounds.CloneWithContextFn != nil {
		return bounds.CloneWithContextFn(context)
	}
	if bounds.CloneFn != nil {
		return bounds.CloneFn()
	}
	return bounds
}
func (bounds IntervalRangeBounds[I]) StartBound() IntervalBound[I] {
	return bounds.StartBoundWithContext(IntervalPanicContext{})
}
func (bounds IntervalRangeBounds[I]) StartBoundWithContext(context IntervalPanicContext) IntervalBound[I] {
	if bounds.StartBoundWithContextFn != nil {
		return bounds.StartBoundWithContextFn(context)
	}
	return bounds.StartBoundFn()
}
func (bounds IntervalRangeBounds[I]) EndBound() IntervalBound[I] {
	return bounds.EndBoundWithContext(IntervalPanicContext{})
}
func (bounds IntervalRangeBounds[I]) EndBoundWithContext(context IntervalPanicContext) IntervalBound[I] {
	if bounds.EndBoundWithContextFn != nil {
		return bounds.EndBoundWithContextFn(context)
	}
	return bounds.EndBoundFn()
}

func IntervalBounds[I any](start, end IntervalBound[I], cloneValue ...func(I) I) IntervalRangeBounds[I] {
	clone := func(value I) I {
		if len(cloneValue) != 0 && cloneValue[0] != nil {
			return cloneValue[0](value)
		}
		return value
	}
	return IntervalRangeBounds[I]{
		StartBoundFn: func() IntervalBound[I] { return start }, EndBoundFn: func() IntervalBound[I] { return end },
		CloneFn: func() IntervalRangeBounds[I] {
			a, b := start, end
			if a.Kind != IntervalUnbounded {
				a.Value = clone(a.Value)
			}
			if b.Kind != IntervalUnbounded {
				b.Value = clone(b.Value)
			}
			return IntervalBounds(a, b, clone)
		},
	}
}
func IntervalHalfOpen[I any](start, end I, cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{IntervalIncluded, start}, IntervalBound[I]{IntervalExcluded, end}, cloneValue...)
}
func IntervalClosed[I any](start, end I, cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalStandardInclusive(RangeInclusive[I]{Start: start, End: end}, cloneValue...)
}
func IntervalFrom[I any](start I, cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{IntervalIncluded, start}, IntervalBound[I]{Kind: IntervalUnbounded}, cloneValue...)
}
func IntervalTo[I any](end I, cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{Kind: IntervalUnbounded}, IntervalBound[I]{IntervalExcluded, end}, cloneValue...)
}
func IntervalToInclusive[I any](end I, cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{Kind: IntervalUnbounded}, IntervalBound[I]{IntervalIncluded, end}, cloneValue...)
}
func IntervalFull[I any]() IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{Kind: IntervalUnbounded}, IntervalBound[I]{Kind: IntervalUnbounded})
}
func IntervalStandardInclusive[I any](value RangeInclusive[I], cloneValue ...func(I) I) IntervalRangeBounds[I] {
	clone := func(n I) I {
		if len(cloneValue) != 0 && cloneValue[0] != nil {
			return cloneValue[0](n)
		}
		return n
	}
	return IntervalRangeBounds[I]{
		StartBoundFn: func() IntervalBound[I] { return IntervalBound[I]{IntervalIncluded, value.Start} },
		EndBoundFn: func() IntervalBound[I] {
			if value.Exhausted {
				return IntervalBound[I]{IntervalExcluded, value.Start}
			}
			return IntervalBound[I]{IntervalIncluded, value.End}
		},
		CloneFn: func() IntervalRangeBounds[I] {
			a := clone(value.Start)
			b := clone(value.End)
			return IntervalStandardInclusive(RangeInclusive[I]{Start: a, End: b, Exhausted: value.Exhausted}, clone)
		},
	}
}
func IntervalCoreInclusive[I any](value CoreRangeInclusive[I], cloneValue ...func(I) I) IntervalRangeBounds[I] {
	return IntervalBounds(IntervalBound[I]{IntervalIncluded, value.Start}, IntervalBound[I]{IntervalIncluded, value.Last}, cloneValue...)
}

type intervalPair struct{ start, end uint32 }

// IntervalSet stores sorted, nonadjacent inclusive u32 intervals. Go slice
// storage replaces the private SmallVec<[pair;2]> allocation optimization;
// every operation preserves its values, order, cloning and failure behavior.
// Iterators borrow storage: callers must not mutate a set while using a view.
type IntervalSet[I any] struct {
	intervals []intervalPair
	domain    uint
	index     Idx[I]
	step      IntervalStep[I]
	config    IntervalConfig
}

func NewIntervalSet[I any](index Idx[I], domain uint, step IntervalStep[I]) *IntervalSet[I] {
	return NewIntervalSetWithConfig(index, domain, step, IntervalConfig{true, true})
}
func NewIntervalSetWithConfig[I any](index Idx[I], domain uint, step IntervalStep[I], config IntervalConfig) *IntervalSet[I] {
	return &IntervalSet[I]{domain: domain, index: index, step: step, config: config}
}
func (set *IntervalSet[I]) Clear()        { set.intervals = set.intervals[:0] }
func (set *IntervalSet[I]) IsEmpty() bool { return len(set.intervals) == 0 }
func (set *IntervalSet[I]) Clone() *IntervalSet[I] {
	copy := *set
	copy.intervals = append([]intervalPair(nil), set.intervals...)
	return &copy
}
func (set *IntervalSet[I]) CloneFrom(other *IntervalSet[I]) { *set = *other.Clone() }
func (set *IntervalSet[I]) inclusiveStart(context IntervalPanicContext, bounds IntervalRangeBounds[I]) uint32 {
	owned := true
	defer intervalDeferRange(bounds, &owned, context)
	start := bounds.StartBoundWithContext(context)
	switch start.Kind {
	case IntervalIncluded:
		return uint32(set.index.Index(start.Value))
	case IntervalExcluded:
		return set.add32(uint32(set.index.Index(start.Value)), 1)
	case IntervalUnbounded:
		return 0
	default:
		panic("invalid bound kind")
	}
}
func (set *IntervalSet[I]) inclusiveEnd(context IntervalPanicContext, bounds IntervalRangeBounds[I]) (uint32, bool) {
	owned := true
	defer intervalDeferRange(bounds, &owned, context)
	end := bounds.EndBoundWithContext(context)
	switch end.Kind {
	case IntervalIncluded:
		return uint32(set.index.Index(end.Value)), true
	case IntervalExcluded:
		n := set.index.Index(end.Value)
		if n == 0 {
			return 0, false
		}
		return uint32(n - 1), true
	case IntervalUnbounded:
		if set.domain == 0 {
			return 0, false
		}
		return uint32(set.domain - 1), true
	default:
		panic("invalid bound kind")
	}
}
func (set *IntervalSet[I]) add32(a, b uint32) uint32 {
	if set.config.OverflowChecks && b > ^uint32(0)-a {
		panic("u32 addition overflow")
	}
	return a + b
}
func (set *IntervalSet[I]) checkInvariants() bool {
	var end uint32
	present := false
	for _, pair := range set.intervals {
		if pair.start > pair.end || (present && set.add32(end, 1) >= pair.start) {
			return false
		}
		end, present = pair.end, true
	}
	return !present || end < uint32(set.domain)
}
func (set *IntervalSet[I]) assertInvariants() {
	if set.config.DebugAssertions && !set.checkInvariants() {
		panic("invalid intervals: " + set.Debug())
	}
}
func (set *IntervalSet[I]) Insert(point I) bool {
	return set.InsertRange(IntervalClosed(point, point, set.step.Clone))
}
func (set *IntervalSet[I]) InsertRange(bounds IntervalRangeBounds[I]) bool {
	return set.InsertRangeWithContext(IntervalPanicContext{}, bounds)
}
func (set *IntervalSet[I]) InsertRangeWithContext(context IntervalPanicContext, bounds IntervalRangeBounds[I]) bool {
	defer intervalProcessingGuard(context)
	start, end, ok := set.bounds(context, bounds)
	if !ok {
		return false
	}
	next := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return pair.start <= set.add32(end, 1) })
	result := true
	if next > 0 {
		right := next - 1
		previous := set.intervals[right]
		if set.add32(previous.end, 1) >= start {
			if start < previous.start {
				left := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return set.add32(pair.end, 1) < start })
				minimum := start
				if set.intervals[left].start < minimum {
					minimum = set.intervals[left].start
				}
				maximum := end
				if previous.end > maximum {
					maximum = previous.end
				}
				set.intervals[right] = intervalPair{minimum, maximum}
				if left != right {
					copy(set.intervals[left:], set.intervals[right:])
					set.intervals = set.intervals[:len(set.intervals)-(right-left)]
				}
			} else if end > previous.end {
				set.intervals[right].end = end
			} else {
				result = false
			}
		} else {
			set.insertPair(right+1, intervalPair{start, end})
		}
	} else if len(set.intervals) == 0 {
		set.intervals = append(set.intervals, intervalPair{start, end})
	} else {
		set.insertPair(next, intervalPair{start, end})
	}
	set.assertInvariants()
	return result
}
func (set *IntervalSet[I]) insertPair(index int, pair intervalPair) {
	set.intervals = append(set.intervals, intervalPair{})
	copy(set.intervals[index+1:], set.intervals[index:])
	set.intervals[index] = pair
}
func (set *IntervalSet[I]) Append(point I) {
	value := uint32(set.index.Index(point))
	if len(set.intervals) == 0 {
		set.intervals = append(set.intervals, intervalPair{value, value})
	} else {
		last := &set.intervals[len(set.intervals)-1]
		if last.end > value {
			panic("append point precedes last end")
		}
		if value == last.end {
		} else if value == set.add32(last.end, 1) {
			last.end = value
		} else {
			set.intervals = append(set.intervals, intervalPair{value, value})
		}
	}
	set.assertInvariants()
}
func (set *IntervalSet[I]) AppendRange(bounds IntervalRangeBounds[I]) {
	set.AppendRangeWithContext(IntervalPanicContext{}, bounds)
}
func (set *IntervalSet[I]) AppendRangeWithContext(context IntervalPanicContext, bounds IntervalRangeBounds[I]) {
	defer intervalProcessingGuard(context)
	start, end, ok := set.bounds(context, bounds)
	if !ok {
		return
	}
	if len(set.intervals) == 0 {
		set.intervals = append(set.intervals, intervalPair{start, end})
	} else {
		last := &set.intervals[len(set.intervals)-1]
		if last.end > start {
			panic("append range precedes last end")
		}
		if start <= set.add32(last.end, 1) {
			last.end = end
		} else {
			set.intervals = append(set.intervals, intervalPair{start, end})
		}
	}
	set.assertInvariants()
}
func (set *IntervalSet[I]) Contains(point I) bool {
	value := uint32(set.index.Index(point))
	next := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return pair.start <= value })
	return next != 0 && value <= set.intervals[next-1].end
}
func (set *IntervalSet[I]) IntersectsRange(bounds IntervalRangeBounds[I]) bool {
	return set.IntersectsRangeWithContext(IntervalPanicContext{}, bounds)
}
func (set *IntervalSet[I]) IntersectsRangeWithContext(context IntervalPanicContext, bounds IntervalRangeBounds[I]) bool {
	defer intervalProcessingGuard(context)
	start, end, ok := set.bounds(context, bounds)
	if !ok {
		return false
	}
	next := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return pair.start <= end })
	return next != 0 && start <= set.intervals[next-1].end
}
func (set *IntervalSet[I]) FirstUnsetIn(bounds IntervalRangeBounds[I]) (I, bool) {
	return set.FirstUnsetInWithContext(IntervalPanicContext{}, bounds)
}
func (set *IntervalSet[I]) FirstUnsetInWithContext(context IntervalPanicContext, bounds IntervalRangeBounds[I]) (I, bool) {
	defer intervalProcessingGuard(context)
	start, end, ok := set.bounds(context, bounds)
	if !ok {
		var zero I
		return zero, false
	}
	next := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return pair.start <= start })
	if next == 0 || start > set.intervals[next-1].end {
		return set.index.New(uint(start)), true
	}
	if previousEnd := set.intervals[next-1].end; previousEnd < end {
		return set.index.New(uint(previousEnd) + 1), true
	}
	var zero I
	return zero, false
}
func (set *IntervalSet[I]) LastSetIn(bounds IntervalRangeBounds[I]) (I, bool) {
	return set.LastSetInWithContext(IntervalPanicContext{}, bounds)
}
func (set *IntervalSet[I]) LastSetInWithContext(context IntervalPanicContext, bounds IntervalRangeBounds[I]) (I, bool) {
	defer intervalProcessingGuard(context)
	start, end, ok := set.bounds(context, bounds)
	if !ok {
		var zero I
		return zero, false
	}
	next := intervalPartitionPoint(set.intervals, func(pair intervalPair) bool { return pair.start <= end })
	if next != 0 {
		previousEnd := set.intervals[next-1].end
		if start <= previousEnd {
			if previousEnd < end {
				end = previousEnd
			}
			return set.index.New(uint(end)), true
		}
	}
	var zero I
	return zero, false
}
func (set *IntervalSet[I]) InsertAll() {
	set.Clear()
	if set.domain != 0 {
		end := set.domain - 1
		if end > uint(^uint32(0)) {
			panic("interval domain end does not fit u32")
		}
		set.intervals = append(set.intervals, intervalPair{0, uint32(end)})
	}
	set.assertInvariants()
}

// IntervalRangeIterator borrows the original pair slice. Each Next constructs
// both typed endpoints lazily, after advancing the backing slice iterator.
type IntervalRangeIterator[I any] struct {
	pairs []intervalPair
	next  int
	index Idx[I]
}

func (set *IntervalSet[I]) IterIntervals() *IntervalRangeIterator[I] {
	return &IntervalRangeIterator[I]{pairs: set.intervals, index: set.index}
}
func (iterator *IntervalRangeIterator[I]) Next() (CoreRangeInclusive[I], bool) {
	if iterator.next == len(iterator.pairs) {
		return CoreRangeInclusive[I]{}, false
	}
	pair := iterator.pairs[iterator.next]
	iterator.next++
	start := iterator.index.New(uint(pair.start))
	end := iterator.index.New(uint(pair.end))
	return CoreRangeInclusive[I]{Start: start, Last: end}, true
}
func (iterator *IntervalRangeIterator[I]) SizeHint() (uint, uint, bool) {
	n := uint(len(iterator.pairs) - iterator.next)
	return n, n, true
}
func (iterator *IntervalRangeIterator[I]) Seq() func(func(CoreRangeInclusive[I]) bool) {
	return func(yield func(CoreRangeInclusive[I]) bool) {
		for {
			value, ok := iterator.Next()
			if !ok || !yield(value) {
				return
			}
		}
	}
}

type IntervalIterator[I any] struct {
	ranges            *IntervalRangeIterator[I]
	step              IntervalStep[I]
	active, exhausted bool
	start, last       I
}

func (set *IntervalSet[I]) Iter() *IntervalIterator[I] {
	return &IntervalIterator[I]{ranges: set.IterIntervals(), step: set.step}
}
func (iterator *IntervalIterator[I]) Next() (I, bool) {
	for {
		if iterator.active {
			if iterator.exhausted || !iterator.step.LessEqual(iterator.start, iterator.last) {
				iterator.active = false
				continue
			}
			old := iterator.start
			next, overflow := iterator.step.ForwardOverflowing(iterator.step.Clone(old), 1)
			iterator.start, iterator.exhausted = next, overflow
			return old, true
		}
		rangeValue, ok := iterator.ranges.Next()
		if !ok {
			var zero I
			return zero, false
		}
		iterator.start, iterator.last, iterator.active, iterator.exhausted = rangeValue.Start, rangeValue.Last, true, false
	}
}
func (iterator *IntervalIterator[I]) Seq() func(func(I) bool) {
	return func(yield func(I) bool) {
		for {
			value, ok := iterator.Next()
			if !ok || !yield(value) {
				return
			}
		}
	}
}

// SizeHint follows the flattened iterator: future intervals contribute no
// guaranteed elements, and only a completely consumed outer iterator permits
// an upper bound from the active inclusive range.
func (iterator *IntervalIterator[I]) SizeHint() (uint, uint, bool) {
	var lower, upper uint
	hasUpper := true
	if iterator.active && !iterator.exhausted && iterator.step.LessEqual(iterator.start, iterator.last) {
		lower, upper, hasUpper = iterator.step.StepsBetween(iterator.start, iterator.last)
		if lower != ^uint(0) {
			lower++
		}
		if hasUpper {
			if upper == ^uint(0) {
				hasUpper = false
			} else {
				upper++
			}
		}
	}
	if iterator.ranges.next != len(iterator.ranges.pairs) {
		return lower, 0, false
	}
	return lower, upper, hasUpper
}
func (set *IntervalSet[I]) Superset(other *IntervalSet[I]) bool {
	supIterator, subIterator := set.IterIntervals(), other.IterIntervals()
	var current CoreRangeInclusive[I]
	hasCurrent := false
	contains := func(sup, sub CoreRangeInclusive[I]) (bool, bool) {
		if set.step.Less(sup.Last, sub.Start) {
			return false, false
		}
		if set.step.GreaterEqual(sup.Last, sub.Last) && set.step.LessEqual(sup.Start, sub.Start) {
			current, hasCurrent = sup, true
			return true, true
		}
		return false, true
	}
	for {
		sub, ok := subIterator.Next()
		if !ok {
			return true
		}
		matched, result := false, false
		if hasCurrent {
			previous := current
			hasCurrent = false
			result, matched = contains(previous, sub)
		}
		for !matched {
			sup, ok := supIterator.Next()
			if !ok {
				break
			}
			result, matched = contains(sup, sub)
		}
		if !matched || !result {
			return false
		}
	}
}
func (set *IntervalSet[I]) Disjoint(other *IntervalSet[I]) bool {
	aIterator, bIterator := set.IterIntervals(), other.IterIntervals()
	a, ok := aIterator.Next()
	if !ok {
		return true
	}
	b, ok := bIterator.Next()
	if !ok {
		return true
	}
	for {
		if set.step.Less(a.Last, b.Start) {
			a, ok = aIterator.Next()
			if !ok {
				return true
			}
			continue
		}
		if set.step.Less(b.Last, a.Start) {
			b, ok = bIterator.Next()
			if !ok {
				return true
			}
			continue
		}
		return false
	}
}
func (set *IntervalSet[I]) Union(other *IntervalSet[I]) bool {
	if set.domain != other.domain {
		panic("interval domains differ")
	}
	if len(set.intervals) < len(other.intervals) {
		backup := set.Clone()
		set.intervals = append(set.intervals[:0], other.intervals...)
		return set.Union(backup)
	}
	didInsert := false
	ranges := other.IterIntervals()
	for {
		bounds, ok := ranges.Next()
		if !ok {
			break
		}
		inserted := set.InsertRange(IntervalCoreInclusive(bounds, set.step.Clone))
		didInsert = didInsert || inserted
	}
	set.assertInvariants()
	return didInsert
}

// Preserve Rust slice partition_point's fixed probing, including its repeated
// final probe. Replacing it with sort.Search changes checked-overflow timing.
func intervalPartitionPoint(pairs []intervalPair, predicate func(intervalPair) bool) int {
	size := len(pairs)
	if size == 0 {
		return 0
	}
	base := 0
	for size > 1 {
		half := size / 2
		middle := base + half
		if predicate(pairs[middle]) {
			base = middle
		}
		size -= half
	}
	if predicate(pairs[base]) {
		return base + 1
	}
	return base
}

// SparseIntervalMatrix stores contiguous intervals in each materialized row.
// Its rows use the actual source-ported IndexVec. rowEqual supplies Rust's
// PartialEq operation independently of the Idx conversion, including for keys
// carrying non-comparable reference metadata.
type SparseIntervalMatrix[R, C any] struct {
	rows        *IndexVec[R, IntervalSet[C]]
	columnSize  uint
	columnIndex Idx[C]
	step        IntervalStep[C]
	rowEqual    func(R, R) bool
	config      IntervalConfig
}

func NewSparseIntervalMatrix[R, C any](rowIndex Idx[R], rowEqual func(R, R) bool, columnIndex Idx[C], columnSize uint, step IntervalStep[C]) *SparseIntervalMatrix[R, C] {
	return NewSparseIntervalMatrixWithConfig(rowIndex, rowEqual, columnIndex, columnSize, step, IntervalConfig{true, true})
}
func NewSparseIntervalMatrixWithConfig[R, C any](rowIndex Idx[R], rowEqual func(R, R) bool, columnIndex Idx[C], columnSize uint, step IntervalStep[C], config IntervalConfig) *SparseIntervalMatrix[R, C] {
	ops := ValueOps[IntervalSet[C]]{
		Clone:       func(value IntervalSet[C]) IntervalSet[C] { return *value.Clone() },
		CloneFrom:   func(target *IntervalSet[C], value IntervalSet[C]) { target.CloneFrom(&value) },
		Debug:       func(value IntervalSet[C]) string { return value.Debug() },
		FormatDebug: func(value *IntervalSet[C], f *IndexDebugFormatter) error { return value.FormatDebug(f) },
	}
	return &SparseIntervalMatrix[R, C]{
		rows:       NewIndexVecWithConfig(rowIndex, ops, IndexStorageConfig{OverflowChecks: config.OverflowChecks, DebugAssertions: config.DebugAssertions}),
		columnSize: columnSize, columnIndex: columnIndex, step: step, rowEqual: rowEqual, config: config,
	}
}
func (matrix *SparseIntervalMatrix[R, C]) Clone() *SparseIntervalMatrix[R, C] {
	copy := *matrix
	copy.rows = matrix.rows.Clone()
	return &copy
}
func (matrix *SparseIntervalMatrix[R, C]) CloneFrom(other *SparseIntervalMatrix[R, C]) {
	*matrix = *other.Clone()
}
func (matrix *SparseIntervalMatrix[R, C]) Rows() *IndexIterator[R] { return matrix.rows.Indices() }
func (matrix *SparseIntervalMatrix[R, C]) Row(row R) (*IntervalSet[C], bool) {
	return matrix.rows.Get(row)
}
func (matrix *SparseIntervalMatrix[R, C]) IterEnumerated() *EnumeratedRefIterator[R, IntervalSet[C]] {
	return matrix.rows.IterEnumerated()
}
func (matrix *SparseIntervalMatrix[R, C]) ClearRow(row R) {
	if value, ok := matrix.rows.GetMut(row); ok {
		value.Clear()
	}
}
func (matrix *SparseIntervalMatrix[R, C]) ensureRow(row R) *IntervalSet[C] {
	return matrix.rows.EnsureContainsElem(row, func() IntervalSet[C] {
		return *NewIntervalSetWithConfig(matrix.columnIndex, matrix.columnSize, matrix.step, matrix.config)
	})
}
func (matrix *SparseIntervalMatrix[R, C]) UnionRow(row R, from *IntervalSet[C]) bool {
	return matrix.ensureRow(row).Union(from)
}
func (matrix *SparseIntervalMatrix[R, C]) UnionRows(read, write R) bool {
	if matrix.rowEqual(read, write) {
		return false
	}
	if _, ok := matrix.rows.Get(read); !ok {
		return false
	}
	matrix.ensureRow(write)
	readRow, writeRow := matrix.rows.Pick2Mut(read, write)
	return writeRow.Union(readRow)
}
func (matrix *SparseIntervalMatrix[R, C]) DisjointRows(a, b R) bool {
	left, hasLeft := matrix.rows.Get(a)
	right, hasRight := matrix.rows.Get(b)
	if !hasLeft || !hasRight {
		return true
	}
	return left.Disjoint(right)
}
func (matrix *SparseIntervalMatrix[R, C]) InsertAllIntoRow(row R) { matrix.ensureRow(row).InsertAll() }
func (matrix *SparseIntervalMatrix[R, C]) InsertRange(row R, bounds IntervalRangeBounds[C]) {
	matrix.InsertRangeWithContext(IntervalPanicContext{}, row, bounds)
}
func (matrix *SparseIntervalMatrix[R, C]) InsertRangeWithContext(context IntervalPanicContext, row R, bounds IntervalRangeBounds[C]) {
	owned := true
	defer intervalDeferRange(bounds, &owned, context)
	target := matrix.ensureRow(row)
	owned = false
	target.InsertRangeWithContext(context, bounds)
}
func (matrix *SparseIntervalMatrix[R, C]) Insert(row R, point C) bool {
	return matrix.ensureRow(row).Insert(point)
}
func (matrix *SparseIntervalMatrix[R, C]) Append(row R, point C) { matrix.ensureRow(row).Append(point) }
func (matrix *SparseIntervalMatrix[R, C]) AppendRange(row R, bounds IntervalRangeBounds[C]) {
	matrix.AppendRangeWithContext(IntervalPanicContext{}, row, bounds)
}
func (matrix *SparseIntervalMatrix[R, C]) AppendRangeWithContext(context IntervalPanicContext, row R, bounds IntervalRangeBounds[C]) {
	owned := true
	defer intervalDeferRange(bounds, &owned, context)
	target := matrix.ensureRow(row)
	owned = false
	target.AppendRangeWithContext(context, bounds)
}
func (matrix *SparseIntervalMatrix[R, C]) Contains(row R, point C) bool {
	value, ok := matrix.Row(row)
	return ok && value.Contains(point)
}
