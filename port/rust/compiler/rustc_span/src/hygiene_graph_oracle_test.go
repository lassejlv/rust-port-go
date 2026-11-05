// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"strings"
	"testing"
)

func hygieneGraphScope(v sp.Option[sp.ExpnId]) uint32 {
	if value, ok := v.Get(); ok {
		return value.LocalID.AsU32()
	}
	return ^uint32(0)
}
func hygieneGraphFlatten(v sp.Option[sp.Option[sp.ExpnId]]) sp.Option[sp.ExpnId] {
	if value, ok := v.Get(); ok {
		return value
	}
	return sp.None[sp.ExpnId]()
}
func runHygieneGraph(n, mode int) string {
	var output strings.Builder
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	sm := sp.NewSourceMap(sp.EmptyFilePathMapping())
	defer sm.Close()
	ids := []sp.ExpnId{sp.RootExpnId()}
	contexts := []sp.SyntaxContext{sp.RootSyntaxContext()}
	current := sp.RootSyntaxContext()
	for i := 0; i < n; i++ {
		var kind sp.ExpnKind
		switch (i + mode) % 5 {
		case 0:
			kind = sp.ExpnKind{Tag: sp.ExpnDesugaring, Desugaring: sp.DesugaringKind{Tag: sp.DesugarFormatLiteral, Source: mode%2 == 0}}
		case 1:
			kind = sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.SymInclude()}
		case 2:
			kind = sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.AttrMacro, Name: sp.InternSymbol(session, "attr")}
		case 3:
			kind = sp.ExpnKind{Tag: sp.ExpnAstPass, AstPass: sp.TestHarness}
		default:
			tag := sp.DesugarForLoop
			if i%2 != 0 {
				tag = sp.DesugarQuestionMark
			}
			kind = sp.ExpnKind{Tag: sp.ExpnDesugaring, Desugaring: sp.DesugaringKind{Tag: tag}}
		}
		parent := ids[i]
		if mode&2 != 0 {
			parent = ids[0]
		}
		callCtxt := sp.RootSyntaxContext()
		if mode&1 != 0 {
			callCtxt = current
		}
		call := sp.NewSpan(session, sp.BytePos(i+10), sp.BytePos(i+11), callCtxt, sp.None[sp.LocalDefId]())
		features := sp.None[sp.Shared[[]sp.Symbol]]()
		if i%2 == 0 {
			features = sp.Some(sp.NewShared([]sp.Symbol{sp.InternSymbol(session, "feature")}, nil))
		}
		data := sp.NewExpnData(kind, parent, call, sp.DummySpan(), features, sp.Edition2024, sp.Some(sp.CrateDefId().ToDefId()), sp.Some(sp.CrateModId().ToModId()), i%2 == 0, i%3 == 0, i%2 == 0, i%3 == 0)
		id := sp.FreshLocalExpnId(session, data, hygieneCoreHcx{}).ToExpnId()
		ids = append(ids, id)
		current = current.ApplyMark(session, id, sp.Transparency((i+mode)%3))
		contexts = append(contexts, current)
	}
	for i, id := range ids {
		data := id.ExpnData(session)
		refs := uint(0)
		if value, ok := data.AllowInternalUnstable.Get(); ok {
			refs = value.StrongCount()
		}
		fmt.Fprintf(&output, "data %d %t %s %d\n", i, data.IsRoot(), data.Kind.Descr(session), refs)
		data.Close()
		cause := id.ExpansionCause(session)
		lo := uint32(0)
		if v, ok := cause.Get(); ok {
			lo = v.Lo(session).AsU32()
		}
		fmt.Fprintf(&output, "cause %d %t %d\n", i, cause.IsSome(), lo)
		for j, other := range ids {
			fmt.Fprintf(&output, "desc %d %d %t\n", i, j, id.IsDescendantOf(session, other))
		}
		for j, c := range contexts {
			fmt.Fprintf(&output, "outerdesc %d %d %t\n", i, j, id.OuterExpnIsDescendantOf(session, c))
		}
	}
	for i, c := range contexts {
		removed := c
		outer := removed.RemoveMark(session)
		fmt.Fprintf(&output, "remove %d %d %d\n", i, removed.AsU32(), outer.LocalID.AsU32())
		fmt.Fprintf(&output, "external %d %t\n", i, c.InExternalMacro(session, sm))
		for j, id := range ids {
			adjusted := c
			result := adjusted.Adjust(session, id)
			fmt.Fprintf(&output, "adjust %d %d %d %d\n", i, j, adjusted.AsU32(), hygieneGraphScope(result))
			normalized := c
			result = normalized.NormalizeToMacros20AndAdjust(session, id)
			fmt.Fprintf(&output, "normaladjust %d %d %d %d\n", i, j, normalized.AsU32(), hygieneGraphScope(result))
			for k, glob := range contexts {
				span := sp.DummySpan().WithCtxt(session, glob)
				value := c
				r := value.GlobAdjust(session, id, span)
				fmt.Fprintf(&output, "glob %d %d %d %d %t %d\n", i, j, k, value.AsU32(), r.IsSome(), hygieneGraphScope(hygieneGraphFlatten(r)))
				value = c
				r = value.ReverseGlobAdjust(session, id, span)
				fmt.Fprintf(&output, "reverse %d %d %d %d %t %d\n", i, j, k, value.AsU32(), r.IsSome(), hygieneGraphScope(hygieneGraphFlatten(r)))
				fmt.Fprintf(&output, "eq %d %d %d %t\n", i, j, k, c.HygienicEqual(session, glob, id))
			}
		}
		span := sp.NewSpan(session, 99, 100, c, sp.None[sp.LocalDefId]())
		for j, to := range contexts {
			walked := sp.WalkChain(session, span, to)
			collapsed := sp.WalkChainCollapsed(session, span, sp.DummySpan().WithCtxt(session, to))
			fmt.Fprintf(&output, "walk %d %d %d %d %d %d\n", i, j, walked.Lo(session).AsU32(), walked.Ctxt().AsU32(), collapsed.Lo(session).AsU32(), collapsed.Ctxt().AsU32())
		}
	}
	fmt.Fprintf(&output, "--final--\n%s\n", sp.DebugHygieneData(session, false))
	return output.String()
}

func TestOriginalHygieneGraph(t *testing.T) {
	readHygieneOracle(t, "graph", func(row hygieneOracleRow) string { return runHygieneGraph(row.N, row.Mode) })
}
