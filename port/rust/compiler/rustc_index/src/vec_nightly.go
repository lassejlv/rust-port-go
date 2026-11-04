//go:build ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_index/src/vec.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179: cfg(feature = "nightly") impls.
package rustcindex

// IndexVecEncoder supplies the Encoder/element Encodable operations needed by
// the actual Vec encoding implementation. A slice specialization, if supplied,
// replaces the complete default [T] encoding, as Rust specialization permits.
// Encoding preserves delayed errors owned by the encoder; callbacks do not
// invent a second error channel or discard encoder state.
type IndexVecEncoder[T any] struct {
	EmitUsize           func(uint)
	EncodeElement       func(*T)
	EncodeSliceOverride func(IndexRaw[T])
}

func (v *IndexVec[I, T]) Encode(encoder IndexVecEncoder[T]) {
	if encoder.EncodeSliceOverride != nil {
		encoder.EncodeSliceOverride(v.Raw)
		return
	}
	encoder.EmitUsize(v.Len())
	for i := uint(0); i < v.Len(); i++ {
		encoder.EncodeElement(v.Raw.At(i))
	}
}

// IndexVecDecoder supplies the Decoder/element Decodable operations used by
// Vec::decode. A Vec specialization may replace the complete default. Length
// decoding never constructs typed indices: source from_raw performs no such
// validation, even when the decoded vector exceeds its index type's domain.
type IndexVecDecoder[T any] struct {
	ReadUsize         func() uint
	DecodeElement     func() T
	DecodeVecOverride func() IndexRaw[T]
}

func DecodeIndexVec[I, T any](decoder IndexVecDecoder[T], idx Idx[I], ops ValueOps[T]) *IndexVec[I, T] {
	return DecodeIndexVecWithConfig(decoder, idx, ops, defaultIndexStorageConfig())
}
func DecodeIndexVecWithConfig[I, T any](decoder IndexVecDecoder[T], idx Idx[I], ops ValueOps[T], config IndexStorageConfig) (v *IndexVec[I, T]) {
	if decoder.DecodeVecOverride != nil {
		return &IndexVec[I, T]{decoder.DecodeVecOverride(), idx, ops, config}
	}
	n := decoder.ReadUsize()
	v = NewIndexVecWithConfig(idx, ops, config)
	defer func() {
		if p := recover(); p != nil {
			v.CloseWithContext(IndexDropContext{true})
			panic(p)
		}
	}()
	v.reserveExactRaw(n)
	for i := uint(0); i < n; i++ {
		v.pushRaw(decoder.DecodeElement())
	}
	return
}

// ExtendOne and ExtendReserve are the feature-gated Extend trait overrides.
// ExtendOne intentionally bypasses IndexVec::push and its typed-index check.
func (v *IndexVec[I, T]) ExtendOne(value T) {
	owned := true
	defer indexDeferValueOnPanic(&value, v.Ops, &owned)
	v.pushRaw(value)
	owned = false
}
func (v *IndexVec[I, T]) ExtendReserve(additional uint) { v.reserveRaw(additional) }
