// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"encoding/json"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type DrainRequest struct {
	Trace bool
	Infos map[string]rustfmt.SourcePanicInfo

	N, Capacity, Front, Back, Nth uint
	Mask                          uint64
	Start, End                    [2]uint
	AgainStart                    [2]uint `json:"again_start"`
	AgainEnd                      [2]uint `json:"again_end"`
	Op, Fail                      string
	Inherited, Processing         bool
}

type drainLog struct {
	events        []string
	fail          string
	failed, trace bool
	info          rustfmt.SourcePanicInfo
}

func (l *drainLog) step(text string, c im.Context) {
	event := fmt.Sprintf("%s:%t", text, c.Panicking())
	l.events = append(l.events, event)
	if l.trace {
		data, _ := json.Marshal(event)
		fmt.Println("E:" + string(data))
	}
	if !l.failed && l.fail == event {
		l.failed = true
		if l.trace {
			c.PanicAt(l.info)
		} else {
			c.Panic("fail")
		}
	}
}

type drainKey struct {
	id  uint64
	log *drainLog
}
type drainValue struct {
	id  uint64
	log *drainLog
}
type drainBuilder struct {
	log  *drainLog
	mask uint64
}
type drainHasher struct {
	log         *drainLog
	mask, state uint64
	c           im.Context
}

func (h *drainHasher) Write([]byte)      { h.c.Panic("unexpected write") }
func (h *drainHasher) WriteU64(n uint64) { h.log.step(fmt.Sprintf("u64:%d", n), h.c); h.state = n }
func (h *drainHasher) Finish64() uint64  { h.log.step("finish", h.c); return h.state & h.mask }

type drainRange struct {
	log          *drainLog
	starts, ends uint
	start, end   [2][2]uint
}

