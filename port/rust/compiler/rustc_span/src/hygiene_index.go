// SPDX-License-Identifier: MIT OR Apache-2.0
// Concrete newtype_index! expansions used by the complete hygiene port.
// Source/API inventory and native comparisons live in tools/port-rust-hygiene.
package rustcspan

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type HygieneConfig struct{ DebugAssertions, OverflowChecks bool }

func DefaultHygieneConfig() HygieneConfig { return HygieneConfig{true, true} }
func hygieneConfiguration(config []HygieneConfig) HygieneConfig {
	if len(config) > 0 {
		return config[0]
	}
	return DefaultHygieneConfig()
}
func hygieneAddIndex(a, b uint, config HygieneConfig, ctx rustfmt.Context, point hygieneCodePoint) uint {
	if config.OverflowChecks && b > ^uint(0)-a {
		hygieneStaticPanic(ctx, point.line, point.column, "attempt to add with overflow")
	}
	return a + b
}
func SyntaxContextFromU32WithContext(v uint32, ctx rustfmt.Context) SyntaxContext {
	if v > SyntaxContextMaxAsU32 {
		hygieneStaticPanic(ctx, 52, 1, "assertion failed: value <= 0x1FFF_FFFF")
	}
	return SyntaxContext{v}
}
func SyntaxContextFromUsizeWithContext(v uint, ctx rustfmt.Context) SyntaxContext {
	if v > uint(SyntaxContextMaxAsU32) {
		hygieneStaticPanic(ctx, 52, 1, "assertion failed: value <= (0x1FFF_FFFF as usize)")
	}
	return SyntaxContext{uint32(v)}
}
func SyntaxContextFromU32Unchecked(v uint32) SyntaxContext { return SyntaxContextFromU32(v) }
func NewSyntaxContext(v uint) SyntaxContext                { return SyntaxContextFromUsize(v) }
func SyntaxContextZero() SyntaxContext                     { return SyntaxContext{} }
func SyntaxContextMax() SyntaxContext                      { return SyntaxContext{SyntaxContextMaxAsU32} }
func (v *SyntaxContext) CloneFrom(other SyntaxContext)     { *v = other.Clone() }
func (v SyntaxContext) NotEqual(other SyntaxContext) bool  { return !v.Equal(other) }
func (v SyntaxContext) Add(amount uint, config ...HygieneConfig) SyntaxContext {
	return v.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (v SyntaxContext) AddWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) SyntaxContext {
	return SyntaxContextFromUsizeWithContext(hygieneAddIndex(v.Index(), amount, hygieneConfiguration(config), ctx, hygienePoint(52, 1)), ctx)
}
func (v *SyntaxContext) AddAssign(amount uint, config ...HygieneConfig) {
	*v = v.Add(amount, config...)
}
func (v *SyntaxContext) AddAssignWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) {
	*v = v.AddWithContext(amount, ctx, config...)
}
func SyntaxContextIdx() index.Idx[SyntaxContext] {
	return SyntaxContextIdxWithContext(rustfmt.Context{})
}
func SyntaxContextIdxWithContext(ctx rustfmt.Context, config ...HygieneConfig) index.Idx[SyntaxContext] {
	return hygieneIdx(SyntaxContextFromUsizeWithContext, func(v SyntaxContext) uint { return v.Index() }, ctx, hygieneConfiguration(config))
}

const ExpnIndexMaxAsU32 uint32 = 0xffffff00

type ExpnIndex struct{ value uint32 }

