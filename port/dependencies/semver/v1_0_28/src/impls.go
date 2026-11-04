// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/impls.rs. Rust Hash's primitive calls are exposed
// through optional Go method overrides, with the same native-endian defaults.
package semver

import (
	"encoding/binary"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"iter"
	"strconv"
	"strings"
)

// Hasher maps Rust's required Hasher::write method. Optional WriteU8, WriteU16,
// WriteU32, WriteU64, WriteUsize and WriteIsize override the primitive
// methods consumed by these source implementations. WriteString overrides Rust's write_str; otherwise a
// string hashes as its UTF-8 bytes followed by WriteU8(255).
type Hasher interface{ Write([]byte) }
type HasherFunc func([]byte)

func (write HasherFunc) Write(bytes []byte) { write(bytes) }
func HashU8(h Hasher, value uint8) {
	if x, ok := h.(interface{ WriteU8(uint8) }); ok {
		x.WriteU8(value)
	} else {
		h.Write([]byte{value})
	}
}
func HashU16(h Hasher, value uint16) {
	if x, ok := h.(interface{ WriteU16(uint16) }); ok {
		x.WriteU16(value)
	} else {
		var b [2]byte
		binary.NativeEndian.PutUint16(b[:], value)
		h.Write(b[:])
	}
}
func HashU32(h Hasher, value uint32) {
	if x, ok := h.(interface{ WriteU32(uint32) }); ok {
		x.WriteU32(value)
	} else {
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], value)
		h.Write(b[:])
	}
}
func HashU64(h Hasher, value uint64) {
	if x, ok := h.(interface{ WriteU64(uint64) }); ok {
		x.WriteU64(value)
	} else {
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], value)
		h.Write(b[:])
	}
}
func HashUsize(h Hasher, value uint) {
	if x, ok := h.(interface{ WriteUsize(uint) }); ok {
		x.WriteUsize(value)
	} else if strconv.IntSize == 64 {
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], uint64(value))
		h.Write(b[:])
	} else {
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], uint32(value))
		h.Write(b[:])
	}
}
func HashIsize(h Hasher, value int) {
	if x, ok := h.(interface{ WriteIsize(int) }); ok {
		x.WriteIsize(value)
	} else if strconv.IntSize == 64 {
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], uint64(value))
		h.Write(b[:])
	} else {
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], uint32(value))
		h.Write(b[:])
	}
}

// HashLengthPrefix maps Hasher::write_length_prefix, whose default delegates
// to write_usize. The optional hook supports Rust's prefix-free hasher profile.
func HashLengthPrefix(h Hasher, value uint) {
	if x, ok := h.(interface{ WriteLengthPrefix(uint) }); ok {
		x.WriteLengthPrefix(value)
	} else {
		HashUsize(h, value)
	}
}

func HashString(h Hasher, value string) {
	requireUTF8(value)
	if x, ok := h.(interface{ WriteString(string) }); ok {
		x.WriteString(value)
	} else {
		h.Write([]byte(value))
		HashU8(h, 255)
	}
}

// Clone and equality follow the concrete source types. Strings are immutable;
// the comparators Vec has independent slice storage after Clone.
func (value Prerelease) Clone() Prerelease       { return value }
func (value BuildMetadata) Clone() BuildMetadata { return value }
func (value Version) Clone() Version             { return value }
func (value Comparator) Clone() Comparator       { return value }
func (value VersionReq) Clone() VersionReq {
	return VersionReq{Comparators: cloneComparators(value.Comparators)}
}
func (value Op) Clone() Op { value.check(); return value }

// CloneFrom follows Clone's provided whole-value replacement. In particular,
// VersionReq does not delegate to Vec::clone_from or reuse destination capacity.
func (value *Prerelease) CloneFrom(source Prerelease)       { *value = source.Clone() }
func (value *BuildMetadata) CloneFrom(source BuildMetadata) { *value = source.Clone() }
func (value *Version) CloneFrom(source Version)             { *value = source.Clone() }
func (value *Comparator) CloneFrom(source Comparator)       { *value = source.Clone() }
func (value *VersionReq) CloneFrom(source VersionReq)       { *value = source.Clone() }
func (value *Op) CloneFrom(source Op)                       { *value = source.Clone() }

func cloneComparators(source []Comparator) []Comparator {
	// Vec::clone allocates exactly len elements, even when the source has spare
	// capacity. append would expose Go allocator size-class rounding through cap.
	result := make([]Comparator, len(source))
	copy(result, source)
	return result
}
func (value Prerelease) Equal(other Prerelease) bool {
	return value.identifier.text == other.identifier.text
}
func (value BuildMetadata) Equal(other BuildMetadata) bool {
	return value.identifier.text == other.identifier.text
}
func (value Version) Equal(other Version) bool {
	return value.Major == other.Major && value.Minor == other.Minor && value.Patch == other.Patch && value.Pre.Equal(other.Pre) && value.Build.Equal(other.Build)
}
func optionU64Equal(a, b Option[uint64]) bool {
	return a.Present == b.Present && (!a.Present || a.Value == b.Value)
}
func (value Comparator) Equal(other Comparator) bool {
	return value.Op.Equal(other.Op) && value.Major == other.Major && optionU64Equal(value.Minor, other.Minor) && optionU64Equal(value.Patch, other.Patch) && value.Pre.Equal(other.Pre)
}
func (value VersionReq) Equal(other VersionReq) bool {
	if len(value.Comparators) != len(other.Comparators) {
		return false
	}
	for index, comparator := range value.Comparators {
		if !comparator.Equal(other.Comparators[index]) {
			return false
		}
	}
	return true
}
func (value Op) Equal(other Op) bool { value.check(); other.check(); return value == other }
func (value Op) check() {
	if value > Wildcard {
		panic("semver: invalid Op")
	}
}

