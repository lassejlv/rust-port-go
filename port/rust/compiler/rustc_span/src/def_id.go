// SPDX-License-Identifier: MIT OR Apache-2.0
// Source port in progress from compiler/rustc_span/src/def_id.rs and the actual
// rustc_index_macros/src/newtype.rs expansion at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Implements the DefIndex identity,
// checked conversions, and LocalDefId identity used by span_encoding. Full-file
// credit remains pending the complete codecs, context/format oracles and the
// real collection-alias dependencies, including locked indexmap2.14.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
)

const DefIndexMaxAsU32 uint32 = 0xffffff00

type DefIndex struct{ value uint32 }

func DefIndexFromU32(value uint32) DefIndex {
	return DefIndexFromU32WithContext(value, defIdDefaultContext())
}
func DefIndexFromUsize(value uint) DefIndex {
	return DefIndexFromUsizeWithContext(value, defIdDefaultContext())
}
func DefIndexFromU16(value uint16) DefIndex      { return DefIndexFromU32(uint32(value)) }
func (value DefIndex) AsU32() uint32             { return value.value }
func (value DefIndex) AsUsize() uint             { return uint(value.value) }
func (value DefIndex) Index() uint               { return value.AsUsize() }
func (value DefIndex) Clone() DefIndex           { return value }
func (value DefIndex) Equal(other DefIndex) bool { return value == other }

// DefIndex is orderable; LocalDefId intentionally has no ordering methods.
func (value DefIndex) Compare(other DefIndex) int {
	if value.value < other.value {
		return -1
	}
	if value.value > other.value {
		return 1
	}
	return 0
}

type LocalDefId struct{ LocalDefIndex DefIndex }

func (value LocalDefId) Clone() LocalDefId           { return value }
func (value LocalDefId) Equal(other LocalDefId) bool { return value == other }

type spanU32Hasher interface{ WriteU32(uint32) }

func (value DefIndex) Hash(h spanU32Hasher)   { h.WriteU32(value.value) }
func (value LocalDefId) Hash(h spanU32Hasher) { value.LocalDefIndex.Hash(h) }

// DefId's source field order depends on target endian/word size. Go stores the
// same two values; Hash explicitly preserves the source's target-independent
// packed-u64 expression on64-bit hosts. No Rust C-layout/niche ABI is claimed.
type DefId struct {
	Index DefIndex
	Krate CrateNum
}

func LocalDefIdFromUsize(index uint) LocalDefId       { return LocalDefId{DefIndexFromUsize(index)} }
func (value LocalDefId) Index() uint                  { return value.LocalDefIndex.Index() }
func (value *LocalDefId) CloneFrom(source LocalDefId) { *value = source.Clone() }
func (value LocalDefId) ToDefId() DefId {
	return DefId{Index: value.LocalDefIndex, Krate: LocalCrate()}
}
func (value LocalDefId) IsTopLevelModule() bool { return value == CrateDefId() }

// Value getters represent source constants whose struct types cannot be Go
// constants. Returning a copy keeps them immutable for every caller.
func LocalCrate() CrateNum    { return CrateNum{} }
func CrateDefIndex() DefIndex { return DefIndex{} }
func CrateDefId() LocalDefId  { return LocalDefId{CrateDefIndex()} }
func CrateModId() LocalModId  { return NewLocalModIdUnchecked(CrateDefId()) }

func DefIdLocal(index DefIndex) DefId         { return DefId{Index: index, Krate: LocalCrate()} }
func (value DefId) Clone() DefId              { return value }
func (value *DefId) CloneFrom(source DefId)   { *value = source.Clone() }
func (value DefId) Equal(other DefId) bool    { return value == other }
func (value DefId) NotEqual(other DefId) bool { return !value.Equal(other) }
func (value DefId) IsLocal() bool             { return value.Krate == LocalCrate() }
func (value DefId) AsLocal() Option[LocalDefId] {
	if value.IsLocal() {
		return Some(LocalDefId{value.Index})
	}
	return None[LocalDefId]()
}
func (value DefId) IsCrateRoot() bool { return value.Index == CrateDefIndex() }
func (value DefId) AsCrateRoot() Option[CrateNum] {
	if value.IsCrateRoot() {
		return Some(value.Krate)
	}
	return None[CrateNum]()
}
func (value DefId) IsTopLevelModule() bool { return value.IsLocal() && value.IsCrateRoot() }
func (value DefId) ToRawDefId() datastructures.RawDefId {
	return datastructures.RawDefId{Crate: value.Krate.AsU32(), Index: value.Index.AsU32()}
}
func DefIdFromRawDefId(value datastructures.RawDefId) DefId {
	return DefId{Krate: CrateNumFromU32(value.Crate), Index: DefIndexFromU32(value.Index)}
}
func DefIdFromLocal(value LocalDefId) DefId { return value.ToDefId() }

// These IDs intentionally have no Compare/PartialCompare methods. The source
// explicitly forbids Ord/PartialOrd for session-local definition identities.
type ModId struct{ defId DefId }
type LocalModId struct{ localDefId LocalDefId }

