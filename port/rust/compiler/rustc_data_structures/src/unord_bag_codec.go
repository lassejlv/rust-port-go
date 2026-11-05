// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func (bag *UnordBag[T]) Encode(e rustcserialize.Encoder, ops rustcserialize.VectorEncodeOps[T]) {
	bag.check(unordCodecContext(e))
	// The NoContext derive emits only its inner Vec field; Vec delegates to
	// the actual slice Encodable method, including a selected specialization.
	rustcserialize.EncodeVector(e, rustcserialize.VectorSlice[T]{Length: bag.inner.Len(), At: bag.inner.Raw.At, AsGoSlice: bag.inner.Raw.AsGoSlice}, ops)
}

// collectDecodedVector is the actual trusted range-map Vec strategy for the
// generic decoder. The iterator's owned cell contains only range cursors and a
// borrowed decoder; the selected default for_each/fold and cleanup still run.
func collectDecodedVector[T any](iterator rustvec.Iterator[T], config UnordBagConfig[T], c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
	input := rustcindex.IndexIntoIterator[T]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[T] {
		return rustcindex.IndexValueIterator[T]{
			TrustedLength:   true,
			NextWithContext: iterator.Next,
			SizeHintWithContext: func(c rustfmt.Context) (uint, uint, bool) {
				hint := iterator.SizeHint(c)
				return hint.Lower, hint.Upper, hint.HasUpper
			},
			CloseWithRuntimeContext:     iterator.CloseWithContext,
			ForEachWithContextOverride:  iterator.ForEachWithContext,
			FoldUnitWithContextOverride: iterator.FoldUnitWithContext,
		}
	}}
	return rustcindex.IndexVecFromIteratorWithConfigAndContext(input, rustcindex.UsizeIdx(), config.indexOps(), config.Profile, c)
}

func DecodeUnordBag[T any](d rustcserialize.Decoder, config UnordBagConfig[T], element rustcserialize.ValueDecoder[T], vectorOverride func(rustcserialize.Decoder, rustfmt.Context) *rustcindex.IndexVec[uint, T]) *UnordBag[T] {
	config.check(unordCodecContext(d))
	inner := rustcserialize.DecodeVector(d, rustcserialize.VectorDecodeOps[T, *rustcindex.IndexVec[uint, T]]{
		Element: element,
		Collect: func(iterator rustvec.Iterator[T], c rustfmt.Context) *rustcindex.IndexVec[uint, T] {
			return collectDecodedVector(iterator, config, c)
		},
		Override: vectorOverride,
	})
	return &UnordBag[T]{inner: inner, config: config, live: true}
}

// UnordByteBagConfig is the fixed primitive Rust u8 trait dictionary. Newtypes
// with distinct traits use their own generic configuration instead.
func UnordByteBagConfig(profile rustcindex.IndexStorageConfig) UnordBagConfig[uint8] {
	return UnordBagConfig[uint8]{Layout: rusthashmap.Layout{Size: 1, Align: 1}, Profile: profile, Values: UnordBagValueOps[uint8]{
		TrivialClone: true,
		Equal:        func(a, b *uint8, _ rustfmt.Context) bool { return *a == *b },
		Debug:        func(value *uint8, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(*value), 8, f) },
	}}
}
func EncodeUnordByteBag(bag *UnordBag[uint8], e rustcserialize.Encoder) {
	bag.Encode(e, rustcserialize.ByteVectorEncodeOps(e))
}
func DecodeUnordByteBag(d rustcserialize.Decoder, profile rustcindex.IndexStorageConfig) *UnordBag[uint8] {
	config := UnordByteBagConfig(profile)
	var override func(rustcserialize.Decoder, rustfmt.Context) *rustcindex.IndexVec[uint, uint8]
	if concrete, ok := d.(*rustcserialize.MemDecoder); ok {
		override = func(_ rustcserialize.Decoder, _ rustfmt.Context) *rustcindex.IndexVec[uint, uint8] {
			return rustcserialize.DecodeOpaqueByteVector(concrete, func(bytes []byte, _ rustfmt.Context) *rustcindex.IndexVec[uint, uint8] {
				return rustcindex.IndexVecFromRawWithConfig(bytes, rustcindex.UsizeIdx(), config.indexOps(), config.Profile)
			})
		}
	}
	element := func(d rustcserialize.Decoder, _ rustfmt.Context) *rustvec.Owned[uint8] {
		return rustvec.Own(d.ReadU8())
	}
	return DecodeUnordBag(d, config, element, override)
}
