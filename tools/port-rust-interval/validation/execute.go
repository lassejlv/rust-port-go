package intervaloracle

import (
	"fmt"
	"strings"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type key struct {
	value    uint
	tag      uint8
	metadata []byte
}
type rowKey struct {
	value    uint
	tag      uint8
	metadata []byte
}
type execution struct {
	request Request
	trace   []string
	counts  map[string]uint
	enabled bool
	sets    [2]*index.IntervalSet[key]
	matrix  *index.SparseIntervalMatrix[rowKey, key]
	step    index.IntervalStep[key]
}

func (e *execution) event(kind string, values ...uint) {
	if !e.enabled {
		return
	}
	var out strings.Builder
	out.WriteString(kind)
	for _, value := range values {
		fmt.Fprintf(&out, ":%d", value)
	}
	e.trace = append(e.trace, out.String())
	e.counts[kind]++
	if kind == e.request.FailKind && e.counts[kind] == e.request.FailAt {
		panic("injected callback panic")
	}
}
func compare(a, b key) index.IntervalOrdering {
	if a.value < b.value || (a.value == b.value && a.tag < b.tag) {
		return index.IntervalLess
	}
	if a.value > b.value || (a.value == b.value && a.tag > b.tag) {
		return index.IntervalGreater
	}
	return index.IntervalEqual
}
func (e *execution) column(value uint64, tag uint8) key { return key{uint(value), tag, []byte{1, 2}} }
func (e *execution) row(value uint64, tag uint8) rowKey {
	return rowKey{uint(value), tag, []byte{1, 2}}
}
func (e *execution) makeStep() index.IntervalStep[key] {
	step := index.IntervalStep[key]{TypeName: "oracle::Key<1>",
		CloneValue: func(a key) key { e.event("cc", a.value, uint(a.tag)); return a },
		EqualFn: func(a, b key) bool {
			e.event("ceq", a.value, b.value, uint(a.tag), uint(b.tag))
			return a.value == b.value && a.tag == b.tag
		},
		PartialCompareFn: func(a, b key) index.IntervalOrdering {
			e.event("ccmp", a.value, b.value, uint(a.tag), uint(b.tag))
			return compare(a, b)
		},
		LessFn: func(a, b key) bool {
			e.event("clt", a.value, b.value, uint(a.tag), uint(b.tag))
			return compare(a, b) == index.IntervalLess
		},
		LessEqualFn: func(a, b key) bool {
			e.event("cle", a.value, b.value, uint(a.tag), uint(b.tag))
			v := compare(a, b)
			return v == index.IntervalLess || v == index.IntervalEqual
		},
		GreaterFn: func(a, b key) bool {
			e.event("cgt", a.value, b.value, uint(a.tag), uint(b.tag))
			return compare(a, b) == index.IntervalGreater
		},
		GreaterEqualFn: func(a, b key) bool {
			e.event("cge", a.value, b.value, uint(a.tag), uint(b.tag))
			v := compare(a, b)
			return v == index.IntervalGreater || v == index.IntervalEqual
		},
		StepsBetweenFn: func(a, b key) (uint, uint, bool) {
			e.event("csb", a.value, b.value)
			if a.value > b.value {
				return 0, 0, false
			}
			return b.value - a.value, b.value - a.value, true
		},
		ForwardCheckedFn: func(a key, n uint) (key, bool) {
			e.event("cfc", a.value, n)
			if uint64(n) > e.request.Limit || uint64(a.value) > e.request.Limit-uint64(n) {
				return key{}, false
			}
			a.value += n
			return a, true
		},
		BackwardCheckedFn: func(a key, n uint) (key, bool) {
			e.event("cbc", a.value, n)
			if n > a.value {
				return key{}, false
			}
			a.value -= n
			return a, true
		},
		ForwardOverflowingFn: func(a key, n uint) (key, bool) {
			e.event("cfo", a.value, n)
			if uint64(n) > e.request.Limit || uint64(a.value) > e.request.Limit-uint64(n) {
				a.value = 0
				return a, true
			}
			a.value += n
			return a, false
		},
		BackwardOverflowingFn: func(a key, n uint) (key, bool) {
			e.event("cbo", a.value, n)
			if n > a.value {
				a.value = 0
				return a, true
			}
			a.value -= n
			return a, false
		},
	}
	return step
}
func (e *execution) bounds(op Operation, matrix bool) index.IntervalRangeBounds[key] {
	a, b := op.A, op.B
	tagA, tagB := op.TagA, op.TagB
	if matrix {
		a, b = op.C, op.D
		tagA, tagB = 0, 0
	}
	start, end := e.column(a, tagA), e.column(b, tagB)
	switch op.Range {
	case "h":
		return index.IntervalHalfOpen(start, end, e.step.Clone)
	case "c":
		return index.IntervalClosed(start, end, e.step.Clone)
	case "f":
		return index.IntervalFrom(start, e.step.Clone)
	case "t":
		return index.IntervalTo(end, e.step.Clone)
	case "i":
		return index.IntervalToInclusive(end, e.step.Clone)
	case "u":
		return index.IntervalFull[key]()
	}
	var makeBounds func(key, key) index.IntervalRangeBounds[key]
	makeBounds = func(a, b key) index.IntervalRangeBounds[key] {
		return index.IntervalRangeBounds[key]{
			CloneFn: func() index.IntervalRangeBounds[key] {
				e.event("crclone")
				copy := a
				copy.value += uint(e.request.CloneDelta)
				return makeBounds(copy, b)
			},
			StartBoundFn: func() index.IntervalBound[key] {
				e.event("cstart")
				return index.IntervalBound[key]{Kind: index.IntervalBoundKind(op.Start), Value: a}
			},
			EndBoundFn: func() index.IntervalBound[key] {
				e.event("cend")
				return index.IntervalBound[key]{Kind: index.IntervalBoundKind(op.End), Value: b}
			},
		}
	}
	return makeBounds(start, end)
}
func catch(operation func() string) (result string) {
	defer func() {
		if recover() != nil {
			result = "panic"
		}
	}()
	return operation()
}
func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
func keyText(value key) string    { return fmt.Sprintf("%d:%d", value.value, value.tag) }
func rowText(value rowKey) string { return fmt.Sprintf("%d:%d", value.value, value.tag) }
func optionText(value key, ok bool) string {
	if !ok {
		return "-"
	}
	return keyText(value)
}
func hintText(lower, upper uint, hasUpper bool) string {
	if !hasUpper {
		return fmt.Sprintf("h(%d,-)", lower)
	}
	return fmt.Sprintf("h(%d,%d)", lower, upper)
}

func (e *execution) operation(op Operation) string {
	set := e.sets[op.Target]
	switch op.Action {
	case "clear":
		set.Clear()
		return "unit"
	case "empty":
		return boolText(set.IsEmpty())
	case "clone":
		e.sets[1-op.Target] = set.Clone()
		return "unit"
	case "debug":
		return set.Debug()
	case "all":
		set.InsertAll()
		return "unit"
	case "insert":
		return boolText(set.Insert(e.column(op.A, op.TagA)))
	case "append":
		set.Append(e.column(op.A, op.TagA))
		return "unit"
	case "contains":
		return boolText(set.Contains(e.column(op.A, op.TagA)))
	case "range":
		return boolText(set.InsertRange(e.bounds(op, false)))
	case "appendrange":
		set.AppendRange(e.bounds(op, false))
		return "unit"
	case "intersects":
		return boolText(set.IntersectsRange(e.bounds(op, false)))
	case "first":
		value, ok := set.FirstUnsetIn(e.bounds(op, false))
		return optionText(value, ok)
	case "last":
		value, ok := set.LastSetIn(e.bounds(op, false))
		return optionText(value, ok)
	case "union":
		return boolText(set.Union(e.sets[1-op.Target]))
	case "superset":
		return boolText(set.Superset(e.sets[1-op.Target]))
	case "disjoint":
		return boolText(set.Disjoint(e.sets[1-op.Target]))
	case "intervals":
		iterator := set.IterIntervals()
		parts := []string{}
		for i := uint64(0); i < op.A; i++ {
			parts = append(parts, catch(func() string {
				value, ok := iterator.Next()
				if !ok {
					return "-"
				}
				return keyText(value.Start) + ".." + keyText(value.Last)
			}))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case "iter", "hint":
		iterator := set.Iter()
		parts := []string{}
		for i := uint64(0); i < op.A; i++ {
			if op.Action == "hint" {
				parts = append(parts, catch(func() string { return hintText(iterator.SizeHint()) }))
			}
			parts = append(parts, catch(func() string { value, ok := iterator.Next(); return optionText(value, ok) }))
		}
		if op.Action == "hint" {
			parts = append(parts, catch(func() string { return hintText(iterator.SizeHint()) }))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case "mrows":
		iterator := e.matrix.Rows()
		parts := []string{}
		for {
			value, ok := iterator.Next()
			if !ok {
				break
			}
			parts = append(parts, rowText(value))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case "menumerated":
		iterator := e.matrix.IterEnumerated()
		parts := []string{}
		for {
			row, value, ok := iterator.Next()
			if !ok {
				break
			}
			parts = append(parts, rowText(row)+"="+value.Debug())
		}
		return "[" + strings.Join(parts, ",") + "]"
	case "mrow":
		value, ok := e.matrix.Row(e.row(op.A, op.TagA))
		if !ok {
			return "-"
		}
		return value.Debug()
	case "mclear":
		e.matrix.ClearRow(e.row(op.A, op.TagA))
		return "unit"
	case "munionrow":
		return boolText(e.matrix.UnionRow(e.row(op.A, op.TagA), set))
	case "munionrows":
		return boolText(e.matrix.UnionRows(e.row(op.A, op.TagA), e.row(op.B, op.TagB)))
	case "mdisjoint":
		return boolText(e.matrix.DisjointRows(e.row(op.A, op.TagA), e.row(op.B, op.TagB)))
	case "mall":
		e.matrix.InsertAllIntoRow(e.row(op.A, op.TagA))
		return "unit"
	case "mrange":
		e.matrix.InsertRange(e.row(op.A, op.TagA), e.bounds(op, true))
		return "unit"
	case "minsert":
		return boolText(e.matrix.Insert(e.row(op.A, op.TagA), e.column(op.C, 0)))
	case "mappend":
		e.matrix.Append(e.row(op.A, op.TagA), e.column(op.C, 0))
		return "unit"
	case "mappendrange":
		e.matrix.AppendRange(e.row(op.A, op.TagA), e.bounds(op, true))
		return "unit"
	case "mcontains":
		return boolText(e.matrix.Contains(e.row(op.A, op.TagA), e.column(op.C, 0)))
	case "mclone":
		e.matrix = e.matrix.Clone()
		return "unit"
	default:
		panic("unknown operation: " + op.Action)
	}
}

// Execute runs only the Go port. Original Rust answers are generated separately.
func Execute(request Request, overflowChecks, debugAssertions bool) string {
	e := &execution{request: request, counts: make(map[string]uint)}
	idxConfig := index.IdxConfig{OverflowChecks: overflowChecks}
	column := index.NewIdxWithConfig(func(value uint) key {
		e.event("cn", value)
		if uint64(value) > request.Limit {
			panic("constructor limit")
		}
		return e.column(uint64(value), 0)
	}, func(value key) uint { e.event("ci", value.value, uint(value.tag)); return value.value }, idxConfig)
	row := index.NewIdxWithConfig(func(value uint) rowKey {
		e.event("rn", value)
		if uint64(value) > request.Limit {
			panic("constructor limit")
		}
		return e.row(uint64(value), 0)
	}, func(value rowKey) uint { e.event("ri", value.value, uint(value.tag)); return value.value }, idxConfig)
	rowEqual := func(a, b rowKey) bool {
		e.event("req", a.value, b.value, uint(a.tag), uint(b.tag))
		return a.value == b.value && a.tag == b.tag
	}
	config := index.IntervalConfig{OverflowChecks: overflowChecks, DebugAssertions: debugAssertions}
	e.step = e.makeStep()
	e.sets[0] = index.NewIntervalSetWithConfig(column, uint(request.Domain), e.step, config)
	e.sets[1] = index.NewIntervalSetWithConfig(column, uint(request.OtherDomain), e.step, config)
	e.matrix = index.NewSparseIntervalMatrixWithConfig(row, rowEqual, column, uint(request.Domain), e.step, config)
	for _, op := range request.Setup {
		catch(func() string { return e.operation(op) })
	}
	e.enabled = true
	results := []string{}
	for _, op := range request.Operations {
		results = append(results, catch(func() string { return e.operation(op) }))
	}
	e.enabled = false
	state := "s0=" + e.sets[0].Debug() + "~s1=" + e.sets[1].Debug() + "~m=["
	rows := []string{}
	for i := uint64(0); i < 128; i++ {
		value, ok := e.matrix.Row(e.row(i, 0))
		if !ok {
			break
		}
		rows = append(rows, value.Debug())
		if i == 127 {
			panic("unexpected unbounded matrix state")
		}
	}
	state += strings.Join(rows, ";") + "]"
	return strings.Join(results, "|") + "\t" + state + "\t" + strings.Join(e.trace, ";")
}
