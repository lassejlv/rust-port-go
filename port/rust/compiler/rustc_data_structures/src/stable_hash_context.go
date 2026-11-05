// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL supporting trait witnesses for the actual StableHashCtxt bridge.
// This does not complete compiler/rustc_data_structures/src/stable_hash.rs.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// StableHashCtxtOps supplies all required source operations on an actual
// borrowed S cell. Span hashing borrows exclusively; path lookup, controls and
// the default-controls assertion borrow shared state and permit only interior
// mutation. These are static operations, not owned Go closure environments.
// Callback source panic sites use Ref.Context.Panic; raw Go callbacks retain
// their own local Go defer/recover behavior and are only guarded on escape.
type StableHashCtxtOps[S any] struct {
	StableHashSpan                  func(rustvec.Ref[S], RawSpan, *StableHasher)
	DefPathHash                     func(rustvec.Ref[S], RawDefId) Fingerprint
	StableHashControls              func(rustvec.Ref[S]) StableHashControls
	AssertDefaultStableHashControls func(rustvec.Ref[S], string)
}
type StableHashCtxtRef[S any] struct {
	Value   *S
	Ops     StableHashCtxtOps[S]
	Context rustfmt.Context
}

func NewStableHashCtxtRef[S any](value *S, ops StableHashCtxtOps[S], ambient ...rustfmt.Context) StableHashCtxtRef[S] {
	context := rustfmt.Context{}
	if len(ambient) > 0 {
		context = ambient[0]
	}
	if value == nil || ops.StableHashSpan == nil || ops.DefPathHash == nil || ops.StableHashControls == nil || ops.AssertDefaultStableHashControls == nil {
		context.Panic("stable_hash: missing required borrowed context witness")
	}
	return StableHashCtxtRef[S]{value, ops, context}
}
func stableHashInvoke[T any](context rustfmt.Context, call func() T) T {
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	return call()
}
func (ref StableHashCtxtRef[S]) StableHashSpan(span RawSpan, hasher *StableHasher) {
	ref.StableHashSpanWithContext(span, hasher, ref.Context)
}
func (ref StableHashCtxtRef[S]) StableHashSpanWithContext(span RawSpan, hasher *StableHasher, context rustfmt.Context) {
	stableHashInvoke(context, func() struct{} {
		ref.Ops.StableHashSpan(rustvec.Ref[S]{Value: ref.Value, Context: context}, span, hasher)
		return struct{}{}
	})
}
func (ref StableHashCtxtRef[S]) DefPathHash(id RawDefId) Fingerprint {
	return ref.DefPathHashWithContext(id, ref.Context)
}
func (ref StableHashCtxtRef[S]) DefPathHashWithContext(id RawDefId, context rustfmt.Context) Fingerprint {
	return stableHashInvoke(context, func() Fingerprint { return ref.Ops.DefPathHash(rustvec.Ref[S]{Value: ref.Value, Context: context}, id) })
}
func (ref StableHashCtxtRef[S]) StableHashControls() StableHashControls {
	return ref.StableHashControlsWithContext(ref.Context)
}
func (ref StableHashCtxtRef[S]) StableHashControlsWithContext(context rustfmt.Context) StableHashControls {
	return stableHashInvoke(context, func() StableHashControls {
		return ref.Ops.StableHashControls(rustvec.Ref[S]{Value: ref.Value, Context: context})
	})
}
func (ref StableHashCtxtRef[S]) AssertDefaultStableHashControls(message string) {
	ref.AssertDefaultStableHashControlsWithContext(message, ref.Context)
}
func (ref StableHashCtxtRef[S]) AssertDefaultStableHashControlsWithContext(message string, context rustfmt.Context) {
	stableHashInvoke(context, func() struct{} {
		ref.Ops.AssertDefaultStableHashControls(rustvec.Ref[S]{Value: ref.Value, Context: context}, message)
		return struct{}{}
	})
}

func StableHashDefPathHashWithContext(context StableHashCtxt, id RawDefId, ambient rustfmt.Context) Fingerprint {
	if typed, ok := context.(interface {
		DefPathHashWithContext(RawDefId, rustfmt.Context) Fingerprint
	}); ok {
		return typed.DefPathHashWithContext(id, ambient)
	}
	return stableHashInvoke(ambient, func() Fingerprint { return context.DefPathHash(id) })
}
func StableHashSpanWithContext(context StableHashCtxt, span RawSpan, hasher *StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	if typed, ok := context.(interface {
		StableHashSpanWithContext(RawSpan, *StableHasher, rustfmt.Context)
	}); ok {
		typed.StableHashSpanWithContext(span, hasher, ambient)
		return
	}
	stableHashInvoke(ambient, func() struct{} { context.StableHashSpan(span, hasher); return struct{}{} })
}
