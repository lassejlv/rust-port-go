// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type storageAction struct {
	Reuse      bool     `json:"reuse,omitempty"`
	CharWriter bool     `json:"char_writer,omitempty"`
	Style      string   `json:"style,omitempty"`
	FailAt     uint     `json:"fail_at,omitempty"`
	Op         string   `json:"op"`
	A          uint     `json:"a,omitempty"`
	B          uint     `json:"b,omitempty"`
	C          uint     `json:"c,omitempty"`
	N          uint     `json:"n,omitempty"`
	Value      int64    `json:"value,omitempty"`
	Values     []int64  `json:"values,omitempty"`
	StartKind  uint     `json:"start_kind,omitempty"`
	EndKind    uint     `json:"end_kind,omitempty"`
	Panic      string   `json:"panic,omitempty"`
	PanicAt    uint     `json:"panic_at,omitempty"`
	Commands   []string `json:"commands,omitempty"`
}
type storageRequest struct {
	LocalDebug      bool            `json:"local_debug,omitempty"`
	Kind            string          `json:"kind,omitempty"`
	Name            string          `json:"name"`
	Nightly         bool            `json:"nightly"`
	Checked         bool            `json:"checked"`
	Debug           bool            `json:"debug"`
	Values          []int64         `json:"values"`
	Other           []int64         `json:"other"`
	Capacity        uint            `json:"capacity"`
	OtherCapacity   uint            `json:"other_capacity"`
	Limit           uint            `json:"limit"`
	CustomCloneFrom bool            `json:"custom_clone_from"`
	Actions         []storageAction `json:"actions"`
}
type storageRecord struct {
	Request  storageRequest  `json:"request"`
	Response json.RawMessage `json:"response"`
}

