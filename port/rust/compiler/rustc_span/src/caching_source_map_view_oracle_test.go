// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

var cachingOracleTexts = [...]string{"", "abc", "abc\n", "\n", "x\nz", "λ🙂\r\nΣx\r\n", "\ufeffa", "\n\n\n", "tab\tend\nlast"}

func cachingOracleCounts(sourceMap *SourceMap) string {
	files := sourceMap.Files()
	defer files.Close()
	counts := make([]string, files.Len())
	for i := range counts {
		counts[i] = strconv.FormatUint(uint64(files.Borrow(uint(i)).StrongCount()), 10)
	}
	return strings.Join(counts, "_")
}

func cachingOracleAdd(sourceMap *SourceMap, index int) {
	file := sourceMap.NewSourceFile(CustomFileName(fmt.Sprintf("file%d", index)), cachingOracleTexts[index])
	file.Close()
}

func cachingOracleRawLineSource(numLines uint32, width uint8, raw []byte) SourceFile {
	seed, err := NewSourceFile(CustomFileName("lazy-lines"), "x", SourceFileHashSha256, None[SourceFileHashAlgorithm]())
	if err != nil {
		panic(err)
	}
	defer seed.CloseWithContext(rustfmt.Context{})
	e := NewSpanMemEncoder(nil)
	seed.Encode(e)
	body := e.Finish()
	frame := append(append([]byte(nil), body...), []byte(rustcserialize.MagicEndBytes)...)
	d, err := NewSpanMemDecoder(nil, frame, 0)
	if err != nil {
		panic(err)
	}
	DecodeFileName(d)
	DecodeSourceFileHash(d)
	if d.ReadU8() == 1 {
		DecodeSourceFileHash(d)
	}
	d.ReadU32()
	d.ReadU32()
	start := d.Position()
	oldLines := d.ReadU32()
	if oldLines > 0 {
		oldWidth := d.ReadU8()
		d.ReadRawBytes(uint(oldWidth) * uint(oldLines-1))
	}
	end := d.Position()
	replacement := rustcserialize.NewMemEncoder()
	replacement.EmitU32(numLines)
	if numLines > 0 {
		replacement.EmitU8(width)
		replacement.EmitRawBytes(raw)
	}
	frame = append([]byte(nil), body[:start]...)
	frame = append(frame, replacement.Finish()...)
	frame = append(frame, body[end:]...)
	frame = append(frame, []byte(rustcserialize.MagicEndBytes)...)
	d, err = NewSpanMemDecoder(nil, frame, 0)
	if err != nil {
		panic(err)
	}
	return DecodeSourceFile(d)
}

func cachingOracleImport(sourceMap *SourceMap, header string) {
	fields := strings.Split(header, ",")
	prefix, _ := strconv.Atoi(fields[1])
	for i := 0; i < prefix; i++ {
		cachingOracleAdd(sourceMap, i)
	}
	n, _ := strconv.ParseUint(fields[2], 10, 32)
	seed, err := NewSourceFile(CustomFileName("import"), "a\nb\nc", SourceFileHashMd5, None[SourceFileHashAlgorithm]())
	if err != nil {
		panic(err)
	}
	defer seed.CloseWithContext(rustfmt.Context{})
	var lines *FreezeLock[SourceFileLines]
	if fields[3] == "R" {
		count, _ := strconv.ParseUint(fields[4], 10, 32)
		width, _ := strconv.ParseUint(fields[5], 10, 8)
		raw, err := hex.DecodeString(fields[6])
		if err != nil {
			panic(err)
		}
		decoded := cachingOracleRawLineSource(uint32(count), uint8(width), raw)
		lines = decoded.Lines.CloneWithContext(func(value *SourceFileLines, _ rustfmt.Context) SourceFileLines { return value.Clone() }, rustfmt.Context{})
		decoded.CloseWithContext(rustfmt.Context{})
	} else {
		values := []RelativeBytePos{}
		if fields[4] != "-" {
			for _, text := range strings.Split(fields[4], "~") {
				value, _ := strconv.ParseUint(text, 10, 32)
				values = append(values, RelativeBytePos(value))
			}
		}
		seed.Lines.Close()
		seed.Lines = NewFreezeLock(LinesSourceFileLines(values))
		switch fields[3] {
		case "L":
			lines = seed.Lines.CloneWithContext(func(value *SourceFileLines, _ rustfmt.Context) SourceFileLines { return value.Clone() }, rustfmt.Context{})
		case "F":
			seed.LinesWithContext(rustfmt.Context{})
			lines = seed.Lines.CloneWithContext(func(value *SourceFileLines, _ rustfmt.Context) SourceFileLines { return value.Clone() }, rustfmt.Context{})
		case "D":
			e := NewSpanMemEncoder(nil)
			seed.Encode(e)
			data := append(e.Finish(), []byte(rustcserialize.MagicEndBytes)...)
			d, err := NewSpanMemDecoder(nil, data, 0)
			if err != nil {
				panic(err)
			}
			decoded := DecodeSourceFile(d)
			lines = decoded.Lines.CloneWithContext(func(value *SourceFileLines, _ rustfmt.Context) SourceFileLines { return value.Clone() }, rustfmt.Context{})
			decoded.CloseWithContext(rustfmt.Context{})
		default:
			panic("unknown metadata mode")
		}
	}
	file := sourceMap.NewImportedSourceFile(seed.Name.Clone(), seed.SrcHash, seed.ChecksumHash, seed.StableId, uint32(n), seed.UnnormalizedSourceLen, CrateNumZero(), lines, append([]MultiByteChar(nil), seed.MultibyteChars...), append([]NormalizedPos(nil), seed.NormalizedPos...), 0)
	file.Close()
}

