// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func (span Span) MarkWithReason(session *SessionGlobals, unstable Option[Shared[[]Symbol]], reason DesugaringKind, edition Edition, hcx datastructures.StableHashCtxt) Span {
	return span.MarkWithReasonWithContext(session, unstable, reason, edition, hcx, rustfmt.Context{})
}
func (span Span) MarkWithReasonWithContext(session *SessionGlobals, unstable Option[Shared[[]Symbol]], reason DesugaringKind, edition Edition, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) Span {
	data := DefaultExpnData(ExpnKind{Tag: ExpnDesugaring, Desugaring: reason}, span, edition, None[DefId](), None[ModId]())
	data.AllowInternalUnstable = unstable
	id := FreshLocalExpnIdWithContext(session, data, hcx, ctx)
	return span.ApplyMarkWithContext(session, id.ToExpnId(), Transparent, ctx)
}
func MarkSpanWithReasonOwned[S any](span Span, session *SessionGlobals, unstable Option[Shared[[]Symbol]], reason DesugaringKind, edition Edition, hcx *rustvec.Owned[S], ops datastructures.StableHashCtxtOps[S], ctx rustfmt.Context) Span {
	data := DefaultExpnData(ExpnKind{Tag: ExpnDesugaring, Desugaring: reason}, span, edition, None[DefId](), None[ModId]())
	data.AllowInternalUnstable = unstable
	id := FreshLocalExpnIdOwned(session, data, hcx, ops, ctx)
	return span.ApplyMarkWithContext(session, id.ToExpnId(), Transparent, ctx)
}
