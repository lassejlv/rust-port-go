// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"fmt"
	"strconv"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

const storageNone int64 = -1000000

func storageOptionRequests() []storageRequest {
	var out []storageRequest
	inputs := [][]int64{{}, {storageNone}, {1}, {storageNone, 3, storageNone, 7}, {2, 4, 6}}
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for i, values := range inputs {
				for j, other := range inputs {
					base := storageRequest{Kind: "option", Name: fmt.Sprintf("option-%d-%d", i, j), Checked: checked, Debug: debug, Values: values, Other: other, Capacity: uint(len(values) + 4), OtherCapacity: uint(len(other)), Limit: 64}
					for _, op := range []string{"insert", "remove", "contains", "get_or_insert", "get_or_insert_owned", "debug", "map", "compact", "clone_into", "clone_from", "eq", "ne", "close"} {
						for a := uint(0); a < 7; a++ {
							if j != 0 && op != "clone_into" && op != "clone_from" && op != "eq" && op != "ne" {
								continue
							}
							for _, fail := range []string{"", "index", "new", "factory", "factory_close", "drop", "clone", "clone_from", "ne", "debug"} {
								relevant := fail == "" || fail == "index" && (op == "insert" || op == "remove" || op == "contains" || op == "get_or_insert" || op == "get_or_insert_owned") || fail == "new" && (op == "map" || op == "compact") || fail == "factory" && (op == "get_or_insert" || op == "get_or_insert_owned") || fail == "factory_close" && op == "get_or_insert_owned" || fail == "drop" && (op == "insert" || op == "clone_into" || op == "clone_from" || op == "close") || (fail == "clone" || fail == "clone_from") && (op == "clone_into" || op == "clone_from") || fail == "ne" && (op == "eq" || op == "ne") || fail == "debug" && (op == "debug" || op == "map" || op == "compact")
								if !relevant {
									continue
								}
								r := base
								r.Name += fmt.Sprintf("-%s-%d-%s", op, a, fail)
								r.Actions = []storageAction{{Op: op, A: a, Value: 11, Panic: fail, PanicAt: 1}}
								out = append(out, r)
							}
						}
					}
				}
			}
		}
	}
	return out
}

func storageOwnedRequests() []storageRequest {
	var out []storageRequest
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, op := range []string{"ensure_owned", "resize_elem_owned", "from_fn_owned", "drain_owned", "drain_enum_owned"} {
				for n := uint(0); n < 7; n++ {
					for _, fail := range []string{"", "index", "new", "factory", "factory_close", "drop", "start", "end", "bounds_close"} {
						for nth := uint(1); nth <= 2; nth++ {
							if (op == "drain_owned" || op == "drain_enum_owned") && n > 3 && fail == "bounds_close" {
								continue
							}
							out = append(out, storageRequest{Name: fmt.Sprintf("owned-%s-%d-%s-%d", op, n, fail, nth), Checked: checked, Debug: debug, Values: []int64{1, 3, 5}, Other: []int64{}, Capacity: 3, Limit: 64, Actions: []storageAction{{Op: op, A: n, B: 3, N: n, Value: 17, StartKind: 1, EndKind: 2, Panic: fail, PanicAt: nth, Commands: []string{"next", "close"}}}})
						}
					}
				}
			}
		}
	}
	return out
}

func storageReplayOption(r storageRequest) map[string]any {
	h := &storageHarness{}
	inner := h.ops(r.CustomCloneFrom)
	idx := index.NewIdx(func(n uint) uint {
		h.event("new", n)
		if n > r.Limit {
			panic("index domain")
		}
		return n
	}, func(n uint) uint { h.event("index", n); return n })
	cfg := index.IndexStorageConfig{OverflowChecks: r.Checked, DebugAssertions: r.Debug}
	ops := index.OptionValueOpsWithLayout(inner, index.OptionLayout{RustSize: 16})
	makeVec := func(xs []int64, capacity uint) *index.OptionIndexVec[uint, int64] {
		data := make([]index.Option[int64], 0, capacity)
		for _, x := range xs {
			if x == storageNone {
				data = append(data, index.None[int64]())
			} else {
				data = append(data, index.Some(x))
			}
		}
		return index.AsOptionIndexVec(index.IndexVecFromRawWithConfig(data, idx, ops, cfg), inner)
	}
	v, w := makeVec(r.Values, r.Capacity), makeVec(r.Other, r.OtherCapacity)
	values := func(v *index.OptionIndexVec[uint, int64]) []any {
		out := []any{}
		for i := uint(0); i < v.Len(); i++ {
			x := v.Raw.At(i)
			if x.Present {
				out = append(out, x.Value)
			} else {
				out = append(out, nil)
			}
		}
		return out
	}
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
			case "insert":
				x := v.Insert(a.A, a.Value)
				if x.Present {
					result = x.Value
				}
			case "remove":
				x := v.Remove(a.A)
				if x.Present {
					result = x.Value
				}
			case "contains":
				result = v.Contains(a.A)
			case "get_or_insert":
				result = *v.GetOrInsertWith(a.A, func() int64 { h.event("factory"); return a.Value })
			case "get_or_insert_owned":
				result = *v.GetOrInsertWithOwned(a.A, index.IndexOnceFactory[int64]{Call: func() int64 { h.event("factory"); return a.Value }, CloseWithContext: func(c index.IndexDropContext) { h.event("factory_close", c.Unwinding) }})
			case "debug":
				result = v.Debug()
			case "map":
				result = v.DebugMapView(func(k uint) string { h.event("key_debug", k); return strconv.FormatUint(uint64(k), 10) }).Debug()
			case "compact":
				result = v.DebugMapViewCompact(func(k uint) string { h.event("key_debug", k); return strconv.FormatUint(uint64(k), 10) }).Debug()
			case "clone_into":
				v.AsSlice().CloneInto(w.IndexVec)
			case "clone_from":
				w.CloneFrom(v.IndexVec)
			case "eq":
				result = v.Equal(w.IndexVec)
			case "ne":
				result = v.NotEqual(w.IndexVec)
			case "close":
				old := v
				v = makeVec(nil, 0)
				old.Close()
			default:
				panic("unknown option action")
			}
		}()
		steps = append(steps, map[string]any{"value": result, "panic": panicked, "events": append([]string{}, h.events...), "v": values(v), "w": values(w), "capacity": v.Raw.Cap(), "other_capacity": w.Raw.Cap()})
	}
	return map[string]any{"steps": steps}
}
