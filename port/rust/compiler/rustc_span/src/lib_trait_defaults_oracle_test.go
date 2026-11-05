// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type libOrderedValue[T any] interface {
	Equal(T) bool
	NotEqual(T) bool
	Compare(T) int
	PartialCompare(T) Option[int]
	Less(T) bool
	LessEqual(T) bool
	Greater(T) bool
	GreaterEqual(T) bool
	Min(T) T
	Max(T) T
	ClampWithContext(T, T, rustfmt.Context) T
	WriteDebugFormatter(*rustfmt.Formatter) error
}

func libDefaultDescription[T interface {
	WriteDebugFormatter(*rustfmt.Formatter) error
}](value T) string {
	var text strings.Builder
	f := rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions()}
	if err := value.WriteDebugFormatter(&f); err != nil {
		panic(err)
	}
	return hex.EncodeToString([]byte(text.String()))
}
func libDefaultLine[T libOrderedValue[T]](values []T, aIndex, bIndex, cIndex int, hookExpected string) string {
	a, b, c := values[aIndex], values[bIndex], values[cIndex]
	var hook string
	operations := &rustfmt.PanicOperations{}
	if hookExpected != "" {
		parts := strings.Split(hookExpected, ":")
		if len(parts) != 7 {
			panic("invalid source clamp hook profile")
		}
		file, err := hex.DecodeString(parts[2])
		if err != nil {
			panic(err)
		}
		line, err := strconv.ParseUint(parts[3], 10, 32)
		if err != nil {
			panic(err)
		}
		column, err := strconv.ParseUint(parts[4], 10, 32)
		if err != nil {
			panic(err)
		}
		operations.Locations = map[string]rustfmt.SourceLocation{LibOrdClampPanicSite: {File: string(file), Line: uint32(line), Column: uint32(column)}}
	}
	operations.Hook = func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) {
		if info.Location == nil {
			panic("missing declared clamp source location")
		}
		hook = fmt.Sprintf("%s:%s:%s:%d:%d:%t:%t", info.Kind, hex.EncodeToString([]byte(info.Payload.(string))), hex.EncodeToString([]byte(info.Location.File)), info.Location.Line, info.Location.Column, info.CanUnwind, info.ForceNoBacktrace)
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, operations)
	clamp := "panic"
	func() {
		defer func() { _ = recover() }()
		clamp = libDefaultDescription(a.ClampWithContext(b, c, context))
	}()
	if hook != hookExpected {
		panic("derived Ord clamp unexpectedly raised or omitted panic")
	}
	partial, ok := a.PartialCompare(b).Get()
	if !ok {
		panic("derived total partial order returned None")
	}
	return fmt.Sprintf("%t %t %d Some(%d) %t %t %t %t %s %s %s", a.Equal(b), a.NotEqual(b), a.Compare(b), partial, a.Less(b), a.LessEqual(b), a.Greater(b), a.GreaterEqual(b), libDefaultDescription(a.Min(b)), libDefaultDescription(a.Max(b)), clamp)
}

func libDefaultReals() []RealFileName {
	values := []RealFileName{EmptyRealFileName()}
	for _, path := range []string{"", "a", "a//b", "a/b", "a/./b", "../a", "/a", "é/β"} {
		values = append(values, RealFileNameFromVirtualPath(NewRustPath(path)))
	}
	return values
}
func libDefaultFilenames() []FileName {
	return []FileName{RealFileNameFileName(EmptyRealFileName()), RealFileNameFileName(RealFileNameFromVirtualPath(NewRustPath("a//b"))), RealFileNameFileName(RealFileNameFromVirtualPath(NewRustPath("a/b"))), CfgSpecFileName(rustchashes.ZeroHash64()), CfgSpecFileName(rustchashes.NewHash64(^uint64(0))), AnonFileName(rustchashes.ZeroHash64()), MacroExpansionFileName(rustchashes.NewHash64(7)), ProcMacroSourceCodeFileName(rustchashes.NewHash64(3)), CliCrateAttrFileName(rustchashes.NewHash64(5)), CustomFileName("a"), CustomFileName("β\x00"), DocTestSourceCode(RealFileNameFromVirtualPath(NewRustPath("a")), -1), InlineAsmFileName(rustchashes.NewHash64(9))}
}

type libEqualityValue[T any] interface {
	Equal(T) bool
	NotEqual(T) bool
	Clone() T
}

func libEqDefaultLine[T libEqualityValue[T]](values []T, a, b int, cloneFrom func(*T, T), close func(*T)) string {
	first, second := values[a], values[b]
	target := first.Clone()
	cloneFrom(&target, second)
	result := fmt.Sprintf("%t %t %t", first.Equal(second), first.NotEqual(second), target.Equal(second))
	if close != nil {
		close(&target)
	}
	return result
}

