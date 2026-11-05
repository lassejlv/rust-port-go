// SPDX-License-Identifier: MIT OR Apache-2.0
// Replays observations made by the complete original compiler crate.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func sourceMapOracleString(s string) string {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
func sourceMapOracleError(e *SpanSnippetError) string {
	switch e.kind {
	case snippetIllFormed:
		return "ill"
	case snippetDistinct:
		return "distinct"
	case snippetMalformed:
		return fmt.Sprintf("malformed:%d:%d:%d", e.malformed.SourceLen, e.malformed.BeginPos, e.malformed.EndPos)
	case snippetUnavailable:
		return "unavailable"
	}
	panic("bad error")
}
func sourceMapOracleStringResult(v string, e *SpanSnippetError) string {
	if e != nil {
		return "err:" + sourceMapOracleError(e)
	}
	return "ok:" + hex.EncodeToString([]byte(v))
}
func sourceMapOracleSpan(s *SessionGlobals, v Span) string {
	return fmt.Sprintf("%d:%d", v.Lo(s), v.Hi(s))
}
func sourceMapOracleSpanResult(s *SessionGlobals, v Span, e *SpanSnippetError) string {
	if e != nil {
		return "err:" + sourceMapOracleError(e)
	}
	return "ok:" + sourceMapOracleSpan(s, v)
}
func sourceMapOracleSpanOption(s *SessionGlobals, v Option[Span]) string {
	if sp, ok := v.Get(); ok {
		return sourceMapOracleSpan(s, sp)
	}
	return "none"
}
func sourceMapOracleRun(session *SessionGlobals, p []string, config SpanEncodingConfig) (status, output string) {
	status = "ok"
	defer func() {
		if recover() != nil {
			status, output = "panic", ""
		}
	}()
	sm := NewSourceMapWithConfig(EmptyFilePathMapping(), config)
	defer sm.Close()
	src := sourceMapOracleString(p[2])
	lo, _ := strconv.ParseUint(p[3], 10, 32)
	hi, _ := strconv.ParseUint(p[4], 10, 32)
	op := p[5]
	arg := sourceMapOracleString(p[6])
	file := sm.NewSourceFile(CustomFileName("input.rs"), src)
	defer file.Close()
	sp := WithRootCtxt(session, BytePos(lo), BytePos(hi), config)
	char := ' '
	for _, c := range arg {
		char = c
		break
	}
	switch op {
	case "snippet":
		v, e := sm.SpanToSnippet(session, sp)
		output = sourceMapOracleStringResult(v, e)
	case "prev":
		v, e := sm.SpanToPrevSource(session, sp)
		output = sourceMapOracleStringResult(v, e)
	case "next":
		v, e := sm.SpanToNextSource(session, sp)
		output = sourceMapOracleStringResult(v, e)
	case "accessible":
		output = strconv.FormatBool(sm.IsSpanAccessible(session, sp))
	case "multiline":
		output = strconv.FormatBool(sm.IsMultiline(session, sp))
	case "imported":
		output = strconv.FormatBool(sm.IsImported(session, sp))
	case "diag":
		output = hex.EncodeToString([]byte(sm.SpanToDiagnosticString(session, sp)))
	case "short":
		output = hex.EncodeToString([]byte(sm.SpanToShortString(session, sp, RemapPathScopeDiagnostics)))
	case "indent":
		if text, ok := sm.IndentationBefore(session, sp).Get(); ok {
			output = "some:" + hex.EncodeToString([]byte(text))
		} else {
			output = "none"
		}
	case "margin":
		if n, ok := sm.SpanToMargin(session, sp).Get(); ok {
			output = strconv.FormatUint(uint64(n), 10)
		} else {
			output = "none"
		}
	case "line_empty":
		output = strconv.FormatBool(sm.IsLineBeforeSpanEmpty(session, sp))
	case "lines":
		v, e := sm.SpanToLines(session, sp)
		if e != nil {
			output = "err:distinct"
		} else {
			defer v.File.Close()
			items := []string{}
			for _, line := range v.Lines {
				items = append(items, fmt.Sprintf("%d/%d/%d", line.LineIndex, line.StartCol, line.EndCol))
			}
			output = fmt.Sprintf("ok:%d:%s", v.File.Borrow().StartPos, strings.Join(items, ","))
		}
	case "start":
		output = sourceMapOracleSpan(session, sm.StartPoint(session, sp))
	case "end":
		output = sourceMapOracleSpan(session, sm.EndPoint(session, sp))
	case "point":
		output = sourceMapOracleSpan(session, sm.NextPoint(session, sp))
	case "follow":
		output = sourceMapOracleSpanOption(session, sm.SpanFollowedBy(session, sp, arg))
	case "extend_prev":
		output = sourceMapOracleSpan(session, sm.SpanExtendToPrevChar(session, sp, char, false))
	case "extend_prev_nl":
		output = sourceMapOracleSpan(session, sm.SpanExtendToPrevChar(session, sp, char, true))
	case "extend_before":
		output = sourceMapOracleSpan(session, sm.SpanExtendToPrevCharBefore(session, sp, char, false))
	case "extend_before_nl":
		output = sourceMapOracleSpan(session, sm.SpanExtendToPrevCharBefore(session, sp, char, true))
	case "extend_next":
		output = sourceMapOracleSpan(session, sm.SpanExtendToNextChar(session, sp, char, false))
	case "extend_next_nl":
		output = sourceMapOracleSpan(session, sm.SpanExtendToNextChar(session, sp, char, true))
	case "extend_line":
		output = sourceMapOracleSpan(session, sm.SpanExtendToLine(session, sp))
	case "until":
		output = sourceMapOracleSpan(session, sm.SpanUntilChar(session, sp, char))
	case "through":
		output = sourceMapOracleSpan(session, sm.SpanThroughChar(session, sp, char))
	case "head":
		output = sourceMapOracleSpan(session, sm.GuessHeadSpan(session, sp))
	case "while":
		v, e := sm.SpanExtendWhile(session, sp, sourceMapWhitespace)
		output = sourceMapOracleSpanResult(session, v, e)
	case "prev_while":
		v, e := sm.SpanExtendPrevWhile(session, sp, sourceMapWhitespace)
		output = sourceMapOracleSpanResult(session, v, e)
	case "take":
		output = sourceMapOracleSpan(session, sm.SpanTakeWhile(session, sp, func(c *rune) bool { return sourceMapWhitespace(*c) }))
	case "until_white":
		output = sourceMapOracleSpan(session, sm.SpanUntilWhitespace(session, sp))
	case "until_nonwhite":
		output = sourceMapOracleSpan(session, sm.SpanUntilNonWhitespace(session, sp))
	case "wrapped":
		output = strconv.FormatBool(sm.SpanWrappedByAngleOrParentheses(session, sp))
	case "prev_str":
		output = sourceMapOracleSpanOption(session, sm.SpanExtendToPrevStr(session, sp, arg, false, false))
	case "prev_str_ws":
		output = sourceMapOracleSpanOption(session, sm.SpanExtendToPrevStr(session, sp, arg, false, true))
	case "prev_str_nl":
		output = sourceMapOracleSpanOption(session, sm.SpanExtendToPrevStr(session, sp, arg, true, false))
	case "prev_str_nl_ws":
		output = sourceMapOracleSpanOption(session, sm.SpanExtendToPrevStr(session, sp, arg, true, true))
	case "metadata":
		f := file.Borrow()
		lines := []string{}
		for _, v := range f.LinesWithContext(rustfmt.Context{}, config) {
			lines = append(lines, fmt.Sprint(uint32(v)))
		}
		mb := []string{}
		for _, v := range f.MultibyteChars {
			mb = append(mb, fmt.Sprintf("%d/%d", v.Pos, v.Bytes))
		}
		output = fmt.Sprintf("%d:%d:%d:%d:%s:%s", f.StartPos, f.NormalizedSourceLen, f.UnnormalizedSourceLen, sm.CountLines(), strings.Join(lines, ","), strings.Join(mb, ","))
	default:
		panic("unknown oracle operation")
	}
	return
}
func TestSourceMapNativeAlgorithmOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_algorithms.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	sessions := map[string]*SessionGlobals{}
	defer func() {
		for _, s := range sessions {
			s.Close()
		}
	}()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	count := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 9 {
			t.Fatalf("record %d fields%d", count, len(p))
		}
		key := p[0]
		config := SpanEncodingConfig{DebugAssertions: key[1] == '1', OverflowChecks: key[3] == '1'}
		session := sessions[key]
		if session == nil {
			session = NewSpanEncodingSession(config)
			sessions[key] = session
		}
		status, output := sourceMapOracleRun(session, p, config)
		want := sourceMapOracleString(p[8])
		if status != p[7] || output != want {
			t.Fatalf("%s/%s op%s src%q span%s:%s: got %s/%q want %s/%q", key, p[1], p[5], sourceMapOracleString(p[2]), p[3], p[4], status, output, p[7], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d unchanged-source records", count)
}
