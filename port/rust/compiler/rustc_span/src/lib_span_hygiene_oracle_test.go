// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type spanWrapperHcx struct{}

func (spanWrapperHcx) StableHashSpan(s ds.RawSpan, h *ds.StableHasher) { h.WriteU64(s.Value) }
func (spanWrapperHcx) DefPathHash(id ds.RawDefId) ds.Fingerprint {
	return ds.FingerprintFromU64(uint64(id.Crate)+100, uint64(id.Index)+1000)
}
func (spanWrapperHcx) StableHashControls() ds.StableHashControls {
	return ds.StableHashControls{HashSpans: true}
}
func (spanWrapperHcx) AssertDefaultStableHashControls(string) {}
func spanWrapperData(session *SessionGlobals, d *ExpnData, cfg SpanEncodingConfig) string {
	refs := uint(0)
	if features, ok := d.AllowInternalUnstable.Get(); ok {
		refs = features.StrongCount()
	}
	return fmt.Sprintf("%d;%s;%s;%s;%s;%t;%t;%d", d.Parent.LocalID.AsU32(), geometryRepr(session, d.CallSite, cfg), geometryRepr(session, d.DefSite, cfg), d.Edition.DebugString(), d.Kind.Descr(session), d.LocalInnerMacros, d.DiagnosticOpaque, refs)
}
func spanWrapperScope(id Option[ExpnId]) string {
	if value, ok := id.Get(); ok {
		return fmt.Sprintf("%d,%d", value.Krate.AsU32(), value.LocalID.AsU32())
	}
	return "_"
}
func spanWrapperReasons() []DesugaringKind {
	result := []DesugaringKind{}
	for tag := DesugarQuestionMark; tag <= DesugarRangeExpr; tag++ {
		result = append(result, DesugaringKind{Tag: tag})
		if tag == DesugarFormatLiteral {
			result = append(result, DesugaringKind{Tag: tag, Source: true})
		}
	}
	return result
}
func spanWrapperReplay(request string, tracked *[]uint32) string {
	p := strings.Split(request, "\t")
	profile := p[0]
	n, mode, ed := int(geometryNumber(p[1])), int(geometryNumber(p[2])), int(geometryNumber(p[3]))
	cfg := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	session := NewSessionGlobals(Edition(ed), nil, None[SourceMapInputs](), cfg)
	defer session.Close()
	ids := []ExpnId{RootExpnId()}
	contexts := []SyntaxContext{RootSyntaxContext()}
	current := RootSyntaxContext()
	for i := 0; i < n; i++ {
		var kind ExpnKind
		switch (i + mode) % 7 {
		case 0:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: InternSymbol(session, "span_macro")}
		case 1:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: AttrMacro, Name: InternSymbol(session, "span_attr")}
		case 2:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: DeriveMacro, Name: InternSymbol(session, "span_derive")}
		case 3:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarAsync}}
		case 4:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarAwait}}
		case 5:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarFormatLiteral, Source: i%2 == 0}}
		default:
			kind = ExpnKind{Tag: ExpnAstPass, AstPass: TestHarness}
		}
		lo := 10 + i
		if mode&4 != 0 {
			lo = 10
		}
		callCtxt := RootSyntaxContext()
		if mode&1 != 0 {
			callCtxt = current
		}
		call := NewSpan(session, BytePos(lo), BytePos(lo+1), callCtxt, Some(LocalDefId{DefIndexFromU32(uint32(i + 1))}), cfg)
		parent := ids[i]
		if mode&2 != 0 {
			parent = ids[0]
		}
		features := None[Shared[[]Symbol]]()
		if i%2 == 0 {
			features = Some(NewShared([]Symbol{InternSymbol(session, "feature")}, nil))
		}
		d := NewExpnData(kind, parent, call, DummySpan(), features, Edition((i+mode)%5), None[DefId](), None[ModId](), i%2 == 0, i%3 == 0, i%2 == 0, i%3 == 0)
		id := FreshLocalExpnId(session, d, spanWrapperHcx{}).ToExpnId()
		ids = append(ids, id)
		current = current.ApplyMark(session, id, Transparency((i+mode)%3))
		contexts = append(contexts, current)
	}
	spans := []Span{}
	for i, c := range contexts {
		lo := 100 + i
		if mode&8 != 0 {
			lo = 10
		}
		spans = append(spans, NewSpan(session, BytePos(lo), BytePos(lo+1), c, Some(LocalDefId{DefIndexFromU32(uint32(i + 20))}), cfg))
	}
	outers := append([]Span{}, spans...)
	outers = append(outers, NewSpan(session, 0, 200, RootSyntaxContext(), None[LocalDefId](), cfg), NewSpan(session, 500, 600, RootSyntaxContext(), None[LocalDefId](), cfg))
	output := []string{}
	emit := func(tag string, call func() string) {
		*tracked = (*tracked)[:0]
		result := func() (result string) {
			defer func() {
				if recover() != nil {
					result = "panic"
				}
			}()
			return call()
		}()
		log := []string{}
		for _, id := range *tracked {
			log = append(log, fmt.Sprint(id))
		}
		output = append(output, tag+"="+result+"@"+strings.Join(log, ","))
	}
	repr := func(s Span) string { return geometryRepr(session, s, cfg) }
	optional := func(s Option[Span]) string { return geometryOptional(session, s, cfg) }
	for i, s := range spans {
		emit(fmt.Sprintf("flags%d", i), func() string {
			return fmt.Sprintf("%t,%t,%t,%t,%t,%t,%t,%t,%t,%t,%t", s.FromExpansion(), s.InDeriveExpansion(session), s.IsFromAsyncAwait(session), s.CanBeUsedForSuggestions(session, cfg), s.AllowsUnsafe(session), s.AllowsUnstable(session, InternSymbol(session, "feature")), s.AllowsUnstable(session, InternSymbol(session, "missing")), s.IsRust2015(session), s.AtLeastRust2018(session), s.AtLeastRust2021(session), s.AtLeastRust2024(session))
		})
		emit(fmt.Sprintf("edition%d", i), func() string { return s.Edition(session).DebugString() })
		emit(fmt.Sprintf("parent%d", i), func() string { return optional(s.ParentCallsite(session)) })
		emit(fmt.Sprintf("source%d", i), func() string { return repr(s.SourceCallsite(session)) })
		emit(fmt.Sprintf("notmacro%d", i), func() string { return optional(s.FindAncestorNotFromMacro(session)) })
		emit(fmt.Sprintf("desugar%d", i), func() string {
			kind := "None"
			if d, ok := s.DesugaringKind(session).Get(); ok {
				kind = "Some(" + d.RustDebug() + ")"
			}
			return fmt.Sprintf("%s,%t,%t", kind, s.IsDesugaring(session, DesugaringKind{Tag: DesugarAsync}), s.IsDesugaring(session, DesugaringKind{Tag: DesugarAwait}))
		})
		emit(fmt.Sprintf("callee%d", i), func() string {
			if d, ok := s.SourceCallee(session).Get(); ok {
				defer d.Close()
				return spanWrapperData(session, &d, cfg)
			}
			return "_"
		})
		emit(fmt.Sprintf("backtrace%d", i), func() string {
			iter := s.MacroBacktrace(session, cfg)
			results := []string{}
			for k := 0; k < n+3; k++ {
				lo, hi := iter.SizeHint()
				upper := "_"
				if v, ok := hi.Get(); ok {
					upper = fmt.Sprint(v)
				}
				next := "_"
				if d, ok := iter.Next().Get(); ok {
					next = spanWrapperData(session, &d, cfg)
					d.Close()
				}
				results = append(results, fmt.Sprintf("%d:%s:%s", lo, upper, next))
			}
			return strings.Join(results, "/")
		})
		emit(fmt.Sprintf("normal20%d", i), func() string { return repr(s.NormalizeToMacros20(session, cfg)) })
		emit(fmt.Sprintf("normalrules%d", i), func() string { return repr(s.NormalizeToMacroRules(session, cfg)) })
		emit(fmt.Sprintf("remove%d", i), func() string {
			v := s
			id := v.RemoveMark(session, cfg)
			return repr(v) + ";" + spanWrapperScope(Some(id))
		})
		for j, outer := range outers {
			emit(fmt.Sprintf("inside%d,%d", i, j), func() string { return optional(s.FindAncestorInside(session, outer, cfg)) })
			emit(fmt.Sprintf("same%d,%d", i, j), func() string { return optional(s.FindAncestorInSameCtxt(session, outer)) })
			emit(fmt.Sprintf("insames%d,%d", i, j), func() string { return optional(s.FindAncestorInsideSameCtxt(session, outer, cfg)) })
		}
		for j, id := range ids {
			emit(fmt.Sprintf("adjust%d,%d", i, j), func() string {
				v := s
				mark := v.Adjust(session, id, cfg)
				return repr(v) + ";" + spanWrapperScope(mark)
			})
			emit(fmt.Sprintf("normaladjust%d,%d", i, j), func() string {
				v := s
				mark := v.NormalizeToMacros20AndAdjust(session, id, cfg)
				return repr(v) + ";" + spanWrapperScope(mark)
			})
			for tr := 0; tr < 3; tr++ {
				emit(fmt.Sprintf("apply%d,%d,%d", i, j, tr), func() string { return repr(s.ApplyMark(session, id, Transparency(tr), cfg)) })
			}
			emit(fmt.Sprintf("sites%d,%d", i, j), func() string {
				return repr(s.WithDefSiteCtxt(session, id, cfg)) + ";" + repr(s.WithCallSiteCtxt(session, id, cfg)) + ";" + repr(s.WithMixedSiteCtxt(session, id, cfg))
			})
		}
	}
	for i, id := range ids {
		emit(fmt.Sprintf("final%d", i), func() string { d := id.ExpnData(session); defer d.Close(); return spanWrapperData(session, &d, cfg) })
	}
	if n <= 2 && mode < 3 {
		for i, s := range spans {
			for ed := 0; ed < 5; ed++ {
				for k, reason := range spanWrapperReasons() {
					for feature := 0; feature < 3; feature++ {
						emit(fmt.Sprintf("reason%d,%d,%d,%d", i, ed, k, feature), func() string {
							held := None[Shared[[]Symbol]]()
							incoming := None[Shared[[]Symbol]]()
							if feature != 0 {
								symbols := []Symbol{}
								if feature == 2 {
									symbols = append(symbols, InternSymbol(session, "feature"))
								}
								v := NewShared(symbols, nil)
								defer v.Close()
								held = Some(v)
								incoming = Some(v.Clone())
							}
							marked := s.MarkWithReason(session, incoming, reason, Edition(ed), spanWrapperHcx{})
							d := marked.Ctxt().OuterExpnData(session)
							detail := spanWrapperData(session, &d, cfg)
							d.Close()
							refs := uint(0)
							if v, ok := held.Get(); ok {
								refs = v.StrongCount()
							}
							return fmt.Sprintf("%s;%s;%d", repr(marked), detail, refs)
						})
					}
				}
			}
		}
	}
	return strings.Join(output, "|")
}