func ExpnIndexFromU32(v uint32) ExpnIndex      { return ExpnIndexFromU32WithContext(v, rustfmt.Context{}) }
func ExpnIndexFromUsize(v uint) ExpnIndex      { return ExpnIndexFromUsizeWithContext(v, rustfmt.Context{}) }
func ExpnIndexFromU16(v uint16) ExpnIndex      { return ExpnIndexFromU32(uint32(v)) }
func (v ExpnIndex) AsU32() uint32              { return v.value }
func (v ExpnIndex) AsUsize() uint              { return uint(v.value) }
func (v ExpnIndex) Index() uint                { return uint(v.value) }
func (v ExpnIndex) Clone() ExpnIndex           { return v }
func (v ExpnIndex) Equal(other ExpnIndex) bool { return v == other }
func (v ExpnIndex) Hash(h hashmap.Hasher)      { hashmap.HashU32(h, v.value) }
func ExpnIndexFromU32WithContext(v uint32, ctx rustfmt.Context) ExpnIndex {
	if v > ExpnIndexMaxAsU32 {
		hygieneStaticPanic(ctx, 105, 1, "assertion failed: value <= 0xFFFF_FF00")
	}
	return ExpnIndex{v}
}
func ExpnIndexFromUsizeWithContext(v uint, ctx rustfmt.Context) ExpnIndex {
	if v > uint(ExpnIndexMaxAsU32) {
		hygieneStaticPanic(ctx, 105, 1, "assertion failed: value <= (0xFFFF_FF00 as usize)")
	}
	return ExpnIndex{uint32(v)}
}
func ExpnIndexFromU32Unchecked(v uint32) ExpnIndex { return ExpnIndexFromU32(v) }
func NewExpnIndex(v uint) ExpnIndex                { return ExpnIndexFromUsize(v) }
func ExpnIndexZero() ExpnIndex                     { return ExpnIndex{} }
func ExpnIndexMax() ExpnIndex                      { return ExpnIndex{ExpnIndexMaxAsU32} }
func (v *ExpnIndex) CloneFrom(other ExpnIndex)     { *v = other.Clone() }
func (v ExpnIndex) NotEqual(other ExpnIndex) bool  { return !v.Equal(other) }
func (v ExpnIndex) Add(amount uint, config ...HygieneConfig) ExpnIndex {
	return v.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (v ExpnIndex) AddWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) ExpnIndex {
	return ExpnIndexFromUsizeWithContext(hygieneAddIndex(v.Index(), amount, hygieneConfiguration(config), ctx, hygienePoint(105, 1)), ctx)
}
func (v *ExpnIndex) AddAssign(amount uint, config ...HygieneConfig) { *v = v.Add(amount, config...) }
func (v *ExpnIndex) AddAssignWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) {
	*v = v.AddWithContext(amount, ctx, config...)
}
func ExpnIndexIdx() index.Idx[ExpnIndex] {
	return ExpnIndexIdxWithContext(rustfmt.Context{})
}
func ExpnIndexIdxWithContext(ctx rustfmt.Context, config ...HygieneConfig) index.Idx[ExpnIndex] {
	return hygieneIdx(ExpnIndexFromUsizeWithContext, func(v ExpnIndex) uint { return v.Index() }, ctx, hygieneConfiguration(config))
}

const LocalExpnIdMaxAsU32 uint32 = 0xffffff00

type LocalExpnId struct{ value uint32 }

func LocalExpnIdFromU32(v uint32) LocalExpnId {
	return LocalExpnIdFromU32WithContext(v, rustfmt.Context{})
}
func LocalExpnIdFromUsize(v uint) LocalExpnId {
	return LocalExpnIdFromUsizeWithContext(v, rustfmt.Context{})
}
func LocalExpnIdFromU16(v uint16) LocalExpnId      { return LocalExpnIdFromU32(uint32(v)) }
func (v LocalExpnId) AsU32() uint32                { return v.value }
func (v LocalExpnId) AsUsize() uint                { return uint(v.value) }
func (v LocalExpnId) Index() uint                  { return uint(v.value) }
func (v LocalExpnId) Clone() LocalExpnId           { return v }
func (v LocalExpnId) Equal(other LocalExpnId) bool { return v == other }
func (v LocalExpnId) Hash(h hashmap.Hasher)        { hashmap.HashU32(h, v.value) }
func LocalExpnIdFromU32WithContext(v uint32, ctx rustfmt.Context) LocalExpnId {
	if v > LocalExpnIdMaxAsU32 {
		hygieneStaticPanic(ctx, 125, 1, "assertion failed: value <= 0xFFFF_FF00")
	}
	return LocalExpnId{v}
}
func LocalExpnIdFromUsizeWithContext(v uint, ctx rustfmt.Context) LocalExpnId {
	if v > uint(LocalExpnIdMaxAsU32) {
		hygieneStaticPanic(ctx, 125, 1, "assertion failed: value <= (0xFFFF_FF00 as usize)")
	}
	return LocalExpnId{uint32(v)}
}
func LocalExpnIdFromU32Unchecked(v uint32) LocalExpnId { return LocalExpnIdFromU32(v) }
func NewLocalExpnId(v uint) LocalExpnId                { return LocalExpnIdFromUsize(v) }
func LocalExpnIdZero() LocalExpnId                     { return LocalExpnId{} }
func LocalExpnIdMax() LocalExpnId                      { return LocalExpnId{LocalExpnIdMaxAsU32} }
func (v *LocalExpnId) CloneFrom(other LocalExpnId)     { *v = other.Clone() }
func (v LocalExpnId) NotEqual(other LocalExpnId) bool  { return !v.Equal(other) }
func (v LocalExpnId) Add(amount uint, config ...HygieneConfig) LocalExpnId {
	return v.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (v LocalExpnId) AddWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) LocalExpnId {
	return LocalExpnIdFromUsizeWithContext(hygieneAddIndex(v.Index(), amount, hygieneConfiguration(config), ctx, hygienePoint(125, 1)), ctx)
}
func (v *LocalExpnId) AddAssign(amount uint, config ...HygieneConfig) { *v = v.Add(amount, config...) }
func (v *LocalExpnId) AddAssignWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) {
	*v = v.AddWithContext(amount, ctx, config...)
}
func LocalExpnIdIdx() index.Idx[LocalExpnId] {
	return LocalExpnIdIdxWithContext(rustfmt.Context{})
}
func LocalExpnIdIdxWithContext(ctx rustfmt.Context, config ...HygieneConfig) index.Idx[LocalExpnId] {
	return hygieneIdx(LocalExpnIdFromUsizeWithContext, func(v LocalExpnId) uint { return v.Index() }, ctx, hygieneConfiguration(config))
}

