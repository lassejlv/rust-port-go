// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL define_id_collections! bindings from def_id.rs. The six names use
// genuine Unord/Fx/SwissTable owners and entries. Higher Unord iterator/sort/
// codec surfaces and remaining APIs of the genuine StableCrateIdMap IndexMap
// dependency remain pending, so this does not complete def_id.rs or grant new source-file credit.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// IdMapConfig supplies the real Rust (K,V) bucket layout, target profile and
// value traits. Keys use the fixed concrete ID Hash/Eq/Clone/Debug impls.
// Missing conditional value-trait witnesses prohibit the corresponding Rust
// methods, as in the underlying source adapters. A zero Profile selects the
// recorded native reference profile; Layout must always be supplied explicitly.
type IdMapConfig[V any] struct {
	Layout  rusthashmap.Layout
	Profile rusthashmap.Profile
	Values  rusthashmap.ValueOps[V]
}

func idCollectionProfile(profile rusthashmap.Profile) rusthashmap.Profile {
	if profile == (rusthashmap.Profile{}) {
		return rusthashmap.DefaultProfile()
	}
	return profile
}
func defIdCollectionKeys(profile rusthashmap.Profile) rusthashmap.KeyOps[DefId] {
	return rusthashmap.KeyOps[DefId]{
		Hash: func(value *DefId, h rusthashmap.Hasher, _ rusthashmap.Context) {
			if profile.PointerBits == 64 {
				rusthashmap.HashU64(h, uint64(value.Krate.AsU32())<<32|uint64(value.Index.AsU32()))
			} else {
				rusthashmap.HashU32(h, value.Index.AsU32())
				rusthashmap.HashU32(h, value.Krate.AsU32())
			}
		},
		Equal:        func(a, b *DefId, _ rusthashmap.Context) bool { return a.Equal(*b) },
		Clone:        func(value *DefId, _ rusthashmap.Context) DefId { return value.Clone() },
		Debug:        func(value *DefId, f *rustfmt.Formatter) error { return value.WriteDebugFormatter(f) },
		TrivialClone: true,
	}
}
func localDefIdCollectionKeys() rusthashmap.KeyOps[LocalDefId] {
	return rusthashmap.KeyOps[LocalDefId]{
		Hash:         func(value *LocalDefId, h rusthashmap.Hasher, _ rusthashmap.Context) { value.HashWithHasher(h) },
		Equal:        func(a, b *LocalDefId, _ rusthashmap.Context) bool { return a.Equal(*b) },
		Clone:        func(value *LocalDefId, _ rusthashmap.Context) LocalDefId { return value.Clone() },
		Debug:        func(value *LocalDefId, f *rustfmt.Formatter) error { return value.WriteDebugFormatter(f) },
		TrivialClone: true,
	}
}
func defIdMapConfig[V any](config IdMapConfig[V]) datastructures.UnordMapConfig[DefId, V] {
	profile := idCollectionProfile(config.Profile)
	return datastructures.UnordMapConfig[DefId, V]{Layout: config.Layout, Profile: profile, Keys: defIdCollectionKeys(profile), Values: config.Values}
}
func localDefIdMapConfig[V any](config IdMapConfig[V]) datastructures.UnordMapConfig[LocalDefId, V] {
	return datastructures.UnordMapConfig[LocalDefId, V]{Layout: config.Layout, Profile: idCollectionProfile(config.Profile), Keys: localDefIdCollectionKeys(), Values: config.Values}
}

// Go1.23 cannot express generic aliases. Embedding preserves the real owner;
// explicit Self/Entry methods retain the source alias return/argument types.
// The embedded UnordMap also provides interoperability with its generic free
// functions. Do not copy a live owning descriptor instead of calling Clone.
type DefIdMap[V any] struct {
	*datastructures.UnordMap[DefId, V]
}
type LocalDefIdMap[V any] struct {
	*datastructures.UnordMap[LocalDefId, V]
}
type DefIdMapEntry[V any] struct{ *rusthashmap.Entry[DefId, V] }
type LocalDefIdMapEntry[V any] struct {
	*rusthashmap.Entry[LocalDefId, V]
}
type DefIdSet = datastructures.UnordSet[DefId]
type LocalDefIdSet = datastructures.UnordSet[LocalDefId]

