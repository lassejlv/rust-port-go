// SPDX-License-Identifier: MIT OR Apache-2.0
// Shared development-only protocol and Go executor for the pinned Rust oracle.
package unionfindoracle

import (
	"fmt"
	"strconv"
	"strings"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

type Operation struct {
	Kind string `json:"kind"`
	A    uint   `json:"a,omitempty"`
	B    uint   `json:"b,omitempty"`
}

type Request struct {
	NumKeys         uint        `json:"num_keys"`
	Limit           uint        `json:"limit"`
	PanicNew        uint        `json:"panic_new,omitempty"`
	PanicIndex      uint        `json:"panic_index,omitempty"`
	PanicEqual      uint        `json:"panic_equal,omitempty"`
	PanicNotEqual   uint        `json:"panic_not_equal,omitempty"`
	DefaultNotEqual bool        `json:"default_not_equal,omitempty"`
	Operations      []Operation `json:"operations"`
}

type Record struct {
	Request         Request `json:"request"`
	DebugAssertions bool    `json:"debug_assertions"`
	Response        string  `json:"response"`
}

func (request Request) Protocol() string {
	var operations []string
	for _, op := range request.Operations {
		operations = append(operations, fmt.Sprintf("%s:%d:%d", op.Kind, op.A, op.B))
	}
	return fmt.Sprintf("%d\t%d\t%d\t%d\t%d\t%d\t%t\t%s\n", request.NumKeys, request.Limit, request.PanicNew, request.PanicIndex, request.PanicEqual, request.PanicNotEqual, request.DefaultNotEqual, strings.Join(operations, ";"))
}

func Requests() []Request {
	var requests []Request
	for n := uint(0); n <= 6; n++ {
		for a := uint(0); a < n+2; a++ {
			for b := uint(0); b < n+2; b++ {
				requests = append(requests, Request{NumKeys: n, Limit: n, Operations: []Operation{
					{Kind: "U", A: a, B: b}, {Kind: "F", A: a}, {Kind: "F", A: b}, {Kind: "S"},
					{Kind: "T", A: a, B: b}, {Kind: "A", A: a}, {Kind: "F", A: a}, {Kind: "S"},
				}})
			}
		}
		requests = append(requests, Request{NumKeys: n + 1, Limit: n, Operations: []Operation{{Kind: "S"}}})
	}
	base := Request{NumKeys: 8, Limit: 8}
	for _, pair := range [][2]uint{{0, 1}, {2, 3}, {4, 5}, {6, 7}, {0, 2}, {4, 6}, {0, 4}} {
		base.Operations = append(base.Operations, Operation{Kind: "U", A: pair[0], B: pair[1]})
	}
	base.Operations = append(base.Operations, []Operation{{Kind: "F", A: 7}, {Kind: "S"}, {Kind: "T", A: 7, B: 3}, {Kind: "A", A: 7}, {Kind: "U", A: 7, B: 3}, {Kind: "L", A: 7}, {Kind: "S"}, {Kind: "L", A: 8}, {Kind: "S"}}...)
	requests = append(requests, base)
	for _, kind := range []string{"new", "index", "equal", "not_equal"} {
		for at := uint(1); at <= 120; at++ {
			request := base
			switch kind {
			case "new":
				request.PanicNew = at
			case "index":
				request.PanicIndex = at
			case "equal":
				request.PanicEqual = at
			case "not_equal":
				request.PanicNotEqual = at
			}
			requests = append(requests, request)
		}
	}
	seed := uint64(0x7d934a38ec125f60)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	for i := 0; i < 256; i++ {
		n := uint(next() % 49)
		request := Request{NumKeys: n, Limit: n}
		for j := 0; j < 48; j++ {
			kind := []string{"F", "U", "U", "U", "S", "T", "A"}[next()%7]
			request.Operations = append(request.Operations, Operation{Kind: kind, A: uint(next() % uint64(n+2)), B: uint(next() % uint64(n+2))})
		}
		requests = append(requests, request)
	}
	// Repeat every script with the actual Rust default PartialEq::ne.
	count := len(requests)
	for i := 0; i < count; i++ {
		request := requests[i]
		request.DefaultNotEqual = true
		requests = append(requests, request)
	}
	return requests
}

// Execute uses the actual Go port, recording the same public callback trace and
// private derived-debug table state as the original Rust oracle.
func Execute(request Request, debug bool) string {
	type key struct {
		index    uint
		metadata []byte
	}
	metadata := []byte{1, 2}
	makeKey := func(i uint) key { return key{index: i, metadata: metadata} }
	var trace []string
	var callsNew, callsIndex, callsEqual, callsNotEqual uint
	limit := request.Limit
	index := datastructures.UnionFindIndex[key]{
		FromIndex: func(i uint) key {
			trace = append(trace, fmt.Sprintf("n:%d", i))
			callsNew++
			if i > limit || (request.PanicNew != 0 && callsNew == request.PanicNew) {
				panic("new")
			}
			return makeKey(i)
		},
		ToIndex: func(k key) uint {
			trace = append(trace, fmt.Sprintf("i:%d", k.index))
			callsIndex++
			if request.PanicIndex != 0 && callsIndex == request.PanicIndex {
				panic("index")
			}
			return k.index
		},
		Equal: func(a, b key) bool {
			trace = append(trace, fmt.Sprintf("eq:%d:%d", a.index, b.index))
			callsEqual++
			if request.PanicEqual != 0 && callsEqual == request.PanicEqual {
				panic("equal")
			}
			return a.index == b.index
		},
		NotEqual: func(a, b key) bool {
			trace = append(trace, fmt.Sprintf("ne:%d:%d", a.index, b.index))
			callsNotEqual++
			if request.PanicNotEqual != 0 && callsNotEqual == request.PanicNotEqual {
				panic("not equal")
			}
			return a.index != b.index
		},
	}
	if request.DefaultNotEqual {
		index.NotEqual = nil
	}
	var sets *datastructures.UnionFind[key]
	if panicked(func() {
		sets = datastructures.NewUnionFindWithConfig(request.NumKeys, index, datastructures.UnionFindConfig{DebugAssertions: debug})
	}) {
		return "new panic\ttrace=" + strings.Join(trace, ",")
	}
	formatKey := func(k key) string { return strconv.FormatUint(uint64(k.index), 10) }
	state := func() string { return sets.Debug(formatKey) }
	result := []string{"new ok " + state()}
	var saved *datastructures.UnionFindSnapshot[key]
	for _, op := range request.Operations {
		value := ""
		failure := panicked(func() {
			switch op.Kind {
			case "F":
				value = "find " + formatKey(sets.Find(makeKey(op.A)))
			case "U":
				value = "unify " + formatKey(sets.Unify(makeKey(op.A), makeKey(op.B)))
			case "S":
				snapshot := sets.Snapshot()
				saved = &snapshot
				values := make([]string, len(snapshot.Raw))
				for i, k := range snapshot.Raw {
					values[i] = formatKey(k)
				}
				value = "snapshot [" + strings.Join(values, ", ") + "]"
			case "A":
				value = "at " + formatKey(saved.At(makeKey(op.A)))
			case "T":
				saved.Set(makeKey(op.A), makeKey(op.B))
				value = "set"
			case "L":
				limit = op.A
				value = fmt.Sprintf("limit %d", limit)
			default:
				panic("unknown operation")
			}
		})
		if failure {
			value = "panic"
		}
		result = append(result, value+" "+state())
	}
	return strings.Join(result, "|") + "\ttrace=" + strings.Join(trace, ",")
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
