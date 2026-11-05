// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type BuilderRequest struct {
	Kind, Mode, Scope, Failure string
	Capacity                   uint
	SourceCapacity             uint `json:"source_capacity"`
}
type builderState struct {
	id, counter uint64
	data        [][]byte
}
type opaqueState struct{ builderState }
type cloningState struct{ builderState }
type customState struct{ builderState }
type buildingState struct{ builderState }
type noHash []byte

func freshState(id uint64) builderState {
	return builderState{id: id, data: [][]byte{{byte(id)}, {2, 3}}}
}
func stateSnapshot(s *builderState) any {
	return map[string]any{"id": s.id, "counter": s.counter, "data": []any{[]any{s.data[0][0]}, []any{s.data[1][0], s.data[1][1]}}}
}
func copiedData(data [][]byte) [][]byte {
	result := make([][]byte, len(data))
	for i := range data {
		result[i] = append([]byte{}, data[i]...)
	}
	return result
}
func observerScope(c im.Context) string {
	if c.Runtime == nil || c.Runtime.Value == nil {
		return "none"
	}
	return c.Runtime.Value.(string)
}
func sourceContext(scope string) im.Context {
	if scope == "none" {
		return im.Context{}
	}
	return im.Context{Runtime: &rustfmt.RuntimeMetadata{Value: scope}}
}
func withBuilderState[S any](q BuilderRequest, wrap func(builderState) S, access func(*S) *builderState) any {
	events := []string{}
	ctx := sourceContext(q.Scope)
	stale := sourceContext("stale-constructor")
	drop := func(value *S, c im.Context) {
		s := access(value)
		id := s.id
		events = append(events, fmt.Sprintf("drop:%d:%s:%t", id, observerScope(c), c.Panicking()))
		s.id += 10000
		if id == 1 && q.Failure == "drop" {
			c.Panic("drop")
		}
	}
	owner := func(value S) *rustvec.Owned[S] {
		return rustvec.NewOwned(value, rustvec.ValueOps[S]{Drop: drop}).MoveWithContext(stale)
	}
	ops := im.BuilderOps[S]{}
	if q.Kind == "default" || q.Kind == "custom" {
		ops.Clone = func(ref rustvec.Ref[S]) *rustvec.Owned[S] {
			s := access(ref.Value)
			s.counter++
			events = append(events, fmt.Sprintf("clone:%d:%s:%t", s.id, observerScope(ref.Context), ref.Context.Panicking()))
			if q.Failure == "clone" {
				ref.Context.Panic("clone")
			}
			return owner(wrap(builderState{id: s.id + 100, counter: s.counter, data: copiedData(s.data)}))
		}
	}
	if q.Kind == "custom" {
		ops.CloneFrom = func(destination, source rustvec.Ref[S]) {
			d, s := access(destination.Value), access(source.Value)
			d.id = s.id + 1000
			d.counter++
			events = append(events, fmt.Sprintf("clone_from:%d:%s:%t", d.id, observerScope(destination.Context), destination.Context.Panicking()))
			if q.Failure == "clone_from" {
				destination.Context.Panic("clone_from")
			}
			d.data = copiedData(s.data)
		}
	}
	newBuilder := func(id uint64) *im.OwnedBuilder[S] { return im.NewOwnedBuilder(owner(wrap(freshState(id))), ops, ctx) }
	if q.Kind == "building" {
		ops.Build = func(ref rustvec.Ref[S]) *rusthashmap.OwnedHasher {
			s := access(ref.Value)
			s.counter++
			events = append(events, fmt.Sprintf("build:%d:%s:%t", s.id, observerScope(ref.Context), ref.Context.Panicking()))
			return rusthashmap.NewOwnedHasher(&builderHasher{events: &events, context: ref.Context}, func(h rusthashmap.Hasher, c im.Context) {
				events = append(events, fmt.Sprintf("hdrop:%d:%s:%t", h.(*builderHasher).state, observerScope(c), c.Panicking()))
			})
		}
		config := im.Config[uint64, uint64]{BucketLayout: im.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ im.Context) bool { return *a == *b }, TrivialClone: true}}, Values: im.ValueOps[uint64]{ValueOps: rusthashmap.ValueOps[uint64]{TrivialClone: true}}}
		builder := newBuilder(1)
		m := im.WithCapacityAndHasher(q.Capacity, config, builder, ctx)
		for i := uint64(0); i < 3; i++ {
			m.Insert(i*17+1, i, ctx)
		}
		key := uint64(18)
		v, _ := m.Get(&key, ctx)
		entries := []any{}
		for i := uint(0); i < m.Len(ctx); i++ {
			k, v, _ := m.GetIndex(i, ctx)
			entries = append(entries, []any{*k, *v})
		}
		result := map[string]any{"capacity": m.Capacity(ctx), "len": m.Len(ctx), "get": *v, "state": stateSnapshot(access(builder.Borrow(ctx))), "entries": entries}
		m.Close(ctx)
		return map[string]any{"value": result, "events": events}
	}
	config := im.Config[noHash, noHash]{BucketLayout: im.Layout{Size: 56, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true}
	if q.Kind != "opaque" {
		config.Keys.Clone = func(v *noHash, _ im.Context) noHash { return append(noHash{}, (*v)...) }
		config.Values.Clone = func(v *noHash, _ im.Context) noHash { return append(noHash{}, (*v)...) }
	}
	targetBuilder := newBuilder(1)
	target := im.WithCapacityAndHasher(q.Capacity, config, targetBuilder, ctx)
	if q.Kind == "opaque" {
		result := map[string]any{"capacity": target.Capacity(ctx), "len": target.Len(ctx), "iter": target.Iter(ctx).Count(ctx), "keys": target.Keys(ctx).Count(ctx), "values": target.Values(ctx).Count(ctx), "slice": target.AsSlice(ctx).Len(), "state": stateSnapshot(access(targetBuilder.Borrow(ctx)))}
		target.Close(ctx)
		return map[string]any{"value": result, "events": events}
	}
	sourceBuilder := newBuilder(2)
	source := im.WithCapacityAndHasher(q.SourceCapacity, config, sourceBuilder, ctx)
	anchor := targetBuilder.Borrow(ctx)
	var cloned *im.Map[noHash, noHash]
	_, failure := caught(func() any {
		if q.Mode == "clone" {
			cloned = source.Clone(ctx)
		} else {
			target.CloneFrom(source, ctx)
		}
		return nil
	})
	if cloned != nil {
		value := cloned.Hasher(ctx).(*im.OwnedBuilder[S])
		access(value.Borrow(ctx)).data[0][0]++
	}
	snapshot := func(m *im.Map[noHash, noHash]) any {
		return map[string]any{"capacity": m.Capacity(ctx), "state": stateSnapshot(access(m.Hasher(ctx).(*im.OwnedBuilder[S]).Borrow(ctx)))}
	}
	var cloneSnapshot any
	if cloned != nil {
		cloneSnapshot = snapshot(cloned)
	}
	result := map[string]any{"failed": failure != nil, "target": snapshot(target), "source": snapshot(source), "cloned": cloneSnapshot, "same_cell": anchor == targetBuilder.Borrow(ctx)}
	if cloned != nil {
		cloned.Close(ctx)
	}
	source.Close(ctx)
	target.Close(ctx)
	return map[string]any{"value": result, "events": events}
}

