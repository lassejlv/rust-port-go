// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type IndexedRequest struct {
	N, Capacity, A, B uint
	Mask              uint64
	Method, Op, Fail  string
	Inherited         bool
}

func RunIndexed(q IndexedRequest) any {
	log := &drainLog{events: []string{}}
	c := im.Context{Unwinding: q.Inherited}
	config := im.Config[drainKey, drainValue]{BucketLayout: im.Layout{Size: 40, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true,
		Keys: im.KeyOps[drainKey]{KeyOps: rusthashmap.KeyOps[drainKey]{Hash: func(k *drainKey, h rusthashmap.Hasher, c im.Context) {
			k.log.step(fmt.Sprintf("hash:%d", k.id), c)
			rusthashmap.HashU64(h, k.id)
		}, Equal: func(a, b *drainKey, c im.Context) bool {
			a.log.step(fmt.Sprintf("eq:%d:%d", a.id, b.id), c)
			return a.id == b.id
		}, Drop: func(k *drainKey, c im.Context) { k.log.step(fmt.Sprintf("kdrop:%d", k.id), c) }}}, Values: im.ValueOps[drainValue]{ValueOps: rusthashmap.ValueOps[drainValue]{Drop: func(v *drainValue, c im.Context) { v.log.step(fmt.Sprintf("vdrop:%d", v.id), c) }}}}
	drop := func(b *drainBuilder, c im.Context) { b.log.step("bdrop", c) }
	builder := im.NewOwnedBuilder(rustvec.NewOwned(drainBuilder{log, q.Mask}, rustvec.ValueOps[drainBuilder]{Drop: drop}), im.BuilderOps[drainBuilder]{Build: func(b rustvec.Ref[drainBuilder]) *rusthashmap.OwnedHasher {
		b.Value.log.step("build", b.Context)
		return rusthashmap.NewOwnedHasher(&drainHasher{log: b.Value.log, mask: b.Value.mask, c: b.Context}, func(_ rusthashmap.Hasher, c im.Context) { log.step("hdrop", c) })
	}, Clone: func(b rustvec.Ref[drainBuilder]) *rustvec.Owned[drainBuilder] {
		b.Value.log.step("bclone", b.Context)
		return rustvec.NewOwned(*b.Value, rustvec.ValueOps[drainBuilder]{Drop: drop})
	}}, c)
	m := im.WithCapacityAndHasher(q.Capacity, config, builder, c)
	for id := uint(0); id < q.N; id++ {
		m.Insert(drainKey{uint64(id), log}, drainValue{uint64(id) + 10, log}, c)
	}
	log.events = log.events[:0]
	log.fail = q.Fail

	var holder *im.IndexedEntry[drainKey, drainValue]
	value, failure := caught(func() any {
		switch q.Method {
		case "first":
			holder, _ = m.FirstEntry(c)
		case "last":
			holder, _ = m.LastEntry(c)
		case "occupied":
			e := m.Entry(drainKey{uint64(q.A), log}, c)
			if o, ok := e.TakeOccupied(c); ok {
				holder = o.IntoIndexed(c)
			} else {
				e.Close(c)
			}
		default:
			holder, _ = m.GetIndexEntry(q.A, c)
		}
		if holder == nil {
			return nil
		}
		takePair := func(p *rustvec.Owned[im.Pair[drainKey, drainValue]]) any {
			value := p.Borrow()
			result := []any{value.Key.id, value.Value.id}
			p.DropWithContext(c)
			return result
		}
		switch q.Op {
		case "get":
			return []any{holder.Index(c), holder.Key(c).id, holder.Get(c).id}
		case "mutate":
			v := holder.GetMut(c)
			v.id += 1000
			return v.id
		case "insert":
			old := holder.InsertOwned(rustvec.NewOwned(drainValue{1000, log}, rustvec.ValueOps[drainValue]{Drop: config.Values.Drop}), c)
			result := old.Borrow().id
			old.DropWithContext(c)
			return result
		case "into_mut":
			e := holder
			holder = nil
			v := e.IntoMut(c)
			v.id += 1000
			return v.id
		case "swap_pair":
			e := holder
			holder = nil
			return takePair(e.SwapRemoveEntryOwned(c))
		case "shift_pair":
			e := holder
			holder = nil
			return takePair(e.ShiftRemoveEntryOwned(c))
		case "swap_value":
			e := holder
			holder = nil
			v := e.SwapRemoveOwned(c)
			result := v.Borrow().id
			v.DropWithContext(c)
			return result
		case "shift_value":
			e := holder
			holder = nil
			v := e.ShiftRemoveOwned(c)
			result := v.Borrow().id
			v.DropWithContext(c)
			return result
		case "move":
			e := holder
			holder = nil
			e.MoveIndex(q.B, c)
		case "swap":
			e := holder
			holder = nil
			e.SwapIndices(q.B, c)
		default:
			panic("bad op")
		}
		return nil
	})
	var held any
	if holder != nil {
		held = []any{holder.Index(c), holder.Key(c).id, holder.Get(c).id}
		holder.Close(c)
	}
	events := append([]string{}, log.events...)
	log.fail = ""
	log.events = log.events[:0]
	entries := []any{}
	s := m.AsSlice(c)
	for i := uint(0); i < s.Len(); i++ {
		k, v, _ := s.GetIndex(i)
		entries = append(entries, []any{k.id, v.id})
	}
	capacity := m.Capacity(c)
	queries := []any{}
	for id := uint(0); id < q.N+2; id++ {
		key := drainKey{uint64(id), log}
		v, ok := m.Get(&key, c)
		if ok {
			queries = append(queries, v.id)
		} else {
			queries = append(queries, nil)
		}
	}
	queryEvents := append([]string{}, log.events...)
	log.events = log.events[:0]
	m.Close(c)
	return map[string]any{"value": value, "panic": failure, "held": held, "events": events, "entries": entries, "capacity": capacity, "queries": queries, "query_events": queryEvents, "drop": log.events}
}
