// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped HashMap/HashSet Decodable implementations from serialize.rs. This
// adds no completion credit for the surrounding serialization/std files.
package rustcserialize

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// ValueDecoder is the selected Decodable<D>::decode operation, borrowing the
// actual decoder. It returns full ownership of T, including its complete Drop.
// The decoder/input-buffer lifetimes and source borrows remain the caller's
// responsibility. Its dictionary has no additional owned closure captures.
type ValueDecoder[T any] func(Decoder, rustfmt.Context) *rustvec.Owned[T]

func decodeValue[T any](d Decoder, decode ValueDecoder[T], ctx rustfmt.Context) (value *rustvec.Owned[T]) {
	defer func() {
		if p := recover(); p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	if decode == nil {
		ctx.Panic("rustcserialize: missing Decodable dictionary")
	}
	value = decode(d, ctx)
	if value == nil || !value.IsLive() {
		ctx.Panic("rustcserialize: Decodable returned a consumed value")
	}
	return value
}

type HashMapDecodeOps[K, V any] struct {
	Layout        rusthashmap.Layout
	Profile       rusthashmap.Profile
	Keys          rusthashmap.KeyOps[K]
	Values        rusthashmap.ValueOps[V]
	DecodeKey     ValueDecoder[K]
	DecodeValue   ValueDecoder[V]
	DefaultHasher func(rustfmt.Context) rusthashmap.Builder
}
type HashSetDecodeOps[K any] struct {
	Layout        rusthashmap.Layout
	Profile       rusthashmap.Profile
	Keys          rusthashmap.KeyOps[K]
	DecodeKey     ValueDecoder[K]
	DefaultHasher func(rustfmt.Context) rusthashmap.Builder
}

func DecodeHashMap[K, V any](d Decoder, ops HashMapDecodeOps[K, V]) *rusthashmap.Map[K, V] {
	ctx := codecContext(d)
	length := d.ReadUsize()
	type state struct {
		decoder   Decoder
		next, end uint
	}
	iterator := rustvec.NewIterator(state{decoder: d, end: length}, rustvec.IteratorOps[state, rusthashmap.Pair[K, V]]{
		TrustedLength: true,
		Next: func(s rustvec.Ref[state]) (rusthashmap.Pair[K, V], bool) {
			if s.Value.next == s.Value.end {
				return rusthashmap.Pair[K, V]{}, false
			}
			s.Value.next++
			key := decodeValue(s.Value.decoder, ops.DecodeKey, s.Context)
			pair := rustvec.ConsumeValue(key, s.Context, func(key *rustvec.Owned[K]) rusthashmap.Pair[K, V] {
				// Tuple evaluation decodes the first field before the second.
				// If decoding V fails, the initialized K is destroyed before
				// the partially collected map is destroyed.
				value := decodeValue(s.Value.decoder, ops.DecodeValue, s.Context)
				return rusthashmap.Pair[K, V]{Key: key.Take(), Value: value.Take()}
			})
			return pair, true
		},
		SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
			n := s.Value.end - s.Value.next
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
	})
	return rusthashmap.MapFromIterator(rustvec.FromIterator(iterator), ops.Layout, ops.Profile, ops.Keys, ops.Values, ops.DefaultHasher, ctx)
}

func DecodeHashSet[K any](d Decoder, ops HashSetDecodeOps[K]) *rusthashmap.Set[K] {
	ctx := codecContext(d)
	length := d.ReadUsize()
	type state struct {
		decoder   Decoder
		next, end uint
	}
	iterator := rustvec.NewIterator(state{decoder: d, end: length}, rustvec.IteratorOps[state, K]{
		TrustedLength: true,
		Next: func(s rustvec.Ref[state]) (K, bool) {
			if s.Value.next == s.Value.end {
				var zero K
				return zero, false
			}
			s.Value.next++
			return decodeValue(s.Value.decoder, ops.DecodeKey, s.Context).Take(), true
		},
		SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
			n := s.Value.end - s.Value.next
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
	})
	return rusthashmap.SetFromIterator(rustvec.FromIterator(iterator), ops.Layout, ops.Profile, ops.Keys, ops.DefaultHasher, ctx)
}
