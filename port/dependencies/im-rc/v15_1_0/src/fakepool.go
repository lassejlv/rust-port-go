// SPDX-License-Identifier: MPL-2.0
// Scoped dependency operations from im-rc 15.1.0 fakepool.rs (Rc, no pool).
// This is supporting code for Cargo graph, not a whole-crate port declaration.
package imrc

import (
	"os"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// Ops is static Rust type metadata. Owned inputs/results must describe this
// same type destructor. A move transfers the value into its destination cell;
// no physical Rust ABI or stack-address equivalence is implied.
type Ops[T any] struct {
	Clone func(rustvec.Ref[T]) *rustvec.Owned[T]
	Drop  func(*T, rustfmt.Context)
}

// Cell is an exclusive live value place. Its address is stable while borrowed.
type Cell[T any] struct {
	Value T
	Ops   Ops[T]
	live  bool
}

func NewCell[T any](value T, ops Ops[T]) *Cell[T] {
	return &Cell[T]{Value: value, Ops: ops, live: true}
}
func MoveIn[T any](value *rustvec.Owned[T], ops Ops[T], ctx rustfmt.Context) *Cell[T] {
	return rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[T]) *Cell[T] { return NewCell(value.Take(), ops) })
}
func (cell *Cell[T]) IsLive() bool { return cell != nil && cell.live }
func (cell *Cell[T]) Ref(ctx rustfmt.Context) rustvec.Ref[T] {
	if !cell.IsLive() {
		ctx.Panic("im-rc: use of moved value place")
	}
	return rustvec.Ref[T]{Value: &cell.Value, Context: ctx}
}
func (cell *Cell[T]) Clone(ctx rustfmt.Context) *Cell[T] {
	return MoveIn(cell.Ops.Clone(cell.Ref(ctx)), cell.Ops, ctx)
}
func (cell *Cell[T]) Take(ctx rustfmt.Context) T {
	cell.Ref(ctx)
	cell.live = false
	return cell.Value
}
func (cell *Cell[T]) Drop(ctx rustfmt.Context) {
	if !cell.IsLive() {
		return
	}
	cell.live = false
	if cell.Ops.Drop != nil {
		defer func() {
			if p := recover(); p != nil {
				ctx.ResumeUnwind(p)
			}
		}()
		cell.Ops.Drop(&cell.Value, ctx)
	}
}

// Assign consumes an already evaluated RHS into the same value place. Rust's
// assignment cleanup installs that RHS even if the old value's Drop unwinds.
// Processing-panic aborts happen at Context.Panic before reaching that cleanup.
func (cell *Cell[T]) Assign(value *rustvec.Owned[T], ctx rustfmt.Context) {
	cell.Ref(ctx)
	incoming := MoveIn(value, cell.Ops, ctx)
	Scope(ctx, func() { cell.Drop(ctx) }, func(ctx rustfmt.Context) { cell.Value = incoming.Take(ctx); cell.live = true })
}

// Scope models a local source cleanup ladder. Ambient unwind and a panic
// originating in body are distinct. Cleanup failure during the latter aborts.
// Source panic sites use Context.Panic; raw Go callback-local defers retain Go
// timing before an escaping panic reaches this boundary.
func Scope(ctx rustfmt.Context, body func(), cleanup func(rustfmt.Context)) {
	defer func() {
		p := recover()
		if p != nil && ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		if p != nil {
			ctx.Unwinding = true
			func() {
				defer func() {
					if recover() != nil {
						ctx.AbortCleanup()
					}
				}()
				cleanup(ctx)
			}()
			ctx.ResumeUnwind(p)
		}
		cleanup(ctx)
	}()
	body()
}
func DropCells[T any](cells []*Cell[T], ctx rustfmt.Context) {
	DropSequence(cells, ctx, func(cell *Cell[T], ctx rustfmt.Context) { cell.Drop(ctx) })
}
func DropSequence[T any](cells []T, ctx rustfmt.Context, drop func(T, rustfmt.Context)) {
	for i, cell := range cells {
		p := func() (p any) { defer func() { p = recover() }(); drop(cell, ctx); return nil }()
		if p != nil {
			if ctx.ProcessingPanic {
				ctx.ResumeUnwind(p)
			}
			ctx.Unwinding = true
			// Slice drop glue enters the remaining elements through a cleanup
			// landing pad. A second escaping element Drop stops that pad.
			func() {
				defer func() {
					if recover() != nil {
						ctx.AbortCleanup()
					}
				}()
				for _, rest := range cells[i+1:] {
					drop(rest, ctx)
				}
			}()
			ctx.ResumeUnwind(p)
		}
	}
}

type rcCell[T any] struct {
	value *Cell[T]
	count uint
}

// Rc is a single counted handle. Copy by Clone, never by Go assignment.
type Rc[T any] struct {
	cell *rcCell[T]
	live bool
}

func NewRc[T any](value *Cell[T]) *Rc[T] {
	return &Rc[T]{cell: &rcCell[T]{value: value, count: 1}, live: true}
}
func (r *Rc[T]) Ref(ctx rustfmt.Context) rustvec.Ref[T] {
	if r == nil || !r.live {
		ctx.Panic("im-rc: use of consumed Rc handle")
	}
	return r.cell.value.Ref(ctx)
}
func (r *Rc[T]) Clone(ctx rustfmt.Context) *Rc[T] {
	r.Ref(ctx)
	r.cell.count++
	if r.cell.count == 0 {
		os.Exit(134)
	} // std Rc strong-count overflow aborts
	return &Rc[T]{cell: r.cell, live: true}
}
func PtrEq[T any](a, b *Rc[T]) bool { return a != nil && b != nil && a.cell == b.cell }
func (r *Rc[T]) Drop(ctx rustfmt.Context) {
	if r == nil || !r.live {
		return
	}
	r.live = false
	r.cell.count--
	if r.cell.count == 0 {
		r.cell.value.Drop(ctx)
	}
}
func MakeMut[T any](r **Rc[T], ctx rustfmt.Context) *T {
	old := *r
	value := old.Ref(ctx)
	if old.cell.count == 1 {
		return value.Value
	}
	cloned := old.cell.value.Clone(ctx)
	*r = NewRc(cloned)
	old.Drop(ctx)
	return (*r).Ref(ctx).Value
}