func cachingOracleMetadata(sourceMap *SourceMap) string {
	files := sourceMap.Files()
	defer files.Close()
	parts := make([]string, files.Len())
	for i := range parts {
		file := files.Borrow(uint(i)).Borrow()
		guard := file.Lines.Read()
		frozen, lines := 0, 0
		if file.Lines.IsFrozen() {
			frozen = 1
		}
		if guard.Get().IsLines() {
			lines = 1
		}
		parts[i] = fmt.Sprintf("%d:%d", frozen, lines)
		guard.Close()
	}
	return strings.Join(parts, "_")
}

func cachingOracleCatch(call func() string) (result string) {
	defer func() {
		if recover() != nil {
			result = "panic"
		}
	}()
	return call()
}

func cachingOracleReplay(profile, request string) string {
	config := SpanEncodingConfig{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
	sourceMap := NewSourceMapWithConfig(EmptyFilePathMapping(), config)
	defer sourceMap.Close()
	parts := strings.SplitN(request, "|", 2)
	if strings.HasPrefix(parts[0], "I,") {
		cachingOracleImport(sourceMap, parts[0])
	} else {
		count, _ := strconv.Atoi(parts[0])
		for i := 0; i < count; i++ {
			cachingOracleAdd(sourceMap, i)
		}
	}
	output := []string{"base:" + cachingOracleCounts(sourceMap)}
	var view *CachingSourceMapView
	construction := cachingOracleCatch(func() string {
		view = NewCachingSourceMapView(sourceMap)
		return "ok"
	})
	output = append(output, "new:"+construction+":"+cachingOracleCounts(sourceMap))
	if construction == "panic" {
		return strings.Join(output, "|")
	}
	defer view.Close()
	for _, operation := range strings.Split(parts[1], ";") {
		if operation == "" {
			continue
		}
		fields := strings.Split(operation, ",")
		var rendered string
		if fields[0] == "m" {
			rendered = "m:" + cachingOracleMetadata(sourceMap)
		} else if fields[0] == "a" {
			files := sourceMap.Files()
			index := int(files.Len())
			files.Close()
			cachingOracleAdd(sourceMap, index)
			rendered = fmt.Sprintf("a:%d", index)
		} else {
			rendered = fields[0] + ":" + cachingOracleCatch(func() string {
				first, err := strconv.ParseUint(fields[1], 10, 32)
				if err != nil {
					panic(err)
				}
				if fields[0] == "b" {
					result, ok := view.BytePosToLineAndCol(BytePos(first))
					if !ok {
						return "none"
					}
					defer result.File.Close()
					return fmt.Sprintf("ok:%d:%d:%d:%s", result.File.Borrow().StartPos, result.Line, result.Column, cachingOracleCounts(sourceMap))
				}
				second, err := strconv.ParseUint(fields[2], 10, 32)
				if err != nil {
					panic(err)
				}
				data := SpanData{Lo: BytePos(first), Hi: BytePos(second), Ctxt: RootSyntaxContext()}
				result, ok := view.SpanDataToLinesAndCols(&data)
				if !ok {
					return "none"
				}
				return fmt.Sprintf("ok:%d:%d:%d:%d:%d:%s", result.File.StartPos, result.LoLine, result.LoColumn, result.HiLine, result.HiColumn, cachingOracleCounts(sourceMap))
			})
		}
		output = append(output, rendered+":after:"+cachingOracleCounts(sourceMap))
	}
	view.Close()
	output = append(output, "drop:"+cachingOracleCounts(sourceMap))
	return strings.Join(output, "|")
}

func TestOriginalCachingSourceMapView(t *testing.T) {
	testOriginalCachingSourceMapView(t, "testdata/caching_source_map_view.tsv.gz")
}

func TestOriginalCachingSourceMapViewMetadata(t *testing.T) {
	testOriginalCachingSourceMapView(t, "testdata/caching_source_map_view_metadata.tsv.gz")
}

func testOriginalCachingSourceMapView(t *testing.T, fixture string) {
	metadata := "manifest.json"
	if strings.Contains(fixture, "metadata") {
		metadata = "metadata-manifest.json"
	}
	manifest, err := os.ReadFile(filepath.Join("../../../../../tools/port-rust-caching-source-map-view", metadata))
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Records int             `json:"records"`
		SHA     string          `json:"fixture_sha256"`
		Files   []spanProofFile `json:"files"`
	}
	if err := json.Unmarshal(manifest, &proof); err != nil {
		t.Fatal(err)
	}
	verifySpanProofFiles(t, proof.Files)
	compressed, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != proof.SHA {
		t.Fatal("native fixture SHA-256 differs")
	}
	file, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	row := 0
	for scanner.Scan() {
		fields := strings.SplitN(scanner.Text(), "\t", 3)
		if len(fields) != 3 {
			t.Fatalf("bad native fixture row%d", row)
		}
		if got := cachingOracleReplay(fields[0], fields[1]); got != fields[2] {
			t.Fatalf("native row%d %s %s\ngot %s\nwant %s", row, fields[0], fields[1], got, fields[2])
		}
		row++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if row != proof.Records {
		t.Fatalf("native fixture count%d differs from%d", row, proof.Records)
	}
	t.Logf("%d retained whole-crate native caching requests match", row)
}
