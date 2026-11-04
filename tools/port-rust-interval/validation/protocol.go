// Package intervaloracle defines the development-only original-source protocol.
package intervaloracle

import (
	"fmt"
	"strconv"
	"strings"
)

type Operation struct {
	Action     string `json:"action"`
	Target     uint8  `json:"target,omitempty"`
	A, B, C, D uint64
	Start, End uint8
	Range      string `json:"range,omitempty"`
	TagA, TagB uint8
}
type Request struct {
	Kind                                   string `json:"kind"`
	Domain, OtherDomain, Limit, CloneDelta uint64
	FailKind                               string `json:"fail_kind,omitempty"`
	FailAt                                 uint   `json:"fail_at,omitempty"`
	Setup, Operations                      []Operation
}
type Record struct {
	Request                         Request `json:"request"`
	OverflowChecks, DebugAssertions bool
	Response                        string `json:"response"`
}

func (op Operation) Protocol() string {
	return fmt.Sprintf("%s,%d,%d,%d,%d,%d,%d,%d,%s,%d,%d", op.Action, op.Target, op.A, op.B, op.C, op.D, op.Start, op.End, op.Range, op.TagA, op.TagB)
}
func operationsProtocol(operations []Operation) string {
	parts := make([]string, len(operations))
	for i, op := range operations {
		parts[i] = op.Protocol()
	}
	return strings.Join(parts, ";")
}
func (r Request) Protocol() string {
	return fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%s\t%d\t%s\t%s\n", r.Kind, r.Domain, r.OtherDomain, r.Limit, r.CloneDelta, r.FailKind, r.FailAt, operationsProtocol(r.Setup), operationsProtocol(r.Operations))
}
func boundRange(action string, a, b uint64, start, end uint8) Operation {
	return Operation{Action: action, A: a, B: b, Start: start, End: end, Range: "b"}
}
func seed() []Operation {
	return []Operation{{Action: "range", A: 1, B: 3, Range: "c"}, {Action: "range", A: 6, B: 8, Range: "c", Target: 1}}
}
func baseRequest(kind string) Request {
	return Request{Kind: kind, Domain: 32, OtherDomain: 32, Limit: 63}
}

