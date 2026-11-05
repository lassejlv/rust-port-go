// SPDX-License-Identifier: MIT OR Apache-2.0
// All test declarations of compiler/rustc_span/src/source_map/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179, using real SourceMap dependencies.
package sourcemap_test

import (
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"

	. "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// The original cfg(windows) path_str helper makes its Unix-written fixtures
// platform specific, including its deliberately selected X: drive root.
func sourceMapTestPathString(value string) string {
	if runtime.GOOS != "windows" {
		return value
	}
	value = strings.ReplaceAll(value, "/", `\`)
	if rest, ok := strings.CutPrefix(value, `\`); ok {
		return `X:\` + rest
	}
	return value
}
func sourceMapTestPath(value string) RustPath { return NewRustPath(sourceMapTestPathString(value)) }

func sourceMapTestName(sm *SourceMap, name string) FileName {
	return RealFileNameFileName(sm.PathMapping().ToRealFilename(sm.WorkingDir(), sourceMapTestPath(name)))
}
func sourceMapTestMap(t *testing.T, multibyte bool) (*SourceMap, *SessionGlobals) {
	t.Helper()
	session := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	sm := NewSourceMap(EmptyFilePathMapping())
	if multibyte {
		sm.NewSourceFile(sourceMapTestName(sm, "blork.rs"), "fir€st €€€€ line.\nsecond line").Close()
		sm.NewSourceFile(sourceMapTestName(sm, "blork2.rs"), "first line€€.\n€ second line").Close()
	} else {
		sm.NewSourceFile(sourceMapTestName(sm, "blork.rs"), "first line.\nsecond line").Close()
		sm.NewSourceFile(sourceMapTestName(sm, "empty.rs"), "").Close()
		sm.NewSourceFile(sourceMapTestName(sm, "blork2.rs"), "first line.\nsecond line").Close()
	}
	t.Cleanup(func() { sm.Close(); session.Close() })
	return sm, session
}
func sourceMapTestCharPos(sm *SourceMap, pos BytePos) CharPos {
	file := sm.LookupSourceFile(pos)
	defer file.Close()
	return SourceMapByteposToFileCharposSourceTestWitness(file.Borrow(), file.Borrow().RelativePosition(pos), rustfmt.Context{}, sm.BuildConfig())
}
func TestSourceMapOriginalT3(t *testing.T) {
	sm, _ := sourceMapTestMap(t, false)
	for _, c := range []struct {
		pos   BytePos
		name  string
		local BytePos
	}{{23, "blork.rs", 23}, {24, "empty.rs", 0}, {25, "blork2.rs", 0}} {
		got := sm.LookupByteOffset(c.pos)
		if !got.SF.Borrow().Name.Equal(sourceMapTestName(sm, c.name)) || got.Pos != c.local {
			t.Errorf("%d: %v/%d", c.pos, got.SF.Borrow().Name, got.Pos)
		}
		got.SF.Close()
	}
}
func TestSourceMapOriginalT4(t *testing.T) {
	sm, _ := sourceMapTestMap(t, false)
	for _, c := range []struct {
		in  BytePos
		out CharPos
	}{{22, 22}, {25, 0}} {
		if got := sourceMapTestCharPos(sm, c.in); got != c.out {
			t.Fatalf("%d: %d", c.in, got)
		}
	}
}
func TestSourceMapOriginalT5(t *testing.T) {
	sm, _ := sourceMapTestMap(t, false)
	for _, c := range []struct {
		pos  BytePos
		name string
		line uint
		col  CharPos
	}{{22, "blork.rs", 2, 10}, {25, "blork2.rs", 1, 0}} {
		got := sm.LookupCharPos(c.pos)
		if !got.File.Borrow().Name.Equal(sourceMapTestName(sm, c.name)) || got.Line != c.line || got.Col != c.col {
			t.Errorf("%d: line%d col%d", c.pos, got.Line, got.Col)
		}
		got.File.Close()
	}
}
func TestSourceMapOriginalT6(t *testing.T) {
	sm, _ := sourceMapTestMap(t, true)
	for _, c := range []struct {
		in  BytePos
		out CharPos
	}{{3, 3}, {6, 4}, {56, 12}, {61, 15}} {
		if got := sourceMapTestCharPos(sm, c.in); got != c.out {
			t.Fatalf("%d: %d", c.in, got)
		}
	}
}
func TestSourceMapOriginalT7(t *testing.T) {
	sm, session := sourceMapTestMap(t, false)
	got, err := sm.SpanToLines(session, WithRootCtxt(session, 12, 23))
	if err != nil {
		t.Fatal(err)
	}
	defer got.File.Close()
	if !got.File.Borrow().Name.Equal(sourceMapTestName(sm, "blork.rs")) || len(got.Lines) != 1 || got.Lines[0].LineIndex != 1 {
		t.Fatal(got.Lines)
	}
}
func sourceMapTestSelection(session *SessionGlobals, input, selection string) Span {
	if len(input) != len(selection) {
		panic("invalid original selection")
	}
	return WithRootCtxt(session, BytePos(strings.IndexByte(selection, '~')), BytePos(strings.LastIndexByte(selection, '~')+1))
}
func TestSourceMapOriginalSpanToSnippetAndLinesSpanningMultipleLines(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	session := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	defer session.Close()
	input := "aaaaa\nbbbbBB\nCCC\nDDDDDddddd\neee\n"
	selection := "     \n    ~~\n~~~\n~~~~~     \n   \n"
	sm.NewSourceFile(sourceMapTestName(sm, "blork.rs"), input).Close()
	sp := sourceMapTestSelection(session, input, selection)
	if text, err := sm.SpanToSnippet(session, sp); err != nil || text != "BB\nCCC\nDDDDD" {
		t.Fatalf("%q %v", text, err)
	}
	lines, err := sm.SpanToLines(session, sp)
	if err != nil {
		t.Fatal(err)
	}
	defer lines.File.Close()
	if want := []LineInfo{{LineIndex: 1, StartCol: 4, EndCol: 6}, {LineIndex: 2, StartCol: 0, EndCol: 3}, {LineIndex: 3, StartCol: 0, EndCol: 5}}; !reflect.DeepEqual(lines.Lines, want) {
		t.Fatalf("%v want%v", lines.Lines, want)
	}
}
func TestSourceMapOriginalT8(t *testing.T) {
	sm, s := sourceMapTestMap(t, false)
	if got, err := sm.SpanToSnippet(s, WithRootCtxt(s, 12, 23)); err != nil || got != "second line" {
		t.Fatalf("%q %v", got, err)
	}
}
func TestSourceMapOriginalT9(t *testing.T) {
	sm, s := sourceMapTestMap(t, false)
	if got := sm.SpanToDiagnosticString(s, WithRootCtxt(s, 12, 23)); got != "blork.rs:2:1: 2:12" {
		t.Fatal(got)
	}
}

// merge_spans belongs only to the source test module and grants no production API.
func sourceMapTestMerge(sm *SourceMap, s *SessionGlobals, lhs, rhs Span) Option[Span] {
	if lhs.Ctxt() != rhs.Ctxt() {
		return None[Span]()
	}
	l, a, ok := sm.LookupLine(lhs.Hi(s))
	if !ok {
		a.Close()
		return None[Span]()
	}
	defer l.SF.Close()
	r, b, ok := sm.LookupLine(rhs.Lo(s))
	if !ok {
		b.Close()
		return None[Span]()
	}
	defer r.SF.Close()
	if l.Line != r.Line {
		return None[Span]()
	}
	if lhs.Lo(s) <= rhs.Lo(s) && lhs.Hi(s) <= rhs.Lo(s) {
		return Some(lhs.To(s, rhs))
	}
	return None[Span]()
}
func TestSourceMapOriginalSpanMergingFail(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	s := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	defer s.Close()
	input := "bbbb BB\ncc CCC\n"
	sm.NewSourceFile(sourceMapTestName(sm, "blork.rs"), input).Close()
	a := sourceMapTestSelection(s, input, "     ~~\n      \n")
	b := sourceMapTestSelection(s, input, "       \n   ~~~\n")
	if sourceMapTestMerge(sm, s, a, b).IsSome() {
		t.Fatal("merged spans on distinct lines")
	}
}
func TestSourceMapOriginalT10(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	raw := "first line.\r\nsecond line"
	normalized := "first line.\nsecond line"
	file := sm.NewSourceFile(sourceMapTestName(sm, "blork.rs"), raw)
	defer file.Close()
	f := file.Borrow()
	text, _ := f.Src.Get()
	if text.String() != normalized || !f.SrcHash.Matches(raw) {
		t.Fatal("source normalization/hash")
	}
	copy := f.Clone()
	defer copy.CloseWithContext(rustfmt.Context{})
	lineGuard := copy.Lines.Read()
	lines := lineGuard.Get().Clone()
	lineGuard.Close()
	imported := sm.NewImportedSourceFile(copy.Name.Clone(), copy.SrcHash, copy.ChecksumHash, copy.StableId, uint32(copy.NormalizedSourceLen), copy.UnnormalizedSourceLen, CrateNumZero(), NewFreezeLock(lines), append([]MultiByteChar(nil), copy.MultibyteChars...), append([]NormalizedPos(nil), copy.NormalizedPos...), 0)
	defer imported.Close()
	guard := imported.Borrow().ExternalSrc.Read()
	before := guard.Get().GetSource().IsSome()
	guard.Close()
	if before {
		t.Fatal("source was eagerly loaded")
	}
	imported.Borrow().AddExternalSrc(func() Option[string] { return Some(raw) })
	guard = imported.Borrow().ExternalSrc.Read()
	defer guard.Close()
	got, _ := guard.Get().GetSource().Get()
	if got != normalized {
		t.Fatalf("%q", got)
	}
}

func sourceMapTestEntries(pairs ...string) []RustPathMappingEntry {
	out := make([]RustPathMappingEntry, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, RustPathMappingEntry{From: sourceMapTestPath(pairs[i]), To: sourceMapTestPath(pairs[i+1])})
	}
	return out
}
func TestSourceMapOriginalPathPrefixRemapping(t *testing.T) {
	for _, from := range []string{"abc/def", "/abc/def"} {
		for _, to := range []string{"foo", "/foo"} {
			mapping := NewFilePathMapping(sourceMapTestEntries(from, to), AllRemapPathScopeComponents())
			for _, tail := range []string{"", "/src/main.rs"} {
				got, changed := mapping.MapPrefix(sourceMapTestPath(from + tail))
				if !changed || got.Path.EncodedString() != sourceMapTestPathString(to+tail) {
					t.Fatalf("%s→%s tail%s: %s", from, to, tail, got.Path.EncodedString())
				}
			}
		}
	}
}
func sourceMapTestRealCases(t *testing.T, scopes RemapPathScopeComponents, cwd string, entries []RustPathMappingEntry, cases [][8]string) {
	t.Helper()
	mapping := NewFilePathMapping(entries, scopes)
	empty := EmptyRealFileName()
	working := mapping.ToRealFilename(&empty, sourceMapTestPath(cwd))
	for _, c := range cases {
		wd := working
		if c[7] != "" {
			wd = mapping.ToRealFilename(&empty, sourceMapTestPath(c[7]))
		}
		got := mapping.ToRealFilename(&wd, sourceMapTestPath(c[0]))
		wantScopes := scopes
		if c[6] == "empty" {
			wantScopes = EmptyRemapPathScopeComponents()
		}
		want := SourceMapRealFileNameSourceTestWitness(sourceMapTestPath(c[0]), sourceMapTestPath(c[1]), sourceMapTestPath(c[2]), sourceMapTestPath(c[3]), sourceMapTestPath(c[4]), sourceMapTestPath(c[5]), wantScopes)
		if !got.Equal(want) {
			t.Fatalf("%s: got %s want %s", c[0], rustfmt.String(got.WriteDebugFormatter), rustfmt.String(want.WriteDebugFormatter))
		}
	}
}
func TestSourceMapOriginalToRealFilenameWithFullScopes(t *testing.T) {
	scopes := AllRemapPathScopeComponents()
	mapping := NewFilePathMapping(sourceMapTestEntries("/foo", "FOO", "/bar", "BAR"), scopes)
	empty := EmptyRealFileName()
	wd := mapping.ToRealFilename(&empty, sourceMapTestPath("/foo"))
	for _, scope := range []RemapPathScopeComponents{RemapPathScopeDiagnostics, RemapPathScopeMacro, RemapPathScopeDebugInfo} {
		if !wd.Path(scope).Equal(sourceMapTestPath("FOO")) {
			t.Fatal("cwd scope")
		}
	}
	sourceMapTestRealCases(t, scopes, "/foo", sourceMapTestEntries("/foo", "FOO", "/bar", "BAR"), [][8]string{
		{"/foo/src/main.rs", "/foo", "/foo/src/main.rs", "FOO/src/main.rs", "FOO", "FOO/src/main.rs", "", ""},
		{"/bar/src/main.rs", "/foo", "/bar/src/main.rs", "BAR/src/main.rs", "FOO", "BAR/src/main.rs", "", ""},
		{"/quux/src/main.rs", "/foo", "/quux/src/main.rs", "/quux/src/main.rs", "FOO", "/quux/src/main.rs", "", ""},
		{"src/main.rs", "/foo", "/foo/src/main.rs", "src/main.rs", "FOO", "FOO/src/main.rs", "", ""},
		{"./src/main.rs", "/foo", "/foo/src/main.rs", "./src/main.rs", "FOO", "FOO/src/main.rs", "", ""},
		{"quux/src/main.rs", "/abc", "/abc/quux/src/main.rs", "quux/src/main.rs", "/abc", "/abc/quux/src/main.rs", "empty", "/abc"},
	})
}
func TestSourceMapOriginalToRealFilenameWithMixedScopes(t *testing.T) {
	scopes := RemapPathScopeObject
	mapping := NewFilePathMapping(sourceMapTestEntries("/foo", "FOO", "/bar", "BAR"), scopes)
	empty := EmptyRealFileName()
	wd := mapping.ToRealFilename(&empty, sourceMapTestPath("/foo"))
	if !wd.Path(RemapPathScopeDiagnostics).Equal(sourceMapTestPath("/foo")) || !wd.Path(RemapPathScopeMacro).Equal(sourceMapTestPath("FOO")) || !wd.Path(RemapPathScopeDebugInfo).Equal(sourceMapTestPath("FOO")) {
		t.Fatal("cwd scopes")
	}
	sourceMapTestRealCases(t, scopes, "/foo", sourceMapTestEntries("/foo", "FOO", "/bar", "BAR"), [][8]string{
		{"/foo/src/main.rs", "/foo", "/foo/src/main.rs", "FOO/src/main.rs", "FOO", "FOO/src/main.rs", "", ""},
		{"/bar/src/main.rs", "/foo", "/bar/src/main.rs", "BAR/src/main.rs", "FOO", "BAR/src/main.rs", "", ""},
		{"/quux/src/main.rs", "/foo", "/quux/src/main.rs", "/quux/src/main.rs", "FOO", "/quux/src/main.rs", "", ""},
		{"src/main.rs", "/foo", "/foo/src/main.rs", "src/main.rs", "FOO", "FOO/src/main.rs", "", ""},
		{"quux/src/main.rs", "/abc", "/abc/quux/src/main.rs", "quux/src/main.rs", "/abc", "/abc/quux/src/main.rs", "empty", "/abc"},
	})
}
func TestSourceMapOriginalToRealFilenameWithoutRemappedCwd(t *testing.T) {
	scopes := RemapPathScopeObject
	mapping := NewFilePathMapping(sourceMapTestEntries("/foo", "FOO", "/cwd/bar", "CWDBAR"), scopes)
	empty := EmptyRealFileName()
	wd := mapping.ToRealFilename(&empty, sourceMapTestPath("/cwd"))
	for _, scope := range []RemapPathScopeComponents{RemapPathScopeDiagnostics, RemapPathScopeMacro, RemapPathScopeDebugInfo} {
		if !wd.Path(scope).Equal(sourceMapTestPath("/cwd")) {
			t.Fatal("cwd scopes")
		}
	}
	sourceMapTestRealCases(t, scopes, "/cwd", sourceMapTestEntries("/foo", "FOO", "/cwd/bar", "CWDBAR"), [][8]string{
		{"/foo/src/main.rs", "/cwd", "/foo/src/main.rs", "FOO/src/main.rs", "/cwd", "FOO/src/main.rs", "", ""},
		{"/bar/src/main.rs", "/cwd", "/bar/src/main.rs", "/bar/src/main.rs", "/cwd", "/bar/src/main.rs", "empty", ""},
		{"/cwd/bar/src/main.rs", "/cwd", "/cwd/bar/src/main.rs", "CWDBAR/src/main.rs", "/cwd", "CWDBAR/src/main.rs", "", ""},
		{"src/main.rs", "/cwd", "/cwd/src/main.rs", "src/main.rs", "/cwd", "/cwd/src/main.rs", "empty", ""},
	})
}
func TestSourceMapOriginalPathPrefixRemappingReverse(t *testing.T) {
	for _, c := range []struct {
		entries  []RustPathMappingEntry
		path     string
		expected Option[string]
	}{
		{sourceMapTestEntries("abc", "/", "def", "."), "/hello.rs", None[string]()}, {sourceMapTestEntries("abc", "/", "def", "."), "./hello.rs", None[string]()},
		{sourceMapTestEntries("abc", "/redacted", "def", "/redacted"), "/redacted/hello.rs", None[string]()},
		{sourceMapTestEntries("abc", "/redacted", "def/ghi", "/fake/dir"), "/redacted/path/hello.rs", Some("abc/path/hello.rs")}, {sourceMapTestEntries("abc", "/redacted", "def/ghi", "/fake/dir"), "/fake/dir/hello.rs", Some("def/ghi/hello.rs")},
	} {
		mapping := NewFilePathMapping(c.entries, AllRemapPathScopeComponents())
		got, ok := mapping.ReverseMapPrefixHeuristically(sourceMapTestPath(c.path)).Get()
		want, present := c.expected.Get()
		if ok != present || ok && got.EncodedString() != sourceMapTestPathString(want) {
			t.Fatalf("%s: %s %v, want%s %v", c.path, got.EncodedString(), ok, want, present)
		}
	}
}
func TestSourceMapOriginalNextPoint(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	s := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	defer s.Close()
	sm.NewSourceFile(sourceMapTestName(sm, "example.rs"), "a…b").Close()
	if text, err := sm.SpanToSnippet(s, WithRootCtxt(s, 0, 1)); err != nil || text != "a" {
		t.Fatalf("pre-advance snippet: %q %v", text, err)
	}
	for _, c := range []struct {
		lo, hi, outLo, outHi BytePos
		text                 string
		err                  bool
	}{{0, 0, 0, 0, "", false}, {0, 1, 1, 4, "…", false}, {1, 1, 1, 4, "…", false}, {1, 4, 4, 5, "b", false}, {4, 5, 5, 6, "", true}, {5, 5, 5, 6, "", true}} {
		got := sm.NextPoint(s, WithRootCtxt(s, c.lo, c.hi))
		if got.Lo(s) != c.outLo || got.Hi(s) != c.outHi {
			t.Fatalf("%d:%d→%d:%d", c.lo, c.hi, got.Lo(s), got.Hi(s))
		}
		text, err := sm.SpanToSnippet(s, got)
		if (err != nil) != c.err || err == nil && text != c.text {
			t.Fatalf("%d:%d: %q %v", c.lo, c.hi, text, err)
		}
	}
}
func TestSourceMapOriginalSpanFollowedByStopsAtEndOfFile(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	s := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	defer s.Close()
	sm.NewSourceFile(sourceMapTestName(sm, "example.rs"), "x").Close()
	if sm.SpanFollowedBy(s, WithRootCtxt(s, 0, 1), "y").IsSome() {
		t.Fatal("matched past EOF")
	}
}
func TestSourceMapOriginalSpanFollowedBySkipsWhitespace(t *testing.T) {
	sm := NewSourceMap(EmptyFilePathMapping())
	defer sm.Close()
	s := NewSpanEncodingSession(DefaultSpanEncodingConfig())
	defer s.Close()
	sm.NewSourceFile(sourceMapTestName(sm, "example.rs"), "x \n yz").Close()
	span, ok := sm.SpanFollowedBy(s, WithRootCtxt(s, 0, 1), "yz").Get()
	if !ok {
		t.Fatal("no match")
	}
	if text, err := sm.SpanToSnippet(s, span); err != nil || text != "yz" {
		t.Fatalf("%q %v", text, err)
	}
}
func TestSourceMapOriginalReadBinaryFileHandlesLyingStat(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("source cfg(target_os=linux)")
	}
	for _, path := range []string{"/proc/self/cmdline", "/sys/devices/system/cpu/kernel_max"} {
		meta, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		real, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if path == "/proc/self/cmdline" && meta.Size() >= int64(len(real)) || path != "/proc/self/cmdline" && meta.Size() <= int64(len(real)) {
			t.Fatal("host fixture no longer lies")
		}
		bin, err := (RealFileLoader{}).ReadBinaryFile(sourceMapTestPath(path))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*bin.Borrow(), real) {
			t.Fatalf("%s: content mismatch", path)
		}
		bin.Close()
	}
}
