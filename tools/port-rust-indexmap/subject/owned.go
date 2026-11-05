// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type OwnedRequest struct {
	N, Capacity, Key uint
	Mask             uint64
	Op, Fail         string
	Inherited        bool
	FromOverride     bool `json:"from_override"`
}
type ownedLog struct {
	events []string
	fail   string
	failed bool
}

func (l *ownedLog) step(text string, c im.Context) {
	event := fmt.Sprintf("%s:%t", text, c.Panicking())
	l.events = append(l.events, event)
	if !l.failed && l.fail == event {
		l.failed = true
		c.Panic("fail")
	}
}

type ownedKey struct {
	id, tag uint64
	log     *ownedLog
}
type ownedValue struct {
	id  int64
	tag uint64
	log *ownedLog
}

func dropKey(v *ownedKey, c im.Context)     { v.log.step(fmt.Sprintf("kdrop:%d:%d", v.id, v.tag), c) }
func dropValue(v *ownedValue, c im.Context) { v.log.step(fmt.Sprintf("vdrop:%d:%d", v.id, v.tag), c) }

type ownedBuilder struct {
	log  *ownedLog
	mask uint64
}
type ownedHasher struct {
	log         *ownedLog
	mask, state uint64
	context     im.Context
}

func (b *ownedBuilder) Build(c im.Context) *rusthashmap.OwnedHasher {
	b.log.step("build", c)
	return rusthashmap.NewOwnedHasher(&ownedHasher{log: b.log, mask: b.mask, context: c}, func(_ rusthashmap.Hasher, c im.Context) { b.log.step("hdrop", c) })
}
func (b *ownedBuilder) Clone(c im.Context) rusthashmap.Builder {
	b.log.step("bclone", c)
	return &ownedBuilder{b.log, b.mask}
}
func (b *ownedBuilder) CloneFrom(v rusthashmap.Builder, c im.Context) {
	b.log.step("bclonefrom", c)
	*b = *v.(*ownedBuilder)
}
func (b *ownedBuilder) Close(c im.Context) { b.log.step("bdrop", c) }
func (h *ownedHasher) Write([]byte)        { h.context.Panic("unexpected write") }
func (h *ownedHasher) WriteU64(n uint64) {
	h.log.step(fmt.Sprintf("u64:%d", n), h.context)
	h.state = n
}
func (h *ownedHasher) Finish64() uint64 { h.log.step("finish", h.context); return h.state & h.mask }

type factoryState struct {
	log   *ownedLog
	value *ownedValue
}

func dropFactory(f *factoryState, c im.Context) {
	pending := true
	defer func() {
		if p := recover(); p != nil {
			unwind := c
			unwind.Unwinding = true
			if pending && f.value != nil {
				dropValue(f.value, unwind)
			}
			c.ResumeUnwind(p)
		}
	}()
	f.log.step("fdrop", c)
	pending = false
	if f.value != nil {
		dropValue(f.value, c)
	}
}
func valueOwner(v ownedValue) *rustvec.Owned[ownedValue] {
	return rustvec.NewOwned(v, rustvec.ValueOps[ownedValue]{Drop: dropValue})
}

