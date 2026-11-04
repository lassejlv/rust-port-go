// SPDX-License-Identifier: MIT OR Apache-2.0
// Development-only typed-index oracle protocol and Go execution harness.
package indexoracle

import (
	"fmt"
	"strconv"
	"strings"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type Request struct {
	Type                   string `json:"type"`
	Operation              string `json:"operation"`
	A, B, Amount, Limit    uint64
	Fail                   [5]uint `json:"fail"`
	ExhaustedStart         uint64  `json:"exhausted_start,omitempty"`
	ExhaustedEnd           uint64  `json:"exhausted_end,omitempty"`
	ExhaustedStateCaptured bool    `json:"exhausted_state_captured,omitempty"`
}

// CaptureRangeState records the actual standard-library iterator endpoints
// before the original IntoSliceIdx call. Range iteration is outside idx.rs;
// the Go port consumes this captured input state rather than emulating it.
func CaptureRangeState(r *Request, response string) error {
	if r.Operation != "exhausted" {
		return nil
	}
	fields := strings.Split(strings.SplitN(response, "\t", 2)[0], ":")
	if len(fields) != 7 || fields[0] != "exhausted" || fields[1] != "true" {
		return fmt.Errorf("invalid exhausted range response: %s", response)
	}
	var err error
	r.ExhaustedStart, err = strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return err
	}
	r.ExhaustedEnd, err = strconv.ParseUint(fields[3], 10, 64)
	if err != nil {
		return err
	}
	r.ExhaustedStateCaptured = true
	return nil
}

type Record struct {
	Request        Request `json:"request"`
	Nightly        bool    `json:"nightly"`
	OverflowChecks bool    `json:"overflow_checks"`
	Response       string  `json:"response"`
}

func (r Request) Protocol() string {
	return fmt.Sprintf("%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n", r.Type, r.Operation, r.A, r.B, r.Amount, r.Limit, r.Fail[0], r.Fail[1], r.Fail[2], r.Fail[3], r.Fail[4])
}

func Requests(nightly bool) []Request {
	operations := []string{"new", "index", "plus", "increment", "single", "full", "range", "from", "to", "inclusive", "to_inclusive", "custom"}
	if nightly {
		operations = append(operations, "core_range", "core_from", "core_inclusive", "core_to_inclusive")
	}
	values := []uint64{0, 1, 2, 15, 31, 32, (1 << 32) - 1, 1 << 32, (1 << 32) + 1, (1 << 63) - 1, 1 << 63, ^uint64(0) - 1, ^uint64(0)}
	var requests []Request
	for _, kind := range []string{"usize", "u32", "default", "plus", "increment", "both"} {
		for _, operation := range operations {
			for i, a := range values {
				for _, amount := range []uint64{0, 1, 2, 8, (1 << 32) - 1, ^uint64(0)} {
					request := Request{Type: kind, Operation: operation, A: a, B: values[(i+5)%len(values)], Amount: amount, Limit: ^uint64(0)}
					requests = append(requests, request)
				}
			}
			for failKind := 0; failKind < 5; failKind++ {
				for at := uint(1); at <= 3; at++ {
					request := Request{Type: kind, Operation: operation, A: 2, B: 5, Amount: 3, Limit: 31}
					request.Fail[failKind] = at
					requests = append(requests, request)
				}
			}
			for _, limit := range []uint64{0, 2, 4, 31} {
				requests = append(requests, Request{Type: kind, Operation: operation, A: 4, B: 8, Amount: 7, Limit: limit})
			}
		}
	}
	for _, a := range values {
		requests = append(requests, Request{Type: "usize", Operation: "exhausted", A: a, Limit: ^uint64(0)})
	}
	return requests
}

func Execute(r Request, checked bool) string {
	config := index.IdxConfig{OverflowChecks: checked}
	if r.Type == "usize" {
		var trace []string
		return executeWith(index.UsizeIdxWithConfig(config), func(v uint) uint { return v }, func(v uint) uint { return v }, r, func(k uint) bool { trace = append(trace, fmt.Sprintf("c:%d", k)); return r.Fail[4] == 1 }, &trace)
	}
	if r.Type == "u32" {
		var trace []string
		return executeWith(index.U32IdxWithConfig(config), func(v uint) uint32 { return uint32(v) }, func(v uint32) uint { return uint(v) }, r, func(k uint32) bool { trace = append(trace, fmt.Sprintf("c:%d", k)); return r.Fail[4] == 1 }, &trace)
	}
	type key struct {
		value    uint
		metadata []byte
	}
	metadata := []byte{1, 2}
	makeKey := func(v uint) key { return key{v, metadata} }
	var trace []string
	var calls [5]uint
	event := func(kind int, a, b uint) bool {
		calls[kind]++
		name := []string{"n", "i", "p", "m", "c"}[kind]
		if kind < 2 || kind == 4 {
			trace = append(trace, fmt.Sprintf("%s:%d", name, a))
		} else {
			trace = append(trace, fmt.Sprintf("%s:%d:%d", name, a, b))
		}
		return r.Fail[kind] != 0 && calls[kind] == r.Fail[kind] || kind == 0 && uint64(a) > r.Limit
	}
	adapter := index.NewIdxWithConfig(func(v uint) key {
		if event(0, v, 0) {
			panic("new")
		}
		return makeKey(v)
	}, func(k key) uint {
		if event(1, k.value, 0) {
			panic("index")
		}
		return k.value
	}, config)
	if r.Type == "plus" || r.Type == "both" {
		adapter.PlusOverride = func(k key, n uint) key {
			if event(2, k.value, n) {
				panic("plus")
			}
			return adapter.New(adapter.Index(k) + n + 7)
		}
	}
	if r.Type == "increment" || r.Type == "both" {
		adapter.IncrementByOverride = func(k *key, n uint) {
			fail := event(3, k.value, n)
			*k = makeKey(k.value + 11)
			if fail {
				panic("increment")
			}
			*k = adapter.Plus(*k, n)
		}
	}
	return executeWith(adapter, makeKey, func(k key) uint { return k.value }, r, func(k key) bool { return event(4, k.value, 0) }, &trace)
}

