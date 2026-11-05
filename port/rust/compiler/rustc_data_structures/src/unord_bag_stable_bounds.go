// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

// UnordBagStableHashValue preserves the source impl's complete V: Hash + Eq +
// StableHash bound, including Hash/Eq which its body never invokes. Operations
// borrow the actual V and caller metadata. This is a borrowed static trait
// dictionary, without an additional owned closure environment or destructor.
// Equal must obey Eq's source laws. A lawful implementation supplies every
// method even when that method is unused by this particular body.
type UnordBagStableHashValue[T any] interface {
	Hash(*T, rusthashmap.Hasher, rustfmt.Context)
	Equal(*T, *T, rustfmt.Context) bool
	StableHash(*T, StableHashCtxt, *StableHasher, rustfmt.Context)
}

// UnordBagStableHashOps adapts the existing actual Hash/Eq and StableHash trait
// shapes. The selected dictionaries must describe the same source V. No method
// is validated or invoked solely to check this source bound.
type UnordBagStableHashOps[T any] struct {
	HashEq     rusthashmap.KeyOps[T]
	HashStable UnordStableHash[T]
}

func (ops UnordBagStableHashOps[T]) Hash(value *T, hasher rusthashmap.Hasher, c rustfmt.Context) {
	ops.HashEq.Hash(value, hasher, c)
}
func (ops UnordBagStableHashOps[T]) Equal(a, b *T, c rustfmt.Context) bool {
	return ops.HashEq.Equal(a, b, c)
}
func (ops UnordBagStableHashOps[T]) StableHash(value *T, hcx StableHashCtxt, hasher *StableHasher, c rustfmt.Context) {
	ops.HashStable(value, hcx, hasher, c)
}

// UnordBagStableHash is the bound-preserving source primary. It executes the
// unchanged body; Hash and Eq remain unused. The accepted bag.StableHash method
// is a convenience whose callers must independently satisfy the same source
// Hash + Eq obligations even though its signature only takes the used method.
func UnordBagStableHash[T any](bag *UnordBag[T], hcx StableHashCtxt, hasher *StableHasher, value UnordBagStableHashValue[T], contexts ...rustfmt.Context) {
	bag.StableHash(hcx, hasher, value.StableHash, contexts...)
}
