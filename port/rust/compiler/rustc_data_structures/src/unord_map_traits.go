// SPDX-License-Identifier: MIT OR Apache-2.0
// Additional genuine UnordMap foundation. The entire unord.rs remains partial.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func (m *UnordMap[K, V]) Extend(input rustvec.IntoIterator[rusthashmap.Pair[K, V]], contexts ...rusthashmap.Context) {
	c := unordContext(contexts)
	m.check(c)
	m.inner.Extend(input, c)
}
func UnordMapFromIterator[K, V any](input rustvec.IntoIterator[rusthashmap.Pair[K, V]], config UnordMapConfig[K, V], contexts ...rusthashmap.Context) *UnordMap[K, V] {
	c := unordContext(contexts)
	if config.Keys.HashOne != nil {
		c.Panic("datastructures: UnordMap's fixed FxBuildHasher uses the default hash_one")
	}
	inner := rusthashmap.MapFromIterator(input, config.Layout, config.Profile, config.Keys, config.Values, func(rusthashmap.Context) rusthashmap.Builder {
		return rusthashmap.FxBuilder{PointerBits: config.Profile.PointerBits}
	}, c)
	return &UnordMap[K, V]{inner: inner, config: config, live: true}
}
func UnordMapGetBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) (*V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return rusthashmap.GetBorrowed(m.inner, query, ops, c)
}
func UnordMapGetMutBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) (*V, bool) {
	return UnordMapGetBorrowed(m, query, ops, contexts...)
}
func UnordMapContainsBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) bool {
	_, ok := UnordMapGetBorrowed(m, query, ops, contexts...)
	return ok
}
func UnordMapRemoveOwnedBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) (*rustvec.Owned[V], bool) {
	c := unordContext(contexts)
	m.check(c)
	return rusthashmap.RemoveBorrowedOwned(m.inner, query, ops, c)
}
func UnordMapRemoveBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) (V, bool) {
	c := unordContext(contexts)
	m.check(c)
	return rusthashmap.RemoveBorrowed(m.inner, query, ops, c)
}
func UnordMapIndexBorrowed[K, V, Q any](m *UnordMap[K, V], query *Q, ops rusthashmap.BorrowedLookup[K, Q], contexts ...rusthashmap.Context) *V {
	c := unordContext(contexts)
	value, ok := UnordMapGetBorrowed(m, query, ops, c)
	if !ok {
		c.Panic("no entry found for key")
	}
	return value
}
func (m *UnordMap[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	m.check(f.Context)
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "UnordMap", pretty, []rustfmt.Field{{Name: "inner", Value: f.Value(pretty, m.inner.WriteDebugFormatter)}})
}
func (m *UnordMap[K, V]) RustDebug() string { return rustfmt.String(m.WriteDebugFormatter) }
