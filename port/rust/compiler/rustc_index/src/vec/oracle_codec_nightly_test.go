//go:build ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"

const storageCodecEnabled = true

func storageReplayCodec(r storageRequest) map[string]any {
	h := &storageHarness{}
	ops := h.ops(false)
	idx := index.NewIdx(func(n uint) uint {
		h.event("new", n)
		if n > r.Limit {
			panic("index domain")
		}
		return n
	}, func(n uint) uint { h.event("index", n); return n })
	cfg := index.IndexStorageConfig{OverflowChecks: r.Checked, DebugAssertions: r.Debug}
	data := make([]int64, len(r.Values), r.Capacity)
	copy(data, r.Values)
	v := index.IndexVecFromRawWithConfig(data, idx, ops, cfg)
	steps := []any{}
	for _, a := range r.Actions {
		h.events = []string{}
		h.fail = a.Panic
		h.nth = a.PanicAt
		if h.nth == 0 {
			h.nth = 1
		}
		h.count = 0
		var result any
		panicked := false
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			switch a.Op {
			case "encode", "encode_override":
				encoded := []int64{}
				encoder := index.IndexVecEncoder[int64]{EmitUsize: func(n uint) { h.event("emit_len", n); encoded = append(encoded, int64(n)) }, EncodeElement: func(x *int64) { h.event("encode", *x); h.event("emit_value", *x); encoded = append(encoded, *x) }}
				if a.Op == "encode_override" {
					encoder.EncodeSliceOverride = func(raw index.IndexRaw[int64]) {
						h.event("encode_slice", raw.Len())
						for i := uint(0); i < raw.Len(); i++ {
							encoded = append(encoded, *raw.At(i))
						}
					}
				}
				v.Encode(encoder)
				result = encoded
			case "decode", "decode_override":
				position := 0
				decoder := index.IndexVecDecoder[int64]{ReadUsize: func() uint { h.event("read_len", a.N); return a.N }, DecodeElement: func() int64 {
					h.event("decode")
					value := a.Values[position]
					h.event("read_value", value)
					position++
					return value
				}}
				if a.Op == "decode_override" {
					decoder.DecodeVecOverride = func() index.IndexRaw[int64] {
						h.event("decode_vec")
						values := make([]int64, len(a.Values))
						copy(values, a.Values)
						return index.IndexVecFromRawWithConfig(values, idx, ops, cfg).Raw
					}
				}
				x := index.DecodeIndexVecWithConfig(decoder, idx, ops, cfg)
				old := v
				v = x
				old.Close()
			case "extend_one":
				v.ExtendOne(a.Value)
			case "extend_reserve":
				v.ExtendReserve(a.N)
			default:
				panic("codec action")
			}
		}()
		steps = append(steps, map[string]any{"value": result, "panic": panicked, "events": append([]string{}, h.events...), "v": storageValues(v), "w": []int64{}, "capacity": v.Raw.Cap(), "other_capacity": uint(0)})
	}
	return map[string]any{"steps": steps}
}
