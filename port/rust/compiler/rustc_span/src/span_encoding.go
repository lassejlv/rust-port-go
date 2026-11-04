// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: compiler/rustc_span/src/span_encoding.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// The surrounding lib/def_id/hygiene runtime is explicitly partial. Debug's
// real formatter is an explicit dependency; no source-map output is fabricated.
package rustcspan

import (
	"fmt"
	"sync/atomic"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Span stores the exact 64 encoded bits in two words, preserving size 8 and
// alignment 4 without an unsafe packed u64 access. No Rust/Go FFI representation
// or target-endian memory-layout guarantee is implied by the logical bit API.
type Span struct{ low, high uint32 }

// spanPanicContext is the cold failure-site runtime witness. Production uses
// rustfmt.Context; a witness must panic or abort and must never return. Keeping
// this structural allows ordering to be observed at the actual raise site.
type spanPanicContext interface{ Panic(any) }

func spanFromBits(bits uint64) Span             { return Span{uint32(bits), uint32(bits >> 32)} }
func (value Span) bits() uint64                 { return uint64(value.low) | uint64(value.high)<<32 }
func DummySpan() Span                           { return Span{} }
func (value Span) Clone() Span                  { return value }
func (value *Span) CloneFrom(other Span)        { *value = other }
func (value Span) Equal(other Span) bool        { return value == other }
func (value Span) NotEqual(other Span) bool     { return !value.Equal(other) }
func (value Span) Hash(h rustchashes.U64Hasher) { h.WriteU64(value.bits()) }

type spanField struct{ shift, width uint32 }

func (value spanField) clone() spanField { return value }

func spanFieldTop(width uint32) spanField            { return spanField{64 - width, width} }
func (field spanField) below(width uint32) spanField { return spanField{field.shift - width, width} }
func (field spanField) mask() uint64                 { return uint64(1)<<field.width - 1 }
func (field spanField) get(bits uint64) uint32       { return uint32(bits >> field.shift & field.mask()) }
func (field spanField) set(value uint32, config SpanEncodingConfig) uint64 {
	if config.DebugAssertions && value > uint32(field.mask()) {
		panic("SpanField: value exceeds mask")
	}
	return uint64(value) << field.shift
}
func (field spanField) tooBig(value uint32) uint32 { return value &^ uint32(field.mask()) }
func (field spanField) assertIsBottom() {
	if field.shift != 0 {
		panic("SpanField: field is not bottom")
	}
}

var (
	inlineCtxtPrefix   = spanFieldTop(1)
	inlineCtxtLen      = inlineCtxtPrefix.below(15)
	inlineCtxtCtxt     = inlineCtxtLen.below(16)
	inlineCtxtLo       = inlineCtxtCtxt.below(32)
	inlineParentPrefix = spanFieldTop(3)
	inlineParentLen    = inlineParentPrefix.below(14)
	inlineParentParent = inlineParentLen.below(15)
	inlineParentLo     = inlineParentParent.below(32)
	inlinePairPrefix   = spanFieldTop(2)
	inlinePairLen      = inlinePairPrefix.below(7)
	inlinePairCtxt     = inlinePairLen.below(15)
	inlinePairParent   = inlinePairCtxt.below(16)
	inlinePairLo       = inlinePairParent.below(24)
	internedPrefix     = spanFieldTop(3)
	internedCtxt       = internedPrefix.below(29)
	internedIndex      = internedCtxt.below(32)
)

func init() {
	inlineCtxtLo.assertIsBottom()
	inlineParentLo.assertIsBottom()
	inlinePairLo.assertIsBottom()
	internedIndex.assertIsBottom()
}

// InlineCtxt exposes the original crate-visible inline-only fast path.
type InlineCtxt struct{ Lo, Len, Ctxt uint32 }

func (value InlineCtxt) Clone() InlineCtxt           { return value }
func (value *InlineCtxt) CloneFrom(other InlineCtxt) { *value = other.Clone() }

func InlineCtxtTryNewSpan(lo, length, ctxt uint32) (Span, bool) {
	if inlineCtxtLen.tooBig(length)|inlineCtxtCtxt.tooBig(ctxt) != 0 {
		return Span{}, false
	}
	config := DefaultSpanEncodingConfig()
	return spanFromBits(inlineCtxtPrefix.set(0, config) | inlineCtxtLen.set(length, config) | inlineCtxtCtxt.set(ctxt, config) | inlineCtxtLo.set(lo, config)), true
}
func inlineCtxtFromSpan(value Span) InlineCtxt {
	bits := value.bits()
	return InlineCtxt{inlineCtxtLo.get(bits), inlineCtxtLen.get(bits), inlineCtxtCtxt.get(bits)}
}
func InlineCtxtTryFromSpan(value Span) (InlineCtxt, bool) {
	if inlineCtxtPrefix.get(value.bits()) == 0 {
		return inlineCtxtFromSpan(value), true
	}
	return InlineCtxt{}, false
}
func (value InlineCtxt) data(config SpanEncodingConfig) SpanData {
	return value.dataWithContext(config, rustfmt.Context{})
}
func (value InlineCtxt) dataWithContext(config SpanEncodingConfig, context spanPanicContext) SpanData {
	return SpanData{BytePos(value.Lo), BytePos(spanDebugStrictAddWithContext(value.Lo, value.Len, config, context)), SyntaxContextFromU32(value.Ctxt), None[LocalDefId]()}
}

type inlineParent struct{ lo, length, parent uint32 }

func (value inlineParent) clone() inlineParent { return value }

func inlineParentTryNewSpan(lo, length, ctxt, parent uint32) (Span, bool) {
	if inlineParentLen.tooBig(length)|ctxt|inlineParentParent.tooBig(parent) != 0 {
		return Span{}, false
	}
	config := DefaultSpanEncodingConfig()
	return spanFromBits(inlineParentPrefix.set(6, config) | inlineParentLen.set(length, config) | inlineParentParent.set(parent, config) | inlineParentLo.set(lo, config)), true
}
func inlineParentFromSpan(value Span) inlineParent {
	bits := value.bits()
	return inlineParent{inlineParentLo.get(bits), inlineParentLen.get(bits), inlineParentParent.get(bits)}
}
func (value inlineParent) data(config SpanEncodingConfig) SpanData {
	return value.dataWithContext(config, rustfmt.Context{})
}
func (value inlineParent) dataWithContext(config SpanEncodingConfig, context spanPanicContext) SpanData {
	return SpanData{BytePos(value.lo), BytePos(spanDebugStrictAddWithContext(value.lo, value.length, config, context)), RootSyntaxContext(), Some(LocalDefId{DefIndexFromU32(value.parent)})}
}

type inlinePair struct{ lo, length, ctxt, parent uint32 }

func (value inlinePair) clone() inlinePair { return value }

func inlinePairTryNewSpan(lo, length, ctxt, parent uint32) (Span, bool) {
	if inlinePairLo.tooBig(lo)|inlinePairLen.tooBig(length)|inlinePairCtxt.tooBig(ctxt)|inlinePairParent.tooBig(parent) != 0 {
		return Span{}, false
	}
	config := DefaultSpanEncodingConfig()
	return spanFromBits(inlinePairPrefix.set(2, config) | inlinePairLen.set(length, config) | inlinePairCtxt.set(ctxt, config) | inlinePairParent.set(parent, config) | inlinePairLo.set(lo, config)), true
}
func inlinePairFromSpan(value Span) inlinePair {
	bits := value.bits()
	return inlinePair{inlinePairLo.get(bits), inlinePairLen.get(bits), inlinePairCtxt.get(bits), inlinePairParent.get(bits)}
}
func (value inlinePair) data(config SpanEncodingConfig) SpanData {
	return value.dataWithContext(config, rustfmt.Context{})
}
func (value inlinePair) dataWithContext(config SpanEncodingConfig, context spanPanicContext) SpanData {
	return SpanData{BytePos(value.lo), BytePos(spanDebugStrictAddWithContext(value.lo, value.length, config, context)), SyntaxContextFromU32(value.ctxt), Some(LocalDefId{DefIndexFromU32(value.parent)})}
}

type internedSpan struct{ ctxt, index uint32 }

func (value internedSpan) clone() internedSpan { return value }

func internedNewSpan(ctxt, index uint32, config SpanEncodingConfig) Span {
	return spanFromBits(internedPrefix.set(7, config) | internedCtxt.set(ctxt, config) | internedIndex.set(index, config))
}
func internedFromSpan(value Span) internedSpan {
	bits := value.bits()
	return internedSpan{internedCtxt.get(bits), internedIndex.get(bits)}
}
func (value internedSpan) data(session *SessionGlobals) SpanData {
	return value.dataWithContext(session, rustfmt.Context{})
}
func (value internedSpan) dataWithContext(session *SessionGlobals, context spanPanicContext) SpanData {
	data := spanInternerDataWithContext(session, value.index, context)
	return SpanData{data.lo, data.hi, SyntaxContextFromU32(value.ctxt), data.parent}
}

func spanDebugStrictAdd(value, other uint32, config SpanEncodingConfig) uint32 {
	return spanDebugStrictAddWithContext(value, other, config, rustfmt.Context{})
}
func spanDebugStrictAddWithContext(value, other uint32, config SpanEncodingConfig, context spanPanicContext) uint32 {
	if config.DebugAssertions && config.OverflowChecks && other > ^uint32(0)-value {
		context.Panic("span_encoding: attempt to add with overflow")
	}
	return value + other
}

// NewSpan canonicalizes reversed positions. NewOrderedSpan retains the source
// debug-only ordered-position assertion and ordinary subtraction profile.
func NewSpan(session *SessionGlobals, lo, hi BytePos, ctxt SyntaxContext, parent Option[LocalDefId], explicit ...SpanEncodingConfig) Span {
	return newSpanWithContext(session, lo, hi, ctxt, parent, spanEncodingConfig(session, explicit...), rustfmt.Context{})
}
func newSpanWithContext(session *SessionGlobals, lo, hi BytePos, ctxt SyntaxContext, parent Option[LocalDefId], config SpanEncodingConfig, context spanPanicContext) Span {
	if lo > hi {
		lo, hi = hi, lo
	}
	return newOrderedSpanWithContext(session, lo, hi, ctxt, parent, config, context)
}
func NewOrderedSpan(session *SessionGlobals, lo, hi BytePos, ctxt SyntaxContext, parent Option[LocalDefId], explicit ...SpanEncodingConfig) Span {
	return newOrderedSpanWithContext(session, lo, hi, ctxt, parent, spanEncodingConfig(session, explicit...), rustfmt.Context{})
}
func newOrderedSpanWithContext(session *SessionGlobals, lo, hi BytePos, ctxt SyntaxContext, parent Option[LocalDefId], config SpanEncodingConfig, context spanPanicContext) Span {
	if config.DebugAssertions && lo > hi {
		context.Panic("span_encoding: assertion failed: lo <= hi")
	}
	if config.OverflowChecks && hi < lo {
		context.Panic("span_encoding: attempt to subtract with overflow")
	}
	length := uint32(hi) - uint32(lo)
	ctxt32 := ctxt.AsU32()
	if p, ok := parent.Get(); ok {
		if span, ok := tryNewSpanWithParent(uint32(lo), length, ctxt32, p); ok {
			return span
		}
	} else if span, ok := InlineCtxtTryNewSpan(uint32(lo), length, ctxt32); ok {
		return span
	}
	if internedCtxt.tooBig(ctxt32) != 0 {
		context.Panic("span_encoding: context exceeds hard limit")
	}
	index := spanInternerInternWithContext(session, spanDataNoCtxt{lo, hi, parent}, context)
	return internedNewSpan(ctxt32, index, config)
}
func tryNewSpanWithParent(lo, length, ctxt uint32, parent LocalDefId) (Span, bool) {
	index := parent.LocalDefIndex.AsU32()
	if span, ok := inlineParentTryNewSpan(lo, length, ctxt, index); ok {
		return span, true
	}
	return inlinePairTryNewSpan(lo, length, ctxt, index)
}

func (value Span) Data(session *SessionGlobals, explicit ...SpanEncodingConfig) SpanData {
	return value.dataWithContext(session, spanEncodingConfig(session, explicit...), rustfmt.Context{})
}
func (value Span) dataWithContext(session *SessionGlobals, config SpanEncodingConfig, context spanPanicContext) SpanData {
	data := value.dataUntrackedWithContext(session, config, context)
	if parent, ok := data.Parent.Get(); ok {
		trackSpanParent(parent)
	}
	return data
}
func (value Span) DataUntracked(session *SessionGlobals, explicit ...SpanEncodingConfig) SpanData {
	return value.dataUntrackedWithContext(session, spanEncodingConfig(session, explicit...), rustfmt.Context{})
}
func (value Span) dataUntrackedWithContext(session *SessionGlobals, config SpanEncodingConfig, context spanPanicContext) SpanData {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		return inlineCtxtFromSpan(value).dataWithContext(config, context)
	}
	if inlineParentPrefix.get(bits) == 6 {
		return inlineParentFromSpan(value).dataWithContext(config, context)
	}
	if inlinePairPrefix.get(bits) == 2 {
		return inlinePairFromSpan(value).dataWithContext(config, context)
	}
	return internedFromSpan(value).dataWithContext(session, context)
}
func (value Span) FromExpansion() bool { return !value.Ctxt().IsRoot() }
func (value Span) IsDummy(session *SessionGlobals) bool {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		span := inlineCtxtFromSpan(value)
		return span.Lo == 0 && span.Len == 0
	}
	if inlineParentPrefix.get(bits) == 6 {
		span := inlineParentFromSpan(value)
		return span.lo == 0 && span.length == 0
	}
	if inlinePairPrefix.get(bits) == 2 {
		span := inlinePairFromSpan(value)
		return span.lo == 0 && span.length == 0
	}
	return internedFromSpan(value).data(session).IsDummy()
}

