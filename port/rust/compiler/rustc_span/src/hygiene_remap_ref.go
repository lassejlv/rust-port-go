// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// This borrowed dictionary expresses one monomorphization of the public trait's
// generic output method. Several O dictionaries can borrow the SAME S cell;
// callers need not invent different source implementors or copy their state.
// Required callbacks are static operations on S, not extra owned environments.
type HygieneRemapperOps[S, K, O any] struct {
	StableHash             func(*S, K, datastructures.StableHashCtxt, *datastructures.StableHasher, rustfmt.Context)
	Len                    func(*S, rustfmt.Context) uint
	FirstNonDetIndexMut    func(*S, rustfmt.Context) *Option[K]
	FirstNonDetIndex       func(*S, rustfmt.Context) uint32
	DropCommitValue        func(*K, rustfmt.Context)
	CommitEndOfDeterminism func(*S, rustfmt.Context)
	RemappingResults       func(*S, datastructures.StableHashCtxt, rustfmt.Context) HygieneRemappingResults[K, O]
}
type HygieneRemapperRef[S, K, O any] struct {
	Value   *S
	Ops     HygieneRemapperOps[S, K, O]
	FromU32 func(uint32, rustfmt.Context) K
}

func BorrowHygieneRemapper[S, K, O any](value *S, ops HygieneRemapperOps[S, K, O], from func(uint32, rustfmt.Context) K) HygieneRemapperRef[S, K, O] {
	return HygieneRemapperRef[S, K, O]{value, ops, from}
}
func (r HygieneRemapperRef[S, K, O]) Len() uint { return r.LenWithContext(rustfmt.Context{}) }
func (r HygieneRemapperRef[S, K, O]) LenWithContext(ctx rustfmt.Context) uint {
	return r.Ops.Len(r.Value, ctx)
}
func (r HygieneRemapperRef[S, K, O]) FirstNonDetIndexMut() *Option[K] {
	return r.FirstNonDetIndexMutWithContext(rustfmt.Context{})
}
func (r HygieneRemapperRef[S, K, O]) FirstNonDetIndexMutWithContext(ctx rustfmt.Context) *Option[K] {
	return r.Ops.FirstNonDetIndexMut(r.Value, ctx)
}
func (r HygieneRemapperRef[S, K, O]) FirstNonDetIndex(ctx rustfmt.Context) uint32 {
	return r.Ops.FirstNonDetIndex(r.Value, ctx)
}
func (r HygieneRemapperRef[S, K, O]) StableHash(key K, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	r.Ops.StableHash(r.Value, key, hcx, h, ctx)
}
func (r HygieneRemapperRef[S, K, O]) CommitEndOfDeterminism(ctx rustfmt.Context) {
	if r.Ops.CommitEndOfDeterminism != nil {
		r.Ops.CommitEndOfDeterminism(r.Value, ctx)
		return
	}
	CommitHygieneEndOfDeterminismDefault[K](r, r.FromU32, ctx)
}
func (r HygieneRemapperRef[S, K, O]) DropHygieneCommitValue(value *K, ctx rustfmt.Context) {
	if r.Ops.DropCommitValue != nil {
		r.Ops.DropCommitValue(value, ctx)
	}
}
func (r HygieneRemapperRef[S, K, O]) RemappingResults(hcx datastructures.StableHashCtxt, ctx rustfmt.Context) HygieneRemappingResults[K, O] {
	if r.Ops.RemappingResults != nil {
		return r.Ops.RemappingResults(r.Value, hcx, ctx)
	}
	return ComputeHygieneRemappingResultsDefault[K, O](r, r.FromU32, hcx, ctx)
}
