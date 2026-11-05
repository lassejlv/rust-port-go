// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	hashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"strings"
	"testing"
)

type hygieneCoreHcx struct{}

func (hygieneCoreHcx) StableHashSpan(s ds.RawSpan, h *ds.StableHasher) { h.WriteU64(s.Value) }
func (hygieneCoreHcx) DefPathHash(id ds.RawDefId) ds.Fingerprint {
	return ds.NewFingerprint(hashes.NewHash64(uint64(id.Crate)+100), hashes.NewHash64(uint64(id.Index)+1000))
}
func (hygieneCoreHcx) StableHashControls() ds.StableHashControls {
	return ds.StableHashControls{HashSpans: true}
}
func (hygieneCoreHcx) AssertDefaultStableHashControls(string) {}
func runHygieneCore(n, mode int) string {
	var output strings.Builder
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	ctxt := sp.RootSyntaxContext()
	parent := sp.RootExpnId()
	for i := 0; i < n; i++ {
		callCtxt := sp.RootSyntaxContext()
		if mode&1 != 0 {
			callCtxt = ctxt
		}
		call := sp.NewSpan(session, sp.BytePos(i*2+1), sp.BytePos(i*2+2), callCtxt, sp.None[sp.LocalDefId]())
		name := "macro🦀"
		if i%2 != 0 {
			name = "macroé"
		}
		kind := sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.MacroKind((i + mode) % 3), Name: sp.InternSymbol(session, name)}
		data := sp.DefaultExpnData(kind, call, sp.Edition(i%5), sp.None[sp.DefId](), sp.None[sp.ModId]())
		data.Parent = parent
		data.LocalInnerMacros = i%2 == 0
		data.DiagnosticOpaque = i%3 == 0
		if i%3 == 0 {
			data.AllowInternalUnstable = sp.Some(sp.NewShared([]sp.Symbol{sp.InternSymbol(session, "feature")}, nil))
		}
		var id sp.LocalExpnId
		if mode&2 == 0 {
			id = sp.FreshLocalExpnId(session, data, hygieneCoreHcx{})
		} else {
			id = sp.FreshEmptyLocalExpnId(session)
			id.SetExpnData(session, data, hygieneCoreHcx{})
		}
		hash := id.ToExpnId().ExpnHash(session)
		fmt.Fprintf(&output, "expn %d %d %d\n", id.AsU32(), hash.StableCrateId().AsU64(), hash.LocalHash().AsU64())
		ctxt = ctxt.ApplyMark(session, id.ToExpnId(), sp.Transparency((i+mode)%3))
		marks := []string{}
		for _, m := range ctxt.Marks(session) {
			marks = append(marks, fmt.Sprintf("(%s, %s)", m.Expansion.RustDebug(), m.Transparency.RustDebug()))
		}
		fmt.Fprintf(&output, "ctxt %d %d %d %s [%s]\n", ctxt.AsU32(), ctxt.NormalizeToMacros20(session).AsU32(), ctxt.NormalizeToMacroRules(session).AsU32(), ctxt.Edition(session).DebugString(), strings.Join(marks, ", "))
		if i%2 == 0 {
			sp.UpdateDollarCrateNames(session, func(c sp.SyntaxContext) sp.Symbol {
				_ = c.Edition(session)
				return sp.InternSymbol(session, fmt.Sprintf("crate%d", c.AsU32()))
			})
		}
		parent = id.ToExpnId()
	}
	fmt.Fprintf(&output, "--compact--\n%s\n", sp.DebugHygieneData(session, false))
	fmt.Fprintf(&output, "--pretty--\n%s\n", sp.DebugHygieneData(session, true))
	return output.String()
}

func TestOriginalHygieneCoreStates(t *testing.T) {
	readHygieneOracle(t, "core", func(row hygieneOracleRow) string { return runHygieneCore(row.N, row.Mode) })
}
