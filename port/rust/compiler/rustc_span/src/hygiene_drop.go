// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func hygieneOwnedContext(snapshot rustfmt.Context, ambient []rustfmt.Context) rustfmt.Context {
	if len(ambient) > 0 {
		return ambient[0]
	}
	// Stored ownership context is only a panic-guard snapshot. Its runtime
	// metadata must never masquerade as a later caller's implicit Scope.
	snapshot.Runtime = nil
	return snapshot
}

// Source field drop glue continues through later fields after one destructor
// panics. A second escaping destructor panic is unrecoverable.
func hygieneDropFields(ctx rustfmt.Context, fields ...func(rustfmt.Context)) {
	for i, drop := range fields {
		func() {
			defer func() {
				if failure := recover(); failure != nil {
					unwind := ctx
					unwind.Unwinding = true
					for _, remaining := range fields[i+1:] {
						hygieneCleanup(unwind, func() { remaining(unwind) })
					}
					ctx.ResumeUnwind(failure)
				}
			}()
			drop(ctx)
		}()
	}
}
func (d *HygieneData) Close() { d.CloseWithContext(rustfmt.Context{}) }
func (d *HygieneData) CloseWithContext(ctx rustfmt.Context) {
	if d == nil {
		return
	}
	previous := *d.operationContext
	*d.operationContext = ctx
	defer func() { *d.operationContext = previous }()
	hygieneDropFields(ctx,
		func(ctx rustfmt.Context) {
			if d.localExpnData == nil {
				return
			}
			v := d.localExpnData
			d.localExpnData = nil
			v.Ops.DropWithContext = func(value *Option[ExpnData], inner index.IndexDropContext) {
				if value.present {
					c := ctx
					c.Unwinding = c.Unwinding || inner.Unwinding
					value.value.CloseWithContext(c)
				}
			}
			v.CloseWithContext(index.IndexDropContext{Unwinding: ctx.Unwinding})
		},
		func(ctx rustfmt.Context) {
			if d.localExpnHashes != nil {
				v := d.localExpnHashes
				d.localExpnHashes = nil
				v.CloseWithContext(index.IndexDropContext{Unwinding: ctx.Unwinding})
			}
		},
		func(ctx rustfmt.Context) {
			if d.foreignExpnData != nil {
				v := d.foreignExpnData
				d.foreignExpnData = nil
				v.Close(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if d.foreignExpnHashes != nil {
				v := d.foreignExpnHashes
				d.foreignExpnHashes = nil
				v.Close(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if d.expnHashToExpnId != nil {
				v := d.expnHashToExpnId
				d.expnHashToExpnId = nil
				v.Close(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if d.syntaxContextData != nil {
				v := d.syntaxContextData
				d.syntaxContextData = nil
				v.CloseWithContext(index.IndexDropContext{Unwinding: ctx.Unwinding})
			}
		},
		func(ctx rustfmt.Context) {
			if d.syntaxContextMap != nil {
				v := d.syntaxContextMap
				d.syntaxContextMap = nil
				v.Close(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if d.expnDataDisambiguators != nil {
				v := d.expnDataDisambiguators
				d.expnDataDisambiguators = nil
				v.Close(ctx)
			}
		},
	)
}
func (v *ExpnData) CloneFrom(other *ExpnData) { v.CloneFromWithContext(other, rustfmt.Context{}) }
func (v *ExpnData) CloneFromWithContext(other *ExpnData, ctx rustfmt.Context) {
	replacement := other.CloneWithContext(ctx)
	defer func() { *v = replacement }()
	v.CloseWithContext(ctx)
}
func (v *Transparency) CloneFrom(other Transparency)        { *v = other }
func (v *MacroKind) CloneFrom(other MacroKind)              { *v = other }
func (v *AstPass) CloneFrom(other AstPass)                  { *v = other }
func (v *DesugaringKind) CloneFrom(other DesugaringKind)    { *v = other }
func (v *ExpnKind) CloneFrom(other ExpnKind)                { *v = other }
func (v Transparency) NotEqual(other Transparency) bool     { return v != other }
func (v MacroKind) NotEqual(other MacroKind) bool           { return v != other }
func (v AstPass) NotEqual(other AstPass) bool               { return v != other }
func (v DesugaringKind) NotEqual(other DesugaringKind) bool { return !v.Equal(other) }
func (v ExpnKind) NotEqual(other ExpnKind) bool             { return !v.Equal(other) }
