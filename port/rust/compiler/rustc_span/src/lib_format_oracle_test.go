package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type libFormatSink struct {
	calls    []string
	accepted strings.Builder
	fail     int
	chars    bool
}

var libFormatFailure = errors.New("source formatter sink failure")

func (s *libFormatSink) WriteString(text string) (int, error) {
	n := len(s.calls)
	s.calls = append(s.calls, "S:"+hex.EncodeToString([]byte(text)))
	if n == s.fail {
		return 0, libFormatFailure
	}
	return s.accepted.WriteString(text)
}
func (s *libFormatSink) Write(data []byte) (int, error) { return s.WriteString(string(data)) }
func (s *libFormatSink) WriteChar(ch rune) error {
	if !s.chars {
		_, err := s.WriteString(string(ch))
		return err
	}
	n := len(s.calls)
	s.calls = append(s.calls, "C:"+hex.EncodeToString([]byte(string(ch))))
	if n == s.fail {
		return libFormatFailure
	}
	s.accepted.WriteRune(ch)
	return nil
}
func libFormatOptions(mode int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch mode {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugLowerHex = true
		o.Alternate = true
	case 4:
		o.DebugUpperHex = true
	case 5:
		o.DebugUpperHex = true
		o.Alternate = true
	case 6:
		o.SignPlus = true
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
	case 7:
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 8
		o.DebugLowerHex = true
	case 8:
		o.Fill = '*'
		o.Align = rustfmt.AlignRight
		o.HasWidth = true
		o.Width = 18
	case 9:
		o.HasPrecision = true
		o.Precision = 2
	case 10:
		o.Fill = '🔥'
		o.Align = rustfmt.AlignCenter
		o.HasWidth = true
		o.Width = 20
	case 11:
		o.SignPlus = true
		o.Alternate = true
		o.ZeroPad = true
		o.HasWidth = true
		o.Width = 18
		o.DebugLowerHex = true
	default:
		panic("invalid formatter mode")
	}
	return o
}
func libFormatValues(session *SessionGlobals) (map[string]func(*rustfmt.Formatter) error, func()) {
	values := make(map[string]func(*rustfmt.Formatter) error)
	for bits := 0; bits < 256; bits++ {
		value := RemapPathScopeComponentsFromBitsRetain(uint8(bits))
		values[fmt.Sprintf("scope/%d", bits)] = value.WriteDebugFormatter
	}
	real := RealFileNameFromVirtualPath(NewRustPath("/réel/\"file.rs"))
	values["real/empty"] = EmptyRealFileName().WriteDebugFormatter
	values["real/virtual"] = real.WriteDebugFormatter
	filenames := []FileName{RealFileNameFileName(real.Clone()), CfgSpecFileName(rustchashes.NewHash64(0xfedcba9876543210)), AnonFileName(rustchashes.ZeroHash64()), MacroExpansionFileName(rustchashes.NewHash64(31)), ProcMacroSourceCodeFileName(rustchashes.NewHash64(26)), CliCrateAttrFileName(rustchashes.NewHash64(10)), CustomFileName("é\n\x00\U000323af"), DocTestSourceCode(real.Clone(), -26), InlineAsmFileName(rustchashes.NewHash64(^uint64(0)))}
	for i, value := range filenames {
		values[fmt.Sprintf("filename/%d", i)] = value.WriteDebugFormatter
	}
	for i, kind := range []SourceFileHashAlgorithm{SourceFileHashMd5, SourceFileHashSha1, SourceFileHashSha256, SourceFileHashBlake3} {
		values[fmt.Sprintf("algorithm/%d", i)] = kind.WriteDebugFormatter
		value := NewSourceFileHashInMemory(kind, []byte("source\x00\xff"))
		values[fmt.Sprintf("hash/%d", i)] = value.WriteDebugFormatter
	}
	external := []ExternalSourceKind{PresentExternalSourceKind(NewSharedString("é\n\x00\U000323af")), AbsentOkExternalSourceKind(), AbsentErrExternalSourceKind()}
	for i, value := range external {
		values[fmt.Sprintf("external-kind/%d", i)] = value.WriteDebugFormatter
		foreign := ForeignExternalSource(value, ^uint32(0)-10)
		values[fmt.Sprintf("external/%d", i)] = foreign.WriteDebugFormatter
	}
	values["external/unneeded"] = UnneededExternalSource().WriteDebugFormatter
	values["bytepos"] = BytePos(^uint32(0) - 10).WriteDebugFormatter
	values["relativepos"] = RelativeBytePos(^uint32(0) - 10).WriteDebugFormatter
	values["charpos"] = CharPos(^uint(0) - 10).WriteDebugFormatter
	values["multibyte"] = (MultiByteChar{Pos: RelativeBytePos(^uint32(0) - 10), Bytes: 255}).WriteDebugFormatter
	values["normalized"] = (NormalizedPos{Pos: RelativeBytePos(^uint32(0) - 10), Diff: ^uint32(0) - 26}).WriteDebugFormatter
	sf, err := NewSourceFile(CustomFileName("é\n"), "abcdef\n", SourceFileHashSha256, None[SourceFileHashAlgorithm]())
	if err != nil {
		panic(err)
	}
	file := NewShared(sf, func(v *SourceFile, ctx rustfmt.Context) { v.CloseWithContext(ctx) })
	values["sourcefile"] = file.Borrow().WriteDebugFormatter
	values["stableid"] = sf.StableId.WriteDebugFormatter
	values["loc"] = (Loc{File: file, Line: ^uint(0) - 10, Col: 31, ColDisplay: 26}).WriteDebugFormatter
	values["sfline"] = (SourceFileAndLine{SF: file, Line: ^uint(0) - 10}).WriteDebugFormatter
	values["sfpos"] = (SourceFileAndBytePos{SF: file, Pos: BytePos(^uint32(0) - 10)}).WriteDebugFormatter
	values["lineinfo"] = (LineInfo{LineIndex: ^uint(0) - 10, StartCol: 31, EndCol: 26}).WriteDebugFormatter
	values["inner"] = NewInnerSpan(^uint(0)-10, 31).WriteDebugFormatter
	values["overflow"] = (OffsetOverflowError{}).WriteDebugFormatter
	values["guaranteed"] = UncheckedErrorGuaranteed().WriteDebugFormatter
	values["attr"] = AttrIdFromU32(^uint32(0) - 255).WriteDebugFormatter
	span := WithRootCtxt(session, 10, 31)
	spanned := Respan(span, int64(-26))
	values["spanned"] = func(f *rustfmt.Formatter) error {
		return spanned.WriteDebugFormatterWith(session, f, SpannedOps[int64]{Debug: func(value *int64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(*value, 64, f) }})
	}
	distinct := DistinctSources{Begin: FileNameAndBytePos{Name: filenames[6].Clone(), Pos: 10}, End: FileNameAndBytePos{Name: filenames[8].Clone(), Pos: 31}}
	values["distinct"] = distinct.WriteDebugFormatter
	values["lines-error"] = DistinctSourcesSpanLinesError(distinct.Clone()).WriteDebugFormatter
	snippets := map[string]*SpanSnippetError{"snippet-ill": IllFormedSpanSnippetError(span), "snippet-distinct": DistinctSourcesSpanSnippetError(distinct.Clone()), "snippet-unavailable": SourceNotAvailableSpanSnippetError(filenames[6].Clone())}
	malformed := MalformedSourceMapPositions{Name: filenames[6].Clone(), SourceLen: ^uint(0) - 10, BeginPos: 10, EndPos: 31}
	values["malformed"] = malformed.WriteDebugFormatter
	snippets["snippet-malformed"] = MalformedSourceMapSpanSnippetError(malformed.Clone())
	for name, value := range snippets {
		values[name] = func(f *rustfmt.Formatter) error { return value.WriteDebugFormatter(session, f) }
	}
	return values, func() {
		file.Close()
		for i := range external {
			external[i].CloseWithContext(rustfmt.Context{})
		}
	}
}
func TestLibOriginalFormatters(t *testing.T) {
	session := NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs]())
	defer session.Close()
	values, cleanup := libFormatValues(session)
	defer cleanup()
	libReplayOriginalFormatters(t, "testdata/lib-format.tsv.gz", values, 63164)
}
func libReplayOriginalFormatters(t *testing.T, path string, values map[string]func(*rustfmt.Formatter) error, expectedCount int) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 64*1024*1024+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 64*1024*1024 {
		t.Fatal("formatter fixture exceeds bound")
	}
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	scan.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	var records [][]string
	fulls := make(map[string][]string)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 8 {
			t.Fatalf("invalid formatter record %d", count)
		}
		records = append(records, p)
		if p[7] != "@" && p[4] == "true" {
			fulls[strings.Join(p[:3], "\t")] = p
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	for _, p := range records {
		mode, err := strconv.Atoi(p[1])
		if err != nil {
			t.Fatal(err)
		}
		chars, err := strconv.ParseBool(p[2])
		if err != nil {
			t.Fatal(err)
		}
		fail, err := strconv.Atoi(p[3])
		if err != nil {
			t.Fatal(err)
		}
		sink := libFormatSink{fail: fail, chars: chars}
		f := rustfmt.Formatter{Writer: &sink, Options: libFormatOptions(mode)}
		before := fmt.Sprintf("%d:%d", f.Options.Fill, f.Options.Align)
		format, ok := values[p[0]]
		if !ok {
			t.Fatal(p[0])
		}
		result := format(&f)
		after := fmt.Sprintf("%d:%d", f.Options.Fill, f.Options.Align)
		got := []string{strconv.FormatBool(result == nil), before + ">" + after, hex.EncodeToString([]byte(sink.accepted.String())), strings.Join(sink.calls, ",")}
		want := append([]string(nil), p[4:]...)
		if p[6] == "@" && p[7] == "@" {
			full := fulls[strings.Join(p[:3], "\t")]
			if full == nil {
				t.Fatal("missing original complete formatter observation")
			}
			calls := []string{}
			if full[7] != "" {
				calls = strings.Split(full[7], ",")
			}
			if fail < 0 || fail >= len(calls) {
				t.Fatal("invalid original formatter prefix reference")
			}
			var accepted strings.Builder
			for _, call := range calls[:fail] {
				parts := strings.SplitN(call, ":", 2)
				if len(parts) != 2 {
					t.Fatal("invalid original call")
				}
				accepted.WriteString(parts[1])
			}
			want[2] = accepted.String()
			want[3] = strings.Join(calls[:fail+1], ",")
		}
		if strings.Join(got, "\t") != strings.Join(want, "\t") {
			t.Fatalf("%s mode%s chars%s fail%s\nGo: %q\nRust: %q", p[0], p[1], p[2], p[3], got, want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != expectedCount {
		t.Fatalf("formatter records %d; expected %d", count, expectedCount)
	}
	t.Logf("original formatter records: %d", count)
}
