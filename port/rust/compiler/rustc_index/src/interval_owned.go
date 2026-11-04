// SPDX-License-Identifier: MIT OR Apache-2.0
// Owned range arguments of pinned interval.rs, sharing the typed-storage local
// abort runtime. Parent and cloned ranges are distinct owned values.
package rustcindex

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// IntervalPanicContext explicitly carries ambient source panic state. A pending
// outer unwind alone is not this operation's local cleanup: callers can catch
// a new panic within a destructor. ProcessingPanic retains immediate-abort
// timing through context.Panic, before callback local destructors execute.
type IntervalPanicContext = rustfmt.Context

func (bounds IntervalRangeBounds[I]) close(context IntervalPanicContext) {
	if bounds.CloseWithContext != nil {
		bounds.CloseWithContext(context)
	} else if bounds.Close != nil {
		bounds.Close()
	}
}
func intervalDeferRange[I any](bounds IntervalRangeBounds[I], owned *bool, ambient IntervalPanicContext) {
	panicValue := recover()
	if panicValue != nil && ambient.ProcessingPanic {
		ambient.Panic(panicValue)
	}
	if *owned {
		context := ambient
		context.Unwinding = context.Unwinding || panicValue != nil
		close := func() { bounds.close(context) }
		if panicValue != nil || ambient.ProcessingPanic {
			indexStorageUnwindCleanup(close)
		} else {
			close()
		}
	}
	if panicValue != nil {
		panic(panicValue)
	}
}
func intervalProcessingGuard(context IntervalPanicContext) {
	if panicValue := recover(); panicValue != nil {
		context.Panic(panicValue)
	}
}
func (set *IntervalSet[I]) bounds(context IntervalPanicContext, bounds IntervalRangeBounds[I]) (uint32, uint32, bool) {
	owned := true
	defer intervalDeferRange(bounds, &owned, context)
	start := set.inclusiveStart(context, bounds.CloneWithContext(context))
	// inclusive_start destroys the clone before inclusive_end takes ownership
	// of the original, including when clone destruction itself panics.
	owned = false
	end, ok := set.inclusiveEnd(context, bounds)
	return start, end, ok && start <= end
}
