// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped actual slice/Vec Encodable and generic Vec Decodable algorithms from
// serialize.rs. No full serialization or standard-library file credit.
package rustcserialize

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// VectorSlice is a static borrow of actual vector storage, including the full
// logical usize length of ZST vectors. At is the storage projection, not an
// additional user-defined callback. Borrows cannot escape their source lifetime.
type VectorSlice[T any] struct {
	Length uint
	At     func(uint) *T
	// AsGoSlice is the actual contiguous storage borrow when addressable by
	// a Go slice. It is used by the concrete u8 specialization, never by the
	// generic full-width ZST path. It has the same shared-borrow contract.
	AsGoSlice func() []T
}

// ByteVectorEncodeOps chooses the genuine generic u8 slice body or the
// existing concrete opaque-byte specialization. No temporary byte copy is
// substituted for the source's borrowed contiguous input.
func ByteVectorEncodeOps(e Encoder) VectorEncodeOps[uint8] {
	if isConcreteOpaqueByteEncoder(e) {
		return OpaqueByteVectorEncodeOps()
	}
	return VectorEncodeOps[uint8]{Element: func(value *uint8, e Encoder, _ rustfmt.Context) { e.EmitU8(*value) }}
}

// OpaqueByteVectorEncodeOps explicitly selects the actual opaque specialization
// for a statically known source MemEncoder/FileEncoder. This primary also serves
// Go adapters representing that same source Self type. It performs no runtime
// discovery, so an unrelated embedding wrapper cannot inherit specialization.
func OpaqueByteVectorEncodeOps() VectorEncodeOps[uint8] {
	return VectorEncodeOps[uint8]{
		Element: func(value *uint8, e Encoder, _ rustfmt.Context) { e.EmitU8(*value) },
		Override: func(values VectorSlice[uint8], e Encoder, c rustfmt.Context) {
			e.EmitUsize(values.Length)
			if values.AsGoSlice == nil {
				c.Panic("rustcserialize: missing actual contiguous byte storage")
			}
			e.EmitRawBytes(values.AsGoSlice())
		},
	}
}

// The Rust specialization names these concrete types. A distinct wrapper must
// use its own generic Encodable implementation even when Go embedding promotes
// the inner encoder's methods. Source type aliases retain the same identity.
func isConcreteOpaqueByteEncoder(e Encoder) bool {
	switch e.(type) {
	case *MemEncoder, *FileEncoder:
		return true
	default:
		return false
	}
}

// DecodeOpaqueByteVector is the actual Vec<u8>/MemDecoder specialization.
// fromOwnedBytes is a static owning-storage constructor, not a source user
// callback. It receives the copied Vec allocation and must not copy it again.
func DecodeOpaqueByteVector[R any](d *MemDecoder, fromOwnedBytes func([]byte, rustfmt.Context) R) R {
	c := codecContext(d)
	defer func() {
		if p := recover(); p != nil {
			c.ResumeUnwind(p)
		}
	}()
	length := d.ReadUsize()
	borrowed := d.ReadRawBytes(length)
	owned := make([]byte, len(borrowed))
	copy(owned, borrowed)
	return fromOwnedBytes(owned, c)
}

type VectorEncodeOps[T any] struct {
	Element func(*T, Encoder, rustfmt.Context)
	// Override is the selected concrete [T] Encodable specialization, e.g.
	// contiguous opaque u8 bytes. Nil selects the actual generic body.
	Override func(VectorSlice[T], Encoder, rustfmt.Context)
}

func EncodeVector[T any](e Encoder, values VectorSlice[T], ops VectorEncodeOps[T]) {
	c := codecContext(e)
	defer func() {
		if p := recover(); p != nil {
			c.ResumeUnwind(p)
		}
	}()
	if ops.Override != nil {
		ops.Override(values, e, c)
		return
	}
	e.EmitUsize(values.Length)
	for i := uint(0); i < values.Length; i++ {
		if ops.Element == nil {
			c.Panic("rustcserialize: missing vector element Encodable dictionary")
		}
		ops.Element(values.At(i), e, c)
	}
}

// VectorDecodeOps separates the actual Vec storage implementation from the
// codec, avoiding a serialize -> rustc_index dependency cycle. Collect is the
// statically selected Vec<T>::from_iter operation for the concrete trusted
// Map<Range<usize>, decode> iterator created below. It owns the iterator from
// entry and must preserve its complete source specialization/cleanup protocol.
// Dictionaries are borrowed static operations with no hidden owned captures.
type VectorDecodeOps[T, R any] struct {
	Element  ValueDecoder[T]
	Collect  func(rustvec.Iterator[T], rustfmt.Context) R
	Override func(Decoder, rustfmt.Context) R
}

func DecodeVector[T, R any](d Decoder, ops VectorDecodeOps[T, R]) R {
	c := codecContext(d)
	defer func() {
		if p := recover(); p != nil {
			c.ResumeUnwind(p)
		}
	}()
	if ops.Override != nil {
		return ops.Override(d, c)
	}
	length := d.ReadUsize()
	type state struct {
		decoder   Decoder
		next, end uint
	}
	iterator := rustvec.NewIterator(state{decoder: d, end: length}, rustvec.IteratorOps[state, T]{
		TrustedLength: true,
		Next: func(s rustvec.Ref[state]) (T, bool) {
			if s.Value.next == s.Value.end {
				var zero T
				return zero, false
			}
			s.Value.next++
			return decodeValue(s.Value.decoder, ops.Element, s.Context).Take(), true
		},
		SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
			n := s.Value.end - s.Value.next
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
	})
	if ops.Collect == nil {
		c.Panic("rustcserialize: missing actual vector FromIterator strategy")
	}
	return ops.Collect(iterator, c)
}
