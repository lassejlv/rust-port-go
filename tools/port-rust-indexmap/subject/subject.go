// Development-only stateful subject for the source-supporting IndexMap core.
package indexmapsubject

import (
	"fmt"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type Request struct {
	Capacity   uint
	Seed, Mask uint64
	Ops        []Operation
}
type Operation struct {
	Op    string
	Key   uint64
	Value int64
	N     uint
}
type builder struct {
	log        *[]string
	seed, mask uint64
}
type hasher struct {
	log         *[]string
	state, mask uint64
	context     im.Context
}

func (b *builder) Build(c im.Context) *rusthashmap.OwnedHasher {
	*b.log = append(*b.log, fmt.Sprintf("build:%t", c.Panicking()))
	return rusthashmap.NewOwnedHasher(&hasher{b.log, b.seed, b.mask, c}, func(_ rusthashmap.Hasher, c im.Context) {
		*b.log = append(*b.log, fmt.Sprintf("hdrop:%t", c.Panicking()))
	})
}
func (b *builder) Clone(c im.Context) rusthashmap.Builder {
	*b.log = append(*b.log, fmt.Sprintf("bclone:%t", c.Panicking()))
	return &builder{b.log, b.seed, b.mask}
}
func (b *builder) CloneFrom(other rusthashmap.Builder, c im.Context) {
	*b.log = append(*b.log, fmt.Sprintf("bclonefrom:%t", c.Panicking()))
	source := other.(*builder)
	b.seed, b.mask = source.seed, source.mask
}
func (b *builder) Close(c im.Context) {
	*b.log = append(*b.log, fmt.Sprintf("bdrop:%t", c.Panicking()))
}
func (h *hasher) Write(bytes []byte) {
	*h.log = append(*h.log, fmt.Sprintf("bytes:%v", bytes))
	h.context.Panic("unexpected byte write")
}
func (h *hasher) WriteU64(v uint64) {
	*h.log = append(*h.log, fmt.Sprintf("u64:%d:%t", v, h.context.Panicking()))
	h.state = (h.state<<5 | h.state>>59) ^ v
}
func (h *hasher) Finish64() uint64 {
	*h.log = append(*h.log, fmt.Sprintf("finish:%t", h.context.Panicking()))
	return h.state & h.mask
}
func caught(call func() any) (v any, p any) {
	defer func() {
		if x := recover(); x != nil {
			p = fmt.Sprint(x)
			v = nil
		}
	}()
	v = call()
	return
}
func Run(q Request) any {
	log := []string{}
	config := im.Config[uint64, int64]{BucketLayout: im.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ im.Context) bool { return *a == *b }, TrivialClone: true, Debug: func(k *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*k, 64, f) }}}, Values: im.ValueOps[int64]{ValueOps: rusthashmap.ValueOps[int64]{Default: func(im.Context) int64 { return 0 }, TrivialClone: true, Equal: func(a, b *int64, _ im.Context) bool { return *a == *b }, Debug: func(v *int64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(*v, 64, f) }}}}
	m := im.WithCapacityAndHasher(q.Capacity, config, &builder{&log, q.Seed, q.Mask})
	rows := []any{}
	for _, op := range q.Ops {
		log = log[:0]
		value, failure := caught(func() any {
			switch op.Op {
			case "insert":
				i, old, ok := m.InsertFull(op.Key, op.Value)
				var v any
				if ok {
					v = old
				}
				return []any{i, v}
			case "entry":
				e := m.Entry(op.Key)
				result := []any{e.Index(im.Context{}), *e.Key(im.Context{}), e.Kind() == im.Occupied}
				e.Close(im.Context{})
				return result
			case "entry_or_insert":
				return *m.Entry(op.Key).OrInsert(op.Value, im.Context{})
			case "entry_or_default":
				return *m.Entry(op.Key).OrDefault(im.Context{})
			case "entry_with":
				return *m.Entry(op.Key).OrInsertWithOwned(rusthashmap.PlainOnce(func(_ struct{}, _ im.Context) *rustvec.Owned[int64] {
					log = append(log, "factory")
					return rustvec.Own(op.Value)
				}), im.Context{})
			case "entry_with_key":
				return *m.Entry(op.Key).OrInsertWithKeyOwned(rusthashmap.PlainOnce(func(k *uint64, _ im.Context) *rustvec.Owned[int64] {
					log = append(log, fmt.Sprintf("factory:%d", *k))
					return rustvec.Own(op.Value)
				}), im.Context{})
			case "entry_modify":
				e := m.Entry(op.Key).AndModify(func(v *int64) { log = append(log, fmt.Sprintf("modify:%d", *v)); *v = op.Value }, im.Context{})
				result := []any{e.Index(im.Context{}), *e.Key(im.Context{})}
				e.Close(im.Context{})
				return result
			case "entry_insert":
				e := m.Entry(op.Key).InsertEntry(op.Value, im.Context{})
				result := []any{e.Index(im.Context{}), *e.Key(im.Context{}), *e.Get(im.Context{})}
				e.Close(im.Context{})
				return result
			case "occupied_replace":
				e := m.Entry(op.Key)
				if o, ok := e.TakeOccupied(im.Context{}); ok {
					i := o.Index(im.Context{})
					old := o.Insert(op.Value, im.Context{})
					result := []any{i, old, *o.GetMut(im.Context{})}
					o.Close(im.Context{})
					return result
				}
				v, _ := e.TakeVacant(im.Context{})
				return []any{v.Index(im.Context{}), v.IntoKey(im.Context{})}
			case "entry_swap_remove", "entry_shift_remove":
				e := m.Entry(op.Key)
				if o, ok := e.TakeOccupied(im.Context{}); ok {
					var pair im.Pair[uint64, int64]
					if op.Op == "entry_shift_remove" {
						pair = o.ShiftRemoveEntryOwned(im.Context{}).Take()
					} else {
						pair = o.SwapRemoveEntryOwned(im.Context{}).Take()
					}
					return []any{pair.Key, pair.Value}
				}
				e.Close(im.Context{})
				return nil
			case "vacant_insert":
				e := m.Entry(op.Key)
				if o, ok := e.TakeOccupied(im.Context{}); ok {
					return []any{o.Index(im.Context{}), *o.IntoMut(im.Context{})}
				}
				v, _ := e.TakeVacant(im.Context{})
				i := v.Index(im.Context{})
				return []any{i, *v.Insert(op.Value, im.Context{})}
			case "get":
				i, k, v, ok := m.GetFull(&op.Key)
				if !ok {
					return nil
				}
				return []any{i, *k, *v}
			case "equivalent":
				i, k, v, ok := im.GetEquivalent(m, &op.Key, im.EquivalentOps[uint64, uint64]{Hash: func(q *uint64, h im.Hasher, _ im.Context) { rusthashmap.HashU64(h, *q) }, Equivalent: func(q, k *uint64, _ im.Context) bool {
					log = append(log, fmt.Sprintf("equivalent:%d:%d", *q, *k))
					return *q == *k
				}})
				if !ok {
					return nil
				}
				return []any{i, *k, *v}
			case "swap_remove":
				v, ok := m.SwapRemove(&op.Key)
				if ok {
					return v
				}
				return nil
			case "shift_remove":
				v, ok := m.ShiftRemove(&op.Key)
				if ok {
					return v
				}
				return nil
			case "swap_index":
				v, ok := m.SwapRemoveIndex(op.N)
				if ok {
					return []any{v.Key, v.Value}
				}
				return nil
			case "shift_index":
				v, ok := m.ShiftRemoveIndex(op.N)
				if ok {
					return []any{v.Key, v.Value}
				}
				return nil
			case "index":
				k, v, ok := m.GetIndex(op.N)
				if ok {
					return []any{*k, *v}
				}
				return nil
			case "reserve":
				m.Reserve(op.N)
				return nil
			case "reserve_exact":
				m.ReserveExact(op.N)
				return nil
			case "reverse":
				m.Reverse()
				return nil
			case "pop":
				v, ok := m.Pop()
				if ok {
					return []any{v.Key, v.Value}
				}
				return nil
			case "clear":
				m.Clear()
				return nil
			case "clone":
				other := m.Clone()
				entries := []any{}
				for i := uint(0); i < other.Len(); i++ {
					k, v, _ := other.GetIndex(i)
					entries = append(entries, []any{*k, *v})
				}
				result := map[string]any{"capacity": other.Capacity(), "entries": entries}
				other.Close()
				return result
			case "equal_clone":
				other := m.Clone()
				same := m.Equal(other)
				other.Close()
				return same
			case "clone_from":
				mask := m.Hasher().(*builder).mask
				other := im.WithCapacityAndHasher(op.N, config, &builder{&log, op.Key, mask})
				for i := uint(0); i < op.N; i++ {
					other.Insert(uint64(i)*17+1, -int64(i)-1)
				}
				m.CloneFrom(other)
				other.Close()
				return nil
			case "debug":
				return rustfmt.String(m.WriteDebugFormatter)
			default:
				panic("unknown operation")
			}
		})
		entries := []any{}
		for i := uint(0); i < m.Len(); i++ {
			k, v, _ := m.GetIndex(i)
			entries = append(entries, []any{*k, *v})
		}
		rows = append(rows, map[string]any{"value": value, "panic": failure, "len": m.Len(), "capacity": m.Capacity(), "entries": entries, "events": append([]string{}, log...)})
	}
	log = log[:0]
	m.Close()
	return map[string]any{"rows": rows, "drop": log}
}
