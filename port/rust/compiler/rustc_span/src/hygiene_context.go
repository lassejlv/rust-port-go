// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (c SyntaxContext) ApplyMark(session *SessionGlobals, id ExpnId, t Transparency) SyntaxContext {
	return c.ApplyMarkWithContext(session, id, t, rustfmt.Context{})
}
func (c SyntaxContext) ApplyMarkWithContext(session *SessionGlobals, id ExpnId, t Transparency, ctx rustfmt.Context) SyntaxContext {
	guard := hygieneBorrow(session, ctx)
	defer guard.Close()
	return guard.Get().applyMark(session, c, id, t, ctx)
}
func (c *SyntaxContext) RemoveMark(session *SessionGlobals) ExpnId {
	return c.RemoveMarkWithContext(session, rustfmt.Context{})
}
func (c *SyntaxContext) RemoveMarkWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnId {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().removeMark(c, ctx).Expansion
}
func (c SyntaxContext) Marks(session *SessionGlobals) []HygieneMark {
	return c.MarksWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) MarksWithContext(session *SessionGlobals, ctx rustfmt.Context) []HygieneMark {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().marks(c, ctx)
}
func (c *SyntaxContext) Adjust(session *SessionGlobals, id ExpnId) Option[ExpnId] {
	return c.AdjustWithContext(session, id, rustfmt.Context{})
}
func (c *SyntaxContext) AdjustWithContext(session *SessionGlobals, id ExpnId, ctx rustfmt.Context) Option[ExpnId] {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().adjust(c, id, ctx)
}
func (c *SyntaxContext) NormalizeToMacros20AndAdjust(session *SessionGlobals, id ExpnId) Option[ExpnId] {
	return c.NormalizeToMacros20AndAdjustWithContext(session, id, rustfmt.Context{})
}
func (c *SyntaxContext) NormalizeToMacros20AndAdjustWithContext(session *SessionGlobals, id ExpnId, ctx rustfmt.Context) Option[ExpnId] {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	*c = d.normalizeMacros20(*c, ctx)
	return d.adjust(c, id, ctx)
}
func (c *SyntaxContext) GlobAdjust(session *SessionGlobals, id ExpnId, glob Span) Option[Option[ExpnId]] {
	return c.GlobAdjustWithContext(session, id, glob, rustfmt.Context{})
}
func (c *SyntaxContext) GlobAdjustWithContext(session *SessionGlobals, id ExpnId, glob Span, ctx rustfmt.Context) Option[Option[ExpnId]] {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	scope := None[ExpnId]()
	globCtxt := d.normalizeMacros20(glob.Ctxt(), ctx)
	for !d.isDescendantOf(id, d.outerExpn(globCtxt, ctx), ctx) {
		scope = Some(d.removeMark(&globCtxt, ctx).Expansion)
		if d.removeMark(c, ctx).Expansion != scope.value {
			return None[Option[ExpnId]]()
		}
	}
	if d.adjust(c, id, ctx).present {
		return None[Option[ExpnId]]()
	}
	return Some(scope)
}
func (c *SyntaxContext) ReverseGlobAdjust(session *SessionGlobals, id ExpnId, glob Span) Option[Option[ExpnId]] {
	return c.ReverseGlobAdjustWithContext(session, id, glob, rustfmt.Context{})
}
func (c *SyntaxContext) ReverseGlobAdjustWithContext(session *SessionGlobals, id ExpnId, glob Span, ctx rustfmt.Context) Option[Option[ExpnId]] {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	if d.adjust(c, id, ctx).present {
		return None[Option[ExpnId]]()
	}
	globCtxt := d.normalizeMacros20(glob.Ctxt(), ctx)
	marks := []HygieneMark{}
	for !d.isDescendantOf(id, d.outerExpn(globCtxt, ctx), ctx) {
		marks = append(marks, d.removeMark(&globCtxt, ctx))
	}
	scope := None[ExpnId]()
	if len(marks) > 0 {
		scope = Some(marks[len(marks)-1].Expansion)
	}
	for i := len(marks) - 1; i >= 0; i-- {
		mark := marks[i]
		*c = d.applyMark(session, *c, mark.Expansion, mark.Transparency, ctx)
	}
	return Some(scope)
}
func (c SyntaxContext) HygienicEqual(session *SessionGlobals, other SyntaxContext, id ExpnId) bool {
	return c.HygienicEqualWithContext(session, other, id, rustfmt.Context{})
}
func (c SyntaxContext) HygienicEqualWithContext(session *SessionGlobals, other SyntaxContext, id ExpnId, ctx rustfmt.Context) bool {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	normalized := d.normalizeMacros20(c, ctx)
	d.adjust(&normalized, id, ctx)
	return normalized == d.normalizeMacros20(other, ctx)
}
func (c SyntaxContext) NormalizeToMacros20(session *SessionGlobals) SyntaxContext {
	return c.NormalizeToMacros20WithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) NormalizeToMacros20WithContext(session *SessionGlobals, ctx rustfmt.Context) SyntaxContext {
	if c.IsRoot() {
		return c
	}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().normalizeMacros20(c, ctx)
}
func (c SyntaxContext) NormalizeToMacroRules(session *SessionGlobals) SyntaxContext {
	return c.NormalizeToMacroRulesWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) NormalizeToMacroRulesWithContext(session *SessionGlobals, ctx rustfmt.Context) SyntaxContext {
	if c.IsRoot() {
		return c
	}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().normalizeMacroRules(c, ctx)
}
func (c SyntaxContext) OuterExpn(session *SessionGlobals) ExpnId {
	return c.OuterExpnWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) OuterExpnWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnId {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().outerExpn(c, ctx)
}
func (c SyntaxContext) OuterExpnData(session *SessionGlobals) ExpnData {
	return c.OuterExpnDataWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) OuterExpnDataWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnData {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	return d.expnData(d.outerExpn(c, ctx), ctx).CloneWithContext(ctx)
}
func (c SyntaxContext) DollarCrateName(session *SessionGlobals) Symbol {
	return c.DollarCrateNameWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) DollarCrateNameWithContext(session *SessionGlobals, ctx rustfmt.Context) Symbol {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().contextData(c, ctx, hygienePoint(1060, 58)).dollarCrateName
}
func (c SyntaxContext) Edition(session *SessionGlobals) Edition {
	return c.EditionWithContext(session, rustfmt.Context{})
}
func (c SyntaxContext) EditionWithContext(session *SessionGlobals, ctx rustfmt.Context) Edition {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	return d.expnData(d.outerExpn(c, ctx), ctx).Edition
}

