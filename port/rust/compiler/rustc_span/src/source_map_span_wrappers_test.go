// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine SourceMap + registered local/foreign expansion graph composition.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const sourceMapWrapperExternal = "external λ file\n"

func sourceMapWrapperRepr(session *SessionGlobals, value Span, cfg SpanEncodingConfig) string {
	d := value.DataUntracked(session, cfg)
	parent := "_"
	if id, ok := d.Parent.Get(); ok {
		parent = strconv.FormatUint(uint64(id.LocalDefIndex.AsU32()), 10)
	}
	return fmt.Sprintf("%d,%d,%d,%s", d.Lo, d.Hi, d.Ctxt.AsU32(), parent)
}
func sourceMapWrapperReplay(p []string, tracked *[]uint32) string {
	config := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
	number := func(s string) int { n, _ := strconv.Atoi(s); return n }
	depth, mode, metas, edition := number(p[1]), number(p[2]), number(p[3]), number(p[4])
	session := NewSessionGlobals(Edition(edition), nil, None[SourceMapInputs](), config)
	defer session.Close()
	var reads atomic.Uint64
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(mode&8 != 0, nil), FileLoaderOps[bool]{
		FileExists: func(present *bool, _ RustPath, _ rustfmt.Context) bool { return *present },
		ReadFile: func(present *bool, _ RustPath, _ rustfmt.Context) (string, error) {
			reads.Add(1)
			if *present {
				return sourceMapWrapperExternal, nil
			}
			return "", NewSourceIOErrorKind("NotFound", "entity not found")
		},
		ReadBinaryFile: func(_ *bool, _ RustPath, _ rustfmt.Context) (Shared[[]byte], error) {
			return NewShared([]byte(sourceMapWrapperExternal), nil), nil
		},
		CurrentDirectory: func(_ *bool, _ rustfmt.Context) (RustPath, error) { return NewRustPath("/proof"), nil },
	})
	sm := NewSourceMapWithInputsAndConfig(SourceMapInputs{FileLoader: loader, PathMapping: EmptyFilePathMapping(), HashKind: SourceFileHashMd5}, config)
	defer sm.Close()
	local := sm.NewSourceFile(CustomFileName("local.rs"), strings.Repeat("local text\n", 32))
	defer local.Close()
	template, failure := NewSourceFile(RealFileNameFileName(RealFileNameFromVirtualPath(NewRustPath("foreign.rs"))), sourceMapWrapperExternal, SourceFileHashMd5, None[SourceFileHashAlgorithm]())
	if failure != nil {
		panic(failure)
	}
	if src, ok := template.Src.Get(); ok {
		defer src.Close()
	}
	imported := sm.NewImportedSourceFile(template.Name, template.SrcHash, template.ChecksumHash, template.StableId, template.NormalizedSourceLen.ToU32(), template.UnnormalizedSourceLen, CrateNumFromU32(1), template.Lines, template.MultibyteChars, template.NormalizedPos, 0)
	defer imported.Close()
	foreign := uint32(imported.Borrow().StartPos)
	ids := []ExpnId{RootExpnId()}
	contexts := []SyntaxContext{RootSyntaxContext()}
	current := RootSyntaxContext()
	for i := 0; i < depth; i++ {
		var kind ExpnKind
		switch (i + mode) % 8 {
		case 0:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: InternSymbol(session, "m")}
		case 1:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: AttrMacro, Name: InternSymbol(session, "a")}
		case 2:
			kind = ExpnKind{Tag: ExpnMacro, MacroKind: DeriveMacro, Name: InternSymbol(session, "d")}
		case 3:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarAsync}}
		case 4:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarAwait}}
		case 5:
			kind = ExpnKind{Tag: ExpnDesugaring, Desugaring: DesugaringKind{Tag: DesugarFormatLiteral, Source: true}}
		case 6:
			kind = ExpnKind{Tag: ExpnAstPass, AstPass: TestHarness}
		default:
			kind = ExpnKind{Tag: ExpnRoot}
		}
		at := uint32(10 + i)
		if mode&1 != 0 && i%2 == 0 {
			at = foreign + 1
		}
		ctxt := RootSyntaxContext()
		if mode&2 != 0 {
			ctxt = current
		}
		call := NewSpan(session, BytePos(at), BytePos(at+1), ctxt, Some(LocalDefId{LocalDefIndex: DefIndexFromU32(uint32(10 + i))}), config)
		def := DummySpan()
		switch (i + mode) % 3 {
		case 1:
			def = WithRootCtxt(session, 5, 6, config)
		case 2:
			def = WithRootCtxt(session, BytePos(foreign+2), BytePos(foreign+3), config)
		}
		foreignID := mode&16 != 0 && i%2 == 0
		parent := ids[i]
		wantedCrate := CrateNumFromU32(0)
		if foreignID {
			wantedCrate = CrateNumFromU32(1)
		}
		if parent.Krate != wantedCrate {
			parent = RootExpnId()
		}
		data := NewExpnData(kind, parent, call, def, None[Shared[[]Symbol]](), Edition(edition), None[DefId](), None[ModId](), false, false, false, mode&4 != 0 && i%2 == 0)
		var id ExpnId
		if foreignID {
			id = RegisterExpnId(session, CrateNumFromU32(1), ExpnIndexFromU32(uint32(i+1)), data, ExpnHash{fingerprint: ds.FingerprintFromU64(111, uint64(1000+i))})
		} else {
			id = FreshLocalExpnId(session, data, spanWrapperHcx{}).ToExpnId()
		}
		ids = append(ids, id)
		current = current.ApplyMark(session, id, Transparency((i+mode)%3))
		contexts = append(contexts, current)
	}
	spans := []Span{DummySpan()}
	for i, ctxt := range contexts {
		for _, pair := range [][2]uint32{{20, 2}, {foreign + 1, 2}, {foreign + 9, 1}} {
			spans = append(spans, NewSpan(session, BytePos(pair[0]), BytePos(pair[0]+pair[1]), ctxt, Some(LocalDefId{LocalDefIndex: DefIndexFromU32(uint32(30 + i))}), config))
		}
	}
	a := NewSpan(session, 2, 3, RootSyntaxContext(), Some(LocalDefId{LocalDefIndex: DefIndexFromU32(80)}), config)
	blo, bhi := uint32(30), uint32(35)
	if mode&1 != 0 {
		blo, bhi = foreign+1, foreign+3
	}
	b := NewSpan(session, BytePos(blo), BytePos(bhi), current, Some(LocalDefId{LocalDefIndex: DefIndexFromU32(81)}), config)
	actxt, bctxt := current, RootSyntaxContext()
	if metas%3 == 2 {
		actxt = RootSyntaxContext()
	}
	if metas%3 == 0 {
		bctxt = current
	}
	metaA := NewSpan(session, 60, 64, actxt, None[LocalDefId](), config)
	metaB := NewSpan(session, 70, 74, bctxt, None[LocalDefId](), config)
	mapping := session.metavarSpans
	if metas != 0 {
		if metas != 2 && !mapping.Insert(a, metaA) {
			panic("metavar insert A")
		}
		if metas != 1 && !mapping.Insert(b, metaB) {
			panic("metavar insert B")
		}
	}
	if metas >= 6 {
		if metas&1 != 0 {
			mapping.Get(a)
		}
		if metas&2 != 0 {
			mapping.Get(b)
		}
		mapping.FreezeAndGetReadSpans().Close()
	}
	spans = append(spans, a, b)
	out := []string{}
	optional := func(value Option[Span]) string {
		if span, ok := value.Get(); ok {
			return sourceMapWrapperRepr(session, span, config)
		}
		return "_"
	}
	emit := func(tag string, call func() string) {
		*tracked = (*tracked)[:0]
		result := func() (value string) {
			defer func() {
				if recover() != nil {
					value = "panic"
				}
			}()
			return call()
		}()
		tracks := make([]string, len(*tracked))
		for i, id := range *tracked {
			tracks[i] = strconv.FormatUint(uint64(id), 10)
		}
		out = append(out, fmt.Sprintf("%s=%s@%s#%d", tag, result, strings.Join(tracks, ","), reads.Load()))
	}
	for i, span := range spans {
		emit(fmt.Sprintf("visible%d", i), func() string { return strconv.FormatBool(span.IsVisible(session, sm)) })
		emit(fmt.Sprintf("external%d", i), func() string { return strconv.FormatBool(span.InExternalMacro(session, sm)) })
		emit(fmt.Sprintf("ancestor%d", i), func() string { return optional(span.FindAncestorNotFromExternMacro(session, sm)) })
		for j, within := range spans {
			emit(fmt.Sprintf("within%d,%d", i, j), func() string { return optional(span.WithinMacro(session, within, sm, config)) })
		}
	}
	frozen := mapping.FreezeAndGetReadSpans()
	defer frozen.Close()
	out = append(out, fmt.Sprintf("read=%t,%t;len%d", mapping.Get(a).IsSome(), mapping.Get(b).IsSome(), frozen.Len()))
	out = append(out, fmt.Sprintf("owners=%d,%d", local.StrongCount(), imported.StrongCount()))
	return strings.Join(out, "|")
}
func TestSourceMapSpanWrappersNativeOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_span_wrappers.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	tracked := []uint32{}
	old := SwapSpanTrackHook(NewSpanTrackHook(func(id LocalDefId, _ rustfmt.Context) { tracked = append(tracked, id.LocalDefIndex.AsU32()) }))
	defer SwapSpanTrackHook(old)
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	count, steps := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("invalid native Span/SourceMap record")
		}
		want := sourceMapOracleString(p[5])
		got := sourceMapWrapperReplay(p, &tracked)
		if got != want {
			a, b := strings.Split(got, "|"), strings.Split(want, "|")
			for i, s := range b {
				if i >= len(a) || a[i] != s {
					v := "<missing>"
					if i < len(a) {
						v = a[i]
					}
					t.Fatalf("case%s step%d Go%q Rust%q", strings.Join(p[:5], "/"), i, v, s)
				}
			}
			t.Fatal("different record length")
		}
		count++
		steps += strings.Count(want, "|") + 1
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d whole-original Span/SourceMap graphs, %d operation steps", count, steps)
}