func libEqSourceLine(kind string, a, b int) string {
	names := []FileName{CustomFileName("a"), CustomFileName("b")}
	distinct := []DistinctSources{{Begin: FileNameAndBytePos{names[0], 1}, End: FileNameAndBytePos{names[1], 2}}, {Begin: FileNameAndBytePos{names[0], 1}, End: FileNameAndBytePos{names[1], 3}}}
	malformed := []MalformedSourceMapPositions{{Name: names[0], SourceLen: 3, BeginPos: 1, EndPos: 2}, {Name: names[1], SourceLen: 3, BeginPos: 1, EndPos: 2}}
	switch kind {
	case "spandata":
		return libEqDefaultLine([]SpanData{{Lo: 0, Hi: 1, Ctxt: RootSyntaxContext()}, {Lo: 0, Hi: 2, Ctxt: RootSyntaxContext()}}, a, b, func(v *SpanData, s SpanData) { v.CloneFrom(s) }, nil)
	case "multibyte":
		return libEqDefaultLine([]MultiByteChar{{Pos: 1, Bytes: 2}, {Pos: RelativeBytePos(^uint32(0)), Bytes: 255}}, a, b, func(v *MultiByteChar, s MultiByteChar) { v.CloneFrom(s) }, nil)
	case "normalized":
		return libEqDefaultLine([]NormalizedPos{{Pos: 1, Diff: 2}, {Pos: RelativeBytePos(^uint32(0)), Diff: ^uint32(0)}}, a, b, func(v *NormalizedPos, s NormalizedPos) { v.CloneFrom(s) }, nil)
	case "external-kind":
		values := []ExternalSourceKind{PresentExternalSourceKind(NewSharedString("a")), AbsentOkExternalSourceKind(), AbsentErrExternalSourceKind()}
		defer values[0].CloseWithContext(rustfmt.Context{})
		return libEqDefaultLine(values, a, b, func(v *ExternalSourceKind, s ExternalSourceKind) { v.CloneFrom(s) }, func(v *ExternalSourceKind) { v.CloseWithContext(rustfmt.Context{}) })
	case "external":
		values := []ExternalSource{UnneededExternalSource(), ForeignExternalSource(PresentExternalSourceKind(NewSharedString("a")), 17), ForeignExternalSource(AbsentErrExternalSourceKind(), ^uint32(0))}
		defer values[1].CloseWithContext(rustfmt.Context{})
		return libEqDefaultLine(values, a, b, func(v *ExternalSource, s ExternalSource) { v.CloneFrom(s) }, func(v *ExternalSource) { v.CloseWithContext(rustfmt.Context{}) })
	case "source-hash":
		return libEqDefaultLine([]SourceFileHash{NewSourceFileHashInMemory(SourceFileHashMd5, []byte("a")), NewSourceFileHashInMemory(SourceFileHashMd5, []byte("b")), NewSourceFileHashInMemory(SourceFileHashSha256, []byte("a"))}, a, b, func(v *SourceFileHash, s SourceFileHash) { v.CloneFrom(s) }, nil)
	case "line-info":
		return libEqDefaultLine([]LineInfo{{LineIndex: 1, StartCol: 2, EndCol: 3}, {LineIndex: ^uint(0), StartCol: 2, EndCol: 3}}, a, b, func(v *LineInfo, s LineInfo) { v.CloneFrom(s) }, nil)
	case "distinct":
		return libEqDefaultLine(distinct, a, b, func(v *DistinctSources, s DistinctSources) { v.CloneFrom(s) }, nil)
	case "malformed":
		return libEqDefaultLine(malformed, a, b, func(v *MalformedSourceMapPositions, s MalformedSourceMapPositions) { v.CloneFrom(s) }, nil)
	case "lines-error":
		return libEqDefaultLine([]*SpanLinesError{DistinctSourcesSpanLinesError(distinct[0]), DistinctSourcesSpanLinesError(distinct[1])}, a, b, func(v **SpanLinesError, s *SpanLinesError) { (*v).CloneFrom(s) }, nil)
	case "snippet-error":
		return libEqDefaultLine([]*SpanSnippetError{IllFormedSpanSnippetError(WithRootCtxt(nil, 1, 2)), DistinctSourcesSpanSnippetError(distinct[0]), MalformedSourceMapSpanSnippetError(malformed[0]), SourceNotAvailableSpanSnippetError(names[0])}, a, b, func(v **SpanSnippetError, s *SpanSnippetError) { (*v).CloneFrom(s) }, nil)
	case "inner":
		return libEqDefaultLine([]InnerSpan{NewInnerSpan(0, 1), NewInnerSpan(^uint(0), 1)}, a, b, func(v *InnerSpan, s InnerSpan) { v.CloneFrom(s) }, nil)
	case "scope":
		return libEqDefaultLine([]RemapPathScopeComponents{0, RemapPathScopeMacro, RemapPathScopeComponentsFromBitsRetain(255)}, a, b, func(v *RemapPathScopeComponents, s RemapPathScopeComponents) { v.CloneFrom(s) }, nil)
	}
	panic("unknown original Eq type")
}

