// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"strconv"
	"strings"
)

func hygieneDebugMap[K, V any](m *hashmap.Map[K, V], f *rustfmt.Formatter, key func(*K, *rustfmt.Formatter) error, value func(*V, *rustfmt.Formatter) error) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginMap(f.Writer, pretty)
	it := m.Iter(f.Context)
	for {
		pair, ok := it.Next(f.Context)
		if !ok {
			break
		}
		b.Entry(f.Value(pretty, func(f *rustfmt.Formatter) error { return key(pair.Key, f) }), f.Value(pretty, func(f *rustfmt.Formatter) error { return value(pair.Value, f) }))
	}
	return b.Finish()
}
func (d *HygieneData) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "HygieneData", pretty)
	b.Field("local_expn_data", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugVector(d.localExpnData, func(i uint) *Option[ExpnData] { return d.localExpnData.Raw.At(i) }, f, func(v *Option[ExpnData], f *rustfmt.Formatter) error {
			return hygieneDebugOption(v, f, func(v *ExpnData, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(session, f) })
		})
	}))
	b.Field("local_expn_hashes", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugVector(d.localExpnHashes, func(i uint) *ExpnHash { return d.localExpnHashes.Raw.At(i) }, f, func(v *ExpnHash, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("foreign_expn_data", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugMap(d.foreignExpnData, f, func(v *ExpnId, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }, func(v *ExpnData, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(session, f) })
	}))
	b.Field("foreign_expn_hashes", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugMap(d.foreignExpnHashes, f, func(v *ExpnId, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }, func(v *ExpnHash, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("expn_hash_to_expn_id", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugMap(d.expnHashToExpnId, f, func(v *ExpnHash, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }, func(v *ExpnId, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("syntax_context_data", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugVector(d.syntaxContextData, func(i uint) *syntaxContextData { return d.syntaxContextData.Raw.At(i) }, f, func(v *syntaxContextData, f *rustfmt.Formatter) error { return v.writeDebugFormatter(session, f) })
	}))
	b.Field("first_non_det_syntax_context", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugOption(&d.firstNonDetSyntaxContext, f, func(v *SyntaxContext, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("first_non_det_local_expn_id", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugOption(&d.firstNonDetLocalExpnId, f, func(v *LocalExpnId, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("syntax_context_map", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugMap(d.syntaxContextMap, f, func(v *SyntaxContextKey, f *rustfmt.Formatter) error {
			pretty := f.Options.Alternate
			return rustfmt.WriteTuple(f.Writer, "", pretty, []rustfmt.Value{f.Value(pretty, v.Parent.WriteDebugFormatter), f.Value(pretty, v.Expansion.WriteDebugFormatter), f.Value(pretty, v.Transparency.WriteDebugFormatter)})
		}, func(v *SyntaxContext, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("expn_data_disambiguators", f.Value(pretty, func(f *rustfmt.Formatter) error {
		return hygieneDebugMap(d.expnDataDisambiguators, f, func(v *rustchashes.Hash64, f *rustfmt.Formatter) error {
			return rustfmt.FormatDebugUint(v.AsU64(), 64, f)
		}, func(v *uint32, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(*v), 32, f) })
	}))
	return b.Finish()
}
func DebugHygieneData(session *SessionGlobals, verbose bool) string {
	return DebugHygieneDataWithContext(session, verbose, rustfmt.Context{})
}
func DebugHygieneDataWithContext(session *SessionGlobals, verbose bool, ctx rustfmt.Context) string {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	if verbose {
		var output strings.Builder
		options := rustfmt.DefaultOptions()
		options.Alternate = true
		f := &rustfmt.Formatter{Writer: &output, Options: options, Context: ctx}
		if err := d.WriteDebugFormatter(session, f); err != nil {
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "a formatting trait implementation returned an error when the underlying stream did not: Error", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::format_error")})
		}
		return output.String()
	}
	var output strings.Builder
	output.WriteString("Expansions:")
	appendExpansion := func(id ExpnId, data *ExpnData) {
		output.WriteByte('\n')
		output.WriteString(id.RustDebug())
		output.WriteString(": parent: ")
		output.WriteString(data.Parent.RustDebug())
		output.WriteString(", call_site_ctxt: ")
		output.WriteString(rustfmt.String(data.CallSite.Ctxt().WriteDebugFormatter))
		output.WriteString(", def_site_ctxt: ")
		output.WriteString(rustfmt.String(data.DefSite.Ctxt().WriteDebugFormatter))
		output.WriteString(", kind: ")
		output.WriteString(defIdDebugString(ctx, func(f *rustfmt.Formatter) error { return data.Kind.WriteDebugFormatter(session, f) }))
	}
	for i := uint(0); i < d.localExpnData.Len(); i++ {
		id := LocalExpnIdFromUsizeWithContext(i, ctx)
		data := d.localExpnData.Raw.At(i)
		if !data.present {
			hygieneOwnedPanic(ctx, 839, 52, "no expansion data for an expansion ID")
		}
		appendExpansion(id.ToExpnId(), &data.value)
	}
	type foreign struct {
		id   *ExpnId
		data *ExpnData
	}
	foreignData := []foreign{}
	iterator := d.foreignExpnData.Iter(ctx)
	for {
		pair, ok := iterator.Next(ctx)
		if !ok {
			break
		}
		foreignData = append(foreignData, foreign{pair.Key, pair.Value})
	}
	// The source collected element is (&ExpnId, &ExpnData): two Copy shared
	// references, size16/align8 on the selected 64-bit profile. sort_by_key
	// compares the Copy (CrateNum, ExpnIndex) keys lexicographically.
	rustsort.Stable(foreignData, rustsort.Type{Size: 16, Align: 8, Freeze: true, Copy: true}, func(left, right *foreign, _ rustfmt.Context) bool {
		a, b := left.id, right.id
		if a.Krate != b.Krate {
			return a.Krate.AsU32() < b.Krate.AsU32()
		}
		return a.LocalID.AsU32() < b.LocalID.AsU32()
	}, ctx)
	for _, foreign := range foreignData {
		appendExpansion(*foreign.id, foreign.data)
	}
	output.WriteString("\n\nSyntaxContexts:")
	for i := uint(0); i < d.syntaxContextData.Len(); i++ {
		data := d.syntaxContextData.Raw.At(i)
		output.WriteString("\n#")
		output.WriteString(strconv.FormatUint(uint64(i), 10))
		output.WriteString(": parent: ")
		output.WriteString(rustfmt.String(data.parent.WriteDebugFormatter))
		output.WriteString(", outer_mark: (")
		output.WriteString(data.outerExpn.RustDebug())
		output.WriteString(", ")
		output.WriteString(data.outerTransparency.RustDebug())
		output.WriteByte(')')
	}
	return output.String()
}
func WalkChain(session *SessionGlobals, span Span, to SyntaxContext) Span {
	return WalkChainWithContext(session, span, to, rustfmt.Context{})
}
func WalkChainWithContext(session *SessionGlobals, span Span, to SyntaxContext, ctx rustfmt.Context) Span {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	original := span
	hygieneTraceMessage(ctx, hygieneTraceWalk, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain(", ", ", ")"}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, to.WriteDebugFormatter)
	})
	hygieneTraceMessage(ctx, hygieneTraceWalkCtxt, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain: span ctxt = ", ""}, span.Ctxt().WriteDebugFormatter)
	})
	for span.Ctxt() != to && span.FromExpansion() {
		outer := d.outerExpn(span.Ctxt(), ctx)
		hygieneTraceMessage(ctx, hygieneTraceWalkOuter, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"walk_chain(", "): outer_expn=", ""}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, outer.WriteDebugFormatter)
		})
		data := d.expnData(outer, ctx)
		hygieneTraceMessage(ctx, hygieneTraceWalkData, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"walk_chain(", "): expn_data=", ""}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return data.WriteDebugFormatter(session, f) })
		})
		span = data.CallSite
	}
	hygieneTraceMessage(ctx, hygieneTraceWalkReturn, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain: for span ", " >>> return span = ", ""}, func(f *rustfmt.Formatter) error { return original.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) })
	})
	return span
}
func WalkChainCollapsed(session *SessionGlobals, span, to Span) Span {
	return WalkChainCollapsedWithContext(session, span, to, rustfmt.Context{})
}
func WalkChainCollapsedWithContext(session *SessionGlobals, span, to Span, ctx rustfmt.Context) Span {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	original := span
	ret := span
	hygieneTraceMessage(ctx, hygieneTraceCollapsed, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain_collapsed(", ", ", ")"}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return to.WriteDebugFormatter(session, f) })
	})
	hygieneTraceMessage(ctx, hygieneTraceCollapsedCtxt, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain_collapsed: span ctxt = ", ""}, span.Ctxt().WriteDebugFormatter)
	})
	for !span.Ctxt().IsRoot() && span.Ctxt() != to.Ctxt() {
		outer := d.outerExpn(span.Ctxt(), ctx)
		hygieneTraceMessage(ctx, hygieneTraceCollapsedOuter, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"walk_chain_collapsed(", "): outer_expn=", ""}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, outer.WriteDebugFormatter)
		})
		data := d.expnData(outer, ctx)
		hygieneTraceMessage(ctx, hygieneTraceCollapsedData, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"walk_chain_collapsed(", "): expn_data=", ""}, func(f *rustfmt.Formatter) error { return span.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return data.WriteDebugFormatter(session, f) })
		})
		span = data.CallSite
		if data.CollapseDebuginfo {
			ret = span
		}
	}
	hygieneTraceMessage(ctx, hygieneTraceCollapsedReturn, func(f *rustfmt.Formatter) error {
		return hygieneMessage(f, []string{"walk_chain_collapsed: for span ", " >>> return span = ", ""}, func(f *rustfmt.Formatter) error { return original.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return ret.WriteDebugFormatter(session, f) })
	})
	return ret
}
