// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

// ReplaceOwnedWithContext models the used drop-and-replace operation with a
// live destination and a distinct initialized replacement. It consumes the
// replacement, destroys the old value in the destination's actual cell, and
// installs the replacement on both the normal and unwinding old-Drop paths.
// The transfer includes the replacement's complete destruction operations.
// This scoped helper does not model assignment to an uninitialized place or
// an aliased self-move. Borrows obey the same Move/Drop rules as Owned.
func ReplaceOwnedWithContext[T any](target, replacement *Owned[T], context Context) {
	if !target.IsLive() || !replacement.IsLive() || target.cell == replacement.cell {
		context.Panic("rustvec: drop-and-replace requires distinct live owners")
	}
	ops := replacement.cell.ops
	value := replacement.Take()
	defer func() {
		p := recover()
		if p != nil && context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		target.cell.value = value
		target.cell.ops = ops
		target.cell.live = true
		target.cell.generation++
		target.generation = target.cell.generation
		target.context = context
		if p != nil {
			context.ResumeUnwind(p)
		}
	}()
	target.DropWithContext(context)
}