func RunOwned(q OwnedRequest) any {
	log := &ownedLog{events: []string{}}
	config := im.Config[ownedKey, ownedValue]{BucketLayout: im.Layout{Size: 56, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true,
		Keys: im.KeyOps[ownedKey]{KeyOps: rusthashmap.KeyOps[ownedKey]{
			Hash: func(k *ownedKey, h rusthashmap.Hasher, c im.Context) {
				k.log.step(fmt.Sprintf("hash:%d:%d", k.id, k.tag), c)
				rusthashmap.HashU64(h, k.id)
			},
			Equal: func(a, b *ownedKey, c im.Context) bool {
				a.log.step(fmt.Sprintf("eq:%d:%d", a.id, b.id), c)
				return a.id == b.id
			},
			Clone: func(v *ownedKey, c im.Context) ownedKey {
				v.log.step(fmt.Sprintf("kclone:%d:%d", v.id, v.tag), c)
				return *v
			}, Drop: dropKey,
		}}, Values: im.ValueOps[ownedValue]{ValueOps: rusthashmap.ValueOps[ownedValue]{Clone: func(v *ownedValue, c im.Context) ownedValue {
			v.log.step(fmt.Sprintf("vclone:%d:%d", v.id, v.tag), c)
			return *v
		}, Drop: dropValue}},
	}

	if q.FromOverride {
		config.Keys.CloneFrom = func(destination, source *ownedKey, c im.Context) {
			destination.tag = source.tag
			destination.log.step(fmt.Sprintf("kclonefrom:%d:%d", destination.id, source.id), c)
			destination.id = source.id
		}
		config.Values.CloneFrom = func(destination, source *ownedValue, c im.Context) {
			destination.tag = source.tag
			destination.log.step(fmt.Sprintf("vclonefrom:%d:%d", destination.id, source.id), c)
			destination.id = source.id
		}
	}
	m := im.WithCapacityAndHasher(q.Capacity, config, &ownedBuilder{log, q.Mask})
	for i := uint(0); i < q.N; i++ {
		m.Insert(ownedKey{uint64(i), 100 + uint64(i), log}, ownedValue{int64(i), 200 + uint64(i), log})
	}
	log.events = log.events[:0]
	log.fail = q.Fail
	ctx := im.Context{Unwinding: q.Inherited}
	result, failure := caught(func() any {
		if q.Op == "clone" {
			other := m.Clone(ctx)
			view := other.AsSlice(ctx)
			entries := []any{}
			for i := uint(0); i < view.Len(); i++ {
				k, v, _ := view.GetIndex(i)
				entries = append(entries, []any{k.id, k.tag, v.id, v.tag})
			}
			result := map[string]any{"capacity": other.Capacity(ctx), "entries": entries}
			other.Close(ctx)
			return result
		}
		if q.Op == "clone_from" {
			count := q.Key % 5
			other := im.WithCapacityAndHasher(count, config, &ownedBuilder{log, 7}, ctx)
			defer func() {
				if p := recover(); p != nil {
					unwind := ctx
					unwind.Unwinding = true
					other.Close(unwind)
					ctx.ResumeUnwind(p)
				}
				other.Close(ctx)
			}()
			for i := uint(0); i < count; i++ {
				other.Insert(ownedKey{10 + uint64(i), 1100 + uint64(i), log}, ownedValue{10 + int64(i), 1200 + uint64(i), log}, ctx)
			}
			m.CloneFrom(other, ctx)
			return nil
		}
		e := m.Entry(ownedKey{uint64(q.Key), 900, log}, ctx)
		value := func() ownedValue { return ownedValue{777, 901, log} }
		factory := func() *rustvec.Owned[factoryState] {
			v := value()
			return rustvec.NewOwned(factoryState{log, &v}, rustvec.ValueOps[factoryState]{Drop: dropFactory})
		}
		switch q.Op {
		case "close":
			e.Close(ctx)
			return nil
		case "or_insert":
			v := e.OrInsertOwned(valueOwner(value()), ctx)
			return []any{v.id, v.tag}
		case "with":
			v := e.OrInsertWithOwned(rusthashmap.NewOnce(factory(), func(f *factoryState, _ struct{}, c im.Context) *rustvec.Owned[ownedValue] {
				f.log.step("fcall", c)
				v := *f.value
				f.value = nil
				return valueOwner(v)
			}), ctx)
			return []any{v.id, v.tag}
		case "with_key":
			v := e.OrInsertWithKeyOwned(rusthashmap.NewOnce(factory(), func(f *factoryState, key *ownedKey, c im.Context) *rustvec.Owned[ownedValue] {
				f.log.step(fmt.Sprintf("fcall-key:%d:%d", key.id, key.tag), c)
				v := *f.value
				f.value = nil
				return valueOwner(v)
			}), ctx)
			return []any{v.id, v.tag}
		case "modify":
			next := e.AndModifyOwned(rusthashmap.NewOnce(factory(), func(f *factoryState, v *ownedValue, c im.Context) struct{} {
				f.log.step(fmt.Sprintf("fcall-mut:%d:%d", v.id, v.tag), c)
				v.id = f.value.id
				return struct{}{}
			}), ctx)
			r := []any{next.Index(ctx), next.Key(ctx).id, next.Key(ctx).tag}
			next.Close(ctx)
			return r
		case "insert_entry":
			o := e.InsertEntryOwned(valueOwner(value()), ctx)
			r := []any{o.Index(ctx), o.Get(ctx).id, o.Get(ctx).tag}
			o.Close(ctx)
			return r
		case "occupied_insert":
			if o, ok := e.TakeOccupied(ctx); ok {
				old := o.InsertOwned(valueOwner(value()), ctx)
				v := old.Borrow()
				r := []any{v.id, v.tag, o.Get(ctx).id, o.Get(ctx).tag}
				old.DropWithContext(ctx)
				o.Close(ctx)
				return r
			}
			e.Close(ctx)
			return nil
		case "swap_remove", "shift_remove":
			if o, ok := e.TakeOccupied(ctx); ok {
				var pair im.Pair[ownedKey, ownedValue]
				if q.Op == "swap_remove" {
					pair = o.SwapRemoveEntryOwned(ctx).Take()
				} else {
					pair = o.ShiftRemoveEntryOwned(ctx).Take()
				}
				r := []any{pair.Key.id, pair.Key.tag, pair.Value.id, pair.Value.tag}
				v := valueOwner(pair.Value)
				defer func() {
					if p := recover(); p != nil {
						unwind := ctx
						unwind.Unwinding = true
						v.DropWithContext(unwind)
						ctx.ResumeUnwind(p)
					}
					v.DropWithContext(ctx)
				}()
				dropKey(&pair.Key, ctx)
				return r
			}
			e.Close(ctx)
			return nil
		case "into_key":
			if v, ok := e.TakeVacant(ctx); ok {
				k := v.IntoKey(ctx)
				r := []any{k.id, k.tag}
				dropKey(&k, ctx)
				return r
			}
			e.Close(ctx)
			return nil
		case "vacant_insert":
			if v, ok := e.TakeVacant(ctx); ok {
				value := v.InsertOwned(valueOwner(value()), ctx)
				return []any{value.id, value.tag}
			}
			e.Close(ctx)
			return nil
		default:
			panic("bad op")
		}
	})
	entries := []any{}
	view := m.AsSlice()
	for i := uint(0); i < view.Len(); i++ {
		k, v, _ := view.GetIndex(i)
		entries = append(entries, []any{k.id, k.tag, v.id, v.tag})
	}
	capacity := m.Capacity()
	log.fail = ""
	events := append([]string{}, log.events...)
	log.events = log.events[:0]
	m.Close()
	return map[string]any{"value": result, "panic": failure, "entries": entries, "capacity": capacity, "events": events, "drop": log.events}
}
