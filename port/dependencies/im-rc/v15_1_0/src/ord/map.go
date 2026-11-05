// SPDX-License-Identifier: MPL-2.0
// Cargo Graph's required OrdMap operations from im-rc 15.1.0 src/ord/map.rs.
// Profile: Rc, no pool, no has_specialisation. This is a scoped dependency
// implementation, not credit for unused im-rc map/set/collection interfaces.
package imrcord

import (
	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	btree "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src/nodes"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type Traits[K, V any] struct {
	CheckedArithmetic bool
	Key               imrc.Ops[K]
	Value             imrc.Ops[V]
	Compare           func(rustvec.Ref[K], rustvec.Ref[K]) int
	NotEqualKey       func(rustvec.Ref[K], rustvec.Ref[K]) bool
	NotEqualValue     func(rustvec.Ref[V], rustvec.Ref[V]) bool
}
type Pair[K, V any] struct {
	Key   *imrc.Cell[K]
	Value *imrc.Cell[V]
}
type Map[K, V any] struct {
	root   *imrc.Rc[btree.Node[Pair[K, V]]]
	size   uint
	traits Traits[K, V]
}

func pairOps[K, V any](traits Traits[K, V]) btree.ValueOps[Pair[K, V]] {
	drop := func(pair *Pair[K, V], ctx rustfmt.Context) {
		imrc.Scope(ctx, func() { pair.Key.Drop(ctx) }, func(ctx rustfmt.Context) { pair.Value.Drop(ctx) })
	}
	return btree.ValueOps[Pair[K, V]]{
		CheckedArithmetic: traits.CheckedArithmetic,
		Value: imrc.Ops[Pair[K, V]]{
			Drop: drop,
			Clone: func(ref rustvec.Ref[Pair[K, V]]) (result *rustvec.Owned[Pair[K, V]]) {
				key := ref.Value.Key.Clone(ref.Context)
				remaining := key
				imrc.Scope(ref.Context, func() {
					value := ref.Value.Value.Clone(ref.Context)
					result = rustvec.NewOwned(Pair[K, V]{Key: key, Value: value}, rustvec.ValueOps[Pair[K, V]]{Drop: drop})
					remaining = nil
				}, func(ctx rustfmt.Context) { remaining.Drop(ctx) })
				return
			},
		},
		Compare: func(a, b rustvec.Ref[Pair[K, V]]) int {
			return traits.Compare(a.Value.Key.Ref(a.Context), b.Value.Key.Ref(b.Context))
		},
		NotEqual: func(a, b rustvec.Ref[Pair[K, V]]) bool {
			return traits.NotEqualKey(a.Value.Key.Ref(a.Context), b.Value.Key.Ref(b.Context)) || traits.NotEqualValue(a.Value.Value.Ref(a.Context), b.Value.Value.Ref(b.Context))
		},
	}
}
func New[K, V any](traits Traits[K, V]) Map[K, V] {
	return Map[K, V]{root: btree.New(pairOps(traits)), traits: traits}
}
func (m *Map[K, V]) Len() uint { return m.size }
func (m *Map[K, V]) Clone(ctx rustfmt.Context) Map[K, V] {
	return Map[K, V]{root: m.root.Clone(ctx), size: m.size, traits: m.traits}
}
func (m *Map[K, V]) Close(ctx rustfmt.Context) {
	root := m.root
	m.root = nil
	m.size = 0
	root.Drop(ctx)
}
func (m *Map[K, V]) GetQuery(compare func(rustvec.Ref[K]) int, ctx rustfmt.Context) *imrc.Cell[V] {
	pair := m.root.Ref(ctx).Value.Lookup(func(pair *Pair[K, V]) int { return compare(pair.Key.Ref(ctx)) }, ctx)
	if pair == nil {
		return nil
	}
	return pair.Ref(ctx).Value.Value
}
func (m *Map[K, V]) Get(key rustvec.Ref[K], ctx rustfmt.Context) *imrc.Cell[V] {
	return m.GetQuery(func(stored rustvec.Ref[K]) int { return m.traits.Compare(stored, key) }, ctx)
}

// IndexAt adapts the source Index operation's tracked caller. Locations come
// from the caller's explicit compiler/source profile and remain nil otherwise.
func (m *Map[K, V]) IndexAt(key rustvec.Ref[K], location *rustfmt.SourceLocation, ctx rustfmt.Context) *imrc.Cell[V] {
	value := m.Get(key, ctx)
	if value == nil {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "OrdMap::index: invalid key", Kind: rustfmt.PanicStaticStr, Location: location, CanUnwind: true})
	}
	return value
}
func (m *Map[K, V]) GetMut(key rustvec.Ref[K], ctx rustfmt.Context) *imrc.Cell[V] {
	pair := imrc.MakeMut(&m.root, ctx).LookupMut(func(pair *Pair[K, V]) int { return m.traits.Compare(pair.Key.Ref(ctx), key) }, ctx)
	if pair == nil {
		return nil
	}
	return pair.Ref(ctx).Value.Value
}

