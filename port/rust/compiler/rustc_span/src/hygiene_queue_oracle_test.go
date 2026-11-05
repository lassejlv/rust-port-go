// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type hygieneQueueState struct {
	ctxtCalls, expnCalls int
	failCtxt, failExpn   bool
}

func runHygieneQueue(n, mode int) string {
	var output strings.Builder
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	ids := []sp.ExpnId{sp.RootExpnId()}
	contexts := []sp.SyntaxContext{sp.RootSyntaxContext()}
	for i := 0; i < n; i++ {
		call := sp.NewSpan(session, sp.BytePos(i+1), sp.BytePos(i+2), contexts[i], sp.None[sp.LocalDefId]())
		data := sp.DefaultExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(session, "queue")}, call, sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId]())
		data.Parent = ids[i]
		if i%2 == 0 {
			data.AllowInternalUnstable = sp.Some(sp.NewShared([]sp.Symbol{sp.InternSymbol(session, "feature")}, nil))
		}
		id := sp.FreshLocalExpnId(session, data, hygieneCoreHcx{}).ToExpnId()
		ids = append(ids, id)
		contexts = append(contexts, contexts[i].ApplyMark(session, id, sp.Opaque))
	}
	hashBytes := make([]byte, 16)
	binary.LittleEndian.PutUint64(hashBytes, 111)
	binary.LittleEndian.PutUint64(hashBytes[8:], 777)
	hashDecoder, err := serialize.NewMemDecoder(append(hashBytes, []byte(serialize.MagicEndBytes)...), 0)
	if err != nil {
		panic(err)
	}
	foreignHash := sp.DecodeExpnHash(hashDecoder)
	foreign := sp.RegisterExpnId(session, sp.CrateNumFromU32(1), sp.ExpnIndexFromU32(2), sp.DefaultExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.DeriveMacro, Name: sp.InternSymbol(session, "foreign")}, sp.DummySpan(), sp.Edition2021, sp.None[sp.DefId](), sp.None[sp.ModId]()), foreignHash)
	cell := sp.NewHygieneEncodeCell()
	defer cell.Close()
	g := cell.BorrowMut()
	g.Get().EnqueueSyntaxContextForEncoding(contexts[n])
	g.Get().EnqueueSyntaxContextForEncoding(contexts[n])
	g.Get().ScheduleExpnDataForEncoding(ids[n])
	g.Get().ScheduleExpnDataForEncoding(foreign)
	g.Close()
	state := hygieneQueueState{failCtxt: mode&1 != 0, failExpn: mode&2 != 0}
	round := func(borrow bool) {
		ctxt := sp.HygieneContextEncoder[struct{}, hygieneQueueState]{
			Call: func(_ *struct{}, s *hygieneQueueState, c sp.SyntaxContext, key *sp.SyntaxContextKey, _ rustfmt.Context) {
				s.ctxtCalls++
				fmt.Fprintf(&output, "ctxt %d %d %d %d %s\n", s.ctxtCalls, c.AsU32(), key.Parent.AsU32(), key.Expansion.LocalID.AsU32(), key.Transparency.RustDebug())
				g := cell.BorrowMut()
				g.Get().EnqueueSyntaxContextForEncoding(key.Parent)
				g.Get().ScheduleExpnDataForEncoding(key.Expansion)
				g.Close()
				if s.failCtxt && s.ctxtCalls == 2 {
					panic("ctxt")
				}
			},
			Drop: func(_ *struct{}, ctx rustfmt.Context) { fmt.Fprintf(&output, "drop ctxt %t\n", ctx.Panicking()) },
		}
		expn := sp.HygieneExpansionEncoder[struct{}, hygieneQueueState]{
			Call: func(_ *struct{}, s *hygieneQueueState, id sp.ExpnId, value sp.Option[*sp.ExpnData], hash sp.ExpnHash, _ rustfmt.Context) {
				s.expnCalls++
				refs := uint(0)
				data, ok := value.Get()
				if ok {
					if features, exists := data.AllowInternalUnstable.Get(); exists {
						refs = features.StrongCount()
					}
				}
				fmt.Fprintf(&output, "expn %d %d %d %t %d %d\n", s.expnCalls, id.Krate.AsU32(), id.LocalID.AsU32(), ok, hash.LocalHash().AsU64(), refs)
				if ok {
					g := cell.BorrowMut()
					g.Get().ScheduleExpnDataForEncoding(data.Parent)
					g.Get().EnqueueSyntaxContextForEncoding(data.CallSite.Ctxt())
					g.Close()
				}
				if s.failExpn && s.expnCalls == 1 {
					panic("expn")
				}
			},
			Drop: func(_ *struct{}, ctx rustfmt.Context) { fmt.Fprintf(&output, "drop expn %t\n", ctx.Panicking()) },
		}
		failed := false
		func() {
			defer func() {
				if recover() != nil {
					failed = true
				}
			}()
			if borrow {
				sp.EncodeHygieneOwned(session, cell, &state, ctxt.Borrow(), expn.Borrow(), rustfmt.Context{})
			} else {
				sp.EncodeHygieneOwned(session, cell, &state, ctxt, expn, rustfmt.Context{})
			}
		}()
		fmt.Fprintf(&output, "caught %t %d %d\n", failed, state.ctxtCalls, state.expnCalls)
		if borrow {
			expn.Drop(nil, rustfmt.Context{})
			ctxt.Drop(nil, rustfmt.Context{})
		}
	}
	round(mode&4 != 0)
	state.failCtxt, state.failExpn = false, false
	round(mode&4 != 0)
	g = cell.BorrowMut()
	for _, c := range contexts {
		g.Get().EnqueueSyntaxContextForEncoding(c)
	}
	for _, id := range ids {
		g.Get().ScheduleExpnDataForEncoding(id)
	}
	g.Close()
	round(false)
	fmt.Fprintf(&output, "--final--\n%s\n", sp.DebugHygieneData(session, false))
	return output.String()
}

func TestOriginalHygieneQueue(t *testing.T) {
	readHygieneOracle(t, "queue", func(row hygieneOracleRow) string { return runHygieneQueue(row.N, row.Mode) })
}
