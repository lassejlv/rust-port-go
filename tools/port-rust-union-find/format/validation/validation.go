// SPDX-License-Identifier: MIT OR Apache-2.0
// Development request/normalization adapter; production Debug is implemented in
// datastructures, and reference output comes only from the unchanged Rust files.
package validation

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type Request struct {
	Profile         uint `json:"profile"`
	Style           uint `json:"style"`
	Size            uint `json:"size"`
	Unify           bool `json:"unify"`
	CharacterWriter bool `json:"character_writer"`
	FailAt          uint `json:"fail_at"`
	PanicKind       uint `json:"panic_kind"`
	PanicAt         uint `json:"panic_at"`
	Catch           bool `json:"catch"`
	Variant         uint `json:"variant"`
	Repeat          uint `json:"repeat"`
}
type Response struct {
	Trace   []string `json:"trace"`
	Text    string   `json:"text"`
	Calls   uint     `json:"calls"`
	Results []string `json:"results"`
	State   []uint   `json:"state"`
}
type Record struct {
	Request  Request  `json:"request"`
	Response Response `json:"response"`
}

func Requests() []Request {
	var out []Request
	for profile := uint(0); profile < 4; profile++ {
		add := func(r Request) { r.Profile = profile; r.Repeat = 2; out = append(out, r) }
		for style := uint(0); style < 15; style++ {
			for _, shape := range []struct {
				n     uint
				unify bool
			}{{0, false}, {1, false}, {3, false}, {3, true}} {
				for char := 0; char < 2; char++ {
					for caught := 0; caught < 2; caught++ {
						add(Request{Style: style, Size: shape.n, Unify: shape.unify, CharacterWriter: char != 0, Catch: caught != 0})
					}
				}
			}
			for char := 0; char < 2; char++ {
				for caught := 0; caught < 2; caught++ {
					maxFailure := uint(24)
					if style == 0 || style == 1 || style == 9 || style == 13 {
						maxFailure = 192
					}
					for fail := uint(1); fail <= maxFailure; fail++ {
						add(Request{Style: style, Size: 3, Unify: true, CharacterWriter: char != 0, Catch: caught != 0, FailAt: fail})
					}
					for kind := uint(1); kind <= 4; kind++ {
						for nth := uint(1); nth <= 3; nth++ {
							add(Request{Style: style, Size: 3, Unify: true, CharacterWriter: char != 0, Catch: caught != 0, PanicKind: kind, PanicAt: nth})
						}
					}
				}
			}
		}
		// IDs15/10 distinguish lower/upper hexadecimal digits without alternate
		// prefixes; compact private radix bits must not disappear in struct dispatch.
		for _, style := range []uint{2, 3, 4, 5} {
			for char := 0; char < 2; char++ {
				add(Request{Style: style, Size: 16, CharacterWriter: char != 0})
			}
		}

		for variant := uint(1); variant <= 2; variant++ {
			for _, style := range []uint{0, 1, 13} {
				for char := 0; char < 2; char++ {
					for fail := uint(0); fail <= 96; fail++ {
						add(Request{Style: style, Size: 3, Unify: true, CharacterWriter: char != 0, Variant: variant, FailAt: fail, Catch: fail%2 == 0})
					}
				}
			}
		}
	}
	return out
}
func boolNumber(b bool) uint {
	if b {
		return 1
	}
	return 0
}
func (r Request) Protocol() string {
	return fmt.Sprintf("%d %d %d %d %d %d %d %d %d %d %d", r.Profile, r.Style, r.Size, boolNumber(r.Unify), boolNumber(r.CharacterWriter), r.FailAt, r.PanicKind, r.PanicAt, boolNumber(r.Catch), r.Variant, r.Repeat)
}
func DecodeResponse(line string) (Response, error) {
	var out Response
	parts := strings.Split(line, "\t")
	if len(parts) != 5 {
		return out, fmt.Errorf("reference response fields: %d", len(parts))
	}
	decode := func(s string) (string, error) { b, e := hex.DecodeString(s); return string(b), e }
	trace, e := decode(parts[0])
	if e != nil {
		return out, e
	}
	out.Trace = []string{}
	if trace != "" {
		out.Trace = strings.Split(trace, "\n")
	}
	out.Text, e = decode(parts[1])
	if e != nil {
		return out, e
	}
	u, e := strconv.ParseUint(parts[2], 10, 64)
	if e != nil {
		return out, e
	}
	out.Calls = uint(u)
	out.Results = strings.Split(parts[3], ",")
	out.State = []uint{}
	if parts[4] != "" {
		for _, v := range strings.Split(parts[4], ",") {
			n, e := strconv.ParseUint(v, 10, 64)
			if e != nil {
				return out, e
			}
			out.State = append(out.State, uint(n))
		}
	}
	return out, nil
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
	width, precision := "-", "-"
	if o.HasWidth {
		width = strconv.FormatUint(uint64(o.Width), 10)
	}
	if o.HasPrecision {
		precision = strconv.FormatUint(uint64(o.Precision), 10)
	}
	return fmt.Sprintf("%d/%s/%s/%d/%d/%d/%d/%d", boolNumber(o.Alternate), width, precision, o.Fill, o.Align, boolNumber(o.SignPlus), boolNumber(o.SignMinus), boolNumber(o.ZeroPad))
}