func NewModIdUnchecked(value DefId) ModId { return ModId{value} }
func (value ModId) ToDefId() DefId        { return value.defId }
func (value ModId) IsLocal() bool         { return value.defId.IsLocal() }
func (value ModId) AsLocal() Option[LocalModId] {
	if local, ok := value.defId.AsLocal().Get(); ok {
		return Some(NewLocalModIdUnchecked(local))
	}
	return None[LocalModId]()
}
func (value ModId) IsCrateRoot() bool         { return value.defId.IsCrateRoot() }
func (value ModId) IsTopLevelModule() bool    { return value.defId.IsTopLevelModule() }
func (value ModId) Clone() ModId              { return value }
func (value *ModId) CloneFrom(source ModId)   { *value = source.Clone() }
func (value ModId) Equal(other ModId) bool    { return value == other }
func (value ModId) NotEqual(other ModId) bool { return !value.Equal(other) }

func NewLocalModIdUnchecked(value LocalDefId) LocalModId { return LocalModId{value} }
func (value LocalModId) IsTopLevelModule() bool          { return value.localDefId.IsTopLevelModule() }
func (value LocalModId) ToDefId() DefId                  { return value.localDefId.ToDefId() }
func (value LocalModId) ToModId() ModId                  { return NewModIdUnchecked(value.ToDefId()) }
func (value LocalModId) ToLocalDefId() LocalDefId        { return value.localDefId }
func (value LocalModId) Clone() LocalModId               { return value }
func (value *LocalModId) CloneFrom(source LocalModId)    { *value = source.Clone() }
func (value LocalModId) Equal(other LocalModId) bool     { return value == other }
func (value LocalModId) NotEqual(other LocalModId) bool  { return !value.Equal(other) }

// DefPathHash maps the source public tuple field to a named exported field.
type DefPathHash struct{ Fingerprint datastructures.Fingerprint }
type StableCrateId struct{ hash rustchashes.Hash64 }

func NewDefPathHash(crate StableCrateId, local rustchashes.Hash64) DefPathHash {
	return DefPathHash{datastructures.NewFingerprint(crate.hash, local)}
}
func ZeroDefPathHash() DefPathHash { return DefPathHash{datastructures.ZeroFingerprint()} }
func (value DefPathHash) StableCrateId() StableCrateId {
	first, _ := value.Fingerprint.Split()
	return StableCrateId{first}
}
func (value DefPathHash) LocalHash() rustchashes.Hash64 {
	_, second := value.Fingerprint.Split()
	return second
}
func (value DefPathHash) Clone() DefPathHash            { return value }
func (value *DefPathHash) CloneFrom(source DefPathHash) { *value = source.Clone() }
func (value DefPathHash) Equal(other DefPathHash) bool {
	return value.Fingerprint.Equal(other.Fingerprint)
}
func (value DefPathHash) NotEqual(other DefPathHash) bool { return !value.Equal(other) }
func (value DefPathHash) Compare(other DefPathHash) int {
	return value.Fingerprint.Compare(other.Fingerprint)
}
func (value DefPathHash) PartialCompare(other DefPathHash) Option[int] {
	return Some(value.Compare(other))
}

const DefPathHashCanUseUnstableSort = true

func (value DefPathHash) StableOrdTripleChecked() struct{}    { return struct{}{} }
func (value DefPathHash) StableCompare(other DefPathHash) int { return value.Compare(other) }
func (value DefPathHash) Hash(hasher rustchashes.U64Hasher)   { value.Fingerprint.Hash(hasher) }
func (value DefPathHash) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.Fingerprint.StableHash(context, hasher)
}

func (value StableCrateId) AsHash() rustchashes.Hash64        { return value.hash }
func (value StableCrateId) AsU64() uint64                     { return value.hash.AsU64() }
func (value StableCrateId) Clone() StableCrateId              { return value }
func (value *StableCrateId) CloneFrom(source StableCrateId)   { *value = source.Clone() }
func (value StableCrateId) Equal(other StableCrateId) bool    { return value.hash == other.hash }
func (value StableCrateId) NotEqual(other StableCrateId) bool { return !value.Equal(other) }
func (value StableCrateId) Compare(other StableCrateId) int   { return value.hash.Compare(other.hash) }
func (value StableCrateId) PartialCompare(other StableCrateId) Option[int] {
	return Some(value.Compare(other))
}
func (value StableCrateId) Hash(hasher rustchashes.U64Hasher) { value.hash.Hash(hasher) }
func (value StableCrateId) StableHash(_ datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.hash.Hash(hasher)
}

func (value DefId) ToStableHashKey(context datastructures.StableHashCtxt) DefPathHash {
	return DefPathHash{context.DefPathHash(value.ToRawDefId())}
}
func (value LocalDefId) ToStableHashKey(context datastructures.StableHashCtxt) DefPathHash {
	return value.ToDefId().ToStableHashKey(context)
}
func (value DefId) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.ToStableHashKey(context).StableHash(context, hasher)
}
func (value LocalDefId) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.ToStableHashKey(context).LocalHash().Hash(hasher)
}
func (value CrateNum) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.AsDefId().ToStableHashKey(context).StableCrateId().StableHash(context, hasher)
}
func (value ModId) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.defId.StableHash(context, hasher)
}
func (value LocalModId) StableHash(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.localDefId.StableHash(context, hasher)
}
