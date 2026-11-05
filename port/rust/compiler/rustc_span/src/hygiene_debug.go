// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
)

func hygieneDefaultNumber(f *rustfmt.Formatter, prefix string, value uint32) error {
	if prefix != "" {
		if err := f.WriteString(prefix); err != nil {
			return err
		}
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
	return rustfmt.FormatDebugUint(uint64(value), 32, child)
}
func (v ExpnIndex) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return hygieneDefaultNumber(f, "", v.AsU32())
}
func (v LocalExpnId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return hygieneDefaultNumber(f, "expn", v.AsU32())
}
func (v DecodedSyntaxContext) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return hygieneDefaultNumber(f, "#decoded", v.AsU32())
}
func (v ExpnId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	// This custom write! gives each argument a fresh default formatter.
	child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
	if err := v.Krate.WriteDebugFormatter(child); err != nil {
		return err
	}
	if err := f.WriteString("::{{expn"); err != nil {
		return err
	}
	if err := rustfmt.FormatDebugUint(uint64(v.LocalID.AsU32()), 32, child); err != nil {
		return err
	}
	return f.WriteString("}}")
}
func (v ExpnHash) WriteDebugFormatter(f *rustfmt.Formatter) error {
	// Fingerprint's two-u64 derived tuple delegates to the same primitive
	// integer formatter as its complete port, preserving the outer state.
	pretty := f.Options.Alternate
	return rustfmt.WriteTuple(f.Writer, "ExpnHash", pretty, []rustfmt.Value{f.Value(pretty, func(f *rustfmt.Formatter) error {
		a, b := v.fingerprint.Split()
		inner := f.Options.Alternate
		return rustfmt.WriteTuple(f.Writer, "Fingerprint", inner, []rustfmt.Value{f.Value(inner, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(a.AsU64(), 64, f) }), f.Value(inner, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(b.AsU64(), 64, f) })})
	})})
}
func (v Transparency) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"Transparent", "SemiOpaque", "Opaque"}
	if int(v) >= len(names) {
		f.Context.Panic("invalid Go-only Transparency")
	}
	return f.WriteString(names[v])
}
func (v MacroKind) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"Bang", "Attr", "Derive"}
	if int(v) >= len(names) {
		f.Context.Panic("invalid Go-only MacroKind")
	}
	return f.WriteString(names[v])
}
func (v AstPass) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"StdImports", "TestHarness", "ProcMacroHarness"}
	if int(v) >= len(names) {
		f.Context.Panic("invalid Go-only AstPass")
	}
	return f.WriteString(names[v])
}
func (v DesugaringKind) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"QuestionMark", "TryBlock", "YeetExpr", "OpaqueTy", "Async", "Await", "ForLoop", "WhileLoop", "BoundModifier", "Contract", "PatTyRange", "FormatLiteral", "RangeExpr"}
	if int(v.Tag) >= len(names) {
		f.Context.Panic("invalid Go-only DesugaringKind")
	}
	if v.Tag != DesugarFormatLiteral {
		return f.WriteString(names[v.Tag])
	}
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "FormatLiteral", pretty, []rustfmt.Field{{Name: "source", Value: f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(v.Source, f) })}})
}
func (v ExpnKind) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	switch v.Tag {
	case ExpnRoot:
		return f.WriteString("Root")
	case ExpnMacro:
		return rustfmt.WriteTuple(f.Writer, "Macro", pretty, []rustfmt.Value{f.Value(pretty, v.MacroKind.WriteDebugFormatter), f.Value(pretty, func(f *rustfmt.Formatter) error { return v.Name.WriteDebugFormatter(session, f) })})
	case ExpnAstPass:
		return rustfmt.WriteTuple(f.Writer, "AstPass", pretty, []rustfmt.Value{f.Value(pretty, v.AstPass.WriteDebugFormatter)})
	case ExpnDesugaring:
		return rustfmt.WriteTuple(f.Writer, "Desugaring", pretty, []rustfmt.Value{f.Value(pretty, v.Desugaring.WriteDebugFormatter)})
	}
	f.Context.Panic("invalid Go-only ExpnKind")
	return nil
}
func hygieneDebugOption[T any](value *Option[T], f *rustfmt.Formatter, format func(*T, *rustfmt.Formatter) error) error {
	if !value.present {
		return f.WriteString("None")
	}
	pretty := f.Options.Alternate
	return rustfmt.WriteTuple(f.Writer, "Some", pretty, []rustfmt.Value{f.Value(pretty, func(f *rustfmt.Formatter) error { return format(&value.value, f) })})
}
func (v *ExpnData) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "ExpnData", pretty)
	b.Field("kind", f.Value(pretty, func(f *rustfmt.Formatter) error { return v.Kind.WriteDebugFormatter(session, f) }))
	b.Field("parent", f.Value(pretty, v.Parent.WriteDebugFormatter))
	b.Field("call_site", f.Value(pretty, func(f *rustfmt.Formatter) error { return v.CallSite.WriteDebugFormatter(session, f) }))
	b.Field("disambiguator", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(v.disambiguator), 32, f) }))
	b.Field("def_site", f.Value(pretty, func(f *rustfmt.Formatter) error { return v.DefSite.WriteDebugFormatter(session, f) }))
	b.Field("allow_internal_unstable", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugOption(&v.AllowInternalUnstable, f, func(shared *Shared[[]Symbol], f *rustfmt.Formatter) error {
			symbols := shared.BorrowWithContext(f.Context)
			pretty := f.Options.Alternate
			b := rustfmt.BeginList(f.Writer, pretty)
			for i := range *symbols {
				symbol := &(*symbols)[i]
				b.Entry(f.Value(pretty, func(f *rustfmt.Formatter) error { return symbol.WriteDebugFormatter(session, f) }))
			}
			return b.Finish()
		})
	}))
	b.Field("edition", f.Value(pretty, func(f *rustfmt.Formatter) error { return f.WriteString(v.Edition.DebugString()) }))
	b.Field("macro_def_id", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugOption(&v.MacroDefID, f, func(id *DefId, f *rustfmt.Formatter) error { return id.WriteDebugFormatter(f) })
	}))
	b.Field("parent_module", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugOption(&v.ParentModule, f, func(id *ModId, f *rustfmt.Formatter) error { return id.WriteDebugFormatter(f) })
	}))
	b.Field("allow_internal_unsafe", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(v.AllowInternalUnsafe, f) }))
	b.Field("local_inner_macros", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(v.LocalInnerMacros, f) }))
	b.Field("collapse_debuginfo", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(v.CollapseDebuginfo, f) }))
	b.Field("diagnostic_opaque", f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(v.DiagnosticOpaque, f) }))
	return b.Finish()
}
func (v ExpnId) RustDebug() string               { return rustfmt.String(v.WriteDebugFormatter) }
func (v ExpnHash) RustDebug() string             { return rustfmt.String(v.WriteDebugFormatter) }
func (v ExpnIndex) RustDebug() string            { return rustfmt.String(v.WriteDebugFormatter) }
func (v LocalExpnId) RustDebug() string          { return rustfmt.String(v.WriteDebugFormatter) }
func (v DecodedSyntaxContext) RustDebug() string { return rustfmt.String(v.WriteDebugFormatter) }
func (v Transparency) RustDebug() string         { return rustfmt.String(v.WriteDebugFormatter) }
func (v MacroKind) RustDebug() string            { return rustfmt.String(v.WriteDebugFormatter) }
func (v AstPass) RustDebug() string              { return rustfmt.String(v.WriteDebugFormatter) }
func (v DesugaringKind) RustDebug() string       { return rustfmt.String(v.WriteDebugFormatter) }
func (v ExpnKind) RustDebug(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return v.WriteDebugFormatter(session, f) })
}
func (v *ExpnData) RustDebug(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return v.WriteDebugFormatter(session, f) })
}