// Requests cover every method, all bound kinds and generic callback failures,
// followed by deterministic stateful programs. They contain no expected answers.
func Requests() []Request {
	var out []Request
	for _, domain := range []uint64{0, 1, 8, 32} {
		for _, a := range []uint64{0, 1, 7, 8, 31} {
			for _, b := range []uint64{0, 1, 7, 8, 31} {
				for start := uint8(0); start < 3; start++ {
					for end := uint8(0); end < 3; end++ {
						for _, action := range []string{"range", "appendrange", "intersects", "first", "last"} {
							r := baseRequest("set")
							r.Domain = domain
							r.OtherDomain = domain
							r.Setup = seed()
							r.Operations = []Operation{boundRange(action, a, b, start, end)}
							out = append(out, r)
						}
					}
				}
			}
		}
	}
	edges := [][2]uint64{{0, 0}, {0, 1}, {1, 0}, {31, 32}, {(1 << 32) - 2, (1 << 32) - 1}, {(1 << 32) - 1, (1 << 32) - 1}, {1 << 32, (1 << 32) + 1}, {^uint64(0), 0}, {0, ^uint64(0)}}
	for _, domain := range []uint64{0, 32, (1 << 32) - 1, 1 << 32, ^uint64(0)} {
		for _, pair := range edges {
			for _, action := range []string{"range", "appendrange", "intersects", "first", "last", "contains", "insert", "append", "all"} {
				for start := uint8(0); start < 3; start++ {
					for end := uint8(0); end < 3; end++ {
						if action == "all" && (start != 0 || end != 0) {
							continue
						}
						r := baseRequest("set")
						r.Domain = domain
						r.OtherDomain = domain
						r.Setup = []Operation{{Action: "range", A: 0, B: 1, Range: "c"}}
						r.Operations = []Operation{boundRange(action, pair[0], pair[1], start, end)}
						out = append(out, r)
					}
				}
			}
		}
	}
	for _, kind := range []string{"h", "c", "f", "t", "i", "u"} {
		for _, action := range []string{"range", "appendrange", "intersects", "first", "last"} {
			for _, pair := range [][2]uint64{{0, 0}, {0, 7}, {7, 0}, {31, 32}, {^uint64(0), 0}} {
				r := baseRequest("set")
				r.Setup = seed()
				r.Operations = []Operation{{Action: action, A: pair[0], B: pair[1], Range: kind}}
				out = append(out, r)
			}
		}
	}
	for _, action := range []string{"clear", "empty", "clone", "debug", "all", "union", "superset", "disjoint", "intervals", "iter", "hint"} {
		for _, limit := range []uint64{0, 1, 3, 7, 63} {
			r := baseRequest("set")
			r.Setup = seed()
			r.Limit = limit
			r.Operations = []Operation{{Action: action, A: 16}}
			out = append(out, r)
		}
	}
	for _, pair := range [][2]uint64{{0, 1}, {1, 0}, {8, 32}, {32, 8}} {
		for _, action := range []string{"union", "superset", "disjoint"} {
			r := baseRequest("set")
			r.Domain = pair[0]
			r.OtherDomain = pair[1]
			r.Setup = seed()
			r.Operations = []Operation{{Action: action}}
			out = append(out, r)
		}
	}
	setActions := []string{"insert", "append", "range", "appendrange", "contains", "intersects", "first", "last", "union", "superset", "disjoint", "intervals", "iter", "hint", "debug", "clone"}
	for _, action := range setActions {
		for _, rangeKind := range []string{"b", "c"} {
			for _, fail := range []string{"cn", "ci", "cc", "crclone", "cstart", "cend", "clt", "cle", "cge", "cfo", "csb"} {
				for at := uint(1); at <= 3; at++ {
					r := baseRequest("set")
					r.Setup = seed()
					r.FailKind = fail
					r.FailAt = at
					r.Operations = []Operation{{Action: action, A: 2, B: 7, Range: rangeKind, Start: 1, End: 1}}
					out = append(out, r)
				}
			}
		}
	}
	for _, delta := range []uint64{1, 2, ^uint64(0)} {
		for _, action := range []string{"range", "appendrange", "intersects", "first", "last"} {
			r := baseRequest("set")
			r.Setup = seed()
			r.CloneDelta = delta
			r.Operations = []Operation{boundRange(action, 2, 7, 1, 1)}
			out = append(out, r)
		}
	}
	matrixActions := []string{"mrows", "menumerated", "mrow", "mclear", "munionrow", "munionrows", "mdisjoint", "mall", "mrange", "minsert", "mappend", "mappendrange", "mcontains", "mclone"}
	matrixSeed := []Operation{{Action: "mrange", A: 0, C: 1, D: 3, Range: "c"}, {Action: "mrange", A: 2, C: 6, D: 8, Range: "c"}, {Action: "range", A: 9, B: 10, Range: "c"}}
	for _, action := range matrixActions {
		for _, rows := range [][2]uint64{{0, 0}, {0, 2}, {2, 0}, {0, 4}, {4, 0}, {4, 4}, {^uint64(0), 0}, {0, ^uint64(0)}} {
			r := baseRequest("matrix")
			r.Setup = matrixSeed
			r.Operations = []Operation{{Action: action, A: rows[0], B: rows[1], C: 2, D: 7, Range: "b", Start: 1, End: 1}}
			out = append(out, r)
		}
		for _, fail := range []string{"rn", "ri", "req", "cn", "ci", "cc", "crclone", "cstart", "cend", "clt", "cle", "cge"} {
			for at := uint(1); at <= 3; at++ {
				r := baseRequest("matrix")
				r.Setup = matrixSeed
				r.FailKind = fail
				r.FailAt = at
				r.Operations = []Operation{{Action: action, A: 0, B: 4, C: 2, D: 7, Range: "b", Start: 1, End: 1}}
				out = append(out, r)
			}
		}
	}
	for _, limit := range []uint64{0, 1, 2, 3} {
		for _, action := range []string{"mrows", "menumerated", "munionrows", "mrange"} {
			r := baseRequest("matrix")
			r.Setup = matrixSeed
			r.Limit = limit
			r.Operations = []Operation{{Action: action, A: 0, B: 4, C: 2, D: 7, Range: "c"}}
			out = append(out, r)
		}
	}
	for _, domain := range []uint64{0, 1, 8, 32, (1 << 32) + 1} {
		r := baseRequest("matrix")
		r.Domain = domain
		r.Setup = matrixSeed
		r.Operations = []Operation{{Action: "munionrow", A: 4}}
		out = append(out, r)
	}
	// Distinct lawful keys can map to one index; equality is not index equality.
	r := baseRequest("matrix")
	r.Setup = matrixSeed
	r.Operations = []Operation{{Action: "munionrows", A: 0, B: 0, TagA: 1, TagB: 2}}
	out = append(out, r)
	state := uint64(0x9e3779b97f4a7c15)
	next := func() uint64 { state ^= state << 13; state ^= state >> 7; state ^= state << 17; return state }
	for sequence := 0; sequence < 384; sequence++ {
		kind := "set"
		actions := setActions
		if sequence%3 == 0 {
			kind = "matrix"
			actions = matrixActions
		}
		r := baseRequest(kind)
		for step := 0; step < 32; step++ {
			action := actions[next()%uint64(len(actions))]
			op := Operation{Action: action, Target: uint8(next() % 2), A: next() % 40, B: next() % 40, C: next() % 40, D: next() % 40, Range: "b", Start: uint8(next() % 3), End: uint8(next() % 3)}
			if kind == "matrix" {
				op.A %= 8
				op.B %= 8
			}
			if action == "iter" || action == "hint" || action == "intervals" {
				op.A = 12
			}
			r.Operations = append(r.Operations, op)
		}
		out = append(out, r)
	}
	return out
}

func parseUint(text string) uint64 {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		panic(err)
	}
	return value
}