type key struct {
	n        uint
	metadata []byte
}

func Replay(r Request) Response {
	out := Response{Trace: []string{}, Results: []string{}, State: []uint{}}
	metadata := []byte{1, 2}
	idx := ds.UnionFindIndex[key]{FromIndex: func(n uint) key { return key{n, metadata} }, ToIndex: func(k key) uint { return k.n }, Equal: func(a, b key) bool { return a.n == b.n }}
	sets := ds.NewUnionFindWithConfig(r.Size, idx, ds.UnionFindConfig{DebugAssertions: r.Profile&1 != 0})
	if r.Unify {
		for i := uint(1); i < r.Size; i++ {
			sets.Unify(idx.FromIndex(0), idx.FromIndex(i))
		}
	}
	// Capture and retain the real cells before tracing. Fresh copied receiver
	// cells cannot pass the subsequent repeated identity comparisons.
	var cells []*key
	warm := index.IndexDebugFormatter{Options: index.DefaultIndexFormatOptions(), Write: func(string) error { return nil }}
	_ = sets.FormatDebug(&warm, func(k *key, _ *index.IndexDebugFormatter) error { cells = append(cells, k); return nil })
	var panicCounts [5]uint
	event := func(kind uint, text string) {
		out.Trace = append(out.Trace, text)
		panicCounts[kind]++
		if r.PanicKind == kind && panicCounts[kind] == r.PanicAt {
			panic("formatter probe")
		}
	}
	sink := func(kind uint, c rune, text string) error {
		out.Calls++
		name := "s:" + hex.EncodeToString([]byte(text))
		if kind == 3 {
			name = fmt.Sprint("c:", uint32(c))
		}
		event(kind, fmt.Sprintf("write:%d:%s", out.Calls, name))
		if out.Calls == r.FailAt {
			return errors.New("fmt::Error")
		}
		out.Text += text
		return nil
	}
	f := index.IndexDebugFormatter{Options: Options(r.Style), Write: func(s string) error { return sink(2, 0, s) }}
	if r.CharacterWriter {
		f.WriteChar = func(c rune) error { return sink(3, c, string(c)) }
	}
	slot := 0
	formatKey := func(k *key, f *index.IndexDebugFormatter) (err error) {
		defer func() {
			if p := recover(); p != nil {
				out.Trace = append(out.Trace, fmt.Sprintf("local:%d:1", k.n))
				panic(p)
			}
			event(4, fmt.Sprintf("local:%d:%d", k.n, boolNumber(f.Context.Panicking())))
		}()
		same := slot < len(cells) && cells[slot] == k
		slot++
		event(1, fmt.Sprintf("key:%d:%s:%d", k.n, Flags(f.Options), boolNumber(same)))
		if r.Variant != 0 {
			text := []string{"", "\nfirst", "a\n\nb\n"}[slot%3]
			if r.Variant == 2 {
				text = []string{"\n", "λ\n", "\n\n"}[slot%3]
			}
			err = f.WriteString(text)
		} else {
			err = f.WriteString("K")
			if err == nil || r.Catch {
				err = index.FormatDebugUint(uint64(k.n), uint(strconv.IntSize), f)
			}
		}
		if r.Catch {
			return nil
		}
		return err
	}
	for repeat := uint(0); repeat < r.Repeat; repeat++ {
		slot = 0
		status := "ok"
		func() {
			defer func() {
				if recover() != nil {
					status = "panic"
				}
			}()
			if sets.FormatDebug(&f, formatKey) != nil {
				status = "error"
			}
		}()
		out.Results = append(out.Results, status)
		out.Trace = append(out.Trace, "result:"+status+":"+Flags(f.Options))
	}
	for _, k := range sets.Snapshot().Raw {
		out.State = append(out.State, k.n)
	}
	return out
}
