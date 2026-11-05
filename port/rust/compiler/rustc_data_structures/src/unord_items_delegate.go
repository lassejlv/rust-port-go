// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordIteratorConstructor is the selected concrete Iterator method and its
// returned concrete iterator type. Go cannot express Rust's generic Iterator
// methods or impl Iterator return types; S and R retain those actual types in
// this static dictionary. Construct owns both source parameters, may move them
// into R, and includes its complete source body/local cleanup. The boundary
// destroys parameters left behind, in source order. In particular, an already
// prepared R is leaked if subsequent parameter destruction panics.
//
// This is a monomorphized trait implementation witness, not an arbitrary user
// closure and not a replacement Next loop. It must select the concrete
// Iterator::map/filter/filter_map/flat_map implementation, including overrides.
// Dictionaries have no hidden owning captures or stored operation context.
type UnordIteratorConstructor[S, T, F, R, U any] struct {
	Construct func(*rustvec.Owned[S], *rustvec.Owned[F], rustfmt.Context) *rustvec.Owned[R]
	Result    UnordIteratorOps[R, U]
}

// UnordUnaryIteratorConstructor is the analogous selected cloned/copied
// implementation. The caller's dictionary witnesses the required T:Clone or
// T:Copy bound, including the actual element implementation in Result.
type UnordUnaryIteratorConstructor[S, T, R, U any] struct {
	Construct func(*rustvec.Owned[S], rustfmt.Context) *rustvec.Owned[R]
	Result    UnordIteratorOps[R, U]
}

// UnordChainConstructor retains both concrete receiver/argument types and the
// actual selected Iterator::chain implementation. Right-parameter destruction
// precedes left-parameter destruction if the constructor leaves them behind.
type UnordChainConstructor[S, J, T, R any] struct {
	Construct func(*rustvec.Owned[S], *rustvec.Owned[J], rustfmt.Context) *rustvec.Owned[R]
	Result    UnordIteratorOps[R, T]
}

func unordTakeConcrete[S, T any](items *UnordItems[T], c rustfmt.Context) *rustvec.Owned[S] {
	items.check(c)
	iterator, ok := items.owner.(*unordIteratorCell[S, T])
	if !ok {
		c.Panic("datastructures: selected iterator dictionary has a different concrete self type")
	}
	iterator.valid = false
	return iterator.state.MoveWithContext(c)
}

func unordFunctionValue[A, B any](f UnordFunction[A, B], c rustfmt.Context) *rustvec.Owned[UnordFunction[A, B]] {
	return rustvec.NewOwned(f.Move(c), rustvec.ValueOps[UnordFunction[A, B]]{
		Drop: func(f *UnordFunction[A, B], c rustfmt.Context) { f.Close(c) },
	})
}

func unordConstruct[S, T, A, B, R, U any](items *UnordItems[T], f UnordFunction[A, B], selected UnordIteratorConstructor[S, T, UnordFunction[A, B], R, U], c rustfmt.Context) *UnordItems[U] {
	if selected.Construct == nil {
		c.Panic("datastructures: missing selected Iterator constructor")
	}
	state := unordTakeConcrete[S](items, c)
	function := unordFunctionValue(f, c)
	result := rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustvec.Owned[R] {
		return rustvec.ConsumeValue(function, c, func(function *rustvec.Owned[UnordFunction[A, B]]) *rustvec.Owned[R] {
			return selected.Construct(state, function, c)
		})
	})
	output := NewUnordItems(result, selected.Result)
	// The source signatures expose only impl Iterator, without +Clone.
	output.clonePublic = false
	return output
}

func MapUnordItems[S, T, U, R any](items *UnordItems[T], f UnordFunction[T, U], selected UnordIteratorConstructor[S, T, UnordFunction[T, U], R, U], contexts ...rustfmt.Context) *UnordItems[U] {
	return unordConstruct(items, f, selected, unordContext(contexts))
}