type builderHasher struct {
	events  *[]string
	context im.Context
	state   uint64
}

func (h *builderHasher) Write([]byte) { h.context.Panic("unexpected bytes") }
func (h *builderHasher) WriteU64(v uint64) {
	*h.events = append(*h.events, fmt.Sprintf("u64:%d:%s", v, observerScope(h.context)))
	h.state = v
}
func (h *builderHasher) Finish64() uint64 {
	*h.events = append(*h.events, fmt.Sprintf("finish:%d:%s", h.state, observerScope(h.context)))
	return h.state
}
func RunBuilder(q BuilderRequest) any {
	switch q.Kind {
	case "opaque":
		return withBuilderState(q, func(s builderState) opaqueState { return opaqueState{s} }, func(s *opaqueState) *builderState { return &s.builderState })
	case "default":
		return withBuilderState(q, func(s builderState) cloningState { return cloningState{s} }, func(s *cloningState) *builderState { return &s.builderState })
	case "custom":
		return withBuilderState(q, func(s builderState) customState { return customState{s} }, func(s *customState) *builderState { return &s.builderState })
	case "building":
		return withBuilderState(q, func(s builderState) buildingState { return buildingState{s} }, func(s *buildingState) *builderState { return &s.builderState })
	default:
		panic("bad kind")
	}
}
