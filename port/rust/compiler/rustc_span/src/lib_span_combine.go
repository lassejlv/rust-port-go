// SPDX-License-Identifier: MIT OR Apache-2.0
// Span combination and iterator algorithms from pinned rustc_span/lib.rs.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func spanDebugStrictSubWithContext(value, other uint32, p SpanEncodingConfig, ctx rustfmt.Context) uint32 {
	if p.DebugAssertions && p.OverflowChecks && other > value {
		ctx.Panic("attempt to subtract with overflow")
	}
	return value - other
}

func spanTryMetavars(session *SessionGlobals, aCtxt, bCtxt SyntaxContext, aOrig, bOrig Span, ctx rustfmt.Context, p SpanEncodingConfig) (Option[SpanData], Option[SpanData]) {
	if session == nil || session.metavarSpans == nil {
		ctx.Panic("cannot access a scoped thread local variable without calling set first")
	}
	a := session.metavarSpans.GetWithContext(aOrig, ctx)
	b := session.metavarSpans.GetWithContext(bOrig, ctx)
	metaA, ap := a.Get()
	metaB, bp := b.Get()
	if ap && bp {
		bData := metaB.dataWithContext(session, p, ctx)
		if aCtxt == bData.Ctxt {
			return None[SpanData](), Some(bData)
		}
		aData := metaA.dataWithContext(session, p, ctx)
		if aData.Ctxt == bCtxt {
			return Some(aData), None[SpanData]()
		} else if aData.Ctxt == bData.Ctxt {
			return Some(aData), Some(bData)
		}
	} else if ap {
		aData := metaA.dataWithContext(session, p, ctx)
		if aData.Ctxt == bCtxt {
			return Some(aData), None[SpanData]()
		}
	} else if bp {
		bData := metaB.dataWithContext(session, p, ctx)
		if aCtxt == bData.Ctxt {
			return None[SpanData](), Some(bData)
		}
	}
	return None[SpanData](), None[SpanData]()
}
func spanPrepareToCombine(session *SessionGlobals, aOrig, bOrig Span, ctx rustfmt.Context, p SpanEncodingConfig) (a, b SpanData, parent Option[LocalDefId], fallback Span, ok bool) {
	a = aOrig.dataWithContext(session, p, ctx)
	b = bOrig.dataWithContext(session, p, ctx)
	if a.Ctxt != b.Ctxt {
		aNew, bNew := spanTryMetavars(session, a.Ctxt, b.Ctxt, aOrig, bOrig, ctx, p)
		if !aNew.IsSome() && !bNew.IsSome() {
			if a.Ctxt.IsRoot() {
				fallback = bOrig
			} else {
				fallback = aOrig
			}
			return
		}
		if value, present := aNew.Get(); present {
			a = value
		}
		if value, present := bNew.Get(); present {
			b = value
		}
		if p.DebugAssertions && a.Ctxt != b.Ctxt {
			ctx.Panic("assertion failed: a.ctxt == b.ctxt")
		}
	}
	if a.Parent == b.Parent {
		parent = a.Parent
	}
	ok = true
	return
}
func (value Span) WithNeighbor(session *SessionGlobals, neighbor Span, config ...SpanEncodingConfig) Span {
	return value.WithNeighborWithContext(session, neighbor, rustfmt.Context{}, config...)
}
func (value Span) WithNeighborWithContext(session *SessionGlobals, neighbor Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	a, _, _, _, ok := spanPrepareToCombine(session, value, neighbor, ctx, p)
	if !ok {
		return value
	}
	return newSpanWithContext(session, a.Lo, a.Hi, a.Ctxt, a.Parent, p, ctx)
}
func (value Span) To(session *SessionGlobals, end Span, config ...SpanEncodingConfig) Span {
	return value.ToWithContext(session, end, rustfmt.Context{}, config...)
}
func (value Span) ToWithContext(session *SessionGlobals, end Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	if from, ok := InlineCtxtTryFromSpan(value); ok {
		if to, ok := InlineCtxtTryFromSpan(end); ok && from.Ctxt == to.Ctxt {
			lo := min(from.Lo, to.Lo)
			fromHi := spanDebugStrictAddWithContext(from.Lo, from.Len, p, ctx)
			toHi := spanDebugStrictAddWithContext(to.Lo, to.Len, p, ctx)
			length := spanDebugStrictSubWithContext(max(fromHi, toHi), lo, p, ctx)
			if span, ok := InlineCtxtTryNewSpan(lo, length, from.Ctxt); ok {
				return span
			}
		}
	}
	return value.toNonInline(session, end, ctx, p)
}
func (value Span) toNonInline(session *SessionGlobals, end Span, ctx rustfmt.Context, p SpanEncodingConfig) Span {
	from, to, parent, fallback, ok := spanPrepareToCombine(session, value, end, ctx, p)
	if !ok {
		return fallback
	}
	return newOrderedSpanWithContext(session, min(from.Lo, to.Lo), max(from.Hi, to.Hi), from.Ctxt, parent, p, ctx)
}
func (value Span) Between(session *SessionGlobals, end Span, config ...SpanEncodingConfig) Span {
	return value.BetweenWithContext(session, end, rustfmt.Context{}, config...)
}
func (value Span) BetweenWithContext(session *SessionGlobals, end Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	from, to, parent, fallback, ok := spanPrepareToCombine(session, value, end, ctx, p)
	if !ok {
		return fallback
	}
	return newSpanWithContext(session, min(from.Hi, to.Hi), max(from.Lo, to.Lo), from.Ctxt, parent, p, ctx)
}
func (value Span) Until(session *SessionGlobals, end Span, config ...SpanEncodingConfig) Span {
	return value.UntilWithContext(session, end, rustfmt.Context{}, config...)
}
func (value Span) UntilWithContext(session *SessionGlobals, end Span, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	from, to, parent, fallback, ok := spanPrepareToCombine(session, value, end, ctx, p)
	if !ok {
		return fallback
	}
	return newSpanWithContext(session, min(from.Lo, to.Lo), max(from.Lo, to.Lo), from.Ctxt, parent, p, ctx)
}
func (value Span) FromInner(session *SessionGlobals, inner InnerSpan, config ...SpanEncodingConfig) Span {
	return value.FromInnerWithContext(session, inner, rustfmt.Context{}, config...)
}
func (value Span) FromInnerWithContext(session *SessionGlobals, inner InnerSpan, ctx rustfmt.Context, config ...SpanEncodingConfig) Span {
	p := spanEncodingConfig(session, config...)
	data := value.dataWithContext(session, p, ctx)
	lo := data.Lo.AddWithContext(BytePosFromUsize(inner.Start), p, ctx)
	hi := data.Lo.AddWithContext(BytePosFromUsize(inner.End), p, ctx)
	return newSpanWithContext(session, lo, hi, data.Ctxt, data.Parent, p, ctx)
}
func (value Span) SplitAt(session *SessionGlobals, pos uint32, config ...SpanEncodingConfig) (Span, Span) {
	return value.SplitAtWithContext(session, pos, rustfmt.Context{}, config...)
}
func (value Span) SplitAtWithContext(session *SessionGlobals, pos uint32, ctx rustfmt.Context, config ...SpanEncodingConfig) (Span, Span) {
	p := spanEncodingConfig(session, config...)
	hi := value.dataWithContext(session, p, ctx).Hi
	lo := value.dataWithContext(session, p, ctx).Lo
	length := spanPosSub(uint32(hi), uint32(lo), p, ctx, spanLibLocation(1084, 19))
	if p.DebugAssertions && pos > length {
		spanStaticPanicAt(ctx, spanLibLocation(1085, 9), "assertion failed: pos <= len")
	}
	split := BytePos(spanPosAdd(uint32(value.dataWithContext(session, p, ctx).Lo), pos, p, ctx, spanLibLocation(1087, 33)))
	leftLo := value.dataWithContext(session, p, ctx).Lo
	leftCtxt := value.Ctxt()
	leftParent := value.ParentWithContext(session, ctx)
	left := newSpanWithContext(session, leftLo, split, leftCtxt, leftParent, p, ctx)
	rightHi := value.dataWithContext(session, p, ctx).Hi
	rightCtxt := value.Ctxt()
	rightParent := value.ParentWithContext(session, ctx)
	right := newSpanWithContext(session, split, rightHi, rightCtxt, rightParent, p, ctx)
	return left, right
}

