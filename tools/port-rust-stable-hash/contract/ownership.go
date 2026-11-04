// Package contract contains development-only replay observers, never production
// hashing fallbacks.
package contract

import (
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strings"
)

type OwnershipCase struct{ Operation, Mode, Context string }
type customH struct {
	tag            string
	value, queries uint64
	nonCopy        []byte
}
type customR struct{ value uint64 }
type customW struct{ value uint64 }

func ObserveOwnership(q OwnershipCase, live func(string)) []string {
	events := []string{}
	event := func(s string) {
		events = append(events, s)
		if live != nil {
			live(s)
		}
	}
	ctx := rustfmt.Context{Unwinding: q.Context == "inherited", ProcessingPanic: q.Context == "processing"}
	failOld, failClone, failFinish, failR, failFrom, failH := q.Mode == "drop", q.Mode == "clone", (q.Mode == "finish" || q.Mode == "finish-h-drop"), (q.Mode == "r-drop" || q.Mode == "from-r-drop"), (q.Mode == "from" || q.Mode == "from-r-drop"), (q.Mode == "h-drop" || q.Mode == "finish-h-drop")
	newH := func(tag string, value uint64) customH {
		return customH{tag: tag, value: value, nonCopy: []byte{1, 2, 3}}
	}
	dropR := func(value *customR, ctx hash.DropContext) {
		event(fmt.Sprintf("drop-r:%d:panic=%t", value.value, ctx.Panicking()))
		if failR {
			failR = false
			ctx.Panic("drop-r")
		}
	}
	ops := hash.ExtendedHasherOps[customH, customR]{
		Write: func(ref hash.Ref[customH], bytes []byte) {
			parts := make([]string, len(bytes))
			for i, b := range bytes {
				parts[i] = fmt.Sprint(b)
			}
			event(fmt.Sprintf("write:%s:[%s]", ref.Value.tag, strings.Join(parts, ", ")))
			ref.Value.nonCopy = append(ref.Value.nonCopy, bytes...)
			ref.Value.value += uint64(len(bytes))
		},
		Finish64: func(ref hash.Ref[customH]) uint64 {
			ref.Value.queries++
			event(fmt.Sprintf("finish64:%s:%d:query=%d", ref.Value.tag, ref.Value.value, ref.Value.queries))
			return ref.Value.value
		},
		Finish: func(arg *hash.Owned[customH]) *hash.Owned[customR] {
			v := arg.Borrow()
			event(fmt.Sprintf("finish-owned:%s:%d", v.tag, v.value))
			if failFinish {
				failFinish = false
				arg.Context().Panic("finish")
			}
			return hash.NewOwned(customR{v.value}, hash.ValueOps[customR]{Drop: dropR})
		},
		Default: func(hash.DropContext) customH { event("default"); return newH("default", 5) },
		Clone: func(ref hash.Ref[customH]) customH {
			v := ref.Value
			event(fmt.Sprintf("clone:%s:%d:panic=%t", v.tag, v.value, ref.Context.Panicking()))
			if failClone {
				failClone = false
				ref.Context.Panic("clone")
			}
			return customH{tag: "clone", value: v.value, queries: v.queries, nonCopy: append([]byte(nil), v.nonCopy...)}
		},
		Drop: func(v *customH, context hash.DropContext) {
			event(fmt.Sprintf("drop-h:%s:%d:panic=%t", v.tag, v.value, context.Panicking()))
			if (v.tag == "old" && failOld) || failH {
				failOld = false
				failH = false
				context.Panic("drop-h")
			}
		},
		Debug: func(ref hash.Ref[customH], f *rustfmt.Formatter) error {
			width := "None"
			if f.Options.HasWidth {
				width = fmt.Sprintf("Some(%d)", f.Options.Width)
			}
			event(fmt.Sprintf("debug:%s:pretty=%t:width=%s:fill=%c:plus=%t", ref.Value.tag, f.Options.Alternate, width, f.Options.Fill, f.Options.SignPlus))
			if e := f.WriteString("H("); e != nil {
				return e
			}
			if e := rustfmt.FormatDebugUint(ref.Value.value, 64, f); e != nil {
				return e
			}
			return f.WriteString(")")
		},
	}
	capture := func(body func()) (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		body()
		return
	}
	if q.Context == "inherited" {
		event("inherited:panic=true")
	}
	if q.Context == "processing" {
		event("processing:panic=true")
	}
	switch q.Operation {
	case "raw-clone", "raw-new":
		var target *hash.SipHasher128
		if q.Operation == "raw-new" {
			// Go's zero-state convenience corresponds to the pure Rust Default
			// construction, but must retain context set before lazy initialization.
			target = &hash.SipHasher128{}
			event("raw-new")
		} else {
			target = hash.NewSipHasher128()
			target.SetContext(ctx)
			source := hash.NewSipHasher128()
			event("raw-clone")
			target.CloneFrom(source)
		}
		if q.Operation == "raw-new" {
			target.SetContext(ctx)
		}
		event("raw-short")
		panicked := capture(func() { target.ShortWrite(9, make([]byte, 9)) })
		event(fmt.Sprintf("caught:%t", panicked))
	case "clone":
		source := hash.WithHasher(newH("source", 9), ops)
		target := hash.WithHasher(newH("old", 2), ops)
		source.SetContext(ctx)
		target.SetContext(ctx)
		panicked := capture(func() { target.CloneFrom(source) })
		event(fmt.Sprintf("caught:%t", panicked))
		event(fmt.Sprintf("after:%d", target.Finish64()))
		target.Close()
		source.Close()
	case "finish":
		h := hash.WithHasher(newH("finish", 7), ops)
		h.SetContext(ctx)
		panicked := capture(func() {
			w := hash.FinishInto(h, hash.FromStableHash[customR, customW]{From: func(arg *hash.Owned[customR]) *hash.Owned[customW] {
				value := arg.Borrow()
				event(fmt.Sprintf("from:%d", value.value))
				if failFrom {
					failFrom = false
					arg.Context().Panic("from")
				}
				return hash.NewOwned(customW{value.value}, hash.ValueOps[customW]{Drop: func(value *customW, context hash.DropContext) {
					event(fmt.Sprintf("drop-w:%d:panic=%t", value.value, context.Panicking()))
				}})
			}})
			event(fmt.Sprintf("returned:%d", w.Borrow().value))
			w.Drop()
		})
		event(fmt.Sprintf("caught:%t", panicked))
	case "borrow":
		h := hash.NewStableHasher(ops)
		h.SetContext(ctx)
		h.WriteU16(7)
		event(fmt.Sprintf("first:%d", h.Finish64()))
		event(fmt.Sprintf("second:%d", h.Finish64()))
		var text strings.Builder
		f := rustfmt.Formatter{Writer: &text, Context: ctx, Options: rustfmt.Options{Fill: ' ', HasWidth: true, Width: 32, Alternate: true, SignPlus: true, ZeroPad: true}}
		if e := h.WriteDebugFormatter(&f); e != nil {
			panic(e)
		}
		event("debug-output:" + text.String())
		h.Close()
	default:
		panic("unknown ownership operation")
	}
	if q.Context == "inherited" || q.Context == "processing" {
		event("outer-caught:true")
	}
	return events
}