func FilterUnordItems[S, T, R any](items *UnordItems[T], f UnordFunction[rustvec.Ref[T], bool], selected UnordIteratorConstructor[S, T, UnordFunction[rustvec.Ref[T], bool], R, T], contexts ...rustfmt.Context) *UnordItems[T] {
	return unordConstruct(items, f, selected, unordContext(contexts))
}

// UnordOptional is the Option<U> associated output used by filter_map's Fn.
// Its owning value dictionary must drop Some if that value has not been moved.
type UnordOptional[T any] struct{ Some *rustvec.Owned[T] }

func FilterMapUnordItems[S, T, U, R any](items *UnordItems[T], f UnordFunction[T, UnordOptional[U]], selected UnordIteratorConstructor[S, T, UnordFunction[T, UnordOptional[U]], R, U], contexts ...rustfmt.Context) *UnordItems[U] {
	return unordConstruct(items, f, selected, unordContext(contexts))
}

// FlatMap constructs the source move |x| f(x).0 closure here. Its real owned
// capture calls F through shared Fn even when the selected Iterator adapter
// invokes its wrapper via FnMut/FnOnce. J is the concrete iterator returned by
// F inside UnordItems; the selected constructor receives that actual J output.
func FlatMapUnordItems[S, T, J, U, R any](items *UnordItems[T], f UnordFunction[T, *UnordItems[U]], selected UnordIteratorConstructor[S, T, UnordFunction[T, J], R, U], contexts ...rustfmt.Context) *UnordItems[U] {
	c := unordContext(contexts)
	environment := rustvec.NewOwned(f.Move(rustfmt.Context{}), rustvec.ValueOps[UnordFunction[T, *UnordItems[U]]]{
		Drop: func(f *UnordFunction[T, *UnordItems[U]], c rustfmt.Context) { f.Close(c) },
	})
	invoke := func(f *UnordFunction[T, *UnordItems[U]], argument *rustvec.Owned[T], c rustfmt.Context) *rustvec.Owned[J] {
		result := f.Call(argument, c)
		return rustvec.ConsumeValue(result, c, func(result *rustvec.Owned[*UnordItems[U]]) *rustvec.Owned[J] {
			return unordTakeConcrete[J](result.Take(), c)
		})
	}
	wrapper := NewUnordFunction(environment, UnordFunctionOps[UnordFunction[T, *UnordItems[U]], T, J]{
		Call: func(ref rustvec.Ref[UnordFunction[T, *UnordItems[U]]], argument *rustvec.Owned[T]) *rustvec.Owned[J] {
			return invoke(ref.Value, argument, ref.Context)
		},
		CallMut: func(ref rustvec.Ref[UnordFunction[T, *UnordItems[U]]], argument *rustvec.Owned[T]) *rustvec.Owned[J] {
			return invoke(ref.Value, argument, ref.Context)
		},
		CallOnce: func(owner *rustvec.Owned[UnordFunction[T, *UnordItems[U]]], argument *rustvec.Owned[T], c rustfmt.Context) *rustvec.Owned[J] {
			return invoke(owner.Borrow(), argument, c)
		},
	})
	return unordConstruct(items, wrapper, selected, c)
}

func ChainUnordItems[S, J, T, R any](left, right *UnordItems[T], selected UnordChainConstructor[S, J, T, R], contexts ...rustfmt.Context) *UnordItems[T] {
	c := unordContext(contexts)
	if selected.Construct == nil {
		c.Panic("datastructures: missing selected Iterator::chain constructor")
	}
	leftState := unordTakeConcrete[S](left, c)
	rightState := unordTakeConcrete[J](right, c)
	result := rustvec.ConsumeValue(leftState, c, func(left *rustvec.Owned[S]) *rustvec.Owned[R] {
		return rustvec.ConsumeValue(rightState, c, func(right *rustvec.Owned[J]) *rustvec.Owned[R] {
			return selected.Construct(left, right, c)
		})
	})
	output := NewUnordItems(result, selected.Result)
	output.clonePublic = false
	return output
}