// MapCtxt accepts a trivial-drop callback. Use MapCtxtOwned for a source FnOnce
// with an owned capture whose destruction can be observed.
func (value Span) MapCtxt(session *SessionGlobals, mapContext func(SyntaxContext) SyntaxContext, explicit ...SpanEncodingConfig) Span {
	profile := spanEncodingConfig(session, explicit...)
	return value.mapCtxtWithContext(session, mapContext, profile, rustfmt.Context{})
}
func (value Span) mapCtxtWithContext(session *SessionGlobals, mapContext func(SyntaxContext) SyntaxContext, profile SpanEncodingConfig, context spanPanicContext) Span {
	bits := value.bits()
	var data SpanData
	if inlineCtxtPrefix.get(bits) == 0 {
		span := inlineCtxtFromSpan(value)
		newContext := mapContext(SyntaxContextFromU32(span.Ctxt))
		if encoded, ok := InlineCtxtTryNewSpan(span.Lo, span.Len, newContext.AsU32()); ok {
			return encoded
		}
		data := span.dataWithContext(profile, context)
		return newOrderedSpanWithContext(session, data.Lo, data.Hi, newContext, data.Parent, profile, context)
	}
	if inlineParentPrefix.get(bits) == 6 {
		data = inlineParentFromSpan(value).dataWithContext(profile, context)
	} else if inlinePairPrefix.get(bits) == 2 {
		data = inlinePairFromSpan(value).dataWithContext(profile, context)
	} else {
		data = internedFromSpan(value).dataWithContext(session, context)
	}
	return newOrderedSpanWithContext(session, data.Lo, data.Hi, mapContext(data.Ctxt), data.Parent, profile, context)
}
func (value Span) Ctxt() SyntaxContext {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		return SyntaxContextFromU32(inlineCtxtFromSpan(value).Ctxt)
	}
	if inlineParentPrefix.get(bits) == 6 {
		return RootSyntaxContext()
	}
	if inlinePairPrefix.get(bits) == 2 {
		return SyntaxContextFromU32(inlinePairFromSpan(value).ctxt)
	}
	return SyntaxContextFromU32(internedFromSpan(value).ctxt)
}
func (value Span) WithParent(session *SessionGlobals, parent Option[LocalDefId], explicit ...SpanEncodingConfig) Span {
	profile := spanEncodingConfig(session, explicit...)
	bits := value.bits()
	var data SpanData
	if inlineCtxtPrefix.get(bits) == 0 {
		span := inlineCtxtFromSpan(value)
		p, ok := parent.Get()
		if !ok {
			return value
		}
		if encoded, ok := tryNewSpanWithParent(span.Lo, span.Len, span.Ctxt, p); ok {
			return encoded
		}
		data = span.data(profile)
	} else if inlineParentPrefix.get(bits) == 6 {
		data = inlineParentFromSpan(value).data(profile)
	} else if inlinePairPrefix.get(bits) == 2 {
		data = inlinePairFromSpan(value).data(profile)
	} else {
		data = internedFromSpan(value).data(session)
	}
	if old, ok := data.Parent.Get(); ok {
		trackSpanParent(old)
	}
	return data.withParent(session, parent, profile)
}
func (value Span) Parent(session *SessionGlobals) Option[LocalDefId] {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		return None[LocalDefId]()
	}
	if inlineParentPrefix.get(bits) == 6 {
		return Some(LocalDefId{DefIndexFromU32(inlineParentFromSpan(value).parent)})
	}
	if inlinePairPrefix.get(bits) == 2 {
		return Some(LocalDefId{DefIndexFromU32(inlinePairFromSpan(value).parent)})
	}
	return internedFromSpan(value).data(session).Parent
}
func (value Span) ToRawSpan() datastructures.RawSpan {
	return datastructures.RawSpan{Value: value.bits()}
}
func SpanFromRawSpan(value datastructures.RawSpan) Span { return spanFromBits(value.Value) }