func rangeBound(v *[2]uint) im.Bound { return im.Bound{Kind: im.BoundKind(v[0]), Value: &v[1]} }
func RunDrain(q DrainRequest) any {
	log := &drainLog{events: []string{}}
	c := im.Context{Unwinding: q.Inherited || q.Processing, ProcessingPanic: q.Processing}
	if q.Trace {
		ops := &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{}}
		if info, ok := q.Infos["panic in a destructor during cleanup"]; ok {
			ops.CleanupAbort = &info
		}
		for payload, info := range q.Infos {
			if payload != "fail" && info.Kind == rustfmt.PanicOwnedString && info.Location != nil {
				for _, site := range []string{im.IndexMapRangeStartPanicSite, im.IndexMapRangeEndPanicSite, im.IndexMapRangeOrderPanicSite} {
					ops.Locations[site] = *info.Location
				}
			}
		}
		ops.Hook = func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) {
			data, _ := json.Marshal(info)
			fmt.Println("H:" + string(data))
		}
		c = rustfmt.WithPanicOperations(c, ops)
	}
	config := im.Config[drainKey, drainValue]{BucketLayout: im.Layout{Size: 40, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true,
		Keys: im.KeyOps[drainKey]{KeyOps: rusthashmap.KeyOps[drainKey]{Hash: func(k *drainKey, h rusthashmap.Hasher, c im.Context) {
			k.log.step(fmt.Sprintf("hash:%d", k.id), c)
			rusthashmap.HashU64(h, k.id)
		}, Equal: func(a, b *drainKey, c im.Context) bool {
			a.log.step(fmt.Sprintf("eq:%d:%d", a.id, b.id), c)
			return a.id == b.id
		}, Drop: func(k *drainKey, c im.Context) { k.log.step(fmt.Sprintf("kdrop:%d", k.id), c) }}},
		Values: im.ValueOps[drainValue]{ValueOps: rusthashmap.ValueOps[drainValue]{Drop: func(v *drainValue, c im.Context) { v.log.step(fmt.Sprintf("vdrop:%d", v.id), c) }}}}
	b := im.NewOwnedBuilder(rustvec.NewOwned(drainBuilder{log, q.Mask}, rustvec.ValueOps[drainBuilder]{Drop: func(b *drainBuilder, c im.Context) { b.log.step("bdrop", c) }}), im.BuilderOps[drainBuilder]{Build: func(b rustvec.Ref[drainBuilder]) *rusthashmap.OwnedHasher {
		b.Value.log.step("build", b.Context)
		return rusthashmap.NewOwnedHasher(&drainHasher{log: b.Value.log, mask: b.Value.mask, c: b.Context}, func(_ rusthashmap.Hasher, c im.Context) { log.step("hdrop", c) })
	}}, c)
	m := im.WithCapacityAndHasher(q.Capacity, config, b, c)
	for i := uint(0); i < q.N; i++ {
		m.Insert(drainKey{uint64(i), log}, drainValue{10 + uint64(i), log}, c)
	}
	log.events = log.events[:0]
	log.fail = q.Fail
	log.trace = q.Trace
	log.info = q.Infos["fail"]
	var holder *im.Drain[drainKey, drainValue]
	forgotten := false
	pairs := func(s im.Slice[drainKey, drainValue]) any {
		result := []any{}
		for i := uint(0); i < s.Len(); i++ {
			k, v, _ := s.GetIndex(i)
			result = append(result, []any{k.id, v.id})
		}
		return result
	}
	take := func(value *rustvec.Owned[im.Pair[drainKey, drainValue]]) any {
		if value == nil {
			return nil
		}
		p := value.Borrow()
		result := []any{p.Key.id, p.Value.id}
		value.DropWithContext(c)
		return result
	}
	value, failure := caught(func() any {
		if q.Op == "truncate" {
			m.Truncate(q.Nth, c)
			return nil
		}
		r := im.NewOwnedRange(rustvec.NewOwned(drainRange{log: log, start: [2][2]uint{q.Start, q.AgainStart}, end: [2][2]uint{q.End, q.AgainEnd}}, rustvec.ValueOps[drainRange]{Drop: func(r *drainRange, c im.Context) { r.log.step("rdrop", c) }}), im.RangeOps[drainRange]{StartBound: func(r rustvec.Ref[drainRange]) im.Bound {
			n := r.Value.starts
			r.Value.starts++
			r.Value.log.step(fmt.Sprintf("start:%d", n), r.Context)
			if n > 1 {
				n = 1
			}
			return rangeBound(&r.Value.start[n])
		}, EndBound: func(r rustvec.Ref[drainRange]) im.Bound {
			n := r.Value.ends
			r.Value.ends++
			r.Value.log.step(fmt.Sprintf("end:%d", n), r.Context)
			if n > 1 {
				n = 1
			}
			return rangeBound(&r.Value.end[n])
		}}, c)
		if q.Op == "get" {
			s, ok := m.GetRangeOwned(r, c)
			if !ok {
				return nil
			}
			return pairs(s)
		}
		if q.Op == "get_mut" {
			s, ok := m.GetRangeMutOwned(r, c)
			if !ok {
				return nil
			}
			for i := uint(0); i < s.Len(); i++ {
				_, v, _ := s.GetIndexMut(i, c)
				v.id += 1000
			}
			return pairs(s)
		}
		holder = m.DrainOwned(r, c)
		for i := uint(0); i < q.Front; i++ {
			if p := holder.NextOwned(c); p != nil {
				p.DropWithContext(c)
			}
		}
		for i := uint(0); i < q.Back; i++ {
			if p := holder.NextBackOwned(c); p != nil {
				p.DropWithContext(c)
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
			d := holder
			holder = nil
			return d.Count(c)
		case "last":
			d := holder
			holder = nil
			return take(d.LastOwned(c))
		case "close":
			d := holder
			holder = nil
			d.Close(c)
			return nil
		case "forget":
			d := holder
			holder = nil
			d.Forget(c)
			forgotten = true
			return nil
		case "inspect":
			return pairs(holder.AsSlice(c))
		default:
			panic("bad op")
		}
	})
	var remaining, length, hint any
	if holder != nil {
		remaining = pairs(holder.AsSlice(c))
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
	closeEvents := append([]string{}, log.events...)
	log.events = log.events[:0]
	after := pairs(m.AsSlice(c))
	capacity := m.Capacity(c)
	queries := []any{}
	for i := uint(0); i < q.N+2; i++ {
		k := drainKey{uint64(i), log}
		query := func() any {
			v, ok := m.Get(&k, c)
			if ok {
				return v.id
			}
			return nil
		}
		if forgotten {
			value, failure := caught(query)
			queries = append(queries, map[string]any{"value": value, "panic": failure})
		} else {
			queries = append(queries, query())
		}
	}

	queryEvents := append([]string{}, log.events...)
	log.events = log.events[:0]
	m.Close(c)
	return map[string]any{"value": value, "panic": failure, "remaining": remaining, "len": length, "hint": hint, "events": events, "close_events": closeEvents, "map": after, "capacity": capacity, "queries": queries, "query_events": queryEvents, "drop_events": log.events}
}