func unordConstructUnary[S, T, R, U any](items *UnordItems[T], selected UnordUnaryIteratorConstructor[S, T, R, U], c rustfmt.Context) *UnordItems[U] {
	if selected.Construct == nil {
		c.Panic("datastructures: missing selected Iterator::cloned/copied constructor")
	}
	state := unordTakeConcrete[S](items, c)
	result := rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustvec.Owned[R] { return selected.Construct(state, c) })
	output := NewUnordItems(result, selected.Result)
	output.clonePublic = false
	return output
}

func ClonedUnordItems[S, T, R any](items *UnordItems[rustvec.Ref[T]], selected UnordUnaryIteratorConstructor[S, rustvec.Ref[T], R, T], contexts ...rustfmt.Context) *UnordItems[T] {
	return unordConstructUnary(items, selected, unordContext(contexts))
}

func CopiedUnordItems[S, T, R any](items *UnordItems[rustvec.Ref[T]], selected UnordUnaryIteratorConstructor[S, rustvec.Ref[T], R, T], contexts ...rustfmt.Context) *UnordItems[T] {
	return unordConstructUnary(items, selected, unordContext(contexts))
}

// UnordIteratorReduction is the actual selected Iterator::max/min or the
// monomorphized Iterator::sum/product implementation (whose defaults dispatch
// to Sum/Product). It owns its iterator parameter, including independent
// iterator Fold/Reduce/Max/Min overrides. A nil owned
// output is permitted only for max/min, where it denotes Option::None.
type UnordIteratorReduction[S, T, R any] func(*rustvec.Owned[S], rustfmt.Context) *rustvec.Owned[R]

func unordReduce[S, T, R any](items *UnordItems[T], selected UnordIteratorReduction[S, T, R], c rustfmt.Context) *rustvec.Owned[R] {
	if selected == nil {
		c.Panic("datastructures: missing selected Iterator reduction")
	}
	state := unordTakeConcrete[S](items, c)
	return rustvec.ConsumeValue(state, c, func(state *rustvec.Owned[S]) *rustvec.Owned[R] { return selected(state, c) })
}
func MaxUnordItems[S, T any](items *UnordItems[T], selected UnordIteratorReduction[S, T, T], contexts ...rustfmt.Context) *rustvec.Owned[T] {
	return unordReduce(items, selected, unordContext(contexts))
}
func MinUnordItems[S, T any](items *UnordItems[T], selected UnordIteratorReduction[S, T, T], contexts ...rustfmt.Context) *rustvec.Owned[T] {
	return unordReduce(items, selected, unordContext(contexts))
}
func SumUnordItems[S, T, R any](items *UnordItems[T], selected UnordIteratorReduction[S, T, R], contexts ...rustfmt.Context) *rustvec.Owned[R] {
	return unordFunctionResult(unordReduce(items, selected, unordContext(contexts)), unordContext(contexts))
}
func ProductUnordItems[S, T, R any](items *UnordItems[T], selected UnordIteratorReduction[S, T, R], contexts ...rustfmt.Context) *rustvec.Owned[R] {
	return unordFunctionResult(unordReduce(items, selected, unordContext(contexts)), unordContext(contexts))
}

// CollectUnordItems invokes C's actual From<UnordItems<T,I>> implementation,
// rather than substituting FromIterator. The conversion owns the wrapper and
// may move or consume it using the source's public unordered operations.
func CollectUnordItems[T, C any](items *UnordItems[T], selected func(*rustvec.Owned[*UnordItems[T]], rustfmt.Context) *rustvec.Owned[C], contexts ...rustfmt.Context) *rustvec.Owned[C] {
	c := unordContext(contexts)
	items.check(c)
	if selected == nil {
		c.Panic("datastructures: missing selected From<UnordItems> implementation")
	}
	moved := &UnordItems[T]{owner: items.owner.move(c), clonePublic: items.clonePublic}
	argument := rustvec.NewOwned(moved, rustvec.ValueOps[*UnordItems[T]]{
		Drop: func(value **UnordItems[T], c rustfmt.Context) { (*value).Close(c) },
	})
	result := rustvec.ConsumeValue(argument, c, func(argument *rustvec.Owned[*UnordItems[T]]) *rustvec.Owned[C] { return selected(argument, c) })
	return unordFunctionResult(result, c)
}
