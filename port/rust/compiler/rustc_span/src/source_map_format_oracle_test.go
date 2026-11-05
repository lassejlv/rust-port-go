// SPDX-License-Identifier: MIT OR Apache-2.0
// Original SourceMap formatter/writer state, including independent continuation.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

type sourceMapFormatSink struct {
	out     strings.Builder
	calls   []string
	n, fail int
	address string
}

func (s *sourceMapFormatSink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *sourceMapFormatSink) WriteString(value string) (int, error) {
	s.n++
	normalized := value
	if value == s.address {
		normalized = "<ptr>"
	} else if value == "0x"+s.address {
		normalized = "0x<ptr>"
	}
	s.calls = append(s.calls, fmt.Sprintf("S:%x", normalized))
	if s.n == s.fail {
		return 0, fmt.Errorf("sink failure")
	}
	s.out.WriteString(normalized)
	return len(value), nil
}
func (s *sourceMapFormatSink) WriteChar(value rune) error {
	s.n++
	s.calls = append(s.calls, fmt.Sprintf("C:%x", value))
	if s.n == s.fail {
		return fmt.Errorf("sink failure")
	}
	_, err := s.out.WriteRune(value)
	return err
}
func sourceMapFormatMap(scenario, bits int, cfg SpanEncodingConfig) *SourceMap {
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(struct{}{}, nil), FileLoaderOps[struct{}]{FileExists: func(*struct{}, RustPath, rustfmt.Context) bool { return true }, ReadFile: func(*struct{}, RustPath, rustfmt.Context) (string, error) { return "", nil }, ReadBinaryFile: func(*struct{}, RustPath, rustfmt.Context) (Shared[[]byte], error) {
		return NewShared([]byte{}, nil), nil
	}, CurrentDirectory: func(*struct{}, rustfmt.Context) (RustPath, error) { return NewRustPath("/foo"), nil }})
	mapping := NewFilePathMapping(sourceMapTestEntries("/foo", "FOO", "/bar", "BAR"), RemapPathScopeComponents(bits))
	sm := NewSourceMapWithInputsAndConfig(SourceMapInputs{FileLoader: loader, PathMapping: mapping, HashKind: SourceFileHashMd5, ChecksumHashKind: Some(SourceFileHashSha1)}, cfg)
	if scenario > 0 {
		sm.NewSourceFile(RealFileNameFileName(sm.PathMapping().ToRealFilename(sm.WorkingDir(), NewRustPath("/foo/input.rs"))), "a…b").Close()
	}
	if scenario > 1 {
		sm.NewSourceFile(CustomFileName("custom\n\"\\ 🦀\u200d"), "x").Close()
		path := RustPathFromUnixBytes([]byte("/invalid/\xff\xed\xa0\x80\x00"))
		sm.NewSourceFile(RealFileNameFileName(RealFileNameFromVirtualPath(path)), "z").Close()
	}
	return sm
}
func sourceMapFormatOptions(style int) rustfmt.Options {
	if style == 2 {
		o := fatalFormatOptions(5)
		o.Fill = '*'
		return o
	}
	return fatalFormatOptions(style)
}
func sourceMapFormatFlags(o rustfmt.Options) string {
	width, precision, align := "None", "None", "None"
	if o.HasWidth {
		width = fmt.Sprintf("Some(%d)", o.Width)
	}
	if o.HasPrecision {
		precision = fmt.Sprintf("Some(%d)", o.Precision)
	}
	switch o.Align {
	case rustfmt.AlignLeft:
		align = "Some(Left)"
	case rustfmt.AlignRight:
		align = "Some(Right)"
	case rustfmt.AlignCenter:
		align = "Some(Center)"
	}
	return fmt.Sprintf("%t:%s:%s:%d:%s:%t:%t:%t", o.Alternate, width, precision, o.Fill, align, o.SignPlus, o.SignMinus, o.ZeroPad)
}
func TestSourceMapNativeFormatterOracle(t *testing.T) {
	file, err := os.Open("testdata/source_map_formats.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 65536), 8<<20)
	maps := map[string]*SourceMap{}
	defer func() {
		for _, sm := range maps {
			sm.Close()
		}
	}()
	n := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 9 {
			t.Fatal("invalid SourceMap formatter fixture")
		}
		scenario, _ := strconv.Atoi(p[1])
		bits, _ := strconv.Atoi(p[2])
		style, _ := strconv.Atoi(p[3])
		fail, _ := strconv.Atoi(p[4])
		key := strings.Join(p[:3], "/")
		sm := maps[key]
		if sm == nil {
			sm = sourceMapFormatMap(scenario, bits, SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'})
			maps[key] = sm
		}
		sink := &sourceMapFormatSink{fail: fail, address: fmt.Sprintf("%x", uintptr(unsafe.Pointer(&sm.fileLoader)))}
		f := &rustfmt.Formatter{Writer: sink, Options: sourceMapFormatOptions(style)}
		first := sm.WriteDebugFormatter(f)
		after := sourceMapFormatFlags(f.Options)
		second := rustfmt.FormatDebugUint(7, 16, f)
		ok := first == nil && second == nil
		if strconv.FormatBool(ok) != p[5] || sink.out.String() != sourceMapOracleString(p[6]) || strings.Join(sink.calls, "\n") != sourceMapOracleString(p[7]) || after != sourceMapOracleString(p[8]) {
			t.Fatalf("%s style%s fail%s: ok%t after%q want%q\noutput%q\nwant%q\ncalls%q\nwantcalls%q", key, p[3], p[4], ok, after, sourceMapOracleString(p[8]), sink.out.String(), sourceMapOracleString(p[6]), strings.Join(sink.calls, "\n"), sourceMapOracleString(p[7]))
		}
		n++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d original SourceMap formatter records", n)
}
