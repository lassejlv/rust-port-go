// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

type hygieneCodecAtom struct {
	kind  string
	value any
}
type hygieneCodec struct {
	session        *sp.SessionGlobals
	atoms          []hygieneCodecAtom
	events         []string
	position, fail int
}

func (c *hygieneCodec) put(kind string, value any) {
	var text string
	switch v := value.(type) {
	case []byte:
		text = hex.EncodeToString(v)
	case sp.Span:
		text = fmt.Sprintf("%d/%d/%d", v.Lo(c.session), v.Hi(c.session), v.Ctxt().AsU32())
	case sp.Symbol:
		text = fmt.Sprint(v.AsU32())
	case sp.ExpnId:
		text = fmt.Sprintf("%d/%d", v.Krate.AsU32(), v.LocalID.AsU32())
	case sp.SyntaxContext:
		text = fmt.Sprint(v.AsU32())
	case sp.DefId:
		text = fmt.Sprintf("%d/%d", v.Krate.AsU32(), v.Index.AsU32())
	default:
		text = fmt.Sprint(value)
	}
	n := len(c.events)
	c.events = append(c.events, kind+":"+text)
	if n == c.fail {
		panic("encoder")
	}
	c.atoms = append(c.atoms, hygieneCodecAtom{kind, value})
}
func (c *hygieneCodec) take(kind string) any {
	n := len(c.events)
	c.events = append(c.events, kind)
	if n == c.fail {
		panic("decoder")
	}
	a := c.atoms[c.position]
	c.position++
	return a.value
}
func (c *hygieneCodec) EmitUsize(v uint)                       { c.put("usize", v) }
func (c *hygieneCodec) EmitU32(v uint32)                       { c.put("u32", v) }
func (c *hygieneCodec) EmitU8(v uint8)                         { c.put("u8", v) }
func (c *hygieneCodec) EmitRawBytes(v []byte)                  { c.put("raw", append([]byte{}, v...)) }
func (c *hygieneCodec) EmitU128(serialize.Uint128)             { panic("unused emit_u128") }
func (c *hygieneCodec) EmitU64(uint64)                         { panic("unused emit_u64") }
func (c *hygieneCodec) EmitU16(uint16)                         { panic("unused emit_u16") }
func (c *hygieneCodec) EmitIsize(int)                          { panic("unused emit_isize") }
func (c *hygieneCodec) EmitI128(serialize.Int128)              { panic("unused emit_i128") }
func (c *hygieneCodec) EmitI64(int64)                          { panic("unused emit_i64") }
func (c *hygieneCodec) EmitI32(int32)                          { panic("unused emit_i32") }
func (c *hygieneCodec) EmitI16(int16)                          { panic("unused emit_i16") }
func (c *hygieneCodec) EncodeSpan(v sp.Span)                   { c.put("span", v) }
func (c *hygieneCodec) EncodeSymbol(v sp.Symbol)               { c.put("symbol", v) }
func (c *hygieneCodec) EncodeExpnId(v sp.ExpnId)               { c.put("expn", v) }
func (c *hygieneCodec) EncodeSyntaxContext(v sp.SyntaxContext) { c.put("ctxt", v) }
func (c *hygieneCodec) EncodeDefId(v sp.DefId)                 { c.put("def", v) }
func (c *hygieneCodec) EncodeByteSymbol(sp.ByteSymbol)         { panic("unused byte symbol") }
func (c *hygieneCodec) EncodeCrateNum(sp.CrateNum)             { panic("unused crate") }
func (c *hygieneCodec) EncodeDefIndex(sp.DefIndex)             { panic("unused def index") }
func (c *hygieneCodec) ReadUsize() uint                        { return c.take("usize").(uint) }
func (c *hygieneCodec) ReadU32() uint32                        { return c.take("u32").(uint32) }
func (c *hygieneCodec) ReadU8() uint8                          { return c.take("u8").(uint8) }
func (c *hygieneCodec) ReadRawBytes(n uint) []byte {
	v := c.take(fmt.Sprintf("raw:%d", n)).([]byte)
	if uint(len(v)) != n {
		panic("raw len")
	}
	return v
}
func (c *hygieneCodec) Position() uint              { return uint(c.position) }
func (c *hygieneCodec) PeekByte() uint8             { return c.atoms[c.position].value.(uint8) }
func (c *hygieneCodec) ReadU128() serialize.Uint128 { panic("unused read_u128") }
func (c *hygieneCodec) ReadU64() uint64             { panic("unused read_u64") }
func (c *hygieneCodec) ReadU16() uint16             { panic("unused read_u16") }
func (c *hygieneCodec) ReadIsize() int              { panic("unused read_isize") }
func (c *hygieneCodec) ReadI128() serialize.Int128  { panic("unused read_i128") }
func (c *hygieneCodec) ReadI64() int64              { panic("unused read_i64") }
func (c *hygieneCodec) ReadI32() int32              { panic("unused read_i32") }
func (c *hygieneCodec) ReadI16() int16              { panic("unused read_i16") }
func (c *hygieneCodec) DecodeSpan() sp.Span         { return c.take("span").(sp.Span) }
func (c *hygieneCodec) DecodeSymbol() sp.Symbol     { return c.take("symbol").(sp.Symbol) }
func (c *hygieneCodec) DecodeExpnId() sp.ExpnId     { return c.take("expn").(sp.ExpnId) }
func (c *hygieneCodec) DecodeSyntaxContext() sp.SyntaxContext {
	return c.take("ctxt").(sp.SyntaxContext)
}
func (c *hygieneCodec) DecodeDefId() sp.DefId           { return c.take("def").(sp.DefId) }
func (c *hygieneCodec) DecodeByteSymbol() sp.ByteSymbol { panic("unused byte symbol") }
func (c *hygieneCodec) DecodeDefIndex() sp.DefIndex     { panic("unused index") }
func (c *hygieneCodec) DecodeLocalDefId() sp.LocalDefId { panic("unused local") }
func (c *hygieneCodec) DecodeCrateNum() sp.CrateNum     { panic("unused crate") }
func (c *hygieneCodec) DecodeAttrId() sp.AttrId         { panic("unused attr") }

