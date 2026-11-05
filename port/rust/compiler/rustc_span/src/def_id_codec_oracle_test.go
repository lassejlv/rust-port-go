package rustcspan

import (
	"encoding/hex"
	"fmt"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"reflect"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type defIDCodecEncoder struct {
	SpanEncoder
	events *[]string
	fail   int
}

func (e *defIDCodecEncoder) record(s string) {
	n := len(*e.events)
	*e.events = append(*e.events, s)
	if n == e.fail {
		panic("encoder")
	}
}
func (e *defIDCodecEncoder) EmitRawBytes(b []byte) { e.record("raw:" + hex.EncodeToString(b)) }
func (e *defIDCodecEncoder) EncodeDefId(id DefId) {
	e.record(fmt.Sprintf("def:%d:%d", id.Krate.AsU32(), id.Index.AsU32()))
}
func (e *defIDCodecEncoder) EncodeDefIndex(id DefIndex) {
	e.record(fmt.Sprintf("index:%d", id.AsU32()))
}
func (e *defIDCodecEncoder) EncodeCrateNum(id CrateNum) {
	e.record(fmt.Sprintf("crate:%d", id.AsU32()))
}

type defIDCodecOverride struct{ *defIDCodecEncoder }

func (e *defIDCodecOverride) EncodeLocalDefId(id LocalDefId) {
	e.record(fmt.Sprintf("local:%d", id.LocalDefIndex.AsU32()))
}

type defIDCodecDecoder struct {
	*SpanMemDecoder
	events *[]string
	fail   int
	raw    []byte
	id     DefId
}

func (d *defIDCodecDecoder) record(s string) {
	n := len(*d.events)
	*d.events = append(*d.events, s)
	if n == d.fail {
		panic("decoder")
	}
}
func (d *defIDCodecDecoder) ReadRawBytes(n uint) []byte {
	d.record(fmt.Sprintf("raw-read:%d", n))
	return d.raw
}
func (d *defIDCodecDecoder) DecodeDefId() DefId { d.record("decode-def"); return d.id }
func (d *defIDCodecDecoder) DecodeLocalDefId() LocalDefId {
	d.record("decode-local")
	return LocalDefId{d.id.Index}
}
func (d *defIDCodecDecoder) DecodeDefIndex() DefIndex { d.record("decode-index"); return d.id.Index }
func (d *defIDCodecDecoder) DecodeCrateNum() CrateNum { d.record("decode-crate"); return d.id.Krate }
func defIDCodec(q defIDRequest) any {
	plain := defIDCodecPath(q, false)
	dictionary := defIDCodecPath(q, true)
	if !reflect.DeepEqual(plain, dictionary) {
		panic(fmt.Sprintf("codec borrowed dictionary differs: interface%v dictionary%v", plain, dictionary))
	}
	return plain
}
func defIDCodecPath(q defIDRequest, dictionary bool) any {
	events := []string{}
	value, failed := defIDCatch(func() any {
		id := DefId{Krate: CrateNumFromU32(q.Crate), Index: DefIndexFromU32(q.Index)}
		if q.Method == "encode" {
			base := &defIDCodecEncoder{SpanEncoder: NewSpanMemEncoder(nil), events: &events, fail: q.Fail}
			var e SpanEncoder = base
			if q.Override {
				e = &defIDCodecOverride{base}
			}
			if dictionary {
				e = defIDCodecEncoderRef(base, q.Override)
			}
			fp := DefPathHash{datastructures.NewFingerprint(rustchashes.NewHash64(q.First), rustchashes.NewHash64(q.Second))}
			switch q.Kind {
			case "id":
				id.Encode(e)
			case "index":
				id.Index.Encode(e)
			case "crate":
				id.Krate.Encode(e)
			case "local":
				(LocalDefId{id.Index}).Encode(e)
			case "mod":
				NewModIdUnchecked(id).Encode(e)
			case "localmod":
				NewLocalModIdUnchecked(LocalDefId{id.Index}).Encode(e)
			case "path":
				fp.Encode(e)
			case "stable":
				fp.StableCrateId().Encode(e)
			default:
				panic("unknown codec kind")
			}
			return nil
		}
		raw, e := hex.DecodeString(q.RawHex)
		if e != nil {
			panic(e)
		}
		bytes := []byte(rustcserialize.MagicEndBytes)
		base, e := NewSpanMemDecoder(nil, bytes, 0)
		if e != nil {
			panic(e)
		}
		state := &defIDCodecDecoder{SpanMemDecoder: base, events: &events, fail: q.Fail, raw: raw, id: id}
		var d SpanDecoder = state
		if dictionary {
			d = defIDCodecDecoderRef(state)
		}
		switch q.Kind {
		case "id":
			return defIDRaw(DecodeDefId(d))
		case "index":
			return DecodeDefIndex(d).AsU32()
		case "crate":
			return DecodeCrateNum(d).AsU32()
		case "local":
			return DecodeLocalDefId(d).LocalDefIndex.AsU32()
		case "mod":
			return defIDRaw(DecodeModId(d).ToDefId())
		case "localmod":
			return DecodeLocalModId(d).ToLocalDefId().LocalDefIndex.AsU32()
		case "path":
			p := DecodeDefPathHash(d)
			return [2]uint64{p.StableCrateId().AsU64(), p.LocalHash().AsU64()}
		case "stable":
			return DecodeStableCrateId(d).AsU64()
		default:
			panic("unknown codec kind")
		}
	})
	var p any
	if failed {
		p = value
		value = nil
	}
	return map[string]any{"value": value, "panic": p, "events": events}
}

// Required unused operations forward through the complete real base encoder;
// relevant callbacks borrow the actual outer state. Nil LocalDefId selects the
// source default on this same Self, rather than a promoted inner Go method.
func defIDCodecEncoderRef(state *defIDCodecEncoder, override bool) *SpanEncoderRef[defIDCodecEncoder] {
	ops := SpanEncoderOps[defIDCodecEncoder]{
		EmitUsize: func(r SpanEncoderBorrow[defIDCodecEncoder], v uint) { r.Value.SpanEncoder.EmitUsize(v) },
		EmitU128: func(r SpanEncoderBorrow[defIDCodecEncoder], v rustcserialize.Uint128) {
			r.Value.SpanEncoder.EmitU128(v)
		},
		EmitU64:          func(r SpanEncoderBorrow[defIDCodecEncoder], v uint64) { r.Value.SpanEncoder.EmitU64(v) },
		EmitU32:          func(r SpanEncoderBorrow[defIDCodecEncoder], v uint32) { r.Value.SpanEncoder.EmitU32(v) },
		EmitU16:          func(r SpanEncoderBorrow[defIDCodecEncoder], v uint16) { r.Value.SpanEncoder.EmitU16(v) },
		EmitU8:           func(r SpanEncoderBorrow[defIDCodecEncoder], v uint8) { r.Value.SpanEncoder.EmitU8(v) },
		EmitIsize:        func(r SpanEncoderBorrow[defIDCodecEncoder], v int) { r.Value.SpanEncoder.EmitIsize(v) },
		EmitI128:         func(r SpanEncoderBorrow[defIDCodecEncoder], v rustcserialize.Int128) { r.Value.SpanEncoder.EmitI128(v) },
		EmitI64:          func(r SpanEncoderBorrow[defIDCodecEncoder], v int64) { r.Value.SpanEncoder.EmitI64(v) },
		EmitI32:          func(r SpanEncoderBorrow[defIDCodecEncoder], v int32) { r.Value.SpanEncoder.EmitI32(v) },
		EmitI16:          func(r SpanEncoderBorrow[defIDCodecEncoder], v int16) { r.Value.SpanEncoder.EmitI16(v) },
		EmitRawBytes:     func(r SpanEncoderBorrow[defIDCodecEncoder], v []byte) { r.Value.EmitRawBytes(v) },
		EncodeSpan:       func(r SpanEncoderBorrow[defIDCodecEncoder], v Span) { r.Value.SpanEncoder.EncodeSpan(v) },
		EncodeSymbol:     func(r SpanEncoderBorrow[defIDCodecEncoder], v Symbol) { r.Value.SpanEncoder.EncodeSymbol(v) },
		EncodeByteSymbol: func(r SpanEncoderBorrow[defIDCodecEncoder], v ByteSymbol) { r.Value.SpanEncoder.EncodeByteSymbol(v) },
		EncodeExpnId:     func(r SpanEncoderBorrow[defIDCodecEncoder], v ExpnId) { r.Value.SpanEncoder.EncodeExpnId(v) },
		EncodeSyntaxContext: func(r SpanEncoderBorrow[defIDCodecEncoder], v SyntaxContext) {
			r.Value.SpanEncoder.EncodeSyntaxContext(v)
		},
		EncodeCrateNum: func(r SpanEncoderBorrow[defIDCodecEncoder], v CrateNum) { r.Value.EncodeCrateNum(v) },
		EncodeDefIndex: func(r SpanEncoderBorrow[defIDCodecEncoder], v DefIndex) { r.Value.EncodeDefIndex(v) },
		EncodeDefId:    func(r SpanEncoderBorrow[defIDCodecEncoder], v DefId) { r.Value.EncodeDefId(v) },
	}
	if override {
		ops.EncodeLocalDefId = func(r SpanEncoderBorrow[defIDCodecEncoder], v LocalDefId) {
			(&defIDCodecOverride{r.Value}).EncodeLocalDefId(v)
		}
	}
	return NewSpanEncoderRef(state, ops, rustfmt.Context{})
}
func defIDCodecDecoderRef(state *defIDCodecDecoder) *SpanDecoderRef[defIDCodecDecoder] {
	ops := SpanDecoderOps[defIDCodecDecoder]{
		ReadUsize:           func(r SpanDecoderBorrow[defIDCodecDecoder]) uint { return r.Value.ReadUsize() },
		ReadU128:            func(r SpanDecoderBorrow[defIDCodecDecoder]) rustcserialize.Uint128 { return r.Value.ReadU128() },
		ReadU64:             func(r SpanDecoderBorrow[defIDCodecDecoder]) uint64 { return r.Value.ReadU64() },
		ReadU32:             func(r SpanDecoderBorrow[defIDCodecDecoder]) uint32 { return r.Value.ReadU32() },
		ReadU16:             func(r SpanDecoderBorrow[defIDCodecDecoder]) uint16 { return r.Value.ReadU16() },
		ReadU8:              func(r SpanDecoderBorrow[defIDCodecDecoder]) uint8 { return r.Value.ReadU8() },
		ReadIsize:           func(r SpanDecoderBorrow[defIDCodecDecoder]) int { return r.Value.ReadIsize() },
		ReadI128:            func(r SpanDecoderBorrow[defIDCodecDecoder]) rustcserialize.Int128 { return r.Value.ReadI128() },
		ReadI64:             func(r SpanDecoderBorrow[defIDCodecDecoder]) int64 { return r.Value.ReadI64() },
		ReadI32:             func(r SpanDecoderBorrow[defIDCodecDecoder]) int32 { return r.Value.ReadI32() },
		ReadI16:             func(r SpanDecoderBorrow[defIDCodecDecoder]) int16 { return r.Value.ReadI16() },
		PeekByte:            func(r SpanDecoderBorrow[defIDCodecDecoder]) uint8 { return r.Value.PeekByte() },
		Position:            func(r SpanDecoderBorrow[defIDCodecDecoder]) uint { return r.Value.Position() },
		ReadRawBytes:        func(r SpanDecoderBorrow[defIDCodecDecoder], n uint) []byte { return r.Value.ReadRawBytes(n) },
		DecodeSymbol:        func(r SpanDecoderBorrow[defIDCodecDecoder]) Symbol { return r.Value.DecodeSymbol() },
		DecodeByteSymbol:    func(r SpanDecoderBorrow[defIDCodecDecoder]) ByteSymbol { return r.Value.DecodeByteSymbol() },
		DecodeDefIndex:      func(r SpanDecoderBorrow[defIDCodecDecoder]) DefIndex { return r.Value.DecodeDefIndex() },
		DecodeLocalDefId:    func(r SpanDecoderBorrow[defIDCodecDecoder]) LocalDefId { return r.Value.DecodeLocalDefId() },
		DecodeSpan:          func(r SpanDecoderBorrow[defIDCodecDecoder]) Span { return r.Value.DecodeSpan() },
		DecodeExpnId:        func(r SpanDecoderBorrow[defIDCodecDecoder]) ExpnId { return r.Value.DecodeExpnId() },
		DecodeSyntaxContext: func(r SpanDecoderBorrow[defIDCodecDecoder]) SyntaxContext { return r.Value.DecodeSyntaxContext() },
		DecodeCrateNum:      func(r SpanDecoderBorrow[defIDCodecDecoder]) CrateNum { return r.Value.DecodeCrateNum() },
		DecodeDefId:         func(r SpanDecoderBorrow[defIDCodecDecoder]) DefId { return r.Value.DecodeDefId() },
		DecodeAttrId:        func(r SpanDecoderBorrow[defIDCodecDecoder]) AttrId { return r.Value.DecodeAttrId() },
	}
	return NewSpanDecoderRef(state, ops, rustfmt.Context{})
}
