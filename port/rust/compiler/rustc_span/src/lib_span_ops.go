// SPDX-License-Identifier: MIT OR Apache-2.0
// Span methods from pinned compiler/rustc_span/src/lib.rs. Getter and callback
// order is intentional: source dependency tracking is observable.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (value Span) ParentWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[LocalDefId] {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		return None[LocalDefId]()
	}
	if inlineParentPrefix.get(bits) == 6 {
		return Some(LocalDefId{DefIndexFromU32WithContext(inlineParentFromSpan(value).parent, ctx)})
	}
	if inlinePairPrefix.get(bits) == 2 {
		return Some(LocalDefId{DefIndexFromU32WithContext(inlinePairFromSpan(value).parent, ctx)})
	}
	return internedFromSpan(value).dataWithContext(session, ctx).Parent
}
func (value Span) IsDummyWithContext(session *SessionGlobals, ctx rustfmt.Context) bool {
	bits := value.bits()
	if inlineCtxtPrefix.get(bits) == 0 {
		s := inlineCtxtFromSpan(value)
		return s.Lo == 0 && s.Len == 0
	}
	if inlineParentPrefix.get(bits) == 6 {
		s := inlineParentFromSpan(value)
		return s.lo == 0 && s.length == 0
	}
	if inlinePairPrefix.get(bits) == 2 {
		s := inlinePairFromSpan(value)
		return s.lo == 0 && s.length == 0
	}
	return internedFromSpan(value).dataWithContext(session, ctx).IsDummy()
}

