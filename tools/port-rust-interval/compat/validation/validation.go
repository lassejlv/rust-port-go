// SPDX-License-Identifier: MIT OR Apache-2.0
// Development-only original-source protocol and Go normalization.
package validation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type Request struct {
	Kind                                string `json:"kind"`
	Profile, Style, Type, Shape         uint
	CharacterWriter                     bool
	FailAt                              uint
	PanicStage                          string
	PanicAt                             uint
	CleanupStage                        string
	Context                             uint
	Action                              string
	StartKind, EndKind                  uint
	Start, End, Domain, CloneDelta, Row uint
	Repeat                              uint
}
type Response struct {
	Value string
	Trace []string
	Text  string
	Calls uint
	State string
}
type Record struct {
	Request  Request
	Response Response
}

func number(b bool) uint {
	if b {
		return 1
	}
	return 0
}
func marker(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
func (r Request) Protocol() string {
	return fmt.Sprintf("%s %d %d %d %d %d %d %s %d %s %d %s %d %d %d %d %d %d %d %d", r.Kind, r.Profile, r.Style, r.Type, r.Shape, number(r.CharacterWriter), r.FailAt, marker(r.PanicStage), r.PanicAt, marker(r.CleanupStage), r.Context, marker(r.Action), r.StartKind, r.EndKind, r.Start, r.End, r.Domain, r.CloneDelta, r.Row, r.Repeat)
}
func DecodeResponse(line string) (Response, error) {
	var out Response
	p := strings.Split(line, "\t")
	if len(p) != 5 {
		return out, fmt.Errorf("response fields %d", len(p))
	}
	decode := func(s string) (string, error) { b, e := hex.DecodeString(s); return string(b), e }
	var e error
	out.Value, e = decode(p[0])
	if e != nil {
		return out, e
	}
	trace, e := decode(p[1])
	if e != nil {
		return out, e
	}
	out.Trace = []string{}
	if trace != "" {
		out.Trace = strings.Split(trace, "\n")
	}
	out.Text, e = decode(p[2])
	if e != nil {
		return out, e
	}
	calls, e := strconv.ParseUint(p[3], 10, 64)
	if e != nil {
		return out, e
	}
	out.Calls = uint(calls)
	out.State, e = decode(p[4])
	return out, e
}
func rangeRequest(action string) Request {
	r := Request{Kind: "r", Action: action, Domain: 16, Start: 2, End: 8, StartKind: 1, EndKind: 1, Row: 4, Repeat: 1}
	if action == "append" || action == "matrix_append" {
		r.Start, r.End = 8, 12
	}
	return r
}
func Requests() []Request {
	var out []Request
	for p := uint(0); p < 4; p++ {
		add := func(r Request) { r.Profile = p; out = append(out, r) }
		for style := uint(0); style < 15; style++ {
			for typ := uint(0); typ < 3; typ++ {
				for shape := uint(0); shape < 3; shape++ {
					for char := 0; char < 2; char++ {
						for context := uint(0); context < 3; context++ {
							add(Request{Kind: "f", Style: style, Type: typ, Shape: shape, CharacterWriter: char != 0, Context: context, Repeat: 2})
						}
					}
				}
			}
		}
		for style := uint(0); style < 15; style++ {
			for char := 0; char < 2; char++ {
				limit := uint(24)
				contexts := uint(1)
				if style == 0 || style == 1 || style == 9 || style == 13 {
					limit = 128
					contexts = 3
				}
				for context := uint(0); context < contexts; context++ {
					for fail := uint(1); fail <= limit; fail++ {
						add(Request{Kind: "f", Style: style, Type: 2, Shape: 1, CharacterWriter: char != 0, Context: context, FailAt: fail, Repeat: 2})
					}
				}
				for context := uint(0); context < 2; context++ {
					for _, stage := range []string{"write_s", "write_c"} {
						for at := uint(1); at <= 3; at++ {
							add(Request{Kind: "f", Style: style, Type: 2, Shape: 1, CharacterWriter: char != 0, Context: context, PanicStage: stage, PanicAt: at, Repeat: 2})
						}
					}
				}
			}
		}
		for _, action := range []string{"insert", "append", "intersects", "first", "last", "matrix_insert", "matrix_append"} {
			for context := uint(0); context < 3; context++ {
				for a := uint(0); a < 3; a++ {
					for b := uint(0); b < 3; b++ {
						for _, domain := range []uint{0, 16} {
							if context == 2 && domain == 0 && b != 0 && (action == "insert" || action == "append" || strings.HasPrefix(action, "matrix_")) {
								continue
							}
							r := rangeRequest(action)
							r.Context = context
							r.StartKind = a
							r.EndKind = b
							r.Domain = domain
							add(r)
						}
					}
				}
			}
			for context := uint(0); context < 2; context++ {
				for _, stage := range []string{"clone", "start", "end", "index_c", "drop1", "drop0", "new_c", "index_r"} {
					for at := uint(1); at <= 3; at++ {
						r := rangeRequest(action)
						r.Context = context
						r.PanicStage = stage
						r.PanicAt = at
						add(r)
					}
				}
			}
			for context := uint(0); context < 2; context++ {
				for _, delta := range []uint{2, ^uint(0)} {
					r := rangeRequest(action)
					r.Context = context
					r.CloneDelta = delta
					add(r)
				}
			}
		}
		for context := uint(0); context < 2; context++ {
			for _, action := range []string{"matrix_insert", "matrix_append"} {
				r := rangeRequest(action)
				r.Context = context
				r.Row = ^uint(0)
				add(r)
			}
		}

		for context := uint(0); context < 2; context++ {
			for _, action := range []string{"insert", "intersects", "first", "matrix_insert"} {
				r := rangeRequest(action)
				r.Context = context
				r.StartKind = 2
				r.Start = uint(^uint32(0))
				add(r)
			}
		}
	}
	return out
}
func AbortRequests() []Request {
	var out []Request
	for p := uint(0); p < 4; p++ {
		for context := uint(0); context < 2; context++ {
			for _, pair := range [][2]string{{"clone", "drop0"}, {"start", "drop1"}, {"index_c", "drop1"}, {"end", "drop0"}, {"index_c", "drop0"}, {"drop1", "drop0"}, {"index_r", "drop0"}} {
				r := rangeRequest("insert")
				r.Profile = p
				r.Context = context
				r.PanicStage = pair[0]
				r.PanicAt = 1
				r.CleanupStage = pair[1]
				if pair[0] == "index_r" {
					r.Action = "matrix_insert"
				}
				out = append(out, r)
			}
		}
		for _, stage := range []string{"clone", "start", "index_c", "end", "drop1", "drop0", "new_c", "index_r"} {
			r := rangeRequest("first")
			r.Profile = p
			r.Context = 2
			r.PanicStage = stage
			r.PanicAt = 1
			if stage == "index_r" {
				r.Action = "matrix_insert"
			}
			out = append(out, r)
		}
		for _, stage := range []string{"write_s", "write_c"} {
			r := Request{Kind: "f", Profile: p, Context: 2, Style: 9, Type: 2, Shape: 1, CharacterWriter: true, PanicStage: stage, PanicAt: 1, Repeat: 2}
			out = append(out, r)
		}
		for _, action := range []string{"matrix_insert", "matrix_append"} {
			r := rangeRequest(action)
			r.Profile = p
			r.Context = 2
			r.Row = ^uint(0)
			out = append(out, r)
		}

		if p&2 != 0 {
			r := rangeRequest("intersects")
			r.Profile = p
			r.Context = 2
			r.StartKind = 2
			r.Start = uint(^uint32(0))
			out = append(out, r)
		}
	}
	return out
}

func Options(style uint) index.IndexFormatOptions {
	o := index.DefaultIndexFormatOptions()
	switch style {
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.DebugLowerHex = true
		o.Alternate = true
	case 5:
		o.DebugUpperHex = true
		o.Alternate = true
	case 6:
		o.HasWidth = true
		o.Width = 10
	case 7:
		o.HasWidth = true
		o.Width = 10
		o.Fill = '*'
		o.Align = index.IndexAlignLeft
	case 8:
		o.HasWidth = true
		o.Width = 10
		o.Fill = 'λ'
		o.Align = index.IndexAlignCenter
	case 9:
		o.HasWidth = true
		o.Width = 10
		o.SignPlus = true
		o.ZeroPad = true
	case 10:
		o.SignMinus = true
	case 11:
		o.HasPrecision = true
		o.Precision = 3
	case 12, 13:
		o.HasWidth = true
		o.Width = 10
		o.HasPrecision = true
		o.Precision = 3
		o.Align = index.IndexAlignRight
		o.Fill = '*'
		o.Alternate = style == 13
	case 14:
		o.HasWidth = true
		o.Width = 10
		o.Align = index.IndexAlignCenter
		o.Fill = 0
	}
	return o
}
func Flags(o index.IndexFormatOptions) string {
	a, b := "-", "-"
	if o.HasWidth {
		a = strconv.FormatUint(uint64(o.Width), 10)
	}
	if o.HasPrecision {
		b = strconv.FormatUint(uint64(o.Precision), 10)
	}
	return fmt.Sprintf("%d/%s/%s/%d/%d/%d/%d/%d", number(o.Alternate), a, b, o.Fill, o.Align, number(o.SignPlus), number(o.SignMinus), number(o.ZeroPad))
}

type column struct {
	n        uint
	metadata []byte
}
type row struct {
	n        uint
	metadata []byte
}
type hook struct {
	request Request
	out     Response
	enabled bool
	counts  map[string]uint
	stream  func(string)
	context index.IntervalPanicContext
}

func (h *hook) event(stage string, value uint, context index.IntervalPanicContext) {
	if !h.enabled {
		return
	}
	text := fmt.Sprintf("%s:%d:%d", stage, value, number(context.Panicking()))
	h.out.Trace = append(h.out.Trace, text)
	if h.stream != nil {
		h.stream(text)
	}
	h.counts[stage]++
	if (stage == h.request.PanicStage && h.counts[stage] == h.request.PanicAt) || stage == h.request.CleanupStage {
		context.Panic("injected callback panic")
	}
}
func (h *hook) trace(text string) {
	h.out.Trace = append(h.out.Trace, text)
	if h.stream != nil {
		h.stream(text)
	}
}
func (h *hook) local(stage string, context index.IntervalPanicContext) {
	if panicValue := recover(); panicValue != nil {
		context.Unwinding = true
		h.trace("local:" + stage + ":" + fmt.Sprint(number(context.Panicking())))
		panic(panicValue)
	}
	h.trace("local:" + stage + ":" + fmt.Sprint(number(context.Panicking())))
}
func (h *hook) colIndex() index.Idx[column] {
	metadata := []byte{1, 2}
	return index.NewIdx(func(n uint) column {
		if h.enabled {
			defer h.local("new_c", h.context)
		}
		h.event("new_c", n, h.context)
		return column{n, metadata}
	}, func(k column) uint {
		if h.enabled {
			defer h.local("index_c", h.context)
		}
		h.event("index_c", k.n, h.context)
		return k.n
	})
}
func (h *hook) rowIndex() index.Idx[row] {
	metadata := []byte{1, 2}
	return index.NewIdx(func(n uint) row {
		if h.enabled {
			defer h.local("new_r", h.context)
		}
		h.event("new_r", n, h.context)
		return row{n, metadata}
	}, func(k row) uint {
		if h.enabled {
			defer h.local("index_r", h.context)
		}
		h.event("index_r", k.n, h.context)
		return k.n
	})
}
func colStep() index.IntervalStep[column] {
	return index.IntervalStep[column]{TypeName: "oracle::Key<1>", EqualFn: func(a, b column) bool { return a.n == b.n }, PartialCompareFn: func(a, b column) index.IntervalOrdering {
		if a.n < b.n {
			return index.IntervalLess
		}
		if a.n > b.n {
			return index.IntervalGreater
		}
		return index.IntervalEqual
	},
		StepsBetweenFn: func(a, b column) (uint, uint, bool) {
			if a.n > b.n {
				return 0, 0, false
			}
			return b.n - a.n, b.n - a.n, true
		},
		ForwardCheckedFn: func(a column, n uint) (column, bool) {
			if n > ^uint(0)-a.n {
				return column{}, false
			}
			return column{n: a.n + n}, true
		},
		BackwardCheckedFn: func(a column, n uint) (column, bool) {
			if n > a.n {
				return column{}, false
			}
			return column{n: a.n - n}, true
		},
		ForwardOverflowingFn:  func(a column, n uint) (column, bool) { return column{n: a.n + n}, n > ^uint(0)-a.n },
		BackwardOverflowingFn: func(a column, n uint) (column, bool) { return column{n: a.n - n}, n > a.n },
	}
}
func (h *hook) bounds(generation uint) index.IntervalRangeBounds[column] {
	start := h.request.Start
	if generation != 0 {
		start += h.request.CloneDelta
	}
	return index.IntervalRangeBounds[column]{
		CloneWithContextFn: func(context index.IntervalPanicContext) index.IntervalRangeBounds[column] {
			defer h.local("clone", context)
			h.event("clone", generation, context)
			return h.bounds(generation + 1)
		},
		StartBoundWithContextFn: func(context index.IntervalPanicContext) index.IntervalBound[column] {
			defer h.local("start", context)
			h.event("start", generation, context)
			return index.IntervalBound[column]{Kind: index.IntervalBoundKind(h.request.StartKind), Value: column{n: start}}
		},
		EndBoundWithContextFn: func(context index.IntervalPanicContext) index.IntervalBound[column] {
			defer h.local("end", context)
			h.event("end", generation, context)
			return index.IntervalBound[column]{Kind: index.IntervalBoundKind(h.request.EndKind), Value: column{n: h.request.End}}
		},
		CloseWithContext: func(context index.IntervalPanicContext) {
			stage := fmt.Sprint("drop", generation)
			defer h.local(stage, context)
			h.event(stage, generation, context)
		},
	}
}
func Replay(r Request, stream func(string)) Response {
	h := &hook{request: r, out: Response{Trace: []string{}}, counts: map[string]uint{}, stream: stream, context: index.IntervalPanicContext{Unwinding: r.Context == 1, ProcessingPanic: r.Context == 2}}
	if r.Kind == "f" {
		return replayFormat(h)
	}
	config := index.IntervalConfig{OverflowChecks: r.Profile&2 != 0, DebugAssertions: r.Profile&1 != 0}
	colIdx, rowIdx := h.colIndex(), h.rowIndex()
	set := index.NewIntervalSetWithConfig(colIdx, r.Domain, colStep(), config)
	if r.Domain > 5 && r.Action != "append" {
		set.InsertRange(index.IntervalClosed(column{n: 3}, column{n: 5}))
	}
	matrix := index.NewSparseIntervalMatrixWithConfig(rowIdx, func(a, b row) bool { return a.n == b.n }, colIdx, r.Domain, colStep(), config)
	h.enabled = true
	func() {
		defer func() {
			if recover() != nil {
				h.out.Value = "panic"
			}
		}()
		bounds := h.bounds(0)
		context := h.context
		switch r.Action {
		case "insert":
			h.out.Value = fmt.Sprint(set.InsertRangeWithContext(context, bounds))
		case "append":
			set.AppendRangeWithContext(context, bounds)
			h.out.Value = "unit"
		case "intersects":
			h.out.Value = fmt.Sprint(set.IntersectsRangeWithContext(context, bounds))
		case "first":
			v, ok := set.FirstUnsetInWithContext(context, bounds)
			h.out.Value = "none"
			if ok {
				h.out.Value = fmt.Sprint("some:", v.n)
			}
		case "last":
			v, ok := set.LastSetInWithContext(context, bounds)
			h.out.Value = "none"
			if ok {
				h.out.Value = fmt.Sprint("some:", v.n)
			}
		case "matrix_insert":
			matrix.InsertRangeWithContext(context, row{n: r.Row}, bounds)
			h.out.Value = "unit"
		case "matrix_append":
			matrix.AppendRangeWithContext(context, row{n: r.Row}, bounds)
			h.out.Value = "unit"
		default:
			panic("operation")
		}
	}()
	h.enabled = false
	var rows []string
	for n := uint(0); n < 128; n++ {
		value, ok := matrix.Row(row{n: n})
		if !ok {
			break
		}
		rows = append(rows, value.Debug())
	}
	h.out.State = set.Debug() + "~[" + strings.Join(rows, ";") + "]"
	return h.out
}
func replayFormat(h *hook) Response {
	r := h.request
	config := index.IntervalConfig{OverflowChecks: r.Profile&2 != 0, DebugAssertions: r.Profile&1 != 0}
	domain := uint(512)
	if r.Shape == 2 {
		domain = ^uint(0)
	}
	var format func(*index.IndexDebugFormatter) error
	var debug func() string
	if r.Type == 0 {
		set := index.NewIntervalSetWithConfig(index.U32Idx(), domain, index.U32IntervalStepWithConfig(config), config)
		if r.Shape == 1 {
			set.InsertRange(index.IntervalClosed(uint32(15), uint32(18)))
			set.InsertRange(index.IntervalClosed(uint32(255), uint32(260)))
		}
		format, debug = set.FormatDebug, set.Debug
	} else if r.Type == 1 {
		set := index.NewIntervalSetWithConfig(index.UsizeIdx(), domain, index.UsizeIntervalStepWithConfig(config), config)
		if r.Shape == 1 {
			set.InsertRange(index.IntervalClosed(uint(15), uint(18)))
			set.InsertRange(index.IntervalClosed(uint(255), uint(260)))
		}
		format, debug = set.FormatDebug, set.Debug
	} else {
		set := index.NewIntervalSetWithConfig(h.colIndex(), domain, colStep(), config)
		if r.Shape == 1 {
			set.InsertRange(index.IntervalClosed(column{n: 15}, column{n: 18}))
			set.InsertRange(index.IntervalClosed(column{n: 255}, column{n: 260}))
		}
		format, debug = set.FormatDebug, set.Debug
	}
	h.enabled = true
	sink := func(stage string, c rune, text string) error {
		if r.PanicStage != "" {
			defer h.local(stage, h.context)
		}
		h.out.Calls++
		detail := hex.EncodeToString([]byte(text))
		if stage == "write_c" {
			detail = fmt.Sprint(uint32(c))
		}
		h.trace(fmt.Sprintf("write:%d:%s:%s:%d", h.out.Calls, stage, detail, number(h.context.Panicking())))
		h.counts[stage]++
		if stage == r.PanicStage && h.counts[stage] == r.PanicAt {
			h.context.Panic("writer")
		}
		if h.out.Calls == r.FailAt {
			return errors.New("fmt::Error")
		}
		h.out.Text += text
		return nil
	}
	f := index.IndexDebugFormatter{Options: Options(r.Style), Context: index.IndexFormatContext{Unwinding: h.context.Unwinding, ProcessingPanic: h.context.ProcessingPanic}, Write: func(s string) error { return sink("write_s", 0, s) }}
	if r.CharacterWriter {
		f.WriteChar = func(c rune) error { return sink("write_c", c, string(c)) }
	}
	var values []string
	for repeat := uint(0); repeat < r.Repeat; repeat++ {
		status := "ok"
		func() {
			defer func() {
				if recover() != nil {
					status = "panic"
				}
			}()
			if format(&f) != nil {
				status = "error"
			}
		}()
		values = append(values, status)
		h.trace("result:" + status + ":" + Flags(f.Options))
	}
	h.enabled = false
	h.out.Value = strings.Join(values, ",")
	h.out.State = debug()
	return h.out
}