// OrdSpan deliberately has no Hash operation and is not a Go comparable key;
// callers must use its source-location Equal/Compare methods instead of bit Eq.
type OrdSpan struct {
	_    [0]func()
	Span Span
}

func (value OrdSpan) Clone() OrdSpan           { return OrdSpan{Span: value.Span} }
func (value *OrdSpan) CloneFrom(other OrdSpan) { *value = other.Clone() }
func (value OrdSpan) Equal(session *SessionGlobals, other OrdSpan, config ...SpanEncodingConfig) bool {
	profile := spanEncodingConfig(session, config...)
	lo, hi := value.Span.LoHi(session, profile)
	otherLo, otherHi := other.Span.LoHi(session, profile)
	return lo == otherLo && hi == otherHi
}
func (value OrdSpan) NotEqual(session *SessionGlobals, other OrdSpan, config ...SpanEncodingConfig) bool {
	return !value.Equal(session, other, config...)
}
func (value OrdSpan) Compare(session *SessionGlobals, other OrdSpan, config ...SpanEncodingConfig) int {
	profile := spanEncodingConfig(session, config...)
	lo, hi := value.Span.LoHi(session, profile)
	otherLo, otherHi := other.Span.LoHi(session, profile)
	if lo < otherLo {
		return -1
	}
	if lo > otherLo {
		return 1
	}
	if hi < otherHi {
		return -1
	}
	if hi > otherHi {
		return 1
	}
	return 0
}
func (value OrdSpan) PartialCompare(session *SessionGlobals, other OrdSpan, config ...SpanEncodingConfig) (int, bool) {
	return value.Compare(session, other, config...), true
}