func TestOriginalSpanHygieneWrappers(t *testing.T) {
	metadata, err := os.ReadFile("../../../../../tools/port-rust-span-lib/span-hygiene-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records, Steps int
		RawSHA         string          `json:"raw_sha256"`
		FixtureSHA     string          `json:"fixture_sha256"`
		Files          []spanProofFile `json:"files"`
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	verifySpanProofFiles(t, manifest.Files)
	blob, err := os.ReadFile("testdata/lib_span_hygiene.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(blob)) != manifest.FixtureSHA {
		t.Fatal("Span hygiene fixture changed")
	}
	r, err := gzip.NewReader(strings.NewReader(string(blob)))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var tracked []uint32
	old := SwapSpanTrackHook(NewSpanTrackHook(func(id LocalDefId, _ rustfmt.Context) { tracked = append(tracked, id.LocalDefIndex.AsU32()) }))
	defer SwapSpanTrackHook(old)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	hash := sha256.New()
	count, steps := 0, 0
	for scanner.Scan() {
		line := scanner.Text()
		hash.Write([]byte(line + "\n"))
		pair := strings.SplitN(line, "\t|\t", 2)
		if len(pair) != 2 {
			t.Fatal("invalid Span hygiene record")
		}
		got := spanWrapperReplay(pair[0], &tracked)
		count++
		steps += strings.Count(pair[1], "|") + 1
		if got != pair[1] {
			wantSteps, gotSteps := strings.Split(pair[1], "|"), strings.Split(got, "|")
			for i, want := range wantSteps {
				if i >= len(gotSteps) || want != gotSteps[i] {
					actual := "<missing>"
					if i < len(gotSteps) {
						actual = gotSteps[i]
					}
					t.Fatalf("graph%s step%d\nGo %s\nRust %s", pair[0], i, actual, want)
				}
			}
			t.Fatalf("Span graph%s step count%d/%d", pair[0], len(gotSteps), len(wantSteps))
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records || steps != manifest.Steps || fmt.Sprintf("%x", hash.Sum(nil)) != manifest.RawSHA {
		t.Fatal("Span hygiene fixture coverage differs")
	}
}
