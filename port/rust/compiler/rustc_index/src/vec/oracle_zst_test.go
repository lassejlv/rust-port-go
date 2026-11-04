// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"fmt"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

func storageZSTRequests() []storageRequest {
	var out []storageRequest
	maximum := ^uint(0)
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, length := range []uint{0, 1, 2, 7, maximum / 2, maximum - 1, maximum} {
				for _, op := range []string{"len", "next_index", "last_index", "get", "at", "push", "pop", "truncate", "resize", "ensure", "resize_elem", "shrink", "clone", "clone_into", "append", "binary", "indices", "owned", "drain", "from_iter_owned", "range", "copy", "close", "raw_nth", "raw_nth_back", "raw_count", "raw_last", "owned_nth", "owned_nth_back", "owned_count", "owned_last"} {
					// Growing with a factory is finite; shrinking and direct clone fills are O(1).
					a := storageAction{Op: op, A: 0, B: length, N: 0, StartKind: 1, EndKind: 2, Commands: []string{"hint", "next", "back", "hint", "close"}}
					if op == "get" || op == "at" || op == "ensure" || op == "resize_elem" {
						a.A = maximum
					}
					if op == "resize" {
						a.N = 7
					}
					if op == "raw_nth" || op == "raw_nth_back" || op == "owned_nth" || op == "owned_nth_back" {
						a.A = 2
					}
					out = append(out, storageRequest{Kind: "zst", Name: fmt.Sprintf("zst-%d-%s", length, op), Checked: checked, Debug: debug, Capacity: length, OtherCapacity: 2, Limit: maximum, Actions: []storageAction{a}})
				}
			}
		}
	}
	base := append([]storageRequest{}, out...)
	for _, r := range base {
		switch r.Actions[0].Op {
		case "raw_nth", "raw_nth_back", "owned_nth", "owned_nth_back":
			r.Actions = append([]storageAction{}, r.Actions...)
			r.Actions[0].A = maximum
			r.Name += "-skip-max"
			out = append(out, r)
		}
	}
	return out
}

func storageReplayZST(r storageRequest) map[string]any {
	h := &storageHarness{}
	idx := index.NewIdx(func(n uint) uint { h.event("new", n); return n }, func(n uint) uint { h.event("index", n); return n })
	ops := index.ValueOps[struct{}]{Copy: true, TrivialClone: true}
	cfg := index.IndexStorageConfig{OverflowChecks: r.Checked, DebugAssertions: r.Debug}
	v := index.IndexVecFromElemNWithConfig(struct{}{}, r.Capacity, idx, ops, cfg)
	w := index.IndexVecFromElemNWithConfig(struct{}{}, r.OtherCapacity, idx, ops, cfg)
	steps := []any{}
	for _, a := range r.Actions {
		h.events = []string{}
		var result any
		panicked := false
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			switch a.Op {
			case "raw_nth", "raw_nth_back":
				it := v.Iter()
				var ok bool
				if a.Op == "raw_nth" {
					_, ok = it.Nth(a.A)
				} else {
					_, ok = it.NthBack(a.A)
				}
				result = []any{ok, it.Len()}
			case "raw_count":
				result = v.Iter().Count()
			case "raw_last":
				_, ok := v.Iter().Last()
				result = ok
			case "owned_nth", "owned_nth_back":
				it := v.IntoIter()
				defer it.Close()
				var ok bool
				if a.Op == "owned_nth" {
					_, ok = it.Nth(a.A)
				} else {
					_, ok = it.NthBack(a.A)
				}
				result = []any{ok, it.Len()}
			case "owned_count":
				result = v.IntoIter().Count()
			case "owned_last":
				_, ok := v.IntoIter().Last()
				result = ok
			case "len":
				result = []any{v.Len(), v.IsEmpty()}
			case "next_index":
				result = v.NextIndex()
			case "last_index":
				k, ok := v.LastIndex()
				if ok {
					result = k
				}
			case "get":
				_, ok := v.Get(a.A)
				result = ok
			case "at":
				_ = v.At(a.A)
				result = true
			case "push":
				result = v.Push(struct{}{})
			case "pop":
				_, ok := v.Pop()
				result = ok
			case "truncate":
				v.Truncate(a.N)
			case "resize":
				v.Resize(a.N, struct{}{})
			case "ensure":
				_ = v.EnsureContainsElem(a.A, func() struct{} { h.event("factory"); return struct{}{} })
				result = true
			case "resize_elem":
				v.ResizeToElem(a.A, func() struct{} { h.event("factory"); return struct{}{} })
			case "shrink":
				v.ShrinkToFit()
			case "clone":
				x := v.Clone()
				result = x.Len()
				x.Close()
			case "clone_into":
				v.AsSlice().CloneInto(w)
			case "append":
				v.Append(w)
			case "binary":
				k, ok := v.BinarySearch(struct{}{})
				result = []any{k, ok}
			case "indices":
				it := v.Indices()
				result = []any{}
				xs := []any{}
				for _, c := range a.Commands {
					switch c {
					case "next":
						k, ok := it.Next()
						if ok {
							xs = append(xs, k)
						} else {
							xs = append(xs, nil)
						}
					case "back":
						k, ok := it.NextBack()
						if ok {
							xs = append(xs, k)
						} else {
							xs = append(xs, nil)
						}
					case "hint":
						x, y := it.SizeHint()
						xs = append(xs, []any{x, y})
					}
				}
				result = xs
			case "owned":
				it := v.IntoIter()
				defer it.Close()
				xs := []any{}
				for _, c := range a.Commands {
					switch c {
					case "next":
						_, ok := it.Next()
						xs = append(xs, ok)
					case "back":
						_, ok := it.NextBack()
						xs = append(xs, ok)
					case "hint":
						x, y := it.SizeHint()
						xs = append(xs, []any{x, y})
					}
				}
				result = xs
			case "drain":
				it := v.Drain(index.BoundsFromSliceIndex(index.TypedSliceIndexResult{Kind: index.SliceIndexRange, Start: a.A, End: a.B}))
				defer it.Close()
				_, ok := it.Next()
				x, y := it.SizeHint()
				result = []any{ok, []any{x, y}}
			case "from_iter_owned":
				v = index.IndexVecFromIteratorWithConfig(v.IntoIter().IntoIterator(), idx, ops, cfg)
			case "range":
				x, ok := v.GetIndex(index.RangeSliceIdx(idx).IntoSliceIdx(index.Range[uint]{Start: a.A, End: a.B}))
				if ok {
					result = x.Raw.Len()
				}
			case "copy":
				v.CopyWithin(index.RangeSliceIdx(idx).IntoSliceIdx(index.Range[uint]{Start: a.A, End: a.B}), 0)
			case "close":
				v.Close()
			default:
				panic("zst action")
			}
		}()
		steps = append(steps, map[string]any{"value": result, "panic": panicked, "events": append([]string{}, h.events...), "length": v.Len(), "other_length": w.Len(), "capacity": v.Raw.Cap(), "other_capacity": w.Raw.Cap()})
	}
	return map[string]any{"steps": steps}
}
