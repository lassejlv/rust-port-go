// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine map/set derive-codec operations from unord.rs. The surrounding
// UnordItems/bag/sort APIs remain pending; the whole source file is partial.
package datastructures

import (
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

func unordCodecContext(value any) rustfmt.Context {
	if runtime, ok := value.(interface{ RuntimeContext() rustfmt.Context }); ok {
		return runtime.RuntimeContext()
	}
	return rustfmt.Context{}
}

func (m *UnordMap[K, V]) Encode(e rustcserialize.Encoder, ops rustcserialize.HashMapEncodeOps[K, V]) {
	m.check(unordCodecContext(e))
	// NoContext derive emits no struct header; it encodes its inner field.
	rustcserialize.EncodeHashMap(e, m.inner, ops)
}
func (s *UnordSet[T]) Encode(e rustcserialize.Encoder, ops rustcserialize.HashSetEncodeOps[T]) {
	s.check(unordCodecContext(e))
	rustcserialize.EncodeHashSet(e, s.inner, ops)
}
func DecodeUnordMap[K, V any](d rustcserialize.Decoder, config UnordMapConfig[K, V], key rustcserialize.ValueDecoder[K], value rustcserialize.ValueDecoder[V]) *UnordMap[K, V] {
	if config.Keys.HashOne != nil {
		unordCodecContext(d).Panic("datastructures: UnordMap's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rustcserialize.DecodeHashMap(d, rustcserialize.HashMapDecodeOps[K, V]{Layout: config.Layout, Profile: config.Profile, Keys: config.Keys, Values: config.Values, DecodeKey: key, DecodeValue: value, DefaultHasher: func(rustfmt.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}})
	return &UnordMap[K, V]{inner: inner, config: config, live: true}
}
func DecodeUnordSet[T any](d rustcserialize.Decoder, config UnordSetConfig[T], key rustcserialize.ValueDecoder[T]) *UnordSet[T] {
	if config.Values.HashOne != nil {
		unordCodecContext(d).Panic("datastructures: UnordSet's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rustcserialize.DecodeHashSet(d, rustcserialize.HashSetDecodeOps[T]{Layout: config.Layout, Profile: config.Profile, Keys: config.Values, DecodeKey: key, DefaultHasher: func(rustfmt.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}})
	return &UnordSet[T]{inner: inner, config: config, live: true}
}
