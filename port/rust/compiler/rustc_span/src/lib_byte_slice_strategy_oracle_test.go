// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

type libByteWrapper struct {
	*rustcserialize.MemEncoder
	calls, raw int
}

func (e *libByteWrapper) EmitU8(value uint8)        { e.calls++; e.MemEncoder.EmitU8(value) }
func (e *libByteWrapper) EmitRawBytes(value []byte) { e.raw++; e.MemEncoder.EmitRawBytes(value) }
func (e *libByteWrapper) OpaqueContiguousBytes() bool {
	panic("non-Rust inheritable marker was discovered")
}

type libSpanByteWrapper struct {
	*SpanMemEncoder
	calls, raw int
}

func (e *libSpanByteWrapper) EmitU8(value uint8) { e.calls++; e.SpanMemEncoder.EmitU8(value) }
func (e *libSpanByteWrapper) EmitRawBytes(value []byte) {
	e.raw++
	e.SpanMemEncoder.EmitRawBytes(value)
}

type libByteRefState struct {
	base       *SpanMemEncoder
	calls, raw int
	current    *rustfmt.RuntimeMetadata
}

func libByteRef(opaque bool, current rustfmt.Context) (*SpanEncoderRef[libByteRefState], *libByteRefState) {
	state := &libByteRefState{base: NewSpanMemEncoder(nil), current: current.Runtime}
	ops := SpanEncoderOps[libByteRefState]{OpaqueBytes: opaque,
		EmitUsize: func(b SpanEncoderBorrow[libByteRefState], value uint) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitUsize(value); return struct{}{} })
		},
		EmitU128: func(b SpanEncoderBorrow[libByteRefState], value rustcserialize.Uint128) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitU128(value); return struct{}{} })
		},
		EmitU64: func(b SpanEncoderBorrow[libByteRefState], value uint64) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitU64(value); return struct{}{} })
		},
		EmitU32: func(b SpanEncoderBorrow[libByteRefState], value uint32) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitU32(value); return struct{}{} })
		},
		EmitU16: func(b SpanEncoderBorrow[libByteRefState], value uint16) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitU16(value); return struct{}{} })
		},
		EmitU8: func(b SpanEncoderBorrow[libByteRefState], value uint8) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			b.Value.calls++
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitU8(value); return struct{}{} })
		},
		EmitIsize: func(b SpanEncoderBorrow[libByteRefState], value int) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitIsize(value); return struct{}{} })
		},
		EmitI128: func(b SpanEncoderBorrow[libByteRefState], value rustcserialize.Int128) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitI128(value); return struct{}{} })
		},
		EmitI64: func(b SpanEncoderBorrow[libByteRefState], value int64) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitI64(value); return struct{}{} })
		},
		EmitI32: func(b SpanEncoderBorrow[libByteRefState], value int32) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitI32(value); return struct{}{} })
		},
		EmitI16: func(b SpanEncoderBorrow[libByteRefState], value int16) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitI16(value); return struct{}{} })
		},
		EmitRawBytes: func(b SpanEncoderBorrow[libByteRefState], value []byte) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			b.Value.raw++
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EmitRawBytes(value); return struct{}{} })
		},
		EncodeSpan: func(b SpanEncoderBorrow[libByteRefState], value Span) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeSpan(value); return struct{}{} })
		},
		EncodeSymbol: func(b SpanEncoderBorrow[libByteRefState], value Symbol) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeSymbol(value); return struct{}{} })
		},
		EncodeByteSymbol: func(b SpanEncoderBorrow[libByteRefState], value ByteSymbol) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeByteSymbol(value); return struct{}{} })
		},
		EncodeExpnId: func(b SpanEncoderBorrow[libByteRefState], value ExpnId) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeExpnId(value); return struct{}{} })
		},
		EncodeSyntaxContext: func(b SpanEncoderBorrow[libByteRefState], value SyntaxContext) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeSyntaxContext(value); return struct{}{} })
		},
		EncodeCrateNum: func(b SpanEncoderBorrow[libByteRefState], value CrateNum) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeCrateNum(value); return struct{}{} })
		},
		EncodeDefIndex: func(b SpanEncoderBorrow[libByteRefState], value DefIndex) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeDefIndex(value); return struct{}{} })
		},
		EncodeDefId: func(b SpanEncoderBorrow[libByteRefState], value DefId) {
			if b.Context.Runtime != b.Value.current {
				panic("byte strategy lost current source Context")
			}
			rustcserialize.WithRuntimeContext(b.Value.base, b.Context, func(e *SpanMemEncoder) struct{} { e.EncodeDefId(value); return struct{}{} })
		},
	}
	encoder := NewSpanEncoderRef(state, ops, rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "constructor"}})
	return encoder.WithContext(current), state
}
func TestLibOriginalByteSliceSourceSelf(t *testing.T) {
	file, err := os.Open("testdata/lib-byte-slice-strategy.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 2*1024*1024+1))
	if err != nil || len(raw) > 2*1024*1024 {
		t.Fatal("invalid byte-slice strategy fixture", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	count := 0
	for scanner.Scan() {
		p := strings.Split(scanner.Text(), "\t")
		if len(p) < 5 {
			t.Fatal("invalid byte-slice row")
		}
		var output []byte
		calls, rawCalls := 0, 0
		wantCalls, wantRaw := 0, 0
		capacityText, encoded := "", ""
		switch p[1] {
		case "slice":
			n, _ := strconv.Atoi(p[3])
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(i*19 + 42)
			}
			switch p[2] {
			case "direct":
				encoder := rustcserialize.NewMemEncoder()
				rustcserialize.EncodeBytes(encoder, data)
				output = encoder.Finish()
			case "alias":
				type alias = rustcserialize.MemEncoder
				var encoder *alias = rustcserialize.NewMemEncoder()
				rustcserialize.EncodeBytes(encoder, data)
				output = encoder.Finish()
			case "span":
				encoder := NewSpanMemEncoder(nil)
				rustcserialize.EncodeBytesWithStrategy(encoder, data, spanByteSliceStrategy(encoder))
				output = encoder.Finish()
				current := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "later source caller"}}
				reference, state := libByteRef(true, current)
				rustcserialize.EncodeBytesWithStrategy(reference, data, SpanEncoderRefByteSliceStrategy(reference))
				observed := state.base.Finish()
				if string(observed) != string(output) || cap(observed) != cap(output) || state.calls != 0 || state.raw != 1 {
					t.Fatalf("explicit source Self dictionary de-specialized n%d", n)
				}
			default:
				t.Fatal("unexpected concrete source type")
			}
			capacityText, encoded = p[4], p[5]
		case "wrapped":
			n, _ := strconv.Atoi(p[2])
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(i*19 + 42)
			}
			encoder := &libByteWrapper{MemEncoder: rustcserialize.NewMemEncoder()}
			rustcserialize.EncodeBytes(encoder, data)
			calls, rawCalls = encoder.calls, encoder.raw
			output = encoder.Finish()
			wantCalls, _ = strconv.Atoi(p[3])
			wantRaw, _ = strconv.Atoi(p[4])
			capacityText, encoded = p[5], p[6]
			projected := &libSpanByteWrapper{SpanMemEncoder: NewSpanMemEncoder(nil)}
			rustcserialize.EncodeBytesWithStrategy(projected, data, spanByteSliceStrategy(projected))
			projection := projected.Finish()
			if projected.calls != wantCalls || projected.raw != wantRaw || cap(projection) != cap(output) || string(projection) != string(output) {
				t.Fatal("source wrapper accidentally inherited Span-Self specialization")
			}
		case "hash":
			value := NewSourceFileHashInMemory(SourceFileHashMd5, []byte("source"))
			if p[2] == "direct" {
				encoder := NewSpanMemEncoder(nil)
				value.Encode(encoder)
				output = encoder.Finish()
				capacityText, encoded = p[3], p[4]
				current := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "explicit source Self"}}
				reference, state := libByteRef(true, current)
				value.EncodeWithByteSliceStrategy(reference, SpanEncoderRefByteSliceStrategy(reference))
				observed := state.base.Finish()
				if string(observed) != string(output) || cap(observed) != cap(output) || state.calls != 1 || state.raw != 1 {
					t.Fatal("source hash explicit-Self implementation lost opaque array strategy")
				}
			} else {
				encoder := &libSpanByteWrapper{SpanMemEncoder: NewSpanMemEncoder(nil)}
				value.Encode(encoder)
				calls, rawCalls = encoder.calls, encoder.raw
				output = encoder.Finish()
				wantCalls, _ = strconv.Atoi(p[3])
				wantRaw, _ = strconv.Atoi(p[4])
				capacityText, encoded = p[5], p[6]
			}
		default:
			t.Fatal("unknown source byte-strategy operation")
		}
		wantCapacity, _ := strconv.Atoi(capacityText)
		if cap(output) != wantCapacity || hex.EncodeToString(output) != encoded || calls != wantCalls || rawCalls != wantRaw {
			t.Fatalf("%s differs: calls%d/%d raw%d/%d capacity%d/%d bytes%s/%s", scanner.Text(), calls, wantCalls, rawCalls, wantRaw, cap(output), wantCapacity, hex.EncodeToString(output), encoded)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 280 {
		t.Fatalf("source byte strategy corpus%d", count)
	}
}
