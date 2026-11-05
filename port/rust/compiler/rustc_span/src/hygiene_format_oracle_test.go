// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func hygieneFormatDesugar(i int) sp.DesugaringKind {
	tags := []sp.DesugaringTag{sp.DesugarAsync, sp.DesugarAwait, sp.DesugarQuestionMark, sp.DesugarTryBlock, sp.DesugarYeetExpr, sp.DesugarOpaqueTy, sp.DesugarForLoop, sp.DesugarWhileLoop, sp.DesugarBoundModifier, sp.DesugarContract, sp.DesugarPatTyRange, sp.DesugarFormatLiteral, sp.DesugarFormatLiteral, sp.DesugarRangeExpr}
	return sp.DesugaringKind{Tag: tags[i], Source: i == 12}
}

func hygieneFormatValues(session *sp.SessionGlobals) ([]func(*rustfmt.Formatter) error, func()) {
	out := []func(*rustfmt.Formatter) error{}
	for _, v := range []uint32{0, 17, 0x1fffffff} {
		out = append(out, sp.SyntaxContextFromU32(v).WriteDebugFormatter)
	}
	for _, v := range []uint32{0, 17, 0xffffff00} {
		out = append(out, sp.ExpnIndexFromU32(v).WriteDebugFormatter)
	}
	for _, v := range []uint32{0, 17, 0xffffff00} {
		out = append(out, sp.LocalExpnIdFromU32(v).WriteDebugFormatter)
	}
	for _, v := range []uint32{0, 17, 0x1fffffff} {
		out = append(out, sp.DecodedSyntaxContextFromU32(v).WriteDebugFormatter)
	}
	for _, v := range [][2]uint32{{0, 0}, {0, 17}, {3, 99}} {
		out = append(out, (sp.ExpnId{Krate: sp.CrateNumFromU32(v[0]), LocalID: sp.ExpnIndexFromU32(v[1])}).WriteDebugFormatter)
	}
	for _, v := range [][2]uint64{{0, 0}, {111, 777}, {^uint64(0), 1 << 63}} {
		bytes := make([]byte, 16)
		binary.LittleEndian.PutUint64(bytes, v[0])
		binary.LittleEndian.PutUint64(bytes[8:], v[1])
		d, err := serialize.NewMemDecoder(append(bytes, []byte(serialize.MagicEndBytes)...), 0)
		if err != nil {
			panic(err)
		}
		out = append(out, sp.DecodeExpnHash(d).WriteDebugFormatter)
	}
	for _, v := range []sp.Transparency{sp.Transparent, sp.SemiOpaque, sp.Opaque} {
		out = append(out, v.WriteDebugFormatter)
	}
	for _, v := range []sp.MacroKind{sp.BangMacro, sp.AttrMacro, sp.DeriveMacro} {
		out = append(out, v.WriteDebugFormatter)
	}
	for _, v := range []sp.AstPass{sp.StdImports, sp.TestHarness, sp.ProcMacroHarness} {
		out = append(out, v.WriteDebugFormatter)
	}
	for i := 0; i < 14; i++ {
		out = append(out, hygieneFormatDesugar(i).WriteDebugFormatter)
	}
	addKind := func(k sp.ExpnKind) {
		out = append(out, func(f *rustfmt.Formatter) error { return k.WriteDebugFormatter(session, f) })
	}
	addKind(sp.ExpnKind{Tag: sp.ExpnRoot})
	name := sp.InternSymbol(session, "macro\n\x00é🦀\u0301")
	for _, v := range []sp.MacroKind{sp.BangMacro, sp.AttrMacro, sp.DeriveMacro} {
		addKind(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: v, Name: name})
	}
	for _, v := range []sp.AstPass{sp.StdImports, sp.TestHarness, sp.ProcMacroHarness} {
		addKind(sp.ExpnKind{Tag: sp.ExpnAstPass, AstPass: v})
	}
	for i := 0; i < 14; i++ {
		addKind(sp.ExpnKind{Tag: sp.ExpnDesugaring, Desugaring: hygieneFormatDesugar(i)})
	}
	datas := []*sp.ExpnData{}
	for i := 0; i < 8; i++ {
		call := sp.NewSpan(session, sp.BytePos(i+10), sp.BytePos(i+20), sp.RootSyntaxContext(), sp.None[sp.LocalDefId]())
		def := sp.DummySpan()
		if i%2 != 0 {
			def = sp.NewSpan(session, 30, 40, sp.RootSyntaxContext(), sp.None[sp.LocalDefId]())
		}
		features := sp.None[sp.Shared[[]sp.Symbol]]()
		if i%3 != 0 {
			values := []sp.Symbol{}
			if i%3 == 2 {
				values = []sp.Symbol{name, sp.InternSymbol(session, "featureβ"), sp.InternSymbol(session, "feature🦀")}
			}
			features = sp.Some(sp.NewShared(values, nil))
		}
		id := sp.DefId{Krate: sp.CrateNumFromU32(2), Index: sp.DefIndexFromU32(99)}
		macro := sp.None[sp.DefId]()
		parent := sp.None[sp.ModId]()
		if i%2 == 0 {
			parent = sp.Some(sp.NewModIdUnchecked(id))
		} else {
			macro = sp.Some(id)
		}
		data := sp.NewExpnData(sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.AttrMacro, Name: name}, sp.RootExpnId(), call, def, features, sp.Edition(i%5), macro, parent, i&1 != 0, i&2 != 0, i&4 != 0, i&3 == 0)
		datas = append(datas, &data)
		out = append(out, func(f *rustfmt.Formatter) error { return data.WriteDebugFormatter(session, f) })
	}
	for _, v := range []sp.Transparency{sp.Transparent, sp.SemiOpaque, sp.Opaque} {
		out = append(out, (sp.SyntaxContextKey{Parent: sp.SyntaxContextFromU32(17), Expansion: sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(99)}, Transparency: v}).WriteDebugFormatter)
	}
	return out, func() {
		for _, d := range datas {
			d.Close()
		}
	}
}