func NewDefIdMap[V any](config IdMapConfig[V], contexts ...rusthashmap.Context) *DefIdMap[V] {
	return &DefIdMap[V]{datastructures.NewUnordMap(defIdMapConfig(config), contexts...)}
}
func NewDefIdMapWithCapacity[V any](capacity uint, config IdMapConfig[V], contexts ...rusthashmap.Context) *DefIdMap[V] {
	return &DefIdMap[V]{datastructures.NewUnordMapWithCapacity(capacity, defIdMapConfig(config), contexts...)}
}
func NewLocalDefIdMap[V any](config IdMapConfig[V], contexts ...rusthashmap.Context) *LocalDefIdMap[V] {
	return &LocalDefIdMap[V]{datastructures.NewUnordMap(localDefIdMapConfig(config), contexts...)}
}
func NewLocalDefIdMapWithCapacity[V any](capacity uint, config IdMapConfig[V], contexts ...rusthashmap.Context) *LocalDefIdMap[V] {
	return &LocalDefIdMap[V]{datastructures.NewUnordMapWithCapacity(capacity, localDefIdMapConfig(config), contexts...)}
}
func DefIdMapFromIterator[V any](input rustvec.IntoIterator[rusthashmap.Pair[DefId, V]], config IdMapConfig[V], contexts ...rusthashmap.Context) *DefIdMap[V] {
	return &DefIdMap[V]{datastructures.UnordMapFromIterator(input, defIdMapConfig(config), contexts...)}
}
func LocalDefIdMapFromIterator[V any](input rustvec.IntoIterator[rusthashmap.Pair[LocalDefId, V]], config IdMapConfig[V], contexts ...rusthashmap.Context) *LocalDefIdMap[V] {
	return &LocalDefIdMap[V]{datastructures.UnordMapFromIterator(input, localDefIdMapConfig(config), contexts...)}
}

func (m *DefIdMap[V]) Clone(contexts ...rusthashmap.Context) *DefIdMap[V] {
	return &DefIdMap[V]{m.UnordMap.Clone(contexts...)}
}
func (m *LocalDefIdMap[V]) Clone(contexts ...rusthashmap.Context) *LocalDefIdMap[V] {
	return &LocalDefIdMap[V]{m.UnordMap.Clone(contexts...)}
}
func (m *DefIdMap[V]) CloneFrom(source *DefIdMap[V], contexts ...rusthashmap.Context) {
	m.UnordMap.CloneFrom(source.UnordMap, contexts...)
}
func (m *LocalDefIdMap[V]) CloneFrom(source *LocalDefIdMap[V], contexts ...rusthashmap.Context) {
	m.UnordMap.CloneFrom(source.UnordMap, contexts...)
}
func (m *DefIdMap[V]) Equal(source *DefIdMap[V], contexts ...rusthashmap.Context) bool {
	return m.UnordMap.Equal(source.UnordMap, contexts...)
}
func (m *LocalDefIdMap[V]) Equal(source *LocalDefIdMap[V], contexts ...rusthashmap.Context) bool {
	return m.UnordMap.Equal(source.UnordMap, contexts...)
}
func (m *DefIdMap[V]) NotEqual(source *DefIdMap[V], contexts ...rusthashmap.Context) bool {
	return !m.Equal(source, contexts...)
}
func (m *LocalDefIdMap[V]) NotEqual(source *LocalDefIdMap[V], contexts ...rusthashmap.Context) bool {
	return !m.Equal(source, contexts...)
}
func (m *DefIdMap[V]) Entry(key DefId, contexts ...rusthashmap.Context) *DefIdMapEntry[V] {
	return &DefIdMapEntry[V]{m.UnordMap.Entry(key, contexts...)}
}
func (m *LocalDefIdMap[V]) Entry(key LocalDefId, contexts ...rusthashmap.Context) *LocalDefIdMapEntry[V] {
	return &LocalDefIdMapEntry[V]{m.UnordMap.Entry(key, contexts...)}
}
func (m *DefIdMap[V]) EntryOwned(key *rustvec.Owned[DefId], contexts ...rusthashmap.Context) *DefIdMapEntry[V] {
	return &DefIdMapEntry[V]{m.UnordMap.EntryOwned(key, contexts...)}
}
func (m *LocalDefIdMap[V]) EntryOwned(key *rustvec.Owned[LocalDefId], contexts ...rusthashmap.Context) *LocalDefIdMapEntry[V] {
	return &LocalDefIdMapEntry[V]{m.UnordMap.EntryOwned(key, contexts...)}
}
func (e *DefIdMapEntry[V]) AndModify(call func(*V), context rusthashmap.Context) *DefIdMapEntry[V] {
	return &DefIdMapEntry[V]{e.Entry.AndModify(call, context)}
}
func (e *LocalDefIdMapEntry[V]) AndModify(call func(*V), context rusthashmap.Context) *LocalDefIdMapEntry[V] {
	return &LocalDefIdMapEntry[V]{e.Entry.AndModify(call, context)}
}
func (e *DefIdMapEntry[V]) AndModifyOwned(call rusthashmap.Once[*V, struct{}], context rusthashmap.Context) *DefIdMapEntry[V] {
	return &DefIdMapEntry[V]{e.Entry.AndModifyOwned(call, context)}
}
func (e *LocalDefIdMapEntry[V]) AndModifyOwned(call rusthashmap.Once[*V, struct{}], context rusthashmap.Context) *LocalDefIdMapEntry[V] {
	return &LocalDefIdMapEntry[V]{e.Entry.AndModifyOwned(call, context)}
}

