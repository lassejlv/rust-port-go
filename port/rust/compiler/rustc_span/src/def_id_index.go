// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual newtype_index! expansions for CrateNum and DefIndex in def_id.rs.
package rustcspan

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const CrateNumMaxAsU32 uint32 = 0xffffff00

type CrateNum struct{ value uint32 }

func defIdDefaultContext() rustfmt.Context { return rustfmt.Context{} }

type DefIdConfig struct{ DebugAssertions, OverflowChecks bool }

func DefaultDefIdConfig() DefIdConfig { return DefIdConfig{true, true} }

func NewCrateNum(value uint) CrateNum { return CrateNumFromUsize(value) }
func CrateNumFromU32(value uint32) CrateNum {
	return CrateNumFromU32WithContext(value, rustfmt.Context{})
}
func CrateNumFromUsize(value uint) CrateNum {
	return CrateNumFromUsizeWithContext(value, rustfmt.Context{})
}
func CrateNumFromU16(value uint16) CrateNum { return CrateNumFromU32(uint32(value)) }
func CrateNumFromU32WithContext(value uint32, context rustfmt.Context) CrateNum {
	if value > CrateNumMaxAsU32 {
		defIdRaise(context, CrateNumPanicSite, "assertion failed: value <= 0xFFFF_FF00", rustfmt.PanicStaticStr)
	}
	return CrateNum{value}
}
func CrateNumFromUsizeWithContext(value uint, context rustfmt.Context) CrateNum {
	if value > uint(CrateNumMaxAsU32) {
		defIdRaise(context, CrateNumPanicSite, "assertion failed: value <= (0xFFFF_FF00 as usize)", rustfmt.PanicStaticStr)
	}
	return CrateNum{uint32(value)}
}
func DefIndexFromU32WithContext(value uint32, context rustfmt.Context) DefIndex {
	if value > DefIndexMaxAsU32 {
		defIdRaise(context, DefIndexPanicSite, "assertion failed: value <= 0xFFFF_FF00", rustfmt.PanicStaticStr)
	}
	return DefIndex{value}
}
func DefIndexFromUsizeWithContext(value uint, context rustfmt.Context) DefIndex {
	if value > uint(DefIndexMaxAsU32) {
		defIdRaise(context, DefIndexPanicSite, "assertion failed: value <= (0xFFFF_FF00 as usize)", rustfmt.PanicStaticStr)
	}
	return DefIndex{uint32(value)}
}

