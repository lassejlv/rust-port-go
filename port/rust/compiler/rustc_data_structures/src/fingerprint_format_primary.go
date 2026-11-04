// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete shared Formatter entrypoint for fingerprint.rs's derived Debug.
// The older NumericFormatter entrypoint remains available to existing callers.
package datastructures

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (value Fingerprint) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "Fingerprint", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(value.first, 64, child) }))
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(value.second, 64, child) }))
	return b.Finish()
}
