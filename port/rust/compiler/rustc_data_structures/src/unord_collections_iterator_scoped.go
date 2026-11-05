// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// ExtendScoped is the ordinary Extend::extend primary for an actual input
// whose selected for_each/fold may invoke insertion under a nested current
// runtime context. The existing Extend convenience selects a same-context
// consumer. Both transfer the actual IntoIterator Self and preserve its Drop.
func (m *UnordMap[K, V]) ExtendScoped(input rustvec.IntoIterator[rusthashmap.Pair[K, V]], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	m.check(c)
	m.inner.ExtendScoped(input, c)
}

func (s *UnordSet[T]) ExtendScoped(input rustvec.IntoIterator[T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	s.check(c)
	s.inner.ExtendScoped(input, c)
}

// UnordMapFromIteratorScoped selects FxHashMap::from_iter on the actual input;
// it does not call an Iterator::collect override or build an intermediate map.
func UnordMapFromIteratorScoped[K, V any](input rustvec.IntoIterator[rusthashmap.Pair[K, V]], config UnordMapConfig[K, V], contexts ...rustfmt.Context) *UnordMap[K, V] {
	c := unordContext(contexts)
	if config.Keys.HashOne != nil {
		c.Panic("datastructures: UnordMap's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rusthashmap.MapFromIteratorScoped(input, config.Layout, config.Profile, config.Keys, config.Values, func(rustfmt.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}, c)
	return &UnordMap[K, V]{inner: inner, config: config, live: true}
}

// UnordSetFromIteratorScoped is the matching ordinary FromIterator primary.
// The actual HashSet map-to-unit adapter selects I::fold independently of
// I::for_each and carries the borrowed insertion consumer's current context.
func UnordSetFromIteratorScoped[T any](input rustvec.IntoIterator[T], config UnordSetConfig[T], contexts ...rustfmt.Context) *UnordSet[T] {
	c := unordContext(contexts)
	if config.Values.HashOne != nil {
		c.Panic("datastructures: UnordSet's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rusthashmap.SetFromIteratorScoped(input, config.Layout, config.Profile, config.Values, func(rustfmt.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}, c)
	return &UnordSet[T]{inner: inner, config: config, live: true}
}