func cmpU64(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func (value Version) ComparePrecedence(other Version) int {
	if c := cmpU64(value.Major, other.Major); c != 0 {
		return c
	}
	if c := cmpU64(value.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmpU64(value.Patch, other.Patch); c != 0 {
		return c
	}
	return value.Pre.Compare(other.Pre)
}
func (value Version) Compare(other Version) int {
	if c := value.ComparePrecedence(other); c != 0 {
		return c
	}
	return value.Build.Compare(other.Build)
}
func (value Version) PartialCompare(other Version) Option[int] { return Some(value.Compare(other)) }
func (value Prerelease) PartialCompare(other Prerelease) Option[int] {
	return Some(value.Compare(other))
}
func (value BuildMetadata) PartialCompare(other BuildMetadata) Option[int] {
	return Some(value.Compare(other))
}
func allDigits(text string) bool {
	for index := range text {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}
func (value Prerelease) Compare(other Prerelease) int {
	if value.identifier.text == other.identifier.text {
		return 0
	}
	if value.IsEmpty() {
		return 1
	}
	if other.IsEmpty() {
		return -1
	}
	left, right := strings.Split(value.AsStr(), "."), strings.Split(other.AsStr(), ".")
	for index, lhs := range left {
		if index >= len(right) {
			return 1
		}
		rhs := right[index]
		lnum, rnum := allDigits(lhs), allDigits(rhs)
		if lnum && !rnum {
			return -1
		}
		if !lnum && rnum {
			return 1
		}
		ordering := 0
		if lnum && rnum {
			ordering = cmpInt(len(lhs), len(rhs))
		}
		if ordering == 0 {
			ordering = cmpString(lhs, rhs)
		}
		if ordering != 0 {
			return ordering
		}
	}
	return cmpInt(len(left), len(right))
}
func (value BuildMetadata) Compare(other BuildMetadata) int {
	if value.identifier.text == other.identifier.text {
		return 0
	}
	left, right := strings.Split(value.AsStr(), "."), strings.Split(other.AsStr(), ".")
	for index, lhs := range left {
		if index >= len(right) {
			return 1
		}
		rhs := right[index]
		lnum, rnum := allDigits(lhs), allDigits(rhs)
		if lnum && !rnum {
			return -1
		}
		if !lnum && rnum {
			return 1
		}
		ordering := 0
		if lnum && rnum {
			lval, rval := strings.TrimLeft(lhs, "0"), strings.TrimLeft(rhs, "0")
			ordering = cmpInt(len(lval), len(rval))
			if ordering == 0 {
				ordering = cmpString(lval, rval)
			}
			if ordering == 0 {
				ordering = cmpInt(len(lhs), len(rhs))
			}
		} else {
			ordering = cmpString(lhs, rhs)
		}
		if ordering != 0 {
			return ordering
		}
	}
	return cmpInt(len(left), len(right))
}
func FromComparators(comparators []Comparator) VersionReq {
	return VersionReq{Comparators: cloneComparators(comparators)}
}

// FromIterator is the trivial-ownership Go sequence convenience. The complete
// consumed IntoIterator contract is exposed by FromOwnedIterator.
func FromIterator(sequence iter.Seq[Comparator]) VersionReq {
	return FromOwnedIterator(rustvec.FromSeq(sequence))
}

func (value Prerelease) Hash(hasher Hasher)    { HashString(hasher, value.AsStr()) }
func (value BuildMetadata) Hash(hasher Hasher) { HashString(hasher, value.AsStr()) }
func (value Version) Hash(hasher Hasher) {
	HashU64(hasher, value.Major)
	HashU64(hasher, value.Minor)
	HashU64(hasher, value.Patch)
	value.Pre.Hash(hasher)
	value.Build.Hash(hasher)
}
func (value Op) Hash(hasher Hasher) { value.check(); HashIsize(hasher, int(value)) }
func hashOptionU64(hasher Hasher, value Option[uint64]) {
	if !value.Present {
		HashIsize(hasher, 0)
	} else {
		HashIsize(hasher, 1)
		HashU64(hasher, value.Value)
	}
}
func (value Comparator) Hash(hasher Hasher) {
	value.Op.Hash(hasher)
	HashU64(hasher, value.Major)
	hashOptionU64(hasher, value.Minor)
	hashOptionU64(hasher, value.Patch)
	value.Pre.Hash(hasher)
}
func (value VersionReq) Hash(hasher Hasher) {
	HashLengthPrefix(hasher, uint(len(value.Comparators)))
	for _, comparator := range value.Comparators {
		comparator.Hash(hasher)
	}
}