// MacroBacktraceIterator is the state of the original from_fn closure. Each
// yielded ExpnData is an independent owned value and must be closed by callers.
type MacroBacktraceIterator struct {
	session        *SessionGlobals
	span, previous Span
	context        rustfmt.Context
	profile        SpanEncodingConfig
}

func (value Span) MacroBacktrace(session *SessionGlobals, config ...SpanEncodingConfig) *MacroBacktraceIterator {
	return value.MacroBacktraceWithContext(session, rustfmt.Context{}, config...)
}
func (value Span) MacroBacktraceWithContext(session *SessionGlobals, ctx rustfmt.Context, config ...SpanEncodingConfig) *MacroBacktraceIterator {
	// The source from_fn closure captures only its Copy span state. Active
	// tracing metadata belongs to each next operation, not iterator creation.
	ctx.Runtime = nil
	return &MacroBacktraceIterator{session: session, span: value, previous: DummySpan(), context: ctx, profile: spanEncodingConfig(session, config...)}
}
func (iter *MacroBacktraceIterator) SizeHint() (uint, Option[uint]) { return 0, None[uint]() }

// IntoIterator moves the concrete from_fn state into the generic adapter.
// Callers relinquish this receiver after the move, as Rust takes self. The
// source closure contains only Copy spans and a session borrow; its Drop has
// no owned payload. Yielded ExpnData values retain their independent ownership.
func (iter *MacroBacktraceIterator) IntoIterator() rustvec.IntoIterator[ExpnData] {
	state := *iter
	*iter = MacroBacktraceIterator{}
	return rustvec.FromIterator(rustvec.NewIterator(state, rustvec.IteratorOps[MacroBacktraceIterator, ExpnData]{
		Next: func(ref rustvec.Ref[MacroBacktraceIterator]) (ExpnData, bool) {
			return ref.Value.NextWithContext(ref.Context).Get()
		},
		SizeHint: func(rustvec.Ref[MacroBacktraceIterator]) rustvec.Hint { return rustvec.Hint{} },
	}))
}
func (iter *MacroBacktraceIterator) Next() Option[ExpnData] {
	return iter.NextWithContext(iter.context)
}
func (iter *MacroBacktraceIterator) NextWithContext(ctx rustfmt.Context) Option[ExpnData] {
	for {
		ctxt := iter.span.Ctxt()
		if ctxt.IsRoot() {
			return None[ExpnData]()
		}
		data := ctxt.OuterExpnDataWithContext(iter.session, ctx)
		yielded := false
		recursive := func() bool {
			defer spanCloseOwnedCallback(ctx, func(drop rustfmt.Context) {
				if !yielded {
					data.CloseWithContext(drop)
				}
			})
			call := data.CallSite.dataWithContext(iter.session, iter.profile, ctx)
			previous := iter.previous.dataWithContext(iter.session, iter.profile, ctx)
			same := call.Lo == previous.Lo && call.Hi == previous.Hi
			iter.previous = iter.span
			iter.span = data.CallSite
			if !same {
				yielded = true
			}
			return same
		}()
		if !recursive {
			return Some(data)
		}
	}
}
