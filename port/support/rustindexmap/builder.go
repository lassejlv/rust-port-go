// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual owned S cells and independent conditional source trait operations.
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// BuilderOps is static trait metadata for the concrete source S. Nil Build or
// Clone means that trait is unavailable, not a fabricated implementation. It
// is still legal to construct an IndexMap and use its unconstrained methods.
// CloneFrom nil selects Clone's genuine provided drop-and-replace default.
// All destructible state belongs in S, including owned fields/captures; the
// supplied Owned cell's destructor witnesses the complete source Drop glue.
// Dictionaries used together must describe the same concrete Rust S, even
// when distinct Rust types have chosen the same Go data representation.
// Build/Clone borrow S shared; CloneFrom borrows destination exclusively and
// source shared, with only actual source interior mutability otherwise allowed.
type BuilderOps[S any] struct {
	Build     func(rustvec.Ref[S]) *rusthashmap.OwnedHasher
	Clone     func(rustvec.Ref[S]) *rustvec.Owned[S]
	CloneFrom func(destination, source rustvec.Ref[S])
}
type OwnedBuilder[S any] struct {
	state *rustvec.Owned[S]
	ops   BuilderOps[S]
}

func NewOwnedBuilder[S any](state *rustvec.Owned[S], ops BuilderOps[S], contexts ...Context) *OwnedBuilder[S] {
	c := current(contexts)
	if !state.IsLive() {
		c.Panic("rustindexmap: consumed builder state")
	}
	return &OwnedBuilder[S]{state: state.MoveWithContext(c), ops: ops}
}

// Borrow is a temporary shared source borrow of the actual live S. It cannot
// outlive owner move/Close or overlap an incompatible mutable operation. A
// pointer retained solely as opaque identity grants no later read permission.
func (b *OwnedBuilder[S]) Borrow(context Context) *S {
	if b == nil || !b.state.IsLive() {
		context.Panic("rustindexmap: consumed builder state")
	}
	return b.state.Borrow()
}
func builderInvoke[R any](context Context, call func() R) R {
	defer func() {
		if p := recover(); p != nil {
			context.ResumeUnwind(p)
		}
	}()
	return call()
}
func (b *OwnedBuilder[S]) SourceBuildHasherAvailable() bool { return b != nil && b.ops.Build != nil }
func (b *OwnedBuilder[S]) SourceCloneAvailable() bool       { return b != nil && b.ops.Clone != nil }
func (b *OwnedBuilder[S]) Build(context Context) *rusthashmap.OwnedHasher {
	value := b.Borrow(context)
	if b.ops.Build == nil {
		context.Panic("rustindexmap: unavailable source BuildHasher trait")
	}
	result := builderInvoke(context, func() *rusthashmap.OwnedHasher { return b.ops.Build(rustvec.Ref[S]{Value: value, Context: context}) })
	result.Borrow(context)
	return result
}
func (b *OwnedBuilder[S]) Clone(context Context) rusthashmap.Builder {
	value := b.Borrow(context)
	if b.ops.Clone == nil {
		context.Panic("rustindexmap: unavailable source builder Clone trait")
	}
	cloned := builderInvoke(context, func() *rustvec.Owned[S] { return b.ops.Clone(rustvec.Ref[S]{Value: value, Context: context}) })
	return NewOwnedBuilder(cloned, b.ops, context)
}
func (b *OwnedBuilder[S]) CloneFrom(source rusthashmap.Builder, context Context) {
	destination := b.Borrow(context)
	other, ok := source.(*OwnedBuilder[S])
	if !ok {
		context.Panic("rustindexmap: different source builder types")
	}
	value := other.Borrow(context)
	if b == other || b.state == other.state {
		context.Panic("rustindexmap: aliased builder CloneFrom source")
	}
	if b.ops.Clone == nil || other.ops.Clone == nil {
		context.Panic("rustindexmap: unavailable source builder Clone trait")
	}
	if b.ops.CloneFrom != nil {
		builderInvoke(context, func() struct{} {
			b.ops.CloneFrom(rustvec.Ref[S]{Value: destination, Context: context}, rustvec.Ref[S]{Value: value, Context: context})
			return struct{}{}
		})
		return
	}
	cloned := builderInvoke(context, func() *rustvec.Owned[S] { return other.ops.Clone(rustvec.Ref[S]{Value: value, Context: context}) })
	rustvec.ReplaceOwnedWithContext(b.state, cloned, context)
}
func (b *OwnedBuilder[S]) Close(context Context) {
	if b != nil {
		b.state.DropWithContext(context)
	}
}
func requireSourceBuildHasher(builder Builder, context Context) {
	// These optional availability markers are pure source-type metadata.
	if traits, ok := builder.(interface{ SourceBuildHasherAvailable() bool }); ok && !traits.SourceBuildHasherAvailable() {
		context.Panic("rustindexmap: unavailable source BuildHasher trait")
	}
}
func requireSourceBuilderClone(builder Builder, context Context) {
	if traits, ok := builder.(interface{ SourceCloneAvailable() bool }); ok && !traits.SourceCloneAvailable() {
		context.Panic("rustindexmap: unavailable source builder Clone trait")
	}
}

func (m *Map[K, V]) requireSelfQuery(context Context) {
	requireSourceBuildHasher(m.builder, context)
	if m.core.config.Keys.Hash == nil || m.core.config.Keys.Equal == nil {
		context.Panic("rustindexmap: unavailable source self-query Hash/Equivalent witnesses")
	}
}