func UpdateDollarCrateNames(session *SessionGlobals, getName func(SyntaxContext) Symbol) {
	UpdateDollarCrateNamesWithContext(session, getName, rustfmt.Context{})
}
func UpdateDollarCrateNamesWithContext(session *SessionGlobals, getName func(SyntaxContext) Symbol, ctx rustfmt.Context) {
	UpdateDollarCrateNamesOwned(session, HygieneNameResolver[struct{}]{Call: func(_ *struct{}, c SyntaxContext, _ rustfmt.Context) Symbol { return getName(c) }, Context: ctx})
}

// HygieneNameResolver is a consumed FnMut environment, or a borrowed FnMut
// cell when Drop is nil. The same Env cell is borrowed on every invocation.
type HygieneNameResolver[E any] struct {
	Env     *E
	Call    func(*E, SyntaxContext, rustfmt.Context) Symbol
	Drop    func(*E, rustfmt.Context)
	Context rustfmt.Context
}

func (r HygieneNameResolver[E]) Borrow() HygieneNameResolver[E] { r.Drop = nil; return r }
func hygieneCleanup(ctx rustfmt.Context, cleanup func()) {
	defer func() {
		if recover() != nil {
			ctx.AbortCleanup()
		}
	}()
	cleanup()
}
func hygieneDropOnReturn(drop func(rustfmt.Context), ctx rustfmt.Context) {
	p := recover()
	if p != nil {
		ctx.Unwinding = true
		hygieneCleanup(ctx, func() { drop(ctx) })
		ctx.ResumeUnwind(p)
	}
	drop(ctx)
}
func UpdateDollarCrateNamesOwned[E any](session *SessionGlobals, resolver HygieneNameResolver[E]) {
	ctx := resolver.Context
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if resolver.Drop != nil {
			resolver.Drop(resolver.Env, c)
		}
	}, ctx)
	type update struct {
		ctxt SyntaxContext
		name Symbol
	}
	toUpdate := []update{}
	func() {
		g := hygieneBorrow(session, ctx)
		defer g.Close()
		d := g.Get()
		for i := d.syntaxContextData.Len(); i > 0; i-- {
			ctxt := SyntaxContextFromUsizeWithContext(i-1, ctx)
			if d.contextData(ctxt, ctx).dollarCrateName != KwDollarCrate() {
				break
			}
			toUpdate = append(toUpdate, update{ctxt, KwDollarCrate()})
		}
	}()
	for i := range toUpdate {
		toUpdate[i].name = resolver.Call(resolver.Env, toUpdate[i].ctxt, ctx)
	}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	for _, update := range toUpdate {
		g.Get().contextData(update.ctxt, ctx).dollarCrateName = update.name
	}
}