// Insert consumes both inputs. Before creating the tuple, parameter cleanup
// drops V then K. The tuple's own glue thereafter drops K then V.
func (m *Map[K, V]) Insert(key *imrc.Cell[K], value *imrc.Cell[V], ctx rustfmt.Context) (old *imrc.Cell[V]) {
	remainingKey, remainingValue := key, value
	imrc.Scope(ctx, func() {
		root := imrc.MakeMut(&m.root, ctx)
		pair := imrc.NewCell(Pair[K, V]{Key: key, Value: value}, pairOps(m.traits).Value)
		remainingKey = nil
		remainingValue = nil
		inserted := root.Insert(pair, ctx)
		switch inserted.Kind {
		case 0:
			m.size++
			return
		case 1:
			replaced := inserted.Old.Take(ctx)
			old = replaced.Value
			replaced.Key.Drop(ctx)
			return
		case 2:
			newRoot := btree.Wrap(btree.Node[Pair[K, V]]{Keys: []*imrc.Cell[Pair[K, V]]{inserted.Median}, Children: []*imrc.Rc[btree.Node[Pair[K, V]]]{inserted.Left, inserted.Right}, Ops: pairOps(m.traits)})
			m.size++
			oldRoot := m.root
			m.root = newRoot
			oldRoot.Drop(ctx)
		default:
			ctx.Panic("im-rc: invalid insertion result")
		}
	}, func(ctx rustfmt.Context) {
		imrc.Scope(ctx, func() { remainingValue.Drop(ctx) }, func(ctx rustfmt.Context) { remainingKey.Drop(ctx) })
	})
	return
}

// EntryOrInsert is the source entry(key).or_insert_with(default) path. Its
// default callback is a static function item in Cargo graph, with trivial Drop.
// Lookup occurs before calling Default; even occupied entries acquire mutable
// storage and then destroy their separately owned input key.
func (m *Map[K, V]) EntryOrInsert(key *imrc.Cell[K], defaultValue func(rustfmt.Context) *rustvec.Owned[V], ctx rustfmt.Context) (result *imrc.Cell[V]) {
	imrc.Scope(ctx, func() {
		if m.Get(key.Ref(ctx), ctx) != nil {
			result = m.GetMut(key.Ref(ctx), ctx)
			return
		}
		value := imrc.MoveIn(defaultValue(ctx), m.traits.Value, ctx)
		remainingValue := value
		imrc.Scope(ctx, func() {
			cloned := key.Clone(ctx)
			remainingValue = nil
			old := m.Insert(cloned, value, ctx)
			old.Drop(ctx)
			result = m.GetMut(key.Ref(ctx), ctx)
			if result == nil {
				ctx.Panic("im-rc: vacant insertion lost key")
			}
		}, func(ctx rustfmt.Context) { remainingValue.Drop(ctx) })
	}, func(ctx rustfmt.Context) { key.Drop(ctx) })
	return
}

type Iter[K, V any] struct{ inner *btree.Iter[Pair[K, V]] }

func (m *Map[K, V]) Iter(ctx rustfmt.Context) *Iter[K, V] {
	return &Iter[K, V]{inner: btree.NewIter(m.root.Ref(ctx).Value, m.size, ctx)}
}
func (it *Iter[K, V]) Next(ctx rustfmt.Context) (*imrc.Cell[K], *imrc.Cell[V], bool) {
	pair := it.inner.Next(ctx)
	if pair == nil {
		return nil, nil, false
	}
	value := pair.Ref(ctx).Value
	return value.Key, value.Value, true
}
func (it *Iter[K, V]) SizeHint() (uint, *uint) { n := it.inner.Remaining(); return n, &n }
func (m *Map[K, V]) Equal(other *Map[K, V], ctx rustfmt.Context) bool {
	if m.size != other.size {
		return false
	}
	return !btree.NewDiff(m.root.Ref(ctx).Value, other.root.Ref(ctx).Value).Different(ctx)
}