func storageRequests() []storageRequest {
	var out []storageRequest
	add := func(name string, values, other []int64, capacity uint, actions ...storageAction) {
		for _, checked := range []bool{false, true} {
			for _, debug := range []bool{false, true} {
				out = append(out, storageRequest{Name: name, Nightly: false, Checked: checked, Debug: debug, Values: values, Other: other, Capacity: capacity, OtherCapacity: uint(len(other)), Limit: 64, Actions: actions})
			}
		}
	}
	for n := 0; n <= 7; n++ {
		values := make([]int64, n)
		for i := range values {
			values[i] = int64(i*2 + 1)
		}
		for _, capN := range []uint{uint(n), uint(n + 9)} {
			add(fmt.Sprintf("access-%d-%d", n, capN), values, nil, capN,
				storageAction{Op: "len"}, storageAction{Op: "next_index"}, storageAction{Op: "last_index"}, storageAction{Op: "debug"}, storageAction{Op: "map"}, storageAction{Op: "hash"})
			for a := uint(0); a <= uint(n+1); a++ {
				add(fmt.Sprintf("index-%d-%d-%d", n, capN, a), values, nil, capN, storageAction{Op: "get", A: a}, storageAction{Op: "at", A: a}, storageAction{Op: "write", A: a, Value: 29}, storageAction{Op: "binary", Value: int64(a)})
			}
		}
		for a := uint(0); a <= uint(n+1); a++ {
			for b := uint(0); b <= uint(n+1); b++ {
				add(fmt.Sprintf("ranges-%d-%d-%d", n, a, b), values, nil, uint(n), storageAction{Op: "range", A: a, B: b}, storageAction{Op: "swap", A: a, B: b}, storageAction{Op: "pick2", A: a, B: b}, storageAction{Op: "pick3", A: a, B: b, C: 0})
			}
		}
		for _, op := range []string{"push", "pop", "truncate", "resize", "ensure", "resize_elem", "shrink", "close"} {
			for m := uint(0); m <= uint(n+2); m++ {
				add(fmt.Sprintf("%s-%d-%d", op, n, m), values, nil, uint(n), storageAction{Op: op, A: m, N: m, Value: 17})
			}
		}
		for _, op := range []string{"clone", "to_owned", "clone_into", "clone_from", "append", "eq", "ne"} {
			for m := 0; m <= 5; m++ {
				other := make([]int64, m)
				for i := range other {
					other[i] = int64(i + 10)
				}
				add(fmt.Sprintf("%s-%d-%d", op, n, m), values, other, uint(n), storageAction{Op: op})
			}
		}
		for _, op := range []string{"iter", "iter_mut", "iter_enum", "iter_enum_mut", "indices", "owned", "owned_enum", "drain", "drain_enum"} {
			add(fmt.Sprintf("%s-%d", op, n), values, nil, uint(n+4), storageAction{Op: op, A: 0, B: uint(n), StartKind: 1, EndKind: 2, Commands: []string{"hint", "len", "next", "back", "hint", "next", "next", "close"}})
		}
	}
	for _, op := range []string{"from_elem", "from_fn", "from_array", "from_iter", "from_iter_no_hint", "from_iter_trusted", "from_iter_trusted_each", "from_iter_trusted_fold", "from_iter_owned", "extend_iter", "extend_iter_no_hint", "extend_iter_trusted", "extend_iter_trusted_each", "extend_iter_trusted_fold"} {
		for n := uint(0); n <= 9; n++ {
			for _, fail := range []string{"", "clone", "factory", "factory_close", "new", "next", "hint", "iterator_close", "for_each", "fold"} {
				vals := make([]int64, n)
				for i := range vals {
					vals[i] = int64(i + 1)
				}
				add(fmt.Sprintf("%s-%d-%s", op, n, fail), []int64{3, 5}, nil, 16, storageAction{Op: op, N: n, Value: 7, Values: vals, Panic: fail, PanicAt: 1})
			}
		}
	}
	for _, op := range []string{"clone", "clone_into", "clone_from", "resize", "ensure", "resize_elem", "eq", "ne", "debug", "map", "hash", "push", "owned_enum", "drain", "drain_enum"} {
		for _, fail := range []string{"clone", "clone_from", "drop", "new", "index", "factory", "factory_close", "eq", "ne", "cmp", "debug", "hash", "start", "end", "bounds_close"} {
			for nth := uint(1); nth <= 3; nth++ {
				add(fmt.Sprintf("panic-%s-%s-%d", op, fail, nth), []int64{1, 3, 5, 7}, []int64{2, 4}, 4, storageAction{Op: op, A: 1, B: 3, N: 7, Value: 9, StartKind: 1, EndKind: 2, Panic: fail, PanicAt: nth, Commands: []string{"next", "next", "next", "close"}})
			}
		}
	}
	for sk := uint(0); sk <= 2; sk++ {
		for ek := uint(0); ek <= 2; ek++ {
			for a := uint(0); a <= 5; a++ {
				for b := uint(0); b <= 5; b++ {
					add(fmt.Sprintf("bounds-%d-%d-%d-%d", sk, ek, a, b), []int64{1, 2, 3, 4}, nil, 9, storageAction{Op: "copy", A: a, B: b, C: 1, StartKind: sk, EndKind: ek}, storageAction{Op: "drain_enum", A: a, B: b, StartKind: sk, EndKind: ek, Commands: []string{"next", "close"}})
				}
			}
		}
	}
	for _, capN := range []uint{8, 16, 32} {
		for front := uint(0); front <= 9; front++ {
			for back := uint(0); back <= 3; back++ {
				add(fmt.Sprintf("owned-recollect-%d-%d-%d", capN, front, back), []int64{1, 2, 3, 4, 5, 6, 7, 8}, nil, capN, storageAction{Op: "from_iter_owned", A: front, B: back})
			}
		}
	}
	// Fixed seeds make edits reproducible and keep failures small enough to inspect.
	rng := rand.New(rand.NewSource(0x53544f52414745))
	ops := []string{"push", "pop", "resize", "truncate", "ensure", "write", "swap", "get", "at", "shrink", "append", "eq", "ne", "clone_into", "clone_from"}
	for i := 0; i < 200; i++ {
		var actions []storageAction
		for j := 0; j < 12; j++ {
			actions = append(actions, storageAction{Op: ops[rng.Intn(len(ops))], A: uint(rng.Intn(10)), B: uint(rng.Intn(10)), N: uint(rng.Intn(10)), Value: int64(rng.Intn(21) - 10)})
		}
		add(fmt.Sprintf("script-%d", i), []int64{1, 2, 3}, []int64{4, 5}, 3, actions...)
	}
	// A small typed index domain checks constructor validation separately from raw storage.
	for _, op := range []string{"from_fn", "push", "next_index", "last_index", "indices", "iter_enum", "owned_enum"} {
		for n := 0; n <= 5; n++ {
			values := make([]int64, n)
			start := len(out)
			add(fmt.Sprintf("limited-%s-%d", op, n), values, nil, uint(n), storageAction{Op: op, N: uint(n), Commands: []string{"next", "back", "close"}})
			for i := start; i < len(out); i++ {
				out[i].Limit = 2
			}
		}
	}
	out = append(out, storageOptionRequests()...)
	out = append(out, storageZSTRequests()...)
	out = append(out, storageInvertRequests()...)
	out = append(out, storageCustomRequests()...)
	out = append(out, storageFormatRequests()...)
	out = append(out, storageIteratorRequests()...)
	out = append(out, storageAbortRequests()...)
	out = append(out, storageOwnedRequests()...)
	base := append([]storageRequest{}, out...)
	for _, r := range base {
		for _, a := range r.Actions {
			if a.Op == "clone_into" {
				r.CustomCloneFrom = true
				r.Name += "-custom-clone-from"
				out = append(out, r)
				break
			}
		}
	}
	base = append([]storageRequest{}, out...)
	for _, r := range base {
		r.Nightly = true
		out = append(out, r)
	}
	out = append(out, storageCodecRequests()...)
	return out
}

func storageRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../../.."))
}

func TestDumpStorageOracleRequests(t *testing.T) {
	path, ok := os.LookupEnv("FERRUGO_STORAGE_DUMP_REQUESTS")
	if !ok {
		t.Skip("generator-only request dump")
	}
	if path == "" {
		t.Fatal("FERRUGO_STORAGE_DUMP_REQUESTS is empty")
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range storageRequests() {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
}

type storageHarness struct {
	events     []string
	fail       string
	nth, count uint
}

func (h *storageHarness) event(kind string, args ...any) {
	h.eventUsing(func(p any) { panic(p) }, kind, args...)
}
func (h *storageHarness) eventUsing(panicFn func(any), kind string, args ...any) {
	s := kind
	for _, a := range args {
		s += ":" + fmt.Sprint(a)
	}
	h.events = append(h.events, s)
	storageStreamEvent(s)
	if h.fail == kind {
		h.count++
		if h.count == h.nth {
			panicFn("oracle injection")
		}
	}
}
func (h *storageHarness) ops(custom bool) index.ValueOps[int64] {
	ops := index.ValueOps[int64]{RustSize: 8,
		CloneRef:    func(v *int64) int64 { h.event("clone", *v); return *v },
		EqualRef:    func(a, b *int64) bool { h.event("eq", *a, *b); return *a == *b },
		NotEqualRef: func(a, b *int64) bool { h.event("ne", *a, *b); return *a != *b },
		DebugRef:    func(v *int64) string { h.event("debug", *v); return strconv.FormatInt(*v, 10) },
		CompareRef: func(a, b *int64) int {
			h.event("cmp", *a, *b)
			if *a < *b {
				return -1
			}
			if *a > *b {
				return 1
			}
			return 0
		},
		Drop:            func(v *int64) { h.event("drop", *v, false) },
		DropWithContext: func(v *int64, context index.IndexDropContext) { h.event("drop", *v, context.Unwinding) },
	}
	if custom {
		ops.CloneFromRef = func(a, b *int64) { h.event("clone_from", *a, *b); *a = *b }
	}
	return ops
}
func storageValues(v *index.IndexVec[uint, int64]) []int64 {
	r := make([]int64, 0, v.Len())
	for i := uint(0); i < v.Len(); i++ {
		r = append(r, *v.Raw.At(i))
	}
	return r
}

func storageReplay(r storageRequest) map[string]any {
	if r.Kind == "format" {
		return storageReplayFormat(r)
	}
	if r.Kind == "invert" || r.Kind == "abort_invert" {
		return storageReplayInvert(r)
	}
	if r.Kind == "codec" {
		return storageReplayCodec(r)
	}
	if r.Kind == "zst" {
		return storageReplayZST(r)
	}
	if r.Kind == "option" {
		return storageReplayOption(r)
	}
	h := &storageHarness{}
	ops := h.ops(r.CustomCloneFrom)
	idx := index.NewIdx(func(n uint) uint {
		h.event("new", n)
		if n > r.Limit {
			panic("index domain")
		}
		return n
	}, func(n uint) uint { h.event("index", n); return n })
	cfg := index.IndexStorageConfig{OverflowChecks: r.Checked, DebugAssertions: r.Debug}
	makeVec := func(values []int64, capacity uint) *index.IndexVec[uint, int64] {
		v := index.IndexVecWithCapacityAndConfig(capacity, idx, ops, cfg)
		for _, x := range values {
			v.Raw = appendStorageRaw(v.Raw, x, idx, ops, cfg)
		}
		return v
	}
	v, w := makeVec(r.Values, r.Capacity), makeVec(r.Other, r.OtherCapacity)
	replace := func(x *index.IndexVec[uint, int64]) { old := v; v = x; old.Close() }
	steps := make([]any, 0, len(r.Actions))
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
			bounds := func() index.IndexRangeBounds {
				return index.IndexRangeBounds{StartBound: func() index.IndexBound {
					h.event("start")
					return index.IndexBound{Kind: index.IndexBoundKind(a.StartKind), Value: a.A}
				}, EndBound: func() index.IndexBound {
					h.event("end")
					return index.IndexBound{Kind: index.IndexBoundKind(a.EndKind), Value: a.B}
				}}
			}
			switch a.Op {
			case "custom_get", "custom_get_mut", "custom_index", "custom_index_mut", "custom_copy":
				result = storageCustomSlice(h, v, a)
			case "outer_panic":
				old := v
				v = index.NewIndexVecWithConfig(idx, ops, cfg)
				defer old.Close()
				panic("outer oracle panic")
			case "len":
				result = []any{v.Len(), v.IsEmpty()}
			case "next_index":
				result = v.NextIndex()
			case "last_index":
				x, ok := v.LastIndex()
				if ok {
					result = x
				}
			case "get":
				x, ok := v.Get(a.A)
				if ok {
					result = *x
				}
			case "at":
				result = *v.At(a.A)
			case "write":
				*v.AtMut(a.A) = a.Value
			case "range":
				s, ok := v.GetIndex(index.RangeSliceIdx(idx).IntoSliceIdx(index.Range[uint]{Start: a.A, End: a.B}))
				if ok {
					xs := []int64{}
					for i := uint(0); i < s.Raw.Len(); i++ {
						xs = append(xs, *s.Raw.At(i))
					}
					result = xs
				}
			case "swap":
				v.Swap(a.A, a.B)
			case "copy":
				v.CopyWithin(index.RangeSliceIdx(idx).IntoSliceIdx(index.Range[uint]{Start: a.A, End: a.B}), a.C)
			case "pick2":
				x, y := v.Pick2Mut(a.A, a.B)
				result = []int64{*x, *y}
			case "pick3":
				x, y, z := v.Pick3Mut(a.A, a.B, a.C)
				result = []int64{*x, *y, *z}
			case "binary":
				x, ok := v.BinarySearch(a.Value)
				result = []any{x, ok}
			case "push":
				result = v.Push(a.Value)
			case "pop":
				x, ok := v.Pop()
				if ok {
					result = x
				}
			case "truncate":
				v.Truncate(a.N)
			case "resize":
				v.Resize(a.N, a.Value)
			case "ensure_owned":
				result = *v.EnsureContainsElemOwned(a.A, index.IndexFactory[int64]{Call: func() int64 { h.event("factory"); return a.Value }, CloseWithContext: func(c index.IndexDropContext) { h.event("factory_close", c.Unwinding) }})
			case "resize_elem_owned":
				v.ResizeToElemOwned(a.A, index.IndexFactory[int64]{Call: func() int64 { h.event("factory"); return a.Value }, CloseWithContext: func(c index.IndexDropContext) { h.event("factory_close", c.Unwinding) }})
			case "ensure":
				result = *v.EnsureContainsElem(a.A, func() int64 { h.event("factory"); return a.Value })
			case "resize_elem":
				v.ResizeToElem(a.A, func() int64 { h.event("factory"); return a.Value })
			case "append":
				v.Append(w)
			case "shrink":
				v.ShrinkToFit()
			case "close":
				old := v
				v = index.NewIndexVecWithConfig(idx, ops, cfg)
				old.Close()
			case "clone", "to_owned":
				var x *index.IndexVec[uint, int64]
				if a.Op == "to_owned" {
					x = v.AsSlice().ToOwned()
				} else {
					x = v.Clone()
				}
				result = storageValues(x)
				x.Close()
			case "clone_into":
				v.AsSlice().CloneInto(w)
			case "clone_from":
				w.CloneFrom(v)
			case "eq":
				result = v.Equal(w)
			case "ne":
				result = v.NotEqual(w)
			case "debug":
				result = v.Debug()
			case "map":
				result = v.DebugMapView(func(k uint) string { h.event("key_debug", k); return strconv.FormatUint(uint64(k), 10) }).Debug()
			case "hash":
				v.Hash(func(raw index.IndexRaw[int64]) {
					index.HashIndexRawRef(raw, struct{}{}, func(_ struct{}, n uint) { h.event("hash_len", n) }, func(_ struct{}, x *int64) { h.event("hash", *x) }, nil)
				})
			case "from_elem":
				x := index.IndexVecFromElemNWithConfig(a.Value, a.N, idx, ops, cfg)
				replace(x)
			case "from_fn_owned":
				x := index.IndexVecFromFnNOwnedWithConfig(index.IndexFnMut[uint, int64]{Call: func(k uint) int64 { h.event("factory", k); return int64(k) }, CloseWithContext: func(c index.IndexDropContext) { h.event("factory_close", c.Unwinding) }}, a.N, idx, ops, cfg)
				replace(x)
			case "from_fn":
				x := index.IndexVecFromFnNWithConfig(func(k uint) int64 { h.event("factory", k); return int64(k) }, a.N, idx, ops, cfg)
				replace(x)
			case "from_array":
				x := index.IndexVecFromArray(append([]int64{}, a.Values...), idx, ops)
				replace(x)
			case "from_iter", "from_iter_no_hint", "from_iter_trusted", "from_iter_trusted_each", "from_iter_trusted_fold", "extend_iter", "extend_iter_no_hint", "extend_iter_trusted", "extend_iter_trusted_each", "extend_iter_trusted_fold":
				position := 0
				it := index.IndexIntoIterator[int64]{IntoIter: func() index.IndexValueIterator[int64] {
					h.event("into_iter")
					iterator := index.IndexValueIterator[int64]{Next: func() (int64, bool) {
						h.event("next")
						if position >= len(a.Values) {
							return 0, false
						}
						x := a.Values[position]
						position++
						return x, true
					}, SizeHint: func() (uint, uint, bool) { h.event("hint"); n := uint(len(a.Values) - position); return n, n, true }, CloseWithContext: func(context index.IndexDropContext) {
						defer func() {
							if p := recover(); p != nil {
								storageDropRemaining(a.Values, &position, ops, true)
								panic(p)
							}
						}()
						h.event("iterator_close")
						storageDropRemaining(a.Values, &position, ops, context.Unwinding)
					}, TrustedLength: strings.Contains(a.Op, "trusted")}
					if strings.HasSuffix(a.Op, "no_hint") {
						iterator.SizeHint = nil
					}
					loop := func(consume func(int64)) {
						for {
							value, ok := iterator.Next()
							if !ok {
								return
							}
							consume(value)
						}
					}
					if strings.HasSuffix(a.Op, "_each") {
						iterator.ForEachOverride = func(f func(int64)) { h.event("for_each"); loop(f) }
					}
					if strings.HasSuffix(a.Op, "_fold") {
						iterator.FoldUnitOverride = func(f func(int64)) { h.event("fold"); loop(f) }
					}
					return iterator
				}}
				if strings.HasPrefix(a.Op, "extend_") {
					v.Extend(it)
				} else {
					x := index.IndexVecFromIteratorWithConfig(it, idx, ops, cfg)
					replace(x)
				}
			case "from_iter_owned":
				it := v.IntoIter()
				defer it.Close()
				for i := uint(0); i < a.A; i++ {
					x, ok := it.Next()
					if ok {
						ops.Drop(&x)
					}
				}
				for i := uint(0); i < a.B; i++ {
					x, ok := it.NextBack()
					if ok {
						ops.Drop(&x)
					}
				}
				x := index.IndexVecFromIteratorWithConfig(it.IntoIterator(), idx, ops, cfg)
				v = x
			case "iter", "iter_mut", "iter_enum", "iter_enum_mut", "indices", "owned", "owned_enum", "drain", "drain_enum", "drain_owned", "drain_enum_owned":
				b := bounds()
				if strings.HasSuffix(a.Op, "_owned") {
					b.CloseWithContext = func(c index.IndexDropContext) { h.event("bounds_close", c.Unwinding) }
					a.Op = strings.TrimSuffix(a.Op, "_owned")
				}
				result = storageIter(h, v, a, b)
			default:
				panic("unknown oracle op " + a.Op)
			}
		}()
		steps = append(steps, map[string]any{"value": result, "panic": panicked, "events": append([]string{}, h.events...), "v": storageValues(v), "w": storageValues(w), "capacity": v.Raw.Cap(), "other_capacity": w.Raw.Cap()})
	}
	return map[string]any{"steps": steps}
}

