package contract

import (
	"fmt"
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type CollectorRequest struct {
	Kind         string   `json:"kind"`
	Values       []uint64 `json:"values"`
	Hint         string   `json:"hint"`
	Lower        uint64   `json:"lower"`
	Upper        *uint64  `json:"upper"`
	PanicNext    *uint64  `json:"panic_next"`
	PanicHint    *uint64  `json:"panic_hint"`
	PanicInto    bool     `json:"panic_into"`
	DropInput    bool     `json:"drop_input"`
	DropIterator bool     `json:"drop_iterator"`
	Context      string   `json:"context"`
	Capacity     uint     `json:"capacity"`
	Front        uint     `json:"front"`
	Back         uint     `json:"back"`
	Debug        bool     `json:"debug"`
	PanicMap     *uint64  `json:"panic_map"`
}

func ObserveCollector(q CollectorRequest, live func(string)) map[string]any {
	events := []string{}
	event := func(s string) {
		events = append(events, s)
		if live != nil {
			live(s)
		}
	}
	context := rustvec.Context{Unwinding: q.Context == "inherited", ProcessingPanic: q.Context == "processing"}
	result := func() (out map[string]any) {
		defer func() {
			if p := recover(); p != nil {
				out = map[string]any{"panic": true, "message": fmt.Sprint(p)}
			}
		}()
		cfg := semver.DefaultCollectorConfig()
		cfg.Context = context
		cfg.DebugAssertions = q.Debug
		var source rustvec.IntoIterator[semver.Comparator]
		var original *semver.Comparator
		makeValue := func(n uint64) semver.Comparator { return semver.Comparator{Op: semver.Exact, Major: n} }
		if q.Kind == "vec" || q.Kind == "map" {
			values := make([]semver.Comparator, 0, int(q.Capacity))
			for _, n := range q.Values {
				if len(values) == cap(values) {
					size := max(4, 2*cap(values))
					grown := make([]semver.Comparator, len(values), size)
					copy(grown, values)
					values = grown
				}
				values = append(values, makeValue(n))
			}
			if cap(values) > 0 {
				original = &values[:cap(values)][0]
			}
			it := rustvec.NewVecIntoIter(values, rustvec.ElementOps[semver.Comparator]{})
			for i := uint(0); i < q.Front; i++ {
				it.Next()
			}
			for i := uint(0); i < q.Back; i++ {
				it.NextBack()
			}
			if q.Kind == "vec" {
				source = it.IntoIterator()
			} else {
				type mapped struct {
					values      []semver.Comparator
					front, back int
				}
				state := mapped{values: values, front: int(q.Front), back: len(values) - int(q.Back)}
				apply := func(v semver.Comparator, context rustvec.Context) semver.Comparator {
					event(fmt.Sprintf("map:%d:panic=%t", v.Major, context.Panicking()))
					if q.PanicMap != nil && *q.PanicMap == v.Major {
						context.Panic("map")
					}
					v.Major += 10
					return v
				}
				ops := rustvec.IteratorOps[mapped, semver.Comparator]{Next: func(ref rustvec.Ref[mapped]) (semver.Comparator, bool) {
					s := ref.Value
					if s.front == s.back {
						return semver.Comparator{}, false
					}
					value := s.values[s.front]
					s.front++
					return apply(value, ref.Context), true
				}, Drop: func(s *mapped, context rustvec.Context) {
					event(fmt.Sprintf("mapper-drop:panic=%t", context.Panicking()))
					if q.DropIterator {
						context.Panic("mapper drop")
					}
					clear(s.values)
				}, VecFromIterator: func(arg *rustvec.Owned[mapped], _ rustvec.Config, _ rustvec.ElementOps[semver.Comparator]) []semver.Comparator {
					s := arg.Borrow()
					written := 0
					for read := s.front; read < s.back; read++ {
						v := apply(s.values[read], arg.Context())
						s.values[written] = v
						written++
					}
					clear(s.values[written:])
					result := s.values[:written]
					s.values = nil
					return result
				}}
				source = rustvec.FromIterator(rustvec.NewIterator(state, ops))
			}
		} else {
			type state struct {
				position uint64
				hints    uint64
			}
			ops := rustvec.IteratorOps[state, semver.Comparator]{TrustedLength: q.Kind != "generic", Next: func(ref rustvec.Ref[state]) (semver.Comparator, bool) {
				s := ref.Value
				event(fmt.Sprintf("next:%d:panic=%t", s.position, ref.Context.Panicking()))
				if q.PanicNext != nil && *q.PanicNext == s.position {
					ref.Context.Panic("next")
				}
				if int(s.position) == len(q.Values) {
					return semver.Comparator{}, false
				}
				v := makeValue(q.Values[s.position])
				s.position++
				return v, true
			}, SizeHint: func(ref rustvec.Ref[state]) rustvec.Hint {
				s := ref.Value
				h := s.hints
				s.hints++
				event(fmt.Sprintf("hint:%d:%d:panic=%t", h, s.position, ref.Context.Panicking()))
				if q.PanicHint != nil && *q.PanicHint == h {
					ref.Context.Panic("hint")
				}
				if q.Hint == "remaining" {
					n := uint(uint64(len(q.Values)) - s.position)
					return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
				}
				hint := rustvec.Hint{Lower: uint(q.Lower)}
				if q.Upper != nil {
					hint.Upper = uint(*q.Upper)
					hint.HasUpper = true
				}
				return hint
			}, Drop: func(s *state, context rustvec.Context) {
				event(fmt.Sprintf("iterator-drop:%d:panic=%t", s.position, context.Panicking()))
				if q.DropIterator {
					context.Panic("iterator drop")
				}
			}}
			body := func(arg *rustvec.Owned[state], consume func(semver.Comparator)) {
				for {
					value, ok := ops.Next(rustvec.Ref[state]{Value: arg.Borrow(), Context: arg.Context()})
					if !ok {
						return
					}
					consume(value)
				}
			}
			if q.Kind == "each" {
				ops.ForEach = func(arg *rustvec.Owned[state], consume func(semver.Comparator)) {
					event(fmt.Sprintf("for-each:panic=%t", arg.Context().Panicking()))
					body(arg, consume)
				}
			}
			if q.Kind == "fold" {
				ops.FoldUnit = func(arg *rustvec.Owned[state], consume func(semver.Comparator)) {
					event(fmt.Sprintf("fold:panic=%t", arg.Context().Panicking()))
					body(arg, consume)
				}
			}
			source = rustvec.NewIntoIterator(struct{}{}, rustvec.IntoIteratorOps[struct{}, semver.Comparator]{IntoIter: func(arg *rustvec.Owned[struct{}]) rustvec.Iterator[semver.Comparator] {
				event(fmt.Sprintf("into-iter:panic=%t", arg.Context().Panicking()))
				if q.PanicInto {
					arg.Context().Panic("into_iter")
				}
				return rustvec.NewIterator(state{}, ops)
			}, Drop: func(_ *struct{}, context rustvec.Context) {
				event(fmt.Sprintf("input-drop:panic=%t", context.Panicking()))
				if q.DropInput {
					context.Panic("input drop")
				}
			}})
		}
		req := semver.FromOwnedIterator(source, cfg)
		values := []uint64{}
		for _, v := range req.Comparators {
			values = append(values, v.Major)
		}
		var reused any
		if q.Kind == "vec" || q.Kind == "map" {
			var pointer *semver.Comparator
			if cap(req.Comparators) > 0 {
				pointer = &req.Comparators[:cap(req.Comparators)][0]
			}
			reused = pointer == original
		}
		return map[string]any{"panic": false, "values": values, "capacity": cap(req.Comparators), "reused": reused}
	}()
	result["events"] = events
	return result
}
func CollectorCorpus() []CollectorRequest {
	result := []CollectorRequest{}
	ptr := func(v uint64) *uint64 { return &v }
	for _, context := range []string{"normal", "inherited", "processing"} {
		for _, kind := range []string{"generic", "trusted", "each", "fold"} {
			for _, n := range []int{0, 1, 2, 4, 5, 8, 9, 17, 33} {
				q := CollectorRequest{Kind: kind, Values: []uint64{}, Hint: "remaining", Context: context, Debug: true}
				for i := 0; i < n; i++ {
					q.Values = append(q.Values, uint64(i))
				}
				result = append(result, q)
				if kind == "generic" {
					zero := q
					zero.Hint = "zero"
					result = append(result, zero)
				}
				if context != "processing" {
					for _, plan := range []string{"into", "input-drop", "iterator-drop", "next-first", "next-late", "hint-first", "hint-second"} {
						p := q
						switch plan {
						case "into":
							p.PanicInto = true
						case "input-drop":
							p.DropInput = true
						case "iterator-drop":
							p.DropIterator = true
						case "next-first":
							p.PanicNext = ptr(0)
						case "next-late":
							p.PanicNext = ptr(uint64(n))
						case "hint-first":
							p.PanicHint = ptr(0)
						case "hint-second":
							p.PanicHint = ptr(1)
						}
						result = append(result, p)
					}
				}
			}
		}
	}
	for n := 0; n < 17; n++ {
		for _, capacity := range []uint{0, 1, 4, 8, 16, 32} {
			for front := 0; front <= n; front++ {
				q := CollectorRequest{Kind: "vec", Values: []uint64{}, Context: "normal", Capacity: capacity, Front: uint(front), Debug: true}
				for i := 0; i < n; i++ {
					q.Values = append(q.Values, uint64(i))
				}
				result = append(result, q)
				if front < n {
					q.Back = 1
					result = append(result, q)
				}
			}
		}
	}
	for _, lower := range []uint64{0, 1, 3, 4, 7, 8, 18446744073709551615} {
		q := CollectorRequest{Kind: "generic", Values: []uint64{1, 2, 3}, Hint: "fixed", Lower: lower, Context: "normal", Debug: true}
		result = append(result, q)
	}
	var mapped []CollectorRequest
	for _, q := range result {
		if q.Kind == "vec" {
			q.Kind = "map"
			mapped = append(mapped, q)
		}
	}
	result = append(result, mapped...)
	for _, context := range []string{"normal", "inherited", "processing"} {
		for _, n := range []int{0, 1, 4, 5} {
			q := CollectorRequest{Kind: "map", Values: []uint64{}, Capacity: 8, Context: context, Debug: true}
			for i := 0; i < n; i++ {
				q.Values = append(q.Values, uint64(i))
			}
			result = append(result, q)
			if context != "processing" {
				q.DropIterator = true
				result = append(result, q)
				q.DropIterator = false
				q.PanicMap = ptr(0)
				result = append(result, q)
			}
		}
	}
	return result
}

func CollectorAbortCorpus() []CollectorRequest {
	result := []CollectorRequest{}
	p := func(v uint64) *uint64 { return &v }
	for _, context := range []string{"normal", "inherited"} {
		result = append(result,
			CollectorRequest{Kind: "generic", Values: []uint64{1, 2}, Hint: "remaining", Context: context, Debug: true, PanicNext: p(0), DropIterator: true},
			CollectorRequest{Kind: "trusted", Values: []uint64{1, 2}, Hint: "remaining", Context: context, Debug: true, PanicHint: p(0), DropIterator: true},
			CollectorRequest{Kind: "generic", Values: []uint64{1, 2}, Hint: "remaining", Context: context, Debug: true, PanicInto: true, DropInput: true},
			CollectorRequest{Kind: "map", Values: []uint64{1, 2}, Capacity: 8, Context: context, Debug: true, PanicMap: p(1), DropIterator: true})
	}
	for _, kind := range []string{"generic", "trusted", "each", "fold"} {
		for _, fault := range []string{"into", "input", "iterator", "next", "hint"} {
			q := CollectorRequest{Kind: kind, Values: []uint64{1, 2}, Hint: "remaining", Context: "processing", Debug: true}
			switch fault {
			case "into":
				q.PanicInto = true
			case "input":
				q.DropInput = true
			case "iterator":
				q.DropIterator = true
			case "next":
				q.PanicNext = p(0)
			case "hint":
				q.PanicHint = p(0)
			}
			result = append(result, q)
		}
	}
	result = append(result, CollectorRequest{Kind: "map", Values: []uint64{1, 2}, Capacity: 8, Context: "processing", Debug: true, PanicMap: p(1)}, CollectorRequest{Kind: "map", Values: []uint64{1, 2}, Capacity: 8, Context: "processing", Debug: true, DropIterator: true})
	return result
}

func CollectorReferenceSize() uint { return semver.DefaultCollectorConfig().ElementSize }