type hygieneBoolCodec struct{ *hygieneCodec }

func (c hygieneBoolCodec) EmitBool(v bool) { c.put("bool", v) }
func (c hygieneBoolCodec) ReadBool() bool  { return c.take("bool").(bool) }

func hygieneCodecKind(session *sp.SessionGlobals, i int) sp.ExpnKind {
	switch {
	case i == 0:
		return sp.ExpnKind{Tag: sp.ExpnRoot}
	case i <= 3:
		return sp.ExpnKind{Tag: sp.ExpnMacro, MacroKind: sp.MacroKind(i - 1), Name: sp.InternSymbol(session, "codec🦀")}
	case i <= 6:
		return sp.ExpnKind{Tag: sp.ExpnAstPass, AstPass: sp.AstPass(i - 4)}
	default:
		return sp.ExpnKind{Tag: sp.ExpnDesugaring, Desugaring: hygieneFormatDesugar(i - 7)}
	}
}
func hygieneCodecHash(a, b uint64) sp.ExpnHash {
	bytes := make([]byte, 16)
	binary.LittleEndian.PutUint64(bytes, a)
	binary.LittleEndian.PutUint64(bytes[8:], b)
	d, e := serialize.NewMemDecoder(append(bytes, []byte(serialize.MagicEndBytes)...), 0)
	if e != nil {
		panic(e)
	}
	return sp.DecodeExpnHash(d)
}
func encodeHygieneCodec(session *sp.SessionGlobals, n int, e sp.SpanEncoder) {
	switch {
	case n <= 2:
		sp.Transparency(n).Encode(e)
	case n <= 5:
		sp.MacroKind(n - 3).Encode(e)
	case n <= 8:
		sp.AstPass(n - 6).Encode(e)
	case n <= 22:
		hygieneFormatDesugar(n - 9).Encode(e)
	case n <= 43:
		hygieneCodecKind(session, n-23).Encode(e)
	case n <= 46:
		p := [][2]uint64{{0, 0}, {111, 777}, {^uint64(0), 1 << 63}}[n-44]
		hygieneCodecHash(p[0], p[1]).Encode(e)
	case n <= 49:
		sp.DecodedSyntaxContextFromU32([]uint32{0, 17, 0x1fffffff}[n-47]).Encode(e)
	case n <= 52:
		sp.LocalExpnIdFromU32([]uint32{0, 17, 0xffffff00}[n-50]).Encode(e)
	case n <= 60:
		i := n - 53
		id := sp.DefId{Krate: sp.CrateNumFromU32(2), Index: sp.DefIndexFromU32(99)}
		sym := sp.InternSymbol(session, "codec🦀")
		features := sp.None[sp.Shared[[]sp.Symbol]]()
		if i%3 != 0 {
			symbols := []sp.Symbol{}
			if i%3 == 2 {
				symbols = []sp.Symbol{sym, sp.InternSymbol(session, "feature")}
			}
			features = sp.Some(sp.NewShared(symbols, nil))
		}
		macro := sp.None[sp.DefId]()
		parent := sp.None[sp.ModId]()
		if i%2 == 0 {
			parent = sp.Some(sp.NewModIdUnchecked(id))
		} else {
			macro = sp.Some(id)
		}
		data := sp.NewExpnData(hygieneCodecKind(session, i+1), sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(7)}, sp.NewSpan(session, sp.BytePos(i+10), sp.BytePos(i+20), sp.RootSyntaxContext(), sp.None[sp.LocalDefId]()), sp.DummySpan(), features, sp.Edition(i%5), macro, parent, i&1 != 0, i&2 != 0, i&4 != 0, i&3 == 0)
		defer data.Close()
		data.Encode(e)
	case n <= 63:
		(sp.SyntaxContextKey{Parent: sp.SyntaxContextFromU32(17), Expansion: sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(99)}, Transparency: sp.Transparency(n - 61)}).Encode(e)
	case n <= 66:
		(sp.HygieneMark{Expansion: sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(99)}, Transparency: sp.Transparency(n - 64)}).Encode(e)
	default:
		panic("case")
	}
}
func decodeHygieneCodec(session *sp.SessionGlobals, n int, d sp.SpanDecoder) string {
	switch {
	case n <= 2:
		return sp.DecodeTransparency(d).RustDebug()
	case n <= 5:
		return sp.DecodeMacroKind(d).RustDebug()
	case n <= 8:
		return sp.DecodeAstPass(d).RustDebug()
	case n <= 22:
		return sp.DecodeDesugaringKind(d).RustDebug()
	case n <= 43:
		return sp.DecodeExpnKind(d).RustDebug(session)
	case n <= 46:
		return sp.DecodeExpnHash(d).RustDebug()
	case n <= 49:
		return sp.DecodeDecodedSyntaxContext(d).RustDebug()
	case n <= 52:
		return sp.DecodeLocalExpnId(d).RustDebug()
	case n <= 60:
		v := sp.DecodeExpnData(d)
		defer v.Close()
		refs := uint(0)
		if f, ok := v.AllowInternalUnstable.Get(); ok {
			refs = f.StrongCount()
		}
		return fmt.Sprintf("%s;refs=%d", v.RustDebug(session), refs)
	case n <= 63:
		v := sp.DecodeSyntaxContextKey(d)
		return fmt.Sprintf("(%s, %s, %s)", fmt.Sprintf("#%d", v.Parent.AsU32()), v.Expansion.RustDebug(), v.Transparency.RustDebug())
	case n <= 66:
		v := sp.DecodeHygieneMark(d)
		return fmt.Sprintf("(%s, %s)", v.Expansion.RustDebug(), v.Transparency.RustDebug())
	default:
		panic("case")
	}
}