func WithRootCtxt(session *SessionGlobals, lo, hi BytePos, config ...SpanEncodingConfig) Span {
	return NewSpan(session, lo, hi, RootSyntaxContext(), None[LocalDefId](), config...)
}
func (value Span) ShrinkToLo(session *SessionGlobals, config ...SpanEncodingConfig) Span {
	return value.ShrinkToLoWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) ShrinkToLoWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	data := value.dataUntrackedWithContext(session, p, ctx)
	return newSpanWithContext(session, data.Lo, data.Lo, data.Ctxt, data.Parent, p, ctx)
}
func (value Span) ShrinkToHi(session *SessionGlobals, config ...SpanEncodingConfig) Span {
	return value.ShrinkToHiWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) ShrinkToHiWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	data := value.dataUntrackedWithContext(session, p, ctx)
	return newSpanWithContext(session, data.Hi, data.Hi, data.Ctxt, data.Parent, p, ctx)
}
func (value Span) IsEmpty(session *SessionGlobals, config ...SpanEncodingConfig) bool {
	return value.IsEmptyWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) IsEmptyWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) bool {
	data := value.dataUntrackedWithContext(session, spanEncodingConfig(session, config...), ctx)
	return data.Hi == data.Lo
}
func (value Span) SubstituteDummy(session *SessionGlobals, other Span, config ...SpanEncodingConfig) Span {
	if value.IsDummy(session) {
		return other
	}
	return value
}
func (value Span) Contains(session *SessionGlobals, other Span, config ...SpanEncodingConfig) bool {
	return value.ContainsWithContext(session, other, rustfmt.Context{}, config...)
}
func (value Span) ContainsWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context, config ...SpanEncodingConfig) bool {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	rhs := other.dataWithContext(session, p, ctx)
	return data.Contains(rhs)
}
func (value Span) Overlaps(session *SessionGlobals, other Span, config ...SpanEncodingConfig) bool {
	return value.OverlapsWithContext(session, other, rustfmt.Context{}, config...)
}
func (value Span) OverlapsWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context, config ...SpanEncodingConfig) bool {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	rhs := other.dataWithContext(session, p, ctx)
	return data.Lo < rhs.Hi && rhs.Lo < data.Hi
}
func (value Span) OverlapsOrAdjacent(session *SessionGlobals, other Span, config ...SpanEncodingConfig) bool {
	return value.OverlapsOrAdjacentWithContext(session, other, rustfmt.Context{}, config...)
}
func (value Span) OverlapsOrAdjacentWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context, config ...SpanEncodingConfig) bool {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	rhs := other.dataWithContext(session, p, ctx)
	return data.Lo <= rhs.Hi && rhs.Lo <= data.Hi
}
func (value Span) TrimStart(session *SessionGlobals, other Span, config ...SpanEncodingConfig) Option[Span] {
	return value.TrimStartWithContext(session, other, rustfmt.Context{}, config...)
}
func (value Span) TrimStartWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[Span] {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	rhs := other.dataWithContext(session, p, ctx)
	if data.Hi > rhs.Hi {
		return Some(newSpanWithContext(session, max(data.Lo, rhs.Hi), data.Hi, data.Ctxt, data.Parent, p, ctx))
	}
	return None[Span]()
}
func (value Span) TrimEnd(session *SessionGlobals, other Span, config ...SpanEncodingConfig) Option[Span] {
	return value.TrimEndWithContext(session, other, rustfmt.Context{}, config...)
}
func (value Span) TrimEndWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[Span] {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	rhs := other.dataWithContext(session, p, ctx)
	if data.Lo < rhs.Lo {
		return Some(newSpanWithContext(session, data.Lo, min(data.Hi, rhs.Lo), data.Ctxt, data.Parent, p, ctx))
	}
	return None[Span]()
}
func (value Span) Edition(session *SessionGlobals) Edition {
	return value.EditionWithContext(session, rustfmt.Context{})
}
func (value Span) EditionWithContext(session *SessionGlobals, ctx rustfmt.Context) Edition {
	return value.Ctxt().EditionWithContext(session, ctx)
}
func (value Span) IsRust2015(session *SessionGlobals) bool {
	return value.Edition(session).IsRust2015()
}
func (value Span) AtLeastRust2018(session *SessionGlobals) bool {
	return value.Edition(session).AtLeastRust2018()
}
func (value Span) AtLeastRust2021(session *SessionGlobals) bool {
	return value.Edition(session).AtLeastRust2021()
}
func (value Span) AtLeastRust2024(session *SessionGlobals) bool {
	return value.Edition(session).AtLeastRust2024()
}
func spanUseExpnData[R any](session *SessionGlobals, ctxt SyntaxContext, ctx rustfmt.Context, body func(*ExpnData) R) R {
	data := ctxt.OuterExpnDataWithContext(session, ctx)
	defer spanCloseOwnedCallback(ctx, data.CloseWithContext)
	return body(&data)
}
func (value Span) InDeriveExpansion(session *SessionGlobals) bool {
	return value.InDeriveExpansionWithContext(session, rustfmt.Context{})
}
func (value Span) InDeriveExpansionWithContext(session *SessionGlobals, ctx rustfmt.Context) bool {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) bool { return data.Kind.Tag == ExpnMacro && data.Kind.MacroKind == DeriveMacro })
}
func (value Span) IsFromAsyncAwait(session *SessionGlobals) bool {
	return value.IsFromAsyncAwaitWithContext(session, rustfmt.Context{})
}
func (value Span) IsFromAsyncAwaitWithContext(session *SessionGlobals, ctx rustfmt.Context) bool {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) bool {
		return data.Kind.Tag == ExpnDesugaring && (data.Kind.Desugaring.Tag == DesugarAsync || data.Kind.Desugaring.Tag == DesugarAwait)
	})
}
func (value Span) ParentCallsite(session *SessionGlobals) Option[Span] {
	return value.ParentCallsiteWithContext(session, rustfmt.Context{})
}
func (value Span) ParentCallsiteWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[Span] {
	ctxt := value.Ctxt()
	if ctxt.IsRoot() {
		return None[Span]()
	}
	return Some(spanUseExpnData(session, ctxt, ctx, func(data *ExpnData) Span { return data.CallSite }))
}
func (value Span) SourceCallsite(session *SessionGlobals) Span {
	return value.SourceCallsiteWithContext(session, rustfmt.Context{})
}
func (value Span) SourceCallsiteWithContext(session *SessionGlobals, ctx rustfmt.Context) Span {
	ctxt := value.Ctxt()
	if ctxt.IsRoot() {
		return value
	}
	return spanUseExpnData(session, ctxt, ctx, func(data *ExpnData) Span { return data.CallSite.SourceCallsiteWithContext(session, ctx) })
}
func (value Span) CanBeUsedForSuggestions(session *SessionGlobals, config ...SpanEncodingConfig) bool {
	return value.CanBeUsedForSuggestionsWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) CanBeUsedForSuggestionsWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) bool {
	if !value.FromExpansion() {
		return true
	}
	if !value.InDeriveExpansionWithContext(session, ctx) {
		return false
	}
	p := spanEncodingConfig(session, config...)
	var parentLo, parentHi BytePos
	parent, present := value.ParentCallsiteWithContext(session, ctx).Get()
	if present {
		parentLo = parent.dataWithContext(session, p, ctx).Lo
		parentHi = parent.dataWithContext(session, p, ctx).Hi
	}
	lo := value.dataWithContext(session, p, ctx).Lo
	hi := value.dataWithContext(session, p, ctx).Hi
	return !present || parentLo != lo || parentHi != hi
}
func (value Span) FindAncestorInside(session *SessionGlobals, outer Span, config ...SpanEncodingConfig) Option[Span] {
	return value.FindAncestorInsideWithContext(session, outer, rustfmt.Context{}, config...)
}
func (value Span) FindAncestorInsideWithContext(session *SessionGlobals, outer Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[Span] {
	for !outer.ContainsWithContext(session, value, ctx, config...) {
		parent, ok := value.ParentCallsiteWithContext(session, ctx).Get()
		if !ok {
			return None[Span]()
		}
		value = parent
	}
	return Some(value)
}
func (value Span) FindAncestorInSameCtxt(session *SessionGlobals, other Span) Option[Span] {
	return value.FindAncestorInSameCtxtWithContext(session, other, rustfmt.Context{})
}
func (value Span) FindAncestorInSameCtxtWithContext(session *SessionGlobals, other Span, ctx rustfmt.Context) Option[Span] {
	for value.Ctxt() != other.Ctxt() {
		parent, ok := value.ParentCallsiteWithContext(session, ctx).Get()
		if !ok {
			return None[Span]()
		}
		value = parent
	}
	return Some(value)
}
func (value Span) FindAncestorInsideSameCtxt(session *SessionGlobals, outer Span, config ...SpanEncodingConfig) Option[Span] {
	return value.FindAncestorInsideSameCtxtWithContext(session, outer, rustfmt.Context{}, config...)
}
func (value Span) FindAncestorInsideSameCtxtWithContext(session *SessionGlobals, outer Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[Span] {
	for !outer.ContainsWithContext(session, value, ctx, config...) || value.Ctxt() != outer.Ctxt() {
		parent, ok := value.ParentCallsiteWithContext(session, ctx).Get()
		if !ok {
			return None[Span]()
		}
		value = parent
	}
	return Some(value)
}
func (value Span) FindAncestorNotFromMacro(session *SessionGlobals) Option[Span] {
	return value.FindAncestorNotFromMacroWithContext(session, rustfmt.Context{})
}
func (value Span) FindAncestorNotFromMacroWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[Span] {
	for value.FromExpansion() {
		parent, ok := value.ParentCallsiteWithContext(session, ctx).Get()
		if !ok {
			return None[Span]()
		}
		value = parent
	}
	return Some(value)
}
func (value Span) SourceCallee(session *SessionGlobals) Option[ExpnData] {
	return value.SourceCalleeWithContext(session, rustfmt.Context{})
}
func (value Span) SourceCalleeWithContext(session *SessionGlobals, ctx rustfmt.Context) (result Option[ExpnData]) {
	ctxt := value.Ctxt()
	success := false
	defer func() {
		if payload := recover(); payload != nil {
			if ctx.ProcessingPanic {
				ctx.ResumeUnwind(payload)
			}
			if !success {
				if data, ok := result.Get(); ok {
					unwind := ctx
					unwind.Unwinding = true
					spanCleanupDuringUnwindWithContext(unwind, func() { data.CloseWithContext(unwind) })
				}
			}
			ctx.ResumeUnwind(payload)
		}
	}()
	for !ctxt.IsRoot() {
		data := ctxt.OuterExpnDataWithContext(session, ctx)
		ctxt = data.CallSite.Ctxt()
		if previous, ok := result.Get(); ok {
			func() { defer func() { result = Some(data) }(); previous.CloseWithContext(ctx) }()
		} else {
			result = Some(data)
		}
	}
	success = true
	return result
}
func (value Span) AllowsUnstable(session *SessionGlobals, feature Symbol) bool {
	return value.AllowsUnstableWithContext(session, feature, rustfmt.Context{})
}
func (value Span) AllowsUnstableWithContext(session *SessionGlobals, feature Symbol, ctx rustfmt.Context) bool {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) bool {
		if features, ok := data.AllowInternalUnstable.Get(); ok {
			for _, f := range *features.BorrowWithContext(ctx) {
				if f == feature {
					return true
				}
			}
		}
		return false
	})
}
func (value Span) IsDesugaring(session *SessionGlobals, kind DesugaringKind) bool {
	return value.IsDesugaringWithContext(session, kind, rustfmt.Context{})
}
func (value Span) IsDesugaringWithContext(session *SessionGlobals, kind DesugaringKind, ctx rustfmt.Context) bool {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) bool { return data.Kind.Tag == ExpnDesugaring && data.Kind.Desugaring == kind })
}
func (value Span) DesugaringKind(session *SessionGlobals) Option[DesugaringKind] {
	return value.DesugaringKindWithContext(session, rustfmt.Context{})
}
func (value Span) DesugaringKindWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[DesugaringKind] {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) Option[DesugaringKind] {
		if data.Kind.Tag == ExpnDesugaring {
			return Some(data.Kind.Desugaring)
		}
		return None[DesugaringKind]()
	})
}
func (value Span) AllowsUnsafe(session *SessionGlobals) bool {
	return value.AllowsUnsafeWithContext(session, rustfmt.Context{})
}
func (value Span) AllowsUnsafeWithContext(session *SessionGlobals, ctx rustfmt.Context) bool {
	return spanUseExpnData(session, value.Ctxt(), ctx, func(data *ExpnData) bool { return data.AllowInternalUnsafe })
}

