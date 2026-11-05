// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type IntoRequest struct {
	N, Front, Back, Nth, Other uint
	Op, Fail                   string
	Inherited                  bool
}
type intoLog struct {
	events []string
	fail   string
	failed bool
}

func (l *intoLog) step(text string, c im.Context) {
	event := fmt.Sprintf("%s:%t", text, c.Panicking())
	l.events = append(l.events, event)
	if !l.failed && l.fail == event {
		l.failed = true
		c.Panic("fail")
	}
}

type intoKey struct {
	id  uint64
	log *intoLog
}
type intoValue struct {
	id  uint64
	log *intoLog
}
type intoBuilder struct{ log *intoLog }
type intoHasher struct{ state uint64 }

func (h *intoHasher) Write([]byte)      { panic("unexpected write") }
func (h *intoHasher) WriteU64(n uint64) { h.state = n }
func (h *intoHasher) Finish64() uint64  { return h.state }
func RunInto(q IntoRequest) any {
	log := &intoLog{events: []string{}}
	c := im.Context{Unwinding: q.Inherited}
	config := im.Config[intoKey, intoValue]{BucketLayout: im.Layout{Size: 40, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true,
		Keys:   im.KeyOps[intoKey]{KeyOps: rusthashmap.KeyOps[intoKey]{Hash: func(k *intoKey, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, k.id) }, Equal: func(a, b *intoKey, _ im.Context) bool { return a.id == b.id }, Clone: func(k *intoKey, c im.Context) intoKey { k.log.step(fmt.Sprintf("kclone:%d", k.id), c); return *k }, Drop: func(k *intoKey, c im.Context) { k.log.step(fmt.Sprintf("kdrop:%d", k.id), c) }}},
		Values: im.ValueOps[intoValue]{ValueOps: rusthashmap.ValueOps[intoValue]{Clone: func(v *intoValue, c im.Context) intoValue { v.log.step(fmt.Sprintf("vclone:%d", v.id), c); return *v }, Drop: func(v *intoValue, c im.Context) { v.log.step(fmt.Sprintf("vdrop:%d", v.id), c) }}},
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
	m := makeMap(q.N, 0)
	log.events = log.events[:0]
	log.fail = q.Fail
	var holder *im.IntoIter[intoKey, intoValue]
	take := func(value *rustvec.Owned[im.Pair[intoKey, intoValue]]) any {
		if value == nil {
			return nil
		}
		pair := value.Borrow()
		result := []any{pair.Key.id, pair.Value.id}
		value.DropWithContext(c)
		return result
	}
	remaining := func(it *im.IntoIter[intoKey, intoValue]) any {
		s := it.AsSlice(c)
		result := []any{}
		for i := uint(0); i < s.Len(); i++ {
			k, v, _ := s.GetIndex(i)
			result = append(result, []any{k.id, v.id})
		}
		return result
	}
	value, failure := caught(func() any {
		holder = m.IntoIter(c)
		for i := uint(0); i < q.Front; i++ {
			if item := holder.NextOwned(c); item != nil {
				item.DropWithContext(c)
			}
		}
		for i := uint(0); i < q.Back; i++ {
			if item := holder.NextBackOwned(c); item != nil {
				item.DropWithContext(c)
			}
		}
		switch q.Op {
		case "next":
			return take(holder.NextOwned(c))
		case "back":
			return take(holder.NextBackOwned(c))
		case "nth":
			return take(holder.NthOwned(q.Nth, c))
		case "nth_back":
			return take(holder.NthBackOwned(q.Nth, c))
		case "count":
			owned := holder
			holder = nil
			return owned.Count(c)
		case "last":
			owned := holder
			holder = nil
			return take(owned.LastOwned(c))
		case "close":
			owned := holder
			holder = nil
			owned.Close(c)
			return nil
		case "clone":
			other := holder.Clone(c)
			result := remaining(other)
			other.Close(c)
			return result
		case "clone_from":
			other := makeMap(q.Other, 100).IntoIter(c)
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
							other.Close(unwind)
						}()
						c.ResumeUnwind(p)
					}
					other.Close(c)
				}()
				holder.CloneFrom(other, c)
			}()
			return nil
		case "mutate":
			s := holder.AsMutSlice(c)
			for i := uint(0); i < s.Len(); i++ {
				_, v, _ := s.GetIndexMut(i, c)
				v.id += 1000
			}
			return remaining(holder)
		case "inspect":
			return remaining(holder)
		default:
			panic("bad op")
		}
	})
	var current, length, hint any
	if holder != nil {
		current = remaining(holder)
		length = holder.Len(c)
		h := holder.SizeHint(c)
		hint = []any{h.Lower, h.Upper}
	}
	events := append([]string{}, log.events...)
	log.fail = ""
	log.events = log.events[:0]
	if holder != nil {
		holder.Close(c)
	}
	return map[string]any{"value": value, "panic": failure, "remaining": current, "len": length, "hint": hint, "events": events, "drop": log.events}
}