func storageDropRemaining(values []int64, position *int, ops index.ValueOps[int64], unwinding bool) {
	for *position < len(values) {
		value := values[*position]
		*position++
		func() {
			defer func() {
				if p := recover(); p != nil {
					storageDropRemaining(values, position, ops, true)
					panic(p)
				}
			}()
			ops.DropWithContext(&value, index.IndexDropContext{Unwinding: unwinding})
		}()
	}
}

// Initial raw values are moved without invoking typed-key validation.
func appendStorageRaw(raw index.IndexRaw[int64], x int64, idx index.Idx[uint], ops index.ValueOps[int64], cfg index.IndexStorageConfig) index.IndexRaw[int64] {
	// FromRaw retains spare capacity and represents the original Vec input.
	data := raw.AsGoSlice()
	data = append(data, x)
	return index.IndexVecFromRawWithConfig(data, idx, ops, cfg).Raw
}

func storageIter(h *storageHarness, v *index.IndexVec[uint, int64], a storageAction, bounds index.IndexRangeBounds) []any {
	var next, back, last func() any
	var nth, nthBack func(uint) any
	var size, count func() uint
	var hint func() (uint, uint)
	closeFn := func() {}
	ref := func(p *int64, ok bool) any {
		if !ok {
			return nil
		}
		return *p
	}
	mut := func(p *int64, ok bool) any {
		if !ok {
			return nil
		}
		*p += 1
		return *p
	}
	pair := func(k uint, p *int64, ok bool) any {
		if !ok {
			return nil
		}
		return []any{k, *p}
	}
	pairMut := func(k uint, p *int64, ok bool) any {
		if !ok {
			return nil
		}
		*p += 1
		return []any{k, *p}
	}
	key := func(k uint, ok bool) any {
		if !ok {
			return nil
		}
		return k
	}
	owned := func(x int64, ok bool) any {
		if !ok {
			return nil
		}
		defer v.Ops.Drop(&x)
		return x
	}
	ownedPair := func(k uint, x int64, ok bool) any {
		if !ok {
			return nil
		}
		defer v.Ops.Drop(&x)
		return []any{k, x}
	}
	switch a.Op {
	case "iter":
		it := v.Iter()
		next = func() any { return ref(it.Next()) }
		nth = func(n uint) any { return ref(it.Nth(n)) }
		last = func() any { return ref(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return ref(it.NextBack()) }
		nthBack = func(n uint) any { return ref(it.NthBack(n)) }
	case "iter_mut":
		it := v.IterMut()
		next = func() any { return mut(it.Next()) }
		nth = func(n uint) any { return mut(it.Nth(n)) }
		last = func() any { return mut(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return mut(it.NextBack()) }
		nthBack = func(n uint) any { return mut(it.NthBack(n)) }
	case "iter_enum":
		it := v.IterEnumerated()
		next = func() any { return pair(it.Next()) }
		nth = func(n uint) any { return pair(it.Nth(n)) }
		last = func() any { return pair(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return pair(it.NextBack()) }
		nthBack = func(n uint) any { return pair(it.NthBack(n)) }
	case "iter_enum_mut":
		it := v.IterEnumeratedMut()
		next = func() any { return pairMut(it.Next()) }
		nth = func(n uint) any { return pairMut(it.Nth(n)) }
		last = func() any { return pairMut(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return pairMut(it.NextBack()) }
		nthBack = func(n uint) any { return pairMut(it.NthBack(n)) }
	case "indices":
		it := v.Indices()
		next = func() any { return key(it.Next()) }
		nth = func(n uint) any { return key(it.Nth(n)) }
		last = func() any { return key(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return key(it.NextBack()) }
		nthBack = func(n uint) any { return key(it.NthBack(n)) }
	case "owned":
		it := v.IntoIter()
		next = func() any { return owned(it.Next()) }
		nth = func(n uint) any { return owned(it.Nth(n)) }
		last = func() any { return owned(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return owned(it.NextBack()) }
		nthBack = func(n uint) any { return owned(it.NthBack(n)) }
		closeFn = it.Close
	case "owned_enum":
		it := v.IntoIterEnumerated()
		next = func() any { return ownedPair(it.Next()) }
		nth = func(n uint) any { return ownedPair(it.Nth(n)) }
		last = func() any { return ownedPair(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		back = func() any { return ownedPair(it.NextBack()) }
		nthBack = func(n uint) any { return ownedPair(it.NthBack(n)) }
		closeFn = it.Close
	case "drain":
		it := v.Drain(bounds)
		next = func() any { return owned(it.Next()) }
		nth = func(n uint) any { return owned(it.Nth(n)) }
		last = func() any { return owned(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		closeFn = it.Close
	case "drain_enum":
		it := v.DrainEnumerated(bounds)
		next = func() any { return ownedPair(it.Next()) }
		nth = func(n uint) any { return ownedPair(it.Nth(n)) }
		last = func() any { return ownedPair(it.Last()) }
		size = it.Len
		count = it.Count
		hint = it.SizeHint
		closeFn = it.Close

	}
	defer closeFn()
	result := []any{}
	capture := func(fn func() any) (out any) {
		var value any
		panicked := false
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			value = fn()
		}()
		return map[string]any{"value": value, "panic": panicked}
	}
	for _, command := range a.Commands {
		parse := func(prefix string) (uint, bool) {
			if !strings.HasPrefix(command, prefix) {
				return 0, false
			}
			n, err := strconv.ParseUint(strings.TrimPrefix(command, prefix), 10, 64)
			if err != nil {
				panic(err)
			}
			return uint(n), true
		}
		if n, ok := parse("catch_nth:"); ok {
			result = append(result, capture(func() any { return nth(n) }))
			continue
		}
		if n, ok := parse("catch_nth_back:"); ok {
			if nthBack != nil {
				result = append(result, capture(func() any { return nthBack(n) }))
			}
			continue
		}
		if n, ok := parse("nth:"); ok {
			result = append(result, nth(n))
			continue
		}
		if n, ok := parse("nth_back:"); ok {
			if nthBack != nil {
				result = append(result, nthBack(n))
			}
			continue
		}
		switch command {
		case "next":
			result = append(result, next())
		case "back":
			if back != nil {
				result = append(result, back())
			}
		case "len":
			result = append(result, size())
		case "hint":
			x, y := hint()
			result = append(result, []any{x, y})
		case "count":
			result = append(result, count())
			return result
		case "last":
			result = append(result, last())
			return result
		case "close":
			closeFn()
			return result
		}
	}
	return result
}

func storageJSON(v any) any {
	data, _ := json.Marshal(v)
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var out any
	if err := d.Decode(&out); err != nil {
		panic(err)
	}
	return out
}

func TestOriginalStorageOracle(t *testing.T) {
	storageVerifyCorpus(t)
	path := filepath.Join(storageRoot(), "tools/port-rust-index-storage/oracle/cases.jsonl.gz")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Fatal("committed storage oracle corpus is missing")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	scan := bufio.NewScanner(gz)
	scan.Buffer(make([]byte, 65536), 4<<20)
	count := 0
	for scan.Scan() {
		var row storageRecord
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if row.Request.Kind == "codec" && !storageCodecEnabled {
			continue
		}
		count++
		var actual any
		if strings.HasPrefix(row.Request.Kind, "abort") {
			actual = storageReplayAbort(t, row.Request)
		} else {
			actual = storageReplay(row.Request)
		}
		got := storageJSON(actual)
		var want any
		d := json.NewDecoder(bytes.NewReader(row.Response))
		d.UseNumber()
		if err := d.Decode(&want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			g, _ := json.Marshal(got)
			t.Fatalf("case %d %s checked=%v debug=%v\nrequest=%s\nGo=%s\nRust=%s", count, row.Request.Name, row.Request.Checked, row.Request.Debug, mustStorageJSON(row.Request), g, row.Response)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no storage oracle cases")
	}
	t.Logf("compared %d complete original-source storage requests", count)
}
func mustStorageJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(data))
}
