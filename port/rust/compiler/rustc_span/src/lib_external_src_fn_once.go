// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// AddExternalSrcConsuming is SourceFile::add_external_src's complete FnOnce
// entry point. Call receives the actual owned Self and may Take/Move/forget it;
// any Self still owned when call finishes is destroyed before source checking
// and normalization. An unused callback is destroyed at the original function
// boundary. The operation dictionary and its borrowed state own no captures;
// all owned captures and complete field destruction belong in environment.
// AddExternalSrcOwned remains the legacy borrowed-Self convenience.
func AddExternalSrcConsuming[E any](value *SourceFile, environment *rustvec.Owned[E], call func(*rustvec.Owned[E], rustfmt.Context) Option[string], context rustfmt.Context, config ...SpanEncodingConfig) bool {
	owned := environment.MoveWithContext(context)
	envelope := NewSpanOwned(owned, func(slot **rustvec.Owned[E], current rustfmt.Context) {
		(*slot).DropWithContext(current)
	})
	return AddExternalSrcOwned(value, envelope, func(slot **rustvec.Owned[E], current rustfmt.Context) Option[string] {
		return rustvec.ConsumeValue(*slot, current, func(argument *rustvec.Owned[E]) Option[string] {
			return call(argument, current)
		})
	}, context, config...)
}
