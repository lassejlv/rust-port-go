// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type ProjectionRequest struct {
	IntoRequest
	Kind string
}
type projectedView[T any] struct {
	next      func(uint, bool, bool) *rustvec.Owned[T]
	last      func() *rustvec.Owned[T]
	count     func() uint
	close     func(im.Context)
	clone     func() *projectedView[T]
	cloneFrom func(*projectedView[T])
	len       func() uint
	hint      func() rustvec.Hint
	debug     func() string
	raw       any
}

func keysView(it *im.IntoKeys[intoKey, intoValue], c im.Context) *projectedView[intoKey] {
	v := &projectedView[intoKey]{raw: it}
	v.next = func(n uint, skip, back bool) *rustvec.Owned[intoKey] {
		if skip {
			if back {
				return it.NthBackOwned(n, c)
			}
			return it.NthOwned(n, c)
		}
		if back {
			return it.NextBackOwned(c)
		}
		return it.NextOwned(c)
	}
	v.last = func() *rustvec.Owned[intoKey] { return it.LastOwned(c) }
	v.count = func() uint { return it.Count(c) }
	v.close = func(c im.Context) { it.Close(c) }
	v.clone = func() *projectedView[intoKey] { return keysView(it.Clone(c), c) }
	v.cloneFrom = func(other *projectedView[intoKey]) { it.CloneFrom(other.raw.(*im.IntoKeys[intoKey, intoValue]), c) }
	v.len = func() uint { return it.Len(c) }
	v.hint = func() rustvec.Hint { return it.SizeHint(c) }
	v.debug = func() string { return rustfmt.String(it.WriteDebugFormatter) }
	return v
}
func valuesView(it *im.IntoValues[intoKey, intoValue], c im.Context) *projectedView[intoValue] {
	v := &projectedView[intoValue]{raw: it}
	v.next = func(n uint, skip, back bool) *rustvec.Owned[intoValue] {
		if skip {
			if back {
				return it.NthBackOwned(n, c)
			}
			return it.NthOwned(n, c)
		}
		if back {
			return it.NextBackOwned(c)
		}
		return it.NextOwned(c)
	}
	v.last = func() *rustvec.Owned[intoValue] { return it.LastOwned(c) }
	v.count = func() uint { return it.Count(c) }
	v.close = func(c im.Context) { it.Close(c) }
	v.clone = func() *projectedView[intoValue] { return valuesView(it.Clone(c), c) }
	v.cloneFrom = func(other *projectedView[intoValue]) { it.CloneFrom(other.raw.(*im.IntoValues[intoKey, intoValue]), c) }
	v.len = func() uint { return it.Len(c) }
	v.hint = func() rustvec.Hint { return it.SizeHint(c) }
	v.debug = func() string { return rustfmt.String(it.WriteDebugFormatter) }
	return v
}
func runProjection[T any](q ProjectionRequest, log *intoLog, c im.Context, makeCursor func(uint, uint64) *projectedView[T], render func(*T) string) any {
	log.fail = q.Fail
	var holder *projectedView[T]
	take := func(value *rustvec.Owned[T]) any {
		if value == nil {
			return nil
		}
		result := render(value.Borrow())
		value.DropWithContext(c)
		return result
	}
	value, failure := caught(func() any {
		holder = makeCursor(q.N, 0)
		for i := uint(0); i < q.Front; i++ {
			if v := holder.next(0, false, false); v != nil {
				v.DropWithContext(c)
			}
		}
		for i := uint(0); i < q.Back; i++ {
			if v := holder.next(0, false, true); v != nil {
				v.DropWithContext(c)
			}
		}
		switch q.Op {
		case "next":
			return take(holder.next(0, false, false))
		case "back":
			return take(holder.next(0, false, true))
		case "nth":
			return take(holder.next(q.Nth, true, false))
		case "nth_back":
			return take(holder.next(q.Nth, true, true))
		case "count":
			owned := holder
			holder = nil
			return owned.count()
		case "last":
			owned := holder
			holder = nil
			return take(owned.last())
		case "close":
			owned := holder
			holder = nil
			owned.close(c)
			return nil
		case "clone":
			other := holder.clone()
			result := other.debug()
			other.close(c)
			return result
		case "clone_from":
			other := makeCursor(q.Other, 100)
			func() {
				defer func() {
					if p := recover(); p != nil {
						unwind := c
						unwind.Unwinding = true
						func() {
							defer func() {
								if recover() != nil {
									unwind.AbortCleanup()
								}
							}()
							other.close(unwind)
						}()
						c.ResumeUnwind(p)
					}
					other.close(c)
				}()
				holder.cloneFrom(other)
			}()
			return nil
		case "inspect":
			return holder.debug()
		default:
			panic("bad op")
		}
	})
	var current, length, hint any
	if holder != nil {
		current = holder.debug()
		length = holder.len()
		h := holder.hint()
		hint = []any{h.Lower, h.Upper}
	}
	events := append([]string{}, log.events...)
	log.fail = ""
	log.events = log.events[:0]
	if holder != nil {
		holder.close(c)
	}
	return map[string]any{"value": value, "panic": failure, "remaining": current, "len": length, "hint": hint, "events": events, "drop": log.events}
}
func RunProjection(q ProjectionRequest) any {
	log := &intoLog{events: []string{}}
	c := im.Context{Unwinding: q.Inherited}
	config := im.Config[intoKey, intoValue]{BucketLayout: im.Layout{Size: 40, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[intoKey]{KeyOps: rusthashmap.KeyOps[intoKey]{Hash: func(k *intoKey, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, k.id) }, Equal: func(a, b *intoKey, _ im.Context) bool { return a.id == b.id }, Drop: func(k *intoKey, c im.Context) { k.log.step(fmt.Sprintf("kdrop:%d", k.id), c) }, Debug: func(k *intoKey, f *rustfmt.Formatter) error { return f.WriteString(fmt.Sprintf("K(%d)", k.id)) }}}, Values: im.ValueOps[intoValue]{ValueOps: rusthashmap.ValueOps[intoValue]{Drop: func(v *intoValue, c im.Context) { v.log.step(fmt.Sprintf("vdrop:%d", v.id), c) }, Debug: func(v *intoValue, f *rustfmt.Formatter) error { return f.WriteString(fmt.Sprintf("V(%d)", v.id)) }}}}
	if q.Kind == "keys" {
		config.Keys.Clone = func(k *intoKey, c im.Context) intoKey { k.log.step(fmt.Sprintf("kclone:%d", k.id), c); return *k }
	} else {
		config.Values.Clone = func(v *intoValue, c im.Context) intoValue { v.log.step(fmt.Sprintf("vclone:%d", v.id), c); return *v }
	}
	makeMap := func(n uint, base uint64) *im.Map[intoKey, intoValue] {
		owner := rustvec.NewOwned(intoBuilder{log}, rustvec.ValueOps[intoBuilder]{Drop: func(s *intoBuilder, c im.Context) { s.log.step("bdrop", c) }})
		b := im.NewOwnedBuilder(owner, im.BuilderOps[intoBuilder]{Build: func(rustvec.Ref[intoBuilder]) *rusthashmap.OwnedHasher {
			return rusthashmap.NewOwnedHasher(&intoHasher{}, nil)
		}}, c)
		m := im.WithCapacityAndHasher(n, config, b, c)
		for i := uint(0); i < n; i++ {
			m.Insert(intoKey{base + uint64(i), log}, intoValue{base + 10 + uint64(i), log}, c)
		}
		return m
	}
	if q.Kind == "keys" {
		return runProjection(q, log, c, func(n uint, base uint64) *projectedView[intoKey] { return keysView(makeMap(n, base).IntoKeys(c), c) }, func(k *intoKey) string { return fmt.Sprintf("K(%d)", k.id) })
	}
	return runProjection(q, log, c, func(n uint, base uint64) *projectedView[intoValue] {
		return valuesView(makeMap(n, base).IntoValues(c), c)
	}, func(v *intoValue) string { return fmt.Sprintf("V(%d)", v.id) })
}
