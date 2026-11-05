// SPDX-License-Identifier: MIT OR Apache-2.0
package subject

import (
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	hashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type Request struct {
	Capacity uint
	Ops      []Operation
}
type Operation struct {
	Op    string
	Key   uint64
	Value uint32
	N     uint
}

func key(n uint64) span.StableCrateId {
	return (span.DefPathHash{Fingerprint: ds.NewFingerprint(hashes.NewHash64(n), hashes.NewHash64(0))}).StableCrateId()
}
func pairs(m *span.StableCrateIdMap) []any {
	result := []any{}
	view := m.AsSlice()
	for i := uint(0); i < view.Len(); i++ {
		k, v, _ := view.GetIndex(i)
		result = append(result, []any{k.AsU64(), v.AsU32()})
	}
	return result
}
func Run(q Request) any {
	m := span.NewStableCrateIdMapWithCapacity(q.Capacity)
	defer m.Close()
	rows := []any{}
	c := rustfmt.Context{}
	for _, op := range q.Ops {
		k := key(op.Key)
		v := span.CrateNumFromU32(op.Value)
		var value any
		switch op.Op {
		case "insert":
			i, old, ok := m.InsertFull(k, v)
			var previous any
			if ok {
				previous = old.AsU32()
			}
			value = []any{i, previous}
		case "get":
			if i, k, v, ok := m.GetFull(&k); ok {
				value = []any{i, k.AsU64(), v.AsU32()}
			}
		case "entry":
			e := m.Entry(k)
			value = []any{e.Index(c), e.Key(c).AsU64(), e.Kind() == im.Occupied}
			e.Close(c)
		case "or_insert":
			value = m.Entry(k).OrInsert(v, c).AsU32()
		case "with":
			value = m.Entry(k).OrInsertWithOwned(rusthashmap.PlainOnce(func(struct{}, rustfmt.Context) *rustvec.Owned[span.CrateNum] { return rustvec.Own(v) }), c).AsU32()
		case "modify":
			e := m.Entry(k).AndModify(func(old *span.CrateNum) { *old = v }, c)
			value = []any{e.Index(c), e.Key(c).AsU64()}
			e.Close(c)
		case "insert_entry":
			e := m.Entry(k).InsertEntry(v, c)
			value = []any{e.Index(c), e.Key(c).AsU64(), e.Get(c).AsU32()}
			e.Close(c)
		case "swap_remove":
			if v, ok := m.SwapRemove(&k); ok {
				value = v.AsU32()
			}
		case "shift_remove":
			if v, ok := m.ShiftRemove(&k); ok {
				value = v.AsU32()
			}
		case "swap_index":
			if p, ok := m.SwapRemoveIndex(op.N); ok {
				value = []any{p.Key.AsU64(), p.Value.AsU32()}
			}
		case "shift_index":
			if p, ok := m.ShiftRemoveIndex(op.N); ok {
				value = []any{p.Key.AsU64(), p.Value.AsU32()}
			}
		case "reverse":
			m.Reverse()
		case "clear":
			m.Clear()
		case "pop":
			if p, ok := m.Pop(); ok {
				value = []any{p.Key.AsU64(), p.Value.AsU32()}
			}
		case "reserve":
			m.Reserve(op.N)
		case "reserve_exact":
			m.ReserveExact(op.N)
		case "clone":
			other := m.Clone()
			value = map[string]any{"capacity": other.Capacity(), "entries": pairs(other), "equal": m.Equal(other)}
			other.Close()
		case "clone_from":
			other := span.NewStableCrateIdMapWithCapacity(op.N)
			for i := uint(0); i < op.N; i++ {
				other.Insert(key(uint64(i)*17+1), span.CrateNumFromU32(uint32(i)+1))
			}
			m.CloneFrom(other)
			other.Close()
		case "truncate":
			m.Truncate(op.N)
		case "drain", "drain_count", "get_range":
			start := op.N
			if start > m.Len() {
				start = m.Len()
			}
			end := start + uint(op.Value)%(m.Len()-start+1)
			r := im.NumericRange(im.BoundValue{Kind: im.Included, Value: start}, im.BoundValue{Kind: im.Excluded, Value: end})
			if op.Op == "get_range" {
				view, ok := m.GetRangeOwned(r)
				if !ok {
					panic("valid alias range rejected")
				}
				result := []any{}
				for i := uint(0); i < view.Len(); i++ {
					key, val, _ := view.GetIndex(i)
					result = append(result, []any{key.AsU64(), val.AsU32()})
				}
				value = result
			} else if op.Op == "drain_count" {
				value = m.DrainOwned(r).Count()
			} else {
				drain := m.DrainOwned(r)
				result := []any{}
				for {
					p := drain.NextOwned()
					if p == nil {
						break
					}
					pair := p.Borrow()
					result = append(result, []any{pair.Key.AsU64(), pair.Value.AsU32()})
					p.Drop()
				}
				drain.Close()
				value = result
			}
		case "hash_one":
			h := m.Hasher().Build(c)
			actual := h.Borrow(c)
			if _, ok := actual.(*ds.Unhasher); !ok {
				panic("alias lost concrete Unhasher")
			}
			k.HashWithHasher(actual)
			value = actual.Finish64()
			h.Close(c)
		case "debug":
			value = rustfmt.String(m.WriteDebugFormatter)
		default:
			panic("unknown op")
		}
		rows = append(rows, map[string]any{"value": value, "entries": pairs(m), "capacity": m.Capacity(), "len": m.Len()})
	}
	return rows
}