func (value Span) NormalizeToMacros20(session *SessionGlobals, config ...SpanEncodingConfig) Span {
	return value.NormalizeToMacros20WithContext(session, rustfmt.Context{}, config...)
}
func (value Span) NormalizeToMacros20WithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext { return c.NormalizeToMacros20WithContext(session, ctx) }, spanEncodingConfig(session, config...), ctx)
}
func (value Span) NormalizeToMacroRules(session *SessionGlobals, config ...SpanEncodingConfig) Span {
	return value.NormalizeToMacroRulesWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) NormalizeToMacroRulesWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext { return c.NormalizeToMacroRulesWithContext(session, ctx) }, spanEncodingConfig(session, config...), ctx)
}
func (value Span) ApplyMark(session *SessionGlobals, id ExpnId, t Transparency, config ...SpanEncodingConfig) Span {
	return value.ApplyMarkWithContext(session, id, t, rustfmt.Context{}, config...)
}
func (value Span) ApplyMarkWithContext(session *SessionGlobals, id ExpnId, t Transparency, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	return value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext { return c.ApplyMarkWithContext(session, id, t, ctx) }, spanEncodingConfig(session, config...), ctx)
}
func (value *Span) RemoveMark(session *SessionGlobals, config ...SpanEncodingConfig) ExpnId {
	return value.RemoveMarkWithContext(session, rustfmt.Context{}, config...)
}
func (value *Span) RemoveMarkWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) ExpnId {
	mark := RootExpnId()
	*value = value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext { mark = c.RemoveMarkWithContext(session, ctx); return c }, spanEncodingConfig(session, config...), ctx)
	return mark
}
func (value *Span) Adjust(session *SessionGlobals, id ExpnId, config ...SpanEncodingConfig) Option[ExpnId] {
	return value.AdjustWithContext(session, id, rustfmt.Context{}, config...)
}
func (value *Span) AdjustWithContext(session *SessionGlobals, id ExpnId, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[ExpnId] {
	mark := None[ExpnId]()
	*value = value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext { mark = c.AdjustWithContext(session, id, ctx); return c }, spanEncodingConfig(session, config...), ctx)
	return mark
}
func (value *Span) NormalizeToMacros20AndAdjust(session *SessionGlobals, id ExpnId, config ...SpanEncodingConfig) Option[ExpnId] {
	return value.NormalizeToMacros20AndAdjustWithContext(session, id, rustfmt.Context{}, config...)
}
func (value *Span) NormalizeToMacros20AndAdjustWithContext(session *SessionGlobals, id ExpnId, ctx rustfmt.Context, config ...SpanEncodingConfig) Option[ExpnId] {
	mark := None[ExpnId]()
	*value = value.mapCtxtWithContext(session, func(c SyntaxContext) SyntaxContext {
		mark = c.NormalizeToMacros20AndAdjustWithContext(session, id, ctx)
		return c
	}, spanEncodingConfig(session, config...), ctx)
	return mark
}
func (value Span) withCtxtFromMark(session *SessionGlobals, id ExpnId, t Transparency, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	ctxt := RootSyntaxContext().ApplyMarkWithContext(session, id, t, ctx)
	return value.mapCtxtWithContext(session, func(SyntaxContext) SyntaxContext { return ctxt }, spanEncodingConfig(session, config...), ctx)
}
func (value Span) WithDefSiteCtxt(session *SessionGlobals, id ExpnId, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, Opaque, rustfmt.Context{}, config...)
}
func (value Span) WithCallSiteCtxt(session *SessionGlobals, id ExpnId, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, Transparent, rustfmt.Context{}, config...)
}
func (value Span) WithMixedSiteCtxt(session *SessionGlobals, id ExpnId, config ...SpanEncodingConfig) Span {
	return value.withCtxtFromMark(session, id, SemiOpaque, rustfmt.Context{}, config...)
}
func (value Span) StableHash(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	hcx.StableHashSpan(value.ToRawSpan(), h)
}
func (value Span) StableHashWithContext(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	datastructures.StableHashSpanWithContext(hcx, value.ToRawSpan(), h, ctx)
}