const DecodedSyntaxContextMaxAsU32 uint32 = 0x1fffffff

type DecodedSyntaxContext struct{ value uint32 }

func DecodedSyntaxContextFromU32(v uint32) DecodedSyntaxContext {
	return DecodedSyntaxContextFromU32WithContext(v, rustfmt.Context{})
}
func DecodedSyntaxContextFromUsize(v uint) DecodedSyntaxContext {
	return DecodedSyntaxContextFromUsizeWithContext(v, rustfmt.Context{})
}
func DecodedSyntaxContextFromU16(v uint16) DecodedSyntaxContext {
	return DecodedSyntaxContextFromU32(uint32(v))
}
func (v DecodedSyntaxContext) AsU32() uint32                         { return v.value }
func (v DecodedSyntaxContext) AsUsize() uint                         { return uint(v.value) }
func (v DecodedSyntaxContext) Index() uint                           { return uint(v.value) }
func (v DecodedSyntaxContext) Clone() DecodedSyntaxContext           { return v }
func (v DecodedSyntaxContext) Equal(other DecodedSyntaxContext) bool { return v == other }
func (v DecodedSyntaxContext) Hash(h hashmap.Hasher)                 { hashmap.HashU32(h, v.value) }
func DecodedSyntaxContextFromU32WithContext(v uint32, ctx rustfmt.Context) DecodedSyntaxContext {
	if v > DecodedSyntaxContextMaxAsU32 {
		hygieneStaticPanic(ctx, 1583, 1, "assertion failed: value <= 0x1FFF_FFFF")
	}
	return DecodedSyntaxContext{v}
}
func DecodedSyntaxContextFromUsizeWithContext(v uint, ctx rustfmt.Context) DecodedSyntaxContext {
	if v > uint(DecodedSyntaxContextMaxAsU32) {
		hygieneStaticPanic(ctx, 1583, 1, "assertion failed: value <= (0x1FFF_FFFF as usize)")
	}
	return DecodedSyntaxContext{uint32(v)}
}
func DecodedSyntaxContextFromU32Unchecked(v uint32) DecodedSyntaxContext {
	return DecodedSyntaxContextFromU32(v)
}
func NewDecodedSyntaxContext(v uint) DecodedSyntaxContext { return DecodedSyntaxContextFromUsize(v) }
func DecodedSyntaxContextZero() DecodedSyntaxContext      { return DecodedSyntaxContext{} }
func DecodedSyntaxContextMax() DecodedSyntaxContext {
	return DecodedSyntaxContext{DecodedSyntaxContextMaxAsU32}
}
func (v *DecodedSyntaxContext) CloneFrom(other DecodedSyntaxContext)    { *v = other.Clone() }
func (v DecodedSyntaxContext) NotEqual(other DecodedSyntaxContext) bool { return !v.Equal(other) }
func (v DecodedSyntaxContext) Add(amount uint, config ...HygieneConfig) DecodedSyntaxContext {
	return v.AddWithContext(amount, rustfmt.Context{}, config...)
}
func (v DecodedSyntaxContext) AddWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) DecodedSyntaxContext {
	return DecodedSyntaxContextFromUsizeWithContext(hygieneAddIndex(v.Index(), amount, hygieneConfiguration(config), ctx, hygienePoint(1583, 1)), ctx)
}
func (v *DecodedSyntaxContext) AddAssign(amount uint, config ...HygieneConfig) {
	*v = v.Add(amount, config...)
}
func (v *DecodedSyntaxContext) AddAssignWithContext(amount uint, ctx rustfmt.Context, config ...HygieneConfig) {
	*v = v.AddWithContext(amount, ctx, config...)
}
func DecodedSyntaxContextIdx() index.Idx[DecodedSyntaxContext] {
	return DecodedSyntaxContextIdxWithContext(rustfmt.Context{})
}
func DecodedSyntaxContextIdxWithContext(ctx rustfmt.Context, config ...HygieneConfig) index.Idx[DecodedSyntaxContext] {
	return hygieneIdx(DecodedSyntaxContextFromUsizeWithContext, func(v DecodedSyntaxContext) uint { return v.Index() }, ctx, hygieneConfiguration(config))
}

