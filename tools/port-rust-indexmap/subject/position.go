// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type PositionRequest struct {
	N, Capacity, A, B uint
	Mask              uint64
	Op, Fail          string
	Inherited         bool
}

func RunPosition(q PositionRequest) any {
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
	var right *im.Map[drainKey, drainValue]
	_, failed := caught(func() any {
		switch q.Op {
		case "split":
			right = m.SplitOff(q.A, c)
		case "move":
			m.MoveIndex(q.A, q.B, c)
		case "swap":
			m.SwapIndices(q.A, q.B, c)
		default:
			panic("bad op")
		}
		return nil
	})
	events := append([]string{}, log.events...)
	log.fail = ""
	log.events = log.events[:0]
	pairs := func(m *im.Map[drainKey, drainValue]) any {
		result := []any{}
		s := m.AsSlice(c)
		for i := uint(0); i < s.Len(); i++ {
			k, v, _ := s.GetIndex(i)
			result = append(result, []any{k.id, v.id})
		}
		return result
	}
	queries := func(m *im.Map[drainKey, drainValue]) any {
		result := []any{}
		for id := uint(0); id < q.N+2; id++ {
			k := drainKey{uint64(id), log}
			v, ok := m.Get(&k, c)
			if ok {
				result = append(result, v.id)
			} else {
				result = append(result, nil)
			}
		}
		return result
	}
	left, leftCap, leftQueries := pairs(m), m.Capacity(c), queries(m)
	var rightData, rightCap, rightQueries any
	if right != nil {
		rightData = pairs(right)
		rightCap = right.Capacity(c)
		rightQueries = queries(right)
	}
	queryEvents := append([]string{}, log.events...)
	log.events = log.events[:0]
	if right != nil {
		right.Close(c)
	}
	m.Close(c)
	return map[string]any{"panic": failed, "events": events, "left": left, "left_cap": leftCap, "right": rightData, "right_cap": rightCap, "left_queries": leftQueries, "right_queries": rightQueries, "query_events": queryEvents, "drop": log.events}
}