func TestLibOriginalTraitDefaults(t *testing.T) {
	file, err := os.Open("testdata/lib-trait-defaults.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 16*1024*1024+1))
	if err != nil || len(data) > 16*1024*1024 {
		t.Fatal("invalid bounded default trait fixture", err)
	}
	algorithms := []SourceFileHashAlgorithm{SourceFileHashMd5, SourceFileHashSha1, SourceFileHashSha256, SourceFileHashBlake3}
	stable := []StableSourceFileId{StableSourceFileIdFromHash(rustchashes.NewHash128(rustchashes.Uint128{})), StableSourceFileIdFromHash(rustchashes.NewHash128(rustchashes.Uint128{Lo: 1})), StableSourceFileIdFromHash(rustchashes.NewHash128(rustchashes.Uint128{Hi: 1})), StableSourceFileIdFromHash(rustchashes.NewHash128(rustchashes.Uint128{Lo: ^uint64(0), Hi: ^uint64(0)}))}
	reals, filenames := libDefaultReals(), libDefaultFilenames()
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	count := 0
	for scan.Scan() {
		parts := strings.Split(scan.Text(), "\t")
		if len(parts) < 5 {
			t.Fatal("malformed trait default row")
		}
		if parts[1] == "eq" {
			if len(parts) != 6 {
				t.Fatal("invalid source Eq row")
			}
			a, _ := strconv.Atoi(parts[3])
			b, _ := strconv.Atoi(parts[4])
			if got := libEqSourceLine(parts[2], a, b); got != parts[5] {
				t.Fatalf("%s/%s Eq%d,%d:%s want%s", parts[0], parts[2], a, b, got, parts[5])
			}
			count++
			continue
		}
		if parts[1] == "owner" {
			operation := parts[3]
			expected, _ := strconv.Atoi(parts[4])
			got := -1
			if parts[2] == "custom" {
				a := CustomFileName(strings.Clone("same"))
				b, c := a.Clone(), a.Clone()
				pointers := []*byte{unsafe.StringData(a.custom), unsafe.StringData(b.custom), unsafe.StringData(c.custom)}
				result := a
				switch operation {
				case "min":
					result = a.Min(b)
				case "max":
					result = a.Max(b)
				case "clamp":
					result = a.Clamp(b, c)
				}
				for i, pointer := range pointers {
					if pointer == unsafe.StringData(result.custom) {
						got = i
					}
				}
			} else {
				a := RealFileNameFromVirtualPath(NewRustPath("same"))
				b, c := a.Clone(), a.Clone()
				pointers := []*byte{unsafe.StringData(a.Path(RemapPathScopeMacro).raw), unsafe.StringData(b.Path(RemapPathScopeMacro).raw), unsafe.StringData(c.Path(RemapPathScopeMacro).raw)}
				result := a
				switch operation {
				case "min":
					result = a.Min(b)
				case "max":
					result = a.Max(b)
				case "clamp":
					result = a.Clamp(b, c)
				}
				for i, pointer := range pointers {
					if pointer == unsafe.StringData(result.Path(RemapPathScopeMacro).raw) {
						got = i
					}
				}
			}
			if got != expected {
				t.Fatalf("owned tie %s/%s selected%d want%d", parts[2], operation, got, expected)
			}
			count++
			continue
		}
		if len(parts) != 8 || parts[1] != "ord" {
			t.Fatal("malformed order fixture")
		}
		a, _ := strconv.Atoi(parts[3])
		b, _ := strconv.Atoi(parts[4])
		c, _ := strconv.Atoi(parts[5])
		expectedHook, _ := hex.DecodeString(parts[7])
		got := ""
		switch parts[2] {
		case "algorithm":
			got = libDefaultLine(algorithms, a, b, c, string(expectedHook))
		case "stable":
			got = libDefaultLine(stable, a, b, c, string(expectedHook))
		case "byte":
			got = libDefaultLine([]BytePos{0, 1, 42, BytePos(^uint32(0))}, a, b, c, string(expectedHook))
		case "relative":
			got = libDefaultLine([]RelativeBytePos{0, 1, 42, RelativeBytePos(^uint32(0))}, a, b, c, string(expectedHook))
		case "char":
			got = libDefaultLine([]CharPos{0, 1, 42, CharPos(^uint(0))}, a, b, c, string(expectedHook))
		case "attr":
			got = libDefaultLine([]AttrId{AttrIdZero(), AttrIdFromU32(1), AttrIdFromU32(42), AttrIdMax()}, a, b, c, string(expectedHook))
		case "scope":
			got = libDefaultLine([]RemapPathScopeComponents{0, RemapPathScopeMacro, RemapPathScopeComponentsFromBitsRetain(4), AllRemapPathScopeComponents(), RemapPathScopeComponentsFromBitsRetain(255)}, a, b, c, string(expectedHook))
		case "real":
			got = libDefaultLine(reals, a, b, c, string(expectedHook))
		case "filename":
			got = libDefaultLine(filenames, a, b, c, string(expectedHook))
		case "guaranteed":
			got = libDefaultLine([]ErrorGuaranteed{UncheckedErrorGuaranteed()}, a, b, c, string(expectedHook))
		default:
			t.Fatal("unknown trait fixture type")
		}
		if got != parts[6] {
			t.Fatalf("%s/%s/%d,%d,%d differs\ngot %s\nwant%s", parts[0], parts[2], a, b, c, got, parts[6])
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 14104 {
		t.Fatalf("trait default corpus count%d", count)
	}
}
