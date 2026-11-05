// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

// ForEachWithContext consumes the actual iterator using its selected source
// for_each implementation, including its fold default/override and state Drop.
// consume is a static operation witness, not an arbitrary owned source FnMut:
// it may capture only trivial state or temporary borrowed state whose lifetime
// covers this call. Destructible source captures must be represented explicitly
// by the enclosing source operation, rather than hidden in a Go closure.
//
// Each T is moved into consume. The consumer must perform the complete source
// value-parameter cleanup if it panics; this helper owns only the iterator and
// its remaining state. Using the source iterator again after this call is invalid.
// This is supporting runtime machinery and grants no std source-file credit.
func (it Iterator[T]) ForEachWithContext(context Context, consume func(T)) {
	it.forEach(context, consume)
}

// FoldUnitWithContext consumes the iterator through the selected source fold
// implementation with unit accumulator. It bypasses a distinct for_each
// override, as std's Map adapter does. The consumer has the same static-capture
// and complete moved-T cleanup requirements as ForEachWithContext.
func (it Iterator[T]) FoldUnitWithContext(context Context, consume func(T)) {
	if !it.IsLive() {
		context.Panic("rustvec: consumed iterator")
	}
	it.cell.live = false
	runContext(context, func() { it.cell.fold(context, consume) })
}

func closeBuiltinFold[T any](it *VecIntoIter[T], context Context) {
	p := recover()
	if p != nil && context.ProcessingPanic {
		context.ResumeUnwind(p)
	}
	if p != nil {
		unwind := context
		unwind.Unwinding = true
		cleanupDuringUnwind(unwind, func() { it.CloseWithContext(unwind) })
		context.ResumeUnwind(p)
	}
	it.CloseWithContext(context)
}