func NewDefIdSet(contexts ...rusthashmap.Context) *DefIdSet {
	return NewDefIdSetWithProfile(rusthashmap.DefaultProfile(), contexts...)
}
func NewLocalDefIdSet(contexts ...rusthashmap.Context) *LocalDefIdSet {
	return NewLocalDefIdSetWithProfile(rusthashmap.DefaultProfile(), contexts...)
}
func NewDefIdSetWithProfile(profile rusthashmap.Profile, contexts ...rusthashmap.Context) *DefIdSet {
	return NewDefIdSetWithCapacityAndProfile(0, profile, contexts...)
}
func NewLocalDefIdSetWithProfile(profile rusthashmap.Profile, contexts ...rusthashmap.Context) *LocalDefIdSet {
	return NewLocalDefIdSetWithCapacityAndProfile(0, profile, contexts...)
}
func NewDefIdSetWithCapacity(capacity uint, contexts ...rusthashmap.Context) *DefIdSet {
	return NewDefIdSetWithCapacityAndProfile(capacity, rusthashmap.DefaultProfile(), contexts...)
}
func NewLocalDefIdSetWithCapacity(capacity uint, contexts ...rusthashmap.Context) *LocalDefIdSet {
	return NewLocalDefIdSetWithCapacityAndProfile(capacity, rusthashmap.DefaultProfile(), contexts...)
}
func NewDefIdSetWithCapacityAndProfile(capacity uint, profile rusthashmap.Profile, contexts ...rusthashmap.Context) *DefIdSet {
	profile = idCollectionProfile(profile)
	return datastructures.NewUnordSetWithCapacity(capacity, datastructures.UnordSetConfig[DefId]{Layout: rusthashmap.Layout{Size: 8, Align: 4}, Profile: profile, Values: defIdCollectionKeys(profile)}, contexts...)
}
func NewLocalDefIdSetWithCapacityAndProfile(capacity uint, profile rusthashmap.Profile, contexts ...rusthashmap.Context) *LocalDefIdSet {
	return datastructures.NewUnordSetWithCapacity(capacity, datastructures.UnordSetConfig[LocalDefId]{Layout: rusthashmap.Layout{Size: 4, Align: 4}, Profile: idCollectionProfile(profile), Values: localDefIdCollectionKeys()}, contexts...)
}
func DefIdSetFromIterator(input rustvec.IntoIterator[DefId], profile rusthashmap.Profile, contexts ...rusthashmap.Context) *DefIdSet {
	profile = idCollectionProfile(profile)
	return datastructures.UnordSetFromIterator(input, datastructures.UnordSetConfig[DefId]{Layout: rusthashmap.Layout{Size: 8, Align: 4}, Profile: profile, Values: defIdCollectionKeys(profile)}, contexts...)
}
func LocalDefIdSetFromIterator(input rustvec.IntoIterator[LocalDefId], profile rusthashmap.Profile, contexts ...rusthashmap.Context) *LocalDefIdSet {
	return datastructures.UnordSetFromIterator(input, datastructures.UnordSetConfig[LocalDefId]{Layout: rusthashmap.Layout{Size: 4, Align: 4}, Profile: idCollectionProfile(profile), Values: localDefIdCollectionKeys()}, contexts...)
}
