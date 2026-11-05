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
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type hygieneDecodeToken struct {
	name string
	raw  uint32
}
type hygieneDecodeState struct {
	mode      int
	reentered bool
}

func runHygieneDecode(n, mode int) string {
	var output strings.Builder
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	cache := sp.NewHygieneDecodeContext()
	defer cache.Close()
	state := hygieneDecodeState{mode: mode}
	token := func(name string, raw uint32) *rustvec.Owned[hygieneDecodeToken] {
		return rustvec.NewOwned(hygieneDecodeToken{name, raw}, rustvec.ValueOps[hygieneDecodeToken]{Drop: func(t *hygieneDecodeToken, ctx rustfmt.Context) {
			fmt.Fprintf(&output, "drop %s %d %t\n", t.name, t.raw, ctx.Panicking())
		}})
	}
	hash := func(raw uint32) sp.ExpnHash {
		bytes := make([]byte, 16)
		binary.LittleEndian.PutUint64(bytes, 111)
		binary.LittleEndian.PutUint64(bytes[8:], uint64(raw)+1000)
		d, err := serialize.NewMemDecoder(append(bytes, []byte(serialize.MagicEndBytes)...), 0)
		if err != nil {
			panic(err)
		}
		return sp.DecodeExpnHash(d)
	}
	var expansion func(uint32) sp.ExpnId
	expansion = func(raw uint32) sp.ExpnId {
		f := hashmap.NewOnce(token("expn", raw), func(_ *hygieneDecodeToken, id sp.ExpnId, _ rustfmt.Context) sp.DecodedExpnData {
			fmt.Fprintf(&output, "call expn %d false\n", raw)
			if state.mode&1 != 0 && raw == 2 {
				panic("expn")
			}
			data := sp.DefaultExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.BangMacro, Name: sp.InternSymbol(session, "decoded")}, sp.DummySpan(), sp.Edition2024, sp.None[sp.DefId](), sp.None[sp.ModId]())
			data.Parent = expansion(raw - 1)
			if id.Krate.AsU32() != 1 {
				panic("crate")
			}
			return sp.DecodedExpnData{Data: data, Hash: hash(raw)}
		})
		return sp.DecodeHygieneExpnIdOwned(session, sp.CrateNumFromU32(1), raw, f, rustfmt.Context{})
	}
	var syntax func(uint32) sp.SyntaxContext
	syntax = func(raw uint32) sp.SyntaxContext {
		e := serialize.NewMemEncoder()
		e.EmitU32(raw)
		d, err := serialize.NewMemDecoder(append(e.Finish(), []byte(serialize.MagicEndBytes)...), 0)
		if err != nil {
			panic(err)
		}
		f := hashmap.NewOnce(token("ctxt", raw), func(_ *hygieneDecodeToken, args sp.HygieneSyntaxDecodeArgs[*serialize.MemDecoder], _ rustfmt.Context) sp.SyntaxContextKey {
			fmt.Fprintf(&output, "call ctxt %d %t false\n", raw, args.Decoder == d)
			if state.mode&1 != 0 && raw == 2 {
				panic("ctxt")
			}
			if state.mode&4 != 0 && !state.reentered {
				state.reentered = true
				nested := syntax(raw)
				fmt.Fprintf(&output, "reentered %d %d\n", raw, nested.AsU32())
			}
			var parent sp.SyntaxContext
			if state.mode&2 != 0 && raw == 3 {
				parent = sp.SyntaxContextFromU32(999)
			} else {
				parent = syntax(raw - 1)
			}
			if args.Context.AsU32() != raw {
				panic("decoded id")
			}
			return sp.SyntaxContextKey{Parent: parent, Expansion: sp.ExpnId{Krate: sp.CrateNumFromU32(1), LocalID: sp.ExpnIndexFromU32(raw)}, Transparency: sp.Opaque}
		})
		value := sp.DecodeHygieneSyntaxContextOwned(session, d, cache, f, rustfmt.Context{})
		fmt.Fprintf(&output, "read %d %d\n", raw, d.Position())
		return value
	}
	for pass := 0; pass < 2; pass++ {
		previous := uint32(0)
		if n > 0 {
			previous = uint32(n - 1)
		}
		for _, raw := range []uint32{uint32(n), 0, uint32(n), previous} {
			failed := false
			var id sp.ExpnId
			func() {
				defer func() {
					if recover() != nil {
						failed = true
					}
				}()
				id = expansion(raw)
			}()
			fmt.Fprintf(&output, "expn result %d %d %t %d\n", pass, raw, failed, id.LocalID.AsU32())
			failed = false
			var c sp.SyntaxContext
			func() {
				defer func() {
					if recover() != nil {
						failed = true
					}
				}()
				c = syntax(raw)
			}()
			fmt.Fprintf(&output, "ctxt result %d %d %t %d\n", pass, raw, failed, c.AsU32())
		}
		state.mode = 0
	}
	fmt.Fprintf(&output, "--final--\n%s\n", sp.DebugHygieneData(session, false))
	return output.String()
}

func TestOriginalHygieneDecode(t *testing.T) {
	readHygieneOracle(t, "decode", func(row hygieneOracleRow) string { return runHygieneDecode(row.N, row.Mode) })
}