func hygieneIdx[I any](from func(uint, rustfmt.Context) I, raw func(I) uint, ctx rustfmt.Context, config HygieneConfig) index.Idx[I] {
	dictionary := index.NewIdxWithConfig(func(n uint) I { return from(n, ctx) }, raw, index.IdxConfig{OverflowChecks: config.OverflowChecks})
	dictionary.PlusOverride = func(value I, amount uint) I {
		n := raw(value)
		if config.OverflowChecks && amount > ^uint(0)-n {
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "attempt to add with overflow", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation("source:rustc_index/src/idx.rs:22:19")})
		}
		return from(n+amount, ctx)
	}
	return dictionary
}
func LocalExpnIdDefault() LocalExpnId                          { return LocalExpnId{} }
func SyntaxContextDefault() SyntaxContext                      { return RootSyntaxContext() }
func RootLocalExpnId() LocalExpnId                             { return LocalExpnId{} }
func (v LocalExpnId) AsRaw() ExpnIndex                         { return ExpnIndexFromU32(v.value) }
func localExpnIdFromRaw(v ExpnIndex) LocalExpnId               { return LocalExpnIdFromU32(v.value) }
func (v DecodedSyntaxContext) isRoot() bool                    { return v.value == 0 }
func (v ExpnIndex) Compare(other ExpnIndex) int                { return compareDefIndexValues(v.value, other.value) }
func (v ExpnIndex) PartialCompare(other ExpnIndex) Option[int] { return Some(v.Compare(other)) }
func ExpnIndexStepsBetween(start, end ExpnIndex) (uint, Option[uint]) {
	if start.Index() > end.Index() {
		return 0, None[uint]()
	}
	n := end.Index() - start.Index()
	return n, Some(n)
}
func (v ExpnIndex) ForwardChecked(n uint) Option[ExpnIndex] {
	return v.ForwardCheckedWithContext(n, rustfmt.Context{})
}
func (v ExpnIndex) ForwardCheckedWithContext(n uint, c rustfmt.Context) Option[ExpnIndex] {
	if n > ^uint(0)-v.Index() {
		return None[ExpnIndex]()
	}
	return Some(ExpnIndexFromUsizeWithContext(v.Index()+n, c))
}
func (v ExpnIndex) BackwardChecked(n uint) Option[ExpnIndex] {
	return v.BackwardCheckedWithContext(n, rustfmt.Context{})
}
func (v ExpnIndex) BackwardCheckedWithContext(n uint, c rustfmt.Context) Option[ExpnIndex] {
	if n > v.Index() {
		return None[ExpnIndex]()
	}
	return Some(ExpnIndexFromUsizeWithContext(v.Index()-n, c))
}
func (v ExpnIndex) ForwardOverflowing(n uint) (ExpnIndex, bool) {
	return v.ForwardOverflowingWithContext(n, rustfmt.Context{})
}
func (v ExpnIndex) ForwardOverflowingWithContext(n uint, c rustfmt.Context) (ExpnIndex, bool) {
	return ExpnIndexFromUsizeWithContext(v.Index()+n, c), n > ^uint(0)-v.Index()
}
func (v ExpnIndex) BackwardOverflowing(n uint) (ExpnIndex, bool) {
	return v.BackwardOverflowingWithContext(n, rustfmt.Context{})
}
func (v ExpnIndex) BackwardOverflowingWithContext(n uint, c rustfmt.Context) (ExpnIndex, bool) {
	return ExpnIndexFromUsizeWithContext(v.Index()-n, c), n > v.Index()
}
func (v ExpnIndex) Forward(n uint) ExpnIndex { return v.ForwardWithContext(n, rustfmt.Context{}) }
func (v ExpnIndex) ForwardWithContext(n uint, c rustfmt.Context) ExpnIndex {
	result := v.ForwardCheckedWithContext(n, c)
	out, ok := result.Get()
	if !ok {
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "overflow in `Step::forward`", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: c.SourcePanicLocation("rustc_span::hygiene::ExpnIndex::Step::forward")})
	}
	return out
}
func (v ExpnIndex) Backward(n uint) ExpnIndex { return v.BackwardWithContext(n, rustfmt.Context{}) }
func (v ExpnIndex) BackwardWithContext(n uint, c rustfmt.Context) ExpnIndex {
	result := v.BackwardCheckedWithContext(n, c)
	out, ok := result.Get()
	if !ok {
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "overflow in `Step::backward`", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: c.SourcePanicLocation("rustc_span::hygiene::ExpnIndex::Step::backward")})
	}
	return out
}

