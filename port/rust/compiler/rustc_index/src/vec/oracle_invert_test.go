// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"fmt"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"strconv"
)

func storageInvertRequests() []storageRequest {
	var out []storageRequest
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for i, values := range [][]int64{{}, {0}, {1, 0}, {2, 0, 1}, {0, 0}, {2, 1, 0}, {0, 0, 3}, {4, 0}, {-1, 0}, {0, 0, 3, 3}} {
				for _, fail := range []string{"", "index", "new", "key_debug"} {
					for nth := uint(1); nth <= 4; nth++ {
						kind := "invert"
						if debug && fail == "key_debug" && nth <= uint(len(values)) && (i == 4 || i == 7 || i == 8 || i == 9) {
							kind = "abort_invert"
						}
						out = append(out, storageRequest{Kind: kind, Name: fmt.Sprintf("invert-%d-%s-%d", i, fail, nth), Checked: checked, Debug: debug, Values: values, Other: []int64{}, Capacity: uint(len(values)), Limit: 64, Actions: []storageAction{{Op: "invert", Panic: fail, PanicAt: nth}}})
					}
				}
			}
		}
	}
	base := append([]storageRequest{}, out...)
	for _, r := range base {
		if r.Kind == "abort_invert" || r.Debug && r.Name == "invert-4--1" {
			r.LocalDebug = true
			r.Name += "-locals"
			out = append(out, r)
		}
	}
	return out
}
func storageReplayInvert(r storageRequest) map[string]any {
	h := &storageHarness{}
	idx := index.NewIdx(func(n uint) uint {
		h.event("new", n)
		if n > r.Limit {
			panic("domain")
		}
		return n
	}, func(n uint) uint { h.event("index", n); return n })
	ops := index.ValueOps[uint]{Copy: true, TrivialClone: true, RustSize: 24, DebugRefWithContext: func(n *uint, context index.IndexFormatContext) string {
		if r.LocalDebug {
			defer storageInvertDebugLocalDrop(h, *n, context)
		}
		h.eventUsing(context.Panic, "key_debug", *n, context.Panicking())
		return strconv.FormatUint(uint64(*n), 10)
	}}
	cfg := index.IndexStorageConfig{OverflowChecks: r.Checked, DebugAssertions: r.Debug}
	values := make([]uint, len(r.Values))
	for i, n := range r.Values {
		values[i] = uint(n)
	}
	v := index.IndexVecFromRawWithConfig(values, idx, ops, cfg)
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
			x := v.AsSlice().InvertBijectiveMapping(idx, ops)
			xs := []uint{}
			for i := uint(0); i < x.Len(); i++ {
				xs = append(xs, *x.Raw.At(i))
			}
			result = map[string]any{"values": xs, "capacity": x.Raw.Cap()}
		}()
		steps = append(steps, map[string]any{"value": result, "panic": panicked, "events": append([]string{}, h.events...), "v": values, "w": []uint{}, "capacity": v.Raw.Cap(), "other_capacity": uint(0)})
	}
	return map[string]any{"steps": steps}
}

func storageInvertDebugLocalDrop(h *storageHarness, n uint, context index.IndexFormatContext) {
	if p := recover(); p != nil {
		h.event("key_debug_local_drop", n, true)
		panic(p)
	}
	h.event("key_debug_local_drop", n, context.Panicking())
}