type hygieneFormatSink struct {
	calls []string
	text  strings.Builder
	fail  int
	chars bool
}

func (s *hygieneFormatSink) add(kind, text string) (int, error) {
	n := len(s.calls)
	s.calls = append(s.calls, kind+":"+hex.EncodeToString([]byte(text)))
	if n == s.fail {
		return 0, errors.New("writer")
	}
	return s.text.WriteString(text)
}
func (s *hygieneFormatSink) Write(b []byte) (int, error)       { return s.WriteString(string(b)) }
func (s *hygieneFormatSink) WriteString(v string) (int, error) { return s.add("S", v) }
func (s *hygieneFormatSink) WriteChar(v rune) error {
	if s.chars {
		_, e := s.add("C", string(v))
		return e
	}
	_, e := s.WriteString(string(v))
	return e
}

func hygieneFormatOptions(style int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch style {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.Alternate = true
		o.DebugLowerHex = true
	case 5:
		o.SignPlus = true
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
	case 6:
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
		o.DebugLowerHex = true
	case 7:
		o.Fill = '*'
		o.Align = rustfmt.AlignRight
		o.HasWidth = true
		o.Width = 12
	case 8:
		o.Fill = '*'
		o.Align = rustfmt.AlignLeft
		o.HasWidth = true
		o.Width = 12
	case 9:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		o.HasWidth = true
		o.Width = 13
	case 10:
		o.HasPrecision = true
		o.Precision = 2
	case 11:
		o.SignPlus = true
		o.Alternate = true
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
		o.DebugLowerHex = true
	case 12:
		o.SignMinus = true
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
	case 13:
		o.Fill = '🦀'
		o.Align = rustfmt.AlignCenter
		o.HasWidth = true
		o.Width = 12
	case 14:
		o.Fill = '\n'
		o.Align = rustfmt.AlignRight
		o.HasWidth = true
		o.Width = 8
	case 15:
		o.Fill = 0
		o.Align = rustfmt.AlignRight
		o.HasWidth = true
		o.Width = 8
	default:
		panic("style")
	}
	return o
}

func TestOriginalHygieneFormatter(t *testing.T) {
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	values, close := hygieneFormatValues(session)
	defer close()
	readHygieneOracle(t, "format", func(row hygieneOracleRow) string {
		mode := row.Mode
		sink := &hygieneFormatSink{fail: mode/32 - 1, chars: mode&16 != 0}
		f := &rustfmt.Formatter{Options: hygieneFormatOptions(mode % 16), Writer: sink}
		first := values[row.N](f)
		second := rustfmt.FormatDebugUint(7, 16, f)
		return fmt.Sprintf("result %t %t %t %d %d\ntext %s\ncalls %s\n", first != nil || second != nil, first != nil, second != nil, f.Options.Fill, f.Options.Align, hex.EncodeToString([]byte(sink.text.String())), strings.Join(sink.calls, ","))
	})
}