func TestOriginalHygieneCodec(t *testing.T) {
	session := sp.NewSessionGlobals(sp.Edition2015, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	readHygieneOracle(t, "codec", func(row hygieneOracleRow) string {
		makeCodec := func(c *hygieneCodec) interface {
			sp.SpanEncoder
			sp.SpanDecoder
		} { if row.Mode&1 != 0 {
			return hygieneBoolCodec{c}
		}; return c }
		success := &hygieneCodec{session: session, fail: -1}
		encodeHygieneCodec(session, row.N, makeCodec(success))
		current := &hygieneCodec{session: session, fail: row.Mode/4 - 1}
		decode := row.Mode&2 != 0
		if decode {
			current.atoms = success.atoms
		}
		failed := false
		value := ""
		func() {
			defer func() {
				if recover() != nil {
					failed = true
				}
			}()
			if decode {
				value = decodeHygieneCodec(session, row.N, makeCodec(current))
			} else {
				encodeHygieneCodec(session, row.N, makeCodec(current))
			}
		}()
		if decode {
			return fmt.Sprintf("decode %t %d %s\nvalue %s\n", failed, current.position, strings.Join(current.events, ","), value)
		}
		return fmt.Sprintf("encode %t %s\n", failed, strings.Join(current.events, ","))
	})
}