type spanDataNoCtxt struct {
	lo, hi BytePos
	parent Option[LocalDefId]
}

func (value spanDataNoCtxt) clone() spanDataNoCtxt           { return value }
func (value spanDataNoCtxt) equal(other spanDataNoCtxt) bool { return value == other }

func (value spanDataNoCtxt) hash(h SpanDataHasher) {
	value.lo.Hash(h)
	value.hi.Hash(h)
	hashSpanParent(h, value.parent)
}

// SpanInterner preserves IndexSet insertion order and deduplication. Context is
// excluded from its key because every encoding stores it inline.
type SpanInterner struct {
	spans   []spanDataNoCtxt
	indices map[spanDataNoCtxt]uint32
}

func (interner *SpanInterner) intern(value spanDataNoCtxt) uint32 {
	if index, ok := interner.indices[value]; ok {
		return index
	}
	if interner.indices == nil {
		interner.indices = make(map[spanDataNoCtxt]uint32)
	}
	index := uint32(len(interner.spans))
	interner.spans = append(interner.spans, value)
	interner.indices[value] = index
	return index
}
func spanInternerIntern(session *SessionGlobals, value spanDataNoCtxt) uint32 {
	return spanInternerInternWithContext(session, value, rustfmt.Context{})
}
func spanInternerInternWithContext(session *SessionGlobals, value spanDataNoCtxt, context spanPanicContext) uint32 {
	return withSpanInternerWithContext(session, context, func(interner *SpanInterner) uint32 { return interner.intern(value) })
}
func spanInternerData(session *SessionGlobals, index uint32) spanDataNoCtxt {
	return spanInternerDataWithContext(session, index, rustfmt.Context{})
}
func spanInternerDataWithContext(session *SessionGlobals, index uint32, context spanPanicContext) spanDataNoCtxt {
	return withSpanInternerWithContext(session, context, func(interner *SpanInterner) spanDataNoCtxt {
		if uint64(index) >= uint64(len(interner.spans)) {
			context.Panic(fmt.Sprintf("runtime error: index out of range [%d] with length %d", index, len(interner.spans)))
		}
		return interner.spans[index]
	})
}
func withSpanInterner[T any](session *SessionGlobals, apply func(*SpanInterner) T) T {
	return withSpanInternerWithContext(session, rustfmt.Context{}, apply)
}
func withSpanInternerWithContext[T any](session *SessionGlobals, context spanPanicContext, apply func(*SpanInterner) T) T {
	if session == nil {
		context.Panic("span_encoding: session globals are not set")
	}
	session.spanMu.Lock()
	defer session.spanMu.Unlock()
	return apply(&session.spanInterner)
}

type spanTrackCallback struct{ call func(LocalDefId) }

var spanTrack atomic.Pointer[spanTrackCallback]

func defaultSpanTrack(LocalDefId) {}

// SwapSpanTrack maps the source's global atomic callback replacement. The
// callback runs after releasing the interner lock and may recurse or panic.
// Like the source fn pointer, callbacks are trivial-drop code/borrowed-state
// witnesses; this operation does not own a destructible capture environment.
func SwapSpanTrack(callback func(LocalDefId)) func(LocalDefId) {
	if callback == nil {
		panic("SPAN_TRACK: nil function is not a Rust fn pointer")
	}
	previous := spanTrack.Swap(&spanTrackCallback{callback})
	if previous == nil {
		return defaultSpanTrack
	}
	return previous.call
}
func trackSpanParent(parent LocalDefId) {
	if callback := spanTrack.Load(); callback != nil {
		callback.call(parent)
	}
}
