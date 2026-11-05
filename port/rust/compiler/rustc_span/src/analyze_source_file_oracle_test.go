// SPDX-License-Identifier: MIT OR Apache-2.0
// Original-file native state/capacity replay, including post-panic prefixes.
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
)

func analysisOracleNumber(s string) uint {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		panic(err)
	}
	return uint(v)
}
func analysisOracleRun(p []string, arch string) (status string, overflow uint, lines []RelativeBytePos, mb []MultiByteChar) {
	lines = make([]RelativeBytePos, analysisOracleNumber(p[6]), analysisOracleNumber(p[7]))
	for i := range lines {
		lines[i] = RelativeBytePos(i)
	}
	mb = make([]MultiByteChar, analysisOracleNumber(p[8]), analysisOracleNumber(p[9]))
	for i := range mb {
		mb[i] = MultiByteChar{RelativeBytePos(i), 2}
	}
	status = "ok"
	defer func() {
		if recover() != nil {
			status = "panic"
		}
	}()
	cfg := SourceFileAnalysisConfig{Architecture: arch, SSE2Available: true, LSXAvailable: true, DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
	src := sourceMapOracleString(p[3])
	context := rustfmt.Context{}
	switch p[2] {
	case "full":
		lines, mb = AnalyzeSourceFileWithConfig(src, cfg)
	case "generic":
		overflow = analyzeSourceFileGeneric(src, analysisOracleNumber(p[4]), RelativeBytePos(analysisOracleNumber(p[5])), &lines, &mb, cfg, context)
	case "sse2":
		if arch == "loong64" {
			analyzeSourceFileLSX(src, &lines, &mb, cfg, context)
		} else {
			analyzeSourceFileSSE2(src, &lines, &mb, cfg, context)
		}
	default:
		panic("invalid analyzer oracle mode")
	}
	return
}
func TestAnalyzeSourceFileNativeStateOracle(t *testing.T) {
	f, err := os.Open("testdata/analyze_source_file.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	records := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 16 {
			t.Fatalf("record%d fields%d", records, len(p))
		}
		arches := []string{"amd64"}
		if p[2] == "full" {
			arches = []string{"generic", "amd64", "loong64"}
		} else if p[2] == "sse2" {
			arches = []string{"amd64", "loong64"}
		}
		for _, arch := range arches {
			status, overflow, lines, mb := analysisOracleRun(p, arch)
			lineStrings := []string{}
			for _, line := range lines {
				lineStrings = append(lineStrings, fmt.Sprint(uint32(line)))
			}
			mbStrings := []string{}
			for _, v := range mb {
				mbStrings = append(mbStrings, fmt.Sprintf("%d/%d", v.Pos, v.Bytes))
			}
			got := fmt.Sprintf("%s\t%d\t%d\t%d\t%s\t%s", status, overflow, cap(lines), cap(mb), strings.Join(lineStrings, ","), strings.Join(mbStrings, ","))
			want := strings.Join(p[10:], "\t")
			if got != want {
				t.Fatalf("%s/%s %s %s scan%s offset%s: got%q want%q", p[0], p[1], arch, p[2], p[4], p[5], got, want)
			}
		}
		records++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d native records, with generic/SSE2/portable-LSX replay", records)
}
