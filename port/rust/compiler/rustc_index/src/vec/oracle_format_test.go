// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import (
	"encoding/json"
	"errors"
	"fmt"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

func storageFormatRequests() []storageRequest {
	var out []storageRequest
	styles := []string{"plain", "pretty", "lower", "pretty_lower", "upper", "pretty_upper", "width", "left", "center", "zero_plus", "minus", "precision", "width_precision", "pretty_width_precision", "nul_fill", "pretty_nul_fill"}
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, op := range []string{"format_list", "format_map", "format_option_map", "format_compact"} {
				for input, values := range [][]int64{{}, {15, -1}, {storageNone, 255, 15}} {
					if input == 2 && (op == "format_list" || op == "format_map") {
						continue
					}
					if input == 1 && (op == "format_option_map" || op == "format_compact") {
						continue
					}
					for _, style := range styles {
						for failAt := uint(0); failAt <= 18; failAt++ {
							out = append(out, storageRequest{Kind: "format", Name: fmt.Sprintf("%s-%d-%s-%d", op, input, style, failAt), Checked: checked, Debug: debug, Values: values, Other: []int64{}, Capacity: uint(len(values)), Limit: 64, Actions: []storageAction{{Op: op, Style: style, FailAt: failAt}}})
						}
						for _, failure := range []string{"format_element", "format_key", "write"} {
							for nth := uint(1); nth <= 2; nth++ {
								out = append(out, storageRequest{Kind: "format", Name: fmt.Sprintf("%s-%d-%s-panic-%s-%d", op, input, style, failure, nth), Checked: checked, Debug: debug, Values: values, Other: []int64{}, Capacity: uint(len(values)), Limit: 64, Actions: []storageAction{{Op: op, Style: style, Panic: failure, PanicAt: nth}}})
							}
						}
					}
				}
			}
		}
	}
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, op := range []string{"format_list", "format_map", "format_option_map", "format_compact"} {
				for _, value := range []int64{17, -2, -3} {
					for _, style := range []string{"plain", "pretty"} {
						for failAt := uint(0); failAt <= 12; failAt++ {
							out = append(out, storageRequest{Kind: "format", Name: fmt.Sprintf("format-recovery-%s-%d-%s-%d", op, value, style, failAt), Checked: checked, Debug: debug, Values: []int64{value}, Other: []int64{}, Capacity: 1, Limit: 64, Actions: []storageAction{{Op: op, Style: style, FailAt: failAt}}})
						}
					}
				}
			}
		}
	}
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for _, op := range []string{"format_list", "format_map", "format_option_map", "format_compact"} {
				for _, style := range []string{"zero_plus", "pretty_zero_plus"} {
					for failAt := uint(0); failAt <= 24; failAt++ {
						out = append(out, storageRequest{Kind: "format", Name: fmt.Sprintf("format-reuse-%s-%s-%d", op, style, failAt), Checked: checked, Debug: debug, Values: []int64{15, 255}, Other: []int64{}, Capacity: 2, Limit: 64, Actions: []storageAction{{Op: op, Style: style, FailAt: failAt, Reuse: true}}})
					}
				}
			}
		}
	}
	base := append([]storageRequest{}, out...)
	for _, r := range base {
		switch r.Actions[0].Style {
		case "width", "left", "center", "zero_plus", "width_precision", "pretty_width_precision", "nul_fill", "pretty_nul_fill":
			r.Actions = append([]storageAction{}, r.Actions...)
			r.Actions[0].CharWriter = true
			r.Name += "-char-writer"
			out = append(out, r)
		}
	}
	return out
}