func (v syntaxContextData) writeDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "SyntaxContextData", pretty, []rustfmt.Field{
		{Name: "outer_expn", Value: f.Value(pretty, v.outerExpn.WriteDebugFormatter)},
		{Name: "outer_transparency", Value: f.Value(pretty, v.outerTransparency.WriteDebugFormatter)},
		{Name: "parent", Value: f.Value(pretty, v.parent.WriteDebugFormatter)},
		{Name: "opaque", Value: f.Value(pretty, v.opaque.WriteDebugFormatter)},
		{Name: "opaque_and_semiopaque", Value: f.Value(pretty, v.opaqueAndSemiopaque.WriteDebugFormatter)},
		{Name: "dollar_crate_name", Value: f.Value(pretty, func(f *rustfmt.Formatter) error { return v.dollarCrateName.WriteDebugFormatter(session, f) })},
	})
}
func hygieneDebugVector[T any](vector interface{ Len() uint }, at func(uint) *T, f *rustfmt.Formatter, format func(*T, *rustfmt.Formatter) error) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginList(f.Writer, pretty)
	for i := uint(0); i < vector.Len(); i++ {
		value := at(i)
		b.Entry(f.Value(pretty, func(f *rustfmt.Formatter) error { return format(value, f) }))
	}
	return b.Finish()
}

// WriteDebug permits fallible rendering with the same borrowed caller state.
func (v *ExpnData) WriteDebug(session *SessionGlobals, writer io.Writer, options rustfmt.Options) error {
	return v.WriteDebugFormatter(session, &rustfmt.Formatter{Writer: writer, Options: options})
}