// The source unsafe constructors require the pattern-type invariant. Go-only
// out-of-domain values are rejected rather than inventing a Rust value with UB.
func CrateNumFromU32Unchecked(value uint32) CrateNum { return CrateNumFromU32(value) }
func DefIndexFromU32Unchecked(value uint32) DefIndex { return DefIndexFromU32(value) }
func CrateNumZero() CrateNum                         { return CrateNum{} }
func CrateNumMax() CrateNum                          { return CrateNum{CrateNumMaxAsU32} }
func DefIndexZero() DefIndex                         { return DefIndex{} }
func DefIndexMax() DefIndex                          { return DefIndex{DefIndexMaxAsU32} }
func (value CrateNum) AsU32() uint32                 { return value.value }
func (value CrateNum) AsUsize() uint                 { return uint(value.value) }
func (value CrateNum) Index() uint                   { return value.AsUsize() }
func (value CrateNum) Clone() CrateNum               { return value }
func (value *CrateNum) CloneFrom(source CrateNum)    { *value = source.Clone() }
func (value CrateNum) Equal(other CrateNum) bool     { return value == other }
func (value CrateNum) NotEqual(other CrateNum) bool  { return !value.Equal(other) }
func (value CrateNum) Hash(h spanU32Hasher)          { h.WriteU32(value.value) }
func (value CrateNum) Compare(other CrateNum) int {
	return compareDefIndexValues(value.value, other.value)
}
func (value CrateNum) PartialCompare(other CrateNum) Option[int] { return Some(value.Compare(other)) }
func (value CrateNum) AsDefId() DefId                            { return DefId{Index: CrateDefIndex(), Krate: value} }
func (value CrateNum) AsModId() ModId                            { return NewModIdUnchecked(value.AsDefId()) }
func (value *DefIndex) CloneFrom(source DefIndex)                { *value = source.Clone() }
func (value DefIndex) NotEqual(other DefIndex) bool              { return !value.Equal(other) }
func (value DefIndex) PartialCompare(other DefIndex) Option[int] { return Some(value.Compare(other)) }
func (value LocalDefId) NotEqual(other LocalDefId) bool          { return !value.Equal(other) }
func compareDefIndexValues(a, b uint32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func idConfig(explicit []DefIdConfig) DefIdConfig {
	if len(explicit) > 0 {
		return explicit[0]
	}
	return DefaultDefIdConfig()
}
func addDefIndex(value, amount uint, config DefIdConfig, context rustfmt.Context, site string) uint {
	if config.OverflowChecks && amount > ^uint(0)-value {
		defIdRaise(context, site, "attempt to add with overflow", rustfmt.PanicStaticStr)
	}
	return value + amount
}
func (value CrateNum) Add(amount uint, config ...DefIdConfig) CrateNum {
	return value.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (value CrateNum) AddWithContext(amount uint, context rustfmt.Context, config ...DefIdConfig) CrateNum {
	return CrateNumFromUsizeWithContext(addDefIndex(value.Index(), amount, idConfig(config), context, CrateNumPanicSite), context)
}
func (value *CrateNum) AddAssign(amount uint, config ...DefIdConfig) {
	*value = value.Add(amount, config...)
}
func (value *CrateNum) AddAssignWithContext(amount uint, context rustfmt.Context, config ...DefIdConfig) {
	*value = value.AddWithContext(amount, context, config...)
}
func (value DefIndex) Add(amount uint, config ...DefIdConfig) DefIndex {
	return value.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (value DefIndex) AddWithContext(amount uint, context rustfmt.Context, config ...DefIdConfig) DefIndex {
	return DefIndexFromUsizeWithContext(addDefIndex(value.Index(), amount, idConfig(config), context, DefIndexPanicSite), context)
}
func (value *DefIndex) AddAssign(amount uint, config ...DefIdConfig) {
	*value = value.Add(amount, config...)
}
func (value *DefIndex) AddAssignWithContext(amount uint, context rustfmt.Context, config ...DefIdConfig) {
	*value = value.AddWithContext(amount, context, config...)
}

func DefIndexStepsBetween(start, end DefIndex) (uint, Option[uint]) {
	if start.Index() > end.Index() {
		return 0, None[uint]()
	}
	n := end.Index() - start.Index()
	return n, Some(n)
}
func CrateNumStepsBetween(start, end CrateNum) (uint, Option[uint]) {
	if start.Index() > end.Index() {
		return 0, None[uint]()
	}
	n := end.Index() - start.Index()
	return n, Some(n)
}
func (value DefIndex) ForwardChecked(amount uint) Option[DefIndex] {
	return value.ForwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) ForwardCheckedWithContext(amount uint, context rustfmt.Context) Option[DefIndex] {
	if amount > ^uint(0)-value.Index() {
		return None[DefIndex]()
	}
	return Some(DefIndexFromUsizeWithContext(value.Index()+amount, context))
}
func (value DefIndex) BackwardChecked(amount uint) Option[DefIndex] {
	return value.BackwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) BackwardCheckedWithContext(amount uint, context rustfmt.Context) Option[DefIndex] {
	if amount > value.Index() {
		return None[DefIndex]()
	}
	return Some(DefIndexFromUsizeWithContext(value.Index()-amount, context))
}
func (value CrateNum) ForwardChecked(amount uint) Option[CrateNum] {
	return value.ForwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) ForwardCheckedWithContext(amount uint, context rustfmt.Context) Option[CrateNum] {
	if amount > ^uint(0)-value.Index() {
		return None[CrateNum]()
	}
	return Some(CrateNumFromUsizeWithContext(value.Index()+amount, context))
}
func (value CrateNum) BackwardChecked(amount uint) Option[CrateNum] {
	return value.BackwardCheckedWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) BackwardCheckedWithContext(amount uint, context rustfmt.Context) Option[CrateNum] {
	if amount > value.Index() {
		return None[CrateNum]()
	}
	return Some(CrateNumFromUsizeWithContext(value.Index()-amount, context))
}
func (value DefIndex) ForwardOverflowing(amount uint) (DefIndex, bool) {
	return value.ForwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) ForwardOverflowingWithContext(amount uint, context rustfmt.Context) (DefIndex, bool) {
	return DefIndexFromUsizeWithContext(value.Index()+amount, context), amount > ^uint(0)-value.Index()
}
func (value DefIndex) BackwardOverflowing(amount uint) (DefIndex, bool) {
	return value.BackwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) BackwardOverflowingWithContext(amount uint, context rustfmt.Context) (DefIndex, bool) {
	return DefIndexFromUsizeWithContext(value.Index()-amount, context), amount > value.Index()
}
func (value CrateNum) ForwardOverflowing(amount uint) (CrateNum, bool) {
	return value.ForwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) ForwardOverflowingWithContext(amount uint, context rustfmt.Context) (CrateNum, bool) {
	return CrateNumFromUsizeWithContext(value.Index()+amount, context), amount > ^uint(0)-value.Index()
}
func (value CrateNum) BackwardOverflowing(amount uint) (CrateNum, bool) {
	return value.BackwardOverflowingWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) BackwardOverflowingWithContext(amount uint, context rustfmt.Context) (CrateNum, bool) {
	return CrateNumFromUsizeWithContext(value.Index()-amount, context), amount > value.Index()
}
func (value DefIndex) Forward(amount uint) DefIndex {
	return value.ForwardWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) ForwardWithContext(amount uint, context rustfmt.Context) DefIndex {
	if result, ok := value.ForwardCheckedWithContext(amount, context).Get(); ok {
		return result
	}
	defIdRaise(context, DefIdStepForwardPanicSite, "overflow in `Step::forward`", rustfmt.PanicOwnedString)
	return DefIndex{}
}
func (value DefIndex) Backward(amount uint) DefIndex {
	return value.BackwardWithContext(amount, rustfmt.Context{})
}
func (value DefIndex) BackwardWithContext(amount uint, context rustfmt.Context) DefIndex {
	if result, ok := value.BackwardCheckedWithContext(amount, context).Get(); ok {
		return result
	}
	defIdRaise(context, DefIdStepBackwardPanicSite, "overflow in `Step::backward`", rustfmt.PanicOwnedString)
	return DefIndex{}
}
func (value CrateNum) Forward(amount uint) CrateNum {
	return value.ForwardWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) ForwardWithContext(amount uint, context rustfmt.Context) CrateNum {
	if result, ok := value.ForwardCheckedWithContext(amount, context).Get(); ok {
		return result
	}
	defIdRaise(context, DefIdStepForwardPanicSite, "overflow in `Step::forward`", rustfmt.PanicOwnedString)
	return CrateNum{}
}
func (value CrateNum) Backward(amount uint) CrateNum {
	return value.BackwardWithContext(amount, rustfmt.Context{})
}
func (value CrateNum) BackwardWithContext(amount uint, context rustfmt.Context) CrateNum {
	if result, ok := value.BackwardCheckedWithContext(amount, context).Get(); ok {
		return result
	}
	defIdRaise(context, DefIdStepBackwardPanicSite, "overflow in `Step::backward`", rustfmt.PanicOwnedString)
	return CrateNum{}
}

// Unchecked stepping is defined only when the checked result is Some.
func (value DefIndex) ForwardUnchecked(amount uint) DefIndex  { return value.Forward(amount) }
func (value DefIndex) BackwardUnchecked(amount uint) DefIndex { return value.Backward(amount) }
func (value CrateNum) ForwardUnchecked(amount uint) CrateNum  { return value.Forward(amount) }
func (value CrateNum) BackwardUnchecked(amount uint) CrateNum { return value.Backward(amount) }

func DefIndexIdx() index.Idx[DefIndex] { return DefIndexIdxWithContext(rustfmt.Context{}) }
func DefIndexIdxWithContext(context rustfmt.Context, explicit ...DefIdConfig) index.Idx[DefIndex] {
	config := idConfig(explicit)
	result := index.NewIdxWithConfig(func(value uint) DefIndex { return DefIndexFromUsizeWithContext(value, context) }, func(value DefIndex) uint { return value.Index() }, index.IdxConfig{OverflowChecks: config.OverflowChecks})
	result.PlusOverride = func(value DefIndex, amount uint) DefIndex {
		return DefIndexFromUsizeWithContext(addDefIndex(value.Index(), amount, config, context, DefIdIdxPlusPanicSite), context)
	}
	return result
}
func CrateNumIdx() index.Idx[CrateNum] { return CrateNumIdxWithContext(rustfmt.Context{}) }
func CrateNumIdxWithContext(context rustfmt.Context, explicit ...DefIdConfig) index.Idx[CrateNum] {
	config := idConfig(explicit)
	result := index.NewIdxWithConfig(func(value uint) CrateNum { return CrateNumFromUsizeWithContext(value, context) }, func(value CrateNum) uint { return value.Index() }, index.IdxConfig{OverflowChecks: config.OverflowChecks})
	result.PlusOverride = func(value CrateNum, amount uint) CrateNum {
		return CrateNumFromUsizeWithContext(addDefIndex(value.Index(), amount, config, context, DefIdIdxPlusPanicSite), context)
	}
	return result
}
func LocalDefIdIdx() index.Idx[LocalDefId] { return LocalDefIdIdxWithContext(rustfmt.Context{}) }
func LocalDefIdIdxWithContext(context rustfmt.Context, explicit ...DefIdConfig) index.Idx[LocalDefId] {
	config := idConfig(explicit)
	from := func(value uint) LocalDefId { return LocalDefId{DefIndexFromUsizeWithContext(value, context)} }
	result := index.NewIdxWithConfig(from, func(value LocalDefId) uint { return value.Index() }, index.IdxConfig{OverflowChecks: config.OverflowChecks})
	result.PlusOverride = func(value LocalDefId, amount uint) LocalDefId {
		return from(addDefIndex(value.Index(), amount, config, context, LocalDefIdIdxPlusPanicSite))
	}
	return result
}

func DefIndexStep(ambient ...rustfmt.Context) index.IntervalStep[DefIndex] {
	context := comparatorContext(ambient)
	return defIdStep("rustc_span::def_id::DefIndex", func(a, b DefIndex) int { return a.Compare(b) }, DefIndexStepsBetween,
		func(value DefIndex, n uint) (DefIndex, bool) {
			return value.ForwardCheckedWithContext(n, context).Get()
		},
		func(value DefIndex, n uint) (DefIndex, bool) {
			return value.BackwardCheckedWithContext(n, context).Get()
		},
		func(value DefIndex, n uint) (DefIndex, bool) { return value.ForwardOverflowingWithContext(n, context) },
		func(value DefIndex, n uint) (DefIndex, bool) { return value.BackwardOverflowingWithContext(n, context) },
		func(value DefIndex, n uint) DefIndex { return value.ForwardWithContext(n, context) },
		func(value DefIndex, n uint) DefIndex { return value.BackwardWithContext(n, context) })
}
func CrateNumStep(ambient ...rustfmt.Context) index.IntervalStep[CrateNum] {
	context := comparatorContext(ambient)
	return defIdStep("rustc_span::def_id::CrateNum", func(a, b CrateNum) int { return a.Compare(b) }, CrateNumStepsBetween,
		func(value CrateNum, n uint) (CrateNum, bool) {
			return value.ForwardCheckedWithContext(n, context).Get()
		},
		func(value CrateNum, n uint) (CrateNum, bool) {
			return value.BackwardCheckedWithContext(n, context).Get()
		},
		func(value CrateNum, n uint) (CrateNum, bool) { return value.ForwardOverflowingWithContext(n, context) },
		func(value CrateNum, n uint) (CrateNum, bool) { return value.BackwardOverflowingWithContext(n, context) },
		func(value CrateNum, n uint) CrateNum { return value.ForwardWithContext(n, context) },
		func(value CrateNum, n uint) CrateNum { return value.BackwardWithContext(n, context) })
}
func defIdStep[I any](name string, cmp func(I, I) int, between func(I, I) (uint, Option[uint]), forwardChecked, backwardChecked, forwardOverflowing, backwardOverflowing func(I, uint) (I, bool), forward, backward func(I, uint) I) index.IntervalStep[I] {
	return index.IntervalStep[I]{TypeName: name, CloneValue: func(value I) I { return value }, EqualFn: func(a, b I) bool { return cmp(a, b) == 0 }, PartialCompareFn: func(a, b I) index.IntervalOrdering { return index.IntervalOrdering(cmp(a, b)) }, StepsBetweenFn: func(a, b I) (uint, uint, bool) {
		lower, upper := between(a, b)
		hi, ok := upper.Get()
		return lower, hi, ok
	}, ForwardCheckedFn: forwardChecked, BackwardCheckedFn: backwardChecked, ForwardOverflowingFn: forwardOverflowing, BackwardOverflowingFn: backwardOverflowing, ForwardOverride: forward, BackwardOverride: backward, ForwardUncheckedOverride: forward, BackwardUncheckedOverride: backward}
}