// These source unsafe defaults require valid in-range stepping inputs.
func (v ExpnIndex) ForwardUnchecked(n uint) ExpnIndex  { return v.Forward(n) }
func (v ExpnIndex) BackwardUnchecked(n uint) ExpnIndex { return v.Backward(n) }
func (v ExpnIndex) ForwardUncheckedWithContext(n uint, c rustfmt.Context) ExpnIndex {
	return v.ForwardWithContext(n, c)
}
func (v ExpnIndex) BackwardUncheckedWithContext(n uint, c rustfmt.Context) ExpnIndex {
	return v.BackwardWithContext(n, c)
}

func ExpnIndexStep(ambient ...rustfmt.Context) index.IntervalStep[ExpnIndex] {
	c := rustfmt.Context{}
	if len(ambient) > 0 {
		c = ambient[0]
	}
	return index.IntervalStep[ExpnIndex]{TypeName: "rustc_span::hygiene::ExpnIndex", CloneValue: func(v ExpnIndex) ExpnIndex { return v }, EqualFn: func(a, b ExpnIndex) bool { return a.Equal(b) }, PartialCompareFn: func(a, b ExpnIndex) index.IntervalOrdering { return index.IntervalOrdering(a.Compare(b)) }, StepsBetweenFn: func(a, b ExpnIndex) (uint, uint, bool) {
		lower, upper := ExpnIndexStepsBetween(a, b)
		hi, ok := upper.Get()
		return lower, hi, ok
	}, ForwardCheckedFn: func(v ExpnIndex, n uint) (ExpnIndex, bool) { return v.ForwardCheckedWithContext(n, c).Get() }, BackwardCheckedFn: func(v ExpnIndex, n uint) (ExpnIndex, bool) { return v.BackwardCheckedWithContext(n, c).Get() }, ForwardOverflowingFn: func(v ExpnIndex, n uint) (ExpnIndex, bool) { return v.ForwardOverflowingWithContext(n, c) }, BackwardOverflowingFn: func(v ExpnIndex, n uint) (ExpnIndex, bool) { return v.BackwardOverflowingWithContext(n, c) }, ForwardOverride: func(v ExpnIndex, n uint) ExpnIndex { return v.ForwardWithContext(n, c) }, BackwardOverride: func(v ExpnIndex, n uint) ExpnIndex { return v.BackwardWithContext(n, c) }, ForwardUncheckedOverride: func(v ExpnIndex, n uint) ExpnIndex { return v.ForwardUncheckedWithContext(n, c) }, BackwardUncheckedOverride: func(v ExpnIndex, n uint) ExpnIndex { return v.BackwardUncheckedWithContext(n, c) }}
}