func storageFormatOptions(style string) index.IndexFormatOptions {
	opts := index.IndexFormatOptions{Fill: ' '}
	switch style {
	case "pretty":
		opts.Alternate = true
	case "lower":
		opts.DebugLowerHex = true
	case "upper":
		opts.DebugUpperHex = true
	case "pretty_lower":
		opts.Alternate = true
		opts.DebugLowerHex = true
	case "pretty_upper":
		opts.Alternate = true
		opts.DebugUpperHex = true
	case "width":
		opts.Width = 10
		opts.HasWidth = true
	case "left":
		opts.Width = 10
		opts.HasWidth = true
		opts.Fill = '*'
		opts.Align = index.IndexAlignLeft
	case "center":
		opts.Width = 10
		opts.HasWidth = true
		opts.Fill = 'λ'
		opts.Align = index.IndexAlignCenter
	case "zero_plus", "pretty_zero_plus":
		opts.Alternate = style == "pretty_zero_plus"
		opts.Width = 10
		opts.HasWidth = true
		opts.SignPlus = true
		opts.ZeroPad = true
	case "minus":
		opts.SignMinus = true
	case "precision":
		opts.Precision = 3
		opts.HasPrecision = true
	case "nul_fill", "pretty_nul_fill":
		opts.Width = 10
		opts.HasWidth = true
		opts.Align = index.IndexAlignRight
		opts.Fill = 0
		opts.Alternate = style == "pretty_nul_fill"
	case "width_precision", "pretty_width_precision":
		opts.Width = 10
		opts.HasWidth = true
		opts.Precision = 3
		opts.HasPrecision = true
		opts.Align = index.IndexAlignRight
		opts.Fill = '*'
		opts.Alternate = style == "pretty_width_precision"
	}
	return opts
}
func storageFormatFlags(opts index.IndexFormatOptions) string {
	align := "None"
	switch opts.Align {
	case index.IndexAlignLeft:
		align = "Some(Left)"
	case index.IndexAlignRight:
		align = "Some(Right)"
	case index.IndexAlignCenter:
		align = "Some(Center)"
	}
	var width, precision any
	if opts.HasWidth {
		width = opts.Width
	}
	if opts.HasPrecision {
		precision = opts.Precision
	}
	data, _ := json.Marshal(map[string]any{"alternate": opts.Alternate, "width": width, "precision": precision, "fill": uint32(opts.Fill), "align": align, "plus": opts.SignPlus, "minus": opts.SignMinus, "zero": opts.ZeroPad})
	return string(data)
}
func storageFormatLocalDrop(h *storageHarness, kind string, value int64, f *index.IndexDebugFormatter) {
	if p := recover(); p != nil {
		h.event("format_local_drop", kind, value, true)
		panic(p)
	}
	h.event("format_local_drop", kind, value, f.Context.Panicking())
}
func storageReplayFormat(r storageRequest) map[string]any {
	h := &storageHarness{}
	idx := index.NewIdx(func(n uint) uint {
		h.event("new", n)
		if n > r.Limit {
			panic("index domain")
		}
		return n
	}, func(n uint) uint { h.event("index", n); return n })
	ops := index.ValueOps[int64]{FormatDebug: func(value *int64, f *index.IndexDebugFormatter) error {
		defer storageFormatLocalDrop(h, "element", *value, f)
		h.eventUsing(f.Context.Panic, "format_element", *value, storageFormatFlags(f.Options))
		if *value == 17 {
			_ = f.WriteString("try")
			return f.WriteString("again")
		}
		if *value == -2 {
			return f.WriteString("\n\nz\n")
		}
		if *value == -3 {
			return f.WriteString("")
		}
		if *value < 0 {
			return f.WriteString("x\nλ")
		}
		if err := f.WriteString("E"); err != nil {
			return err
		}
		return index.FormatDebugInt(*value, 64, f)
	}}
	key := func(value *uint, f *index.IndexDebugFormatter) error {
		defer storageFormatLocalDrop(h, "key", int64(*value), f)
		h.eventUsing(f.Context.Panic, "format_key", *value, storageFormatFlags(f.Options))
		if err := f.WriteString("K"); err != nil {
			return err
		}
		return index.FormatDebugUint(uint64(*value), 64, f)
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
		calls := uint(0)
		output := ""
		ok := false
		panicked := false
		f := &index.IndexDebugFormatter{Options: storageFormatOptions(a.Style), Write: func(text string) error {
			calls++
			h.event("write", calls, text)
			if calls == a.FailAt {
				return errors.New("fmt::Error")
			}
			output += text
			return nil
		}}
		if a.CharWriter {
			f.WriteChar = func(c rune) error {
				calls++
				h.event("write_char", calls, uint32(c))
				if calls == a.FailAt {
					return errors.New("fmt::Error")
				}
				output += string(c)
				return nil
			}
		}
		var render func() error
		if a.Op == "format_list" || a.Op == "format_map" {
			data := make([]int64, len(r.Values))
			copy(data, r.Values)
			v := index.IndexVecFromRaw(data, idx, ops)
			if a.Op == "format_list" {
				render = func() error { return v.FormatDebug(f) }
			} else {
				view := v.DebugMapView().WithKeyFormatter(key)
				render = func() error { return view.FormatDebug(f) }
			}
		} else {
			data := make([]index.Option[int64], len(r.Values))
			for i, x := range r.Values {
				if x != storageNone {
					data[i] = index.Some(x)
				}
			}
			v := index.AsOptionIndexVec(index.IndexVecFromRaw(data, idx, index.OptionValueOps(ops)), ops)
			if a.Op == "format_compact" {
				view := v.DebugMapViewCompact().WithKeyFormatter(key)
				render = func() error { return view.FormatDebug(f) }
			} else {
				view := v.DebugMapView().WithKeyFormatter(key)
				render = func() error { return view.FormatDebug(f) }
			}
		}
		var after, second, secondOK any
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			err := render()
			after = storageFormatFlags(f.Options)
			if a.Reuse {
				again := render()
				second = storageFormatFlags(f.Options)
				secondOK = again == nil
			}
			ok = err == nil
		}()

		steps = append(steps, map[string]any{"value": map[string]any{"ok": ok, "text": output, "calls": calls, "after_options": after, "second_options": second, "second_ok": secondOK}, "panic": panicked, "events": append([]string{}, h.events...)})
	}
	return map[string]any{"steps": steps}
}