func executeWith[K any](adapter index.Idx[K], makeKey func(uint) K, raw func(K) uint, r Request, customEvent func(K) bool, traces ...*[]string) string {
	key := makeKey(uint(r.A))
	out := ""
	failure := panicked(func() {
		switch r.Operation {
		case "new":
			out = fmt.Sprintf("key:%d", raw(adapter.New(uint(r.A))))
		case "index":
			out = fmt.Sprintf("index:%d", adapter.Index(key))
		case "plus":
			out = fmt.Sprintf("key:%d", raw(adapter.Plus(key, uint(r.Amount))))
		case "increment":
			adapter.IncrementBy(&key, uint(r.Amount))
			out = "unit"
		case "single":
			out = format(index.SingleSliceIdx(adapter).IntoSliceIdx(key))
		case "full":
			out = format(index.FullSliceIdx().IntoSliceIdx(index.RangeFull{}))
		case "range":
			out = format(index.RangeSliceIdx(adapter).IntoSliceIdx(index.Range[K]{Start: key, End: makeKey(uint(r.B))}))
		case "from":
			out = format(index.RangeFromSliceIdx(adapter).IntoSliceIdx(index.RangeFrom[K]{Start: key}))
		case "to":
			out = format(index.RangeToSliceIdx(adapter).IntoSliceIdx(index.RangeTo[K]{End: makeKey(uint(r.B))}))
		case "inclusive":
			out = format(index.RangeInclusiveSliceIdx(adapter).IntoSliceIdx(index.RangeInclusive[K]{Start: key, End: makeKey(uint(r.B))}))
		case "to_inclusive":
			out = format(index.RangeToInclusiveSliceIdx(adapter).IntoSliceIdx(index.RangeToInclusive[K]{End: makeKey(uint(r.B))}))
		case "exhausted":
			if !r.ExhaustedStateCaptured {
				panic("exhausted iterator state was not captured")
			}
			start, end := uint(r.ExhaustedStart), uint(r.ExhaustedEnd)
			result := index.RangeInclusiveSliceIdx(index.UsizeIdx()).IntoSliceIdx(index.RangeInclusive[uint]{Start: start, End: end, Exhausted: true})
			out = fmt.Sprintf("exhausted:true:%d:%d:%d:%d:%t", start, end, result.Start, result.End, result.Exhausted || result.Start > result.End)
		case "custom":
			custom := index.IntoSliceIdx[K, index.TypedSliceIndexResult]{Convert: func(k K) index.TypedSliceIndexResult {
				if customEvent != nil && customEvent(k) {
					panic("convert")
				}
				return index.TypedSliceIndexResult{Kind: index.SliceIndexTo, End: adapter.Index(k) + 9}
			}}
			out = fmt.Sprintf("custom_to:%d", custom.IntoSliceIdx(key).End)
		default:
			out = format(nightlyConvert(adapter, key, makeKey(uint(r.B)), r.Operation))
		}
	})
	if failure {
		out = "panic"
	}
	trace := ""
	if len(traces) != 0 {
		trace = strings.Join(*traces[0], ",")
	}
	return fmt.Sprintf("%s\tkey=%d\ttrace=%s", out, raw(key), trace)
}

func format(r index.TypedSliceIndexResult) string {
	switch r.Kind {
	case index.SliceIndexSingle:
		return fmt.Sprintf("single:%d", r.Index)
	case index.SliceIndexFull:
		return "full"
	case index.SliceIndexRange:
		return fmt.Sprintf("range:%d:%d", r.Start, r.End)
	case index.SliceIndexFrom:
		return fmt.Sprintf("from:%d", r.Start)
	case index.SliceIndexTo:
		return fmt.Sprintf("to:%d", r.End)
	case index.SliceIndexInclusive:
		return fmt.Sprintf("inclusive:%d:%d:%t", r.Start, r.End, r.Exhausted || r.Start > r.End)
	case index.SliceIndexToInclusive:
		return fmt.Sprintf("to_inclusive:%d", r.End)
	case index.SliceIndexCoreRange:
		return fmt.Sprintf("core_range:%d:%d", r.Start, r.End)
	case index.SliceIndexCoreFrom:
		return fmt.Sprintf("core_from:%d", r.Start)
	case index.SliceIndexCoreInclusive:
		return fmt.Sprintf("core_inclusive:%d:%d", r.Start, r.Last)
	case index.SliceIndexCoreToInclusive:
		return fmt.Sprintf("core_to_inclusive:%d", r.Last)
	}
	panic("unknown output kind")
}
func panicked(f func()) (result bool) {
	defer func() {
		if recover() != nil {
			result = true
		}
	}()
	f()
	return false
}
